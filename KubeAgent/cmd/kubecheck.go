package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"kubeagent/pkg/agent"
	"kubeagent/pkg/agent/specialists"
	"kubeagent/pkg/k8s"
	pkgtools "kubeagent/pkg/tools"
)

var kubecheckCmd = &cobra.Command{
	Use:   "kubecheck",
	Short: "Check cluster state and search for Kubernetes best practices",
	Long:  `Interactive cluster inspection mode. Run kubectl commands and search for Kubernetes documentation.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := agent.NewSimpleLogger("KubeAgent")
		stateStore := agent.NewMemoryStateStore()

		llmClient, err := agent.NewAnthropicLLMClient(nil)
		if err != nil {
			fmt.Printf("Failed to initialize LLM client: %v\n", err)
			return
		}

		k8sClient, err := k8s.NewClient()
		if err != nil {
			fmt.Printf("Failed to initialize K8s client: %v\n", err)
			return
		}

		coordinator := agent.NewCoordinator(nil, llmClient, stateStore, logger)

		// DiagnosticianAgent handles query/inspect tasks with kubectl + web search tools
		diagnostician := specialists.NewDiagnosticianAgent(llmClient, logger)
		diagnostician.AddTool(pkgtools.NewKubeTool())
		diagnostician.AddTool(pkgtools.NewListTool(k8sClient))
		diagnostician.AddTool(pkgtools.NewLogTool(k8sClient))
		diagnostician.AddTool(pkgtools.NewEventTool(k8sClient))
		diagnostician.AddTool(pkgtools.NewRequestTool())

		tavilyAPIKey := os.Getenv("TAVILY_API_KEY")
		if tavilyAPIKey != "" {
			diagnostician.AddTool(pkgtools.NewTavilyTool(tavilyAPIKey))
		}

		coordinator.RegisterAgent(diagnostician)

		scanner := bufio.NewScanner(cmd.InOrStdin())
		fmt.Println("Hi, I am KubeAgent (KubeCheck mode). Ask me about your cluster health or Kubernetes best practices. (Input 'exit' to quit):")
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

			fmt.Println("\n========== Result ==========")
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
	rootCmd.AddCommand(kubecheckCmd)
}
