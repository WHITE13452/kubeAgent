package cmd

import (
	"bufio"
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"kubeagent/pkg/agent"
	"kubeagent/pkg/agent/specialists"
	pkgtools "kubeagent/pkg/tools"
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Diagnose Kubernetes issues using the multi-agent framework",
	Long:  `Interactive diagnostic mode. Describe pod issues and let the AI agents analyze logs, events, and cluster state.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := agent.NewSimpleLogger("KubeAgent")
		stateStore := agent.NewMemoryStateStore()

		llmClient, err := agent.NewOpenAILLMClient(nil)
		if err != nil {
			fmt.Printf("Failed to initialize LLM client: %v\n", err)
			return
		}

		coordinator := agent.NewCoordinator(nil, llmClient, stateStore, logger)

		// DiagnosticianAgent with read-only K8s tools
		diagnostician := specialists.NewDiagnosticianAgent(llmClient, logger)
		diagnostician.AddTool(pkgtools.NewLogTool())
		diagnostician.AddTool(pkgtools.NewEventTool())
		diagnostician.AddTool(pkgtools.NewListTool())
		diagnostician.AddTool(pkgtools.NewKubeTool())
		coordinator.RegisterAgent(diagnostician)

		scanner := bufio.NewScanner(cmd.InOrStdin())
		fmt.Println("Hi, I am KubeAgent (Analyze mode). Describe the issue you want to diagnose. (Input 'exit' to quit):")
		for {
			fmt.Print(">>> ")
			if !scanner.Scan() {
				break
			}
			input := scanner.Text()
			if input == "exit" {
				fmt.Println("Goodbye!")
				return
			}

			ctx := agent.NewAgentContext(
				context.Background(),
				uuid.New().String(),
				"cli-user",
				uuid.New().String(),
			)
			request := &agent.Request{
				ID:    uuid.New().String(),
				User:  "cli-user",
				Input: input,
			}

			plan, err := coordinator.Plan(ctx, request)
			if err != nil {
				fmt.Printf("Planning failed: %v\n", err)
				continue
			}

			response, err := coordinator.ExecutePlan(ctx, plan)
			if err != nil {
				fmt.Printf("Execution failed: %v\n", err)
				continue
			}

			fmt.Println("\n========== Analysis Result ==========")
			fmt.Println(response.Result)
			if len(response.Errors) > 0 {
				fmt.Println("\nErrors encountered:")
				for _, e := range response.Errors {
					fmt.Println(" -", e)
				}
			}
			fmt.Println()
		}
	},
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
}
