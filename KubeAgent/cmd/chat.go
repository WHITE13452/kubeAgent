package cmd

import (
	"bufio"
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"kubeagent/pkg/agent"
	"kubeagent/pkg/agent/specialists"
	"kubeagent/pkg/k8s"
	pkgtools "kubeagent/pkg/tools"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Manage Kubernetes resources through natural language",
	Long:  `Interactive resource management mode. Create, list, and delete K8s resources using natural language.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := agent.NewSimpleLogger("KubeAgent")
		stateStore := agent.NewMemoryStateStore()

		llmClient, err := agent.NewOpenAILLMClient(nil)
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

		// DiagnosticianAgent handles read/query operations
		diagnostician := specialists.NewDiagnosticianAgent(llmClient, logger)
		diagnostician.AddTool(pkgtools.NewListTool(k8sClient))
		diagnostician.AddTool(pkgtools.NewLogTool(k8sClient))
		coordinator.RegisterAgent(diagnostician)

		// RemediatorAgent handles write operations (create, delete) with human confirmation
		remediator := specialists.NewRemediatorAgent(llmClient, logger)
		remediator.AddTool(pkgtools.NewHumanTool())
		remediator.AddTool(pkgtools.NewCreateTool(k8sClient))
		remediator.AddTool(pkgtools.NewDeleteTool(k8sClient))
		coordinator.RegisterAgent(remediator)

		scanner := bufio.NewScanner(cmd.InOrStdin())
		fmt.Println("Hi, I am KubeAgent (Chat mode). How can I help you manage your Kubernetes resources? (Input 'exit' to quit):")
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
	rootCmd.AddCommand(chatCmd)
}
