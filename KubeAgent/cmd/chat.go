/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"regexp"

	"kubeagent/cmd/ai"
	prompttpl "kubeagent/cmd/promptTpl"
	"kubeagent/cmd/tools"

	"github.com/spf13/cobra"
)

// chatCmd represents the chat command
var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Initialize tools 
		functionTools := initFunctionTools()

		scanner := bufio.NewScanner(cmd.InOrStdin())
		fmt.Println("Hi，I am kubeAgent, What can I help you?（Input 'exit' to quit）:")
		for {
			fmt.Print(">>> ")
			if !scanner.Scan() {
				break // Exit if there's no input
			}
			input := scanner.Text()
			if input == "exit" {
				fmt.Println("Exiting chat. Goodbye!")
				return
			}
			prompt := buildPrompt(functionTools.CreateTool, functionTools.HumanTool, input)
			ai.MessageStore.AddForUser(prompt)
			i := 1
			for {
				firstResponse := ai.NormalChat(ai.MessageStore.ToMessage())
				fmt.Printf("========第%d轮回答========\n", i)
				fmt.Println(firstResponse.Content)
				regaxPattern := regexp.MustCompile(`Final Answer:\s*(.*)`)
				finalAnswer := regaxPattern.FindStringSubmatch(firstResponse.Content)
				if len(finalAnswer) > 1 {
					fmt.Println("Final Answer:", finalAnswer[1])
					break
				}

				ai.MessageStore.AddForAssistant(firstResponse.Content)
				
				regexAction := regexp.MustCompile(`Action:\s*(.*?)[\n]`)
				regexActionInput := regexp.MustCompile(`Action Input:\s*(.*?)[\n]`)

				action := regexAction.FindStringSubmatch(firstResponse.Content)
				actionInput := regexActionInput.FindStringSubmatch(firstResponse.Content)

				if len(action) > 1 && len(actionInput) > 1 {
					i++
					Observation := "Observation: %s"
					switch action[1] {
					case functionTools.CreateTool.Name:
						var param tools.CreateToolParam
						err := json.Unmarshal([]byte(actionInput[1]), &param)
						if err != nil {
							fmt.Println("Error parsing CreateTool parameters:", err)
							continue
						}
						output := functionTools.CreateTool.Run(param.Prompt, param.Resource)
						Observation = fmt.Sprintf(Observation, output)
					case functionTools.HumanTool.Name:
						var param tools.HumanToolOaram
						err := json.Unmarshal([]byte(actionInput[1]), &param)
						if err != nil {
							fmt.Println("Error parsing HumanTool parameters:", err)
							continue
						}
						output := functionTools.HumanTool.Run(param.Prompt)
						Observation = fmt.Sprintf(Observation, output)
					default:
						fmt.Println("Unknown action:", action[1])
						continue
					}
					prompt := firstResponse.Content + Observation
					fmt.Printf("========第%d轮的prompt========\n", i)
					fmt.Println(prompt)
					ai.MessageStore.AddForUser(prompt)
				}
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(chatCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// chatCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// chatCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

type FunctionTools struct {
	CreateTool *tools.CreateTool
	HumanTool *tools.HumanTool
}

func initFunctionTools() *FunctionTools {
	return &FunctionTools{
		CreateTool: tools.NewCreateTool(),
		HumanTool: tools.NewHumanTool(),
	}
}

func buildPrompt(createTool *tools.CreateTool, humanTool *tools.HumanTool, query string) string {
	createToolDef := "Name: " + createTool.Name + "\nDescription: " + createTool.Description + "\nArgsSchema: " + createTool.ArgsSchema + "\n"
	humanToolDef := "Name: " + humanTool.Name + "\nDescription: " + humanTool.Description + "\nArgsSchema: " + humanTool.ArgsSchema + "\n"

	toolsList := make([]string, 0)
	toolsList = append(toolsList, createToolDef, humanToolDef)

	tool_names := make([]string, 0)
	tool_names = append(tool_names, createTool.Name, humanTool.Name)

	prompt := fmt.Sprintf(prompttpl.Template, toolsList, tool_names, "", query)

	return prompt
}
