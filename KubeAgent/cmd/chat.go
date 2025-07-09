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
		functionTools := initHTTPFunctionTools()

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
			prompt := buildHTTPPrompt(functionTools.DeleteTool, functionTools.CreateTool, functionTools.ListTool, functionTools.HumanTool, input)
			ai.MessageStore.AddForUser(prompt)
			i := 1
			for {
				firstResponse := ai.NormalChat(ai.MessageStore.ToMessage())
				fmt.Printf("========第%d轮回答========\n", i)
				fmt.Println(firstResponse.Content)
				// Check if the response contains a final answer
				finalAnswerRegex := regexp.MustCompile(`Final Answer:\s*(.*)`)
				if finalAnswerRegex.MatchString(firstResponse.Content) {
					finalAnswer := finalAnswerRegex.FindStringSubmatch(firstResponse.Content)
					if len(finalAnswer) > 1 {
						fmt.Println("========最终 GPT 回复========")
						fmt.Println(firstResponse.Content)
					}
					break
				}

				// ai.MessageStore.AddForAssistant(firstResponse.Content)

				// Check if the response contains a tool call
				actionRegex := regexp.MustCompile(`Action:\s*(.*?)[\n]`)
				actionInputRegex := regexp.MustCompile(`Action Input:\s*(.*)`)

				actionMatch := actionRegex.FindStringSubmatch(firstResponse.Content)
				actionInputMatch := actionInputRegex.FindStringSubmatch(firstResponse.Content)

				if len(actionMatch) > 1 && len(actionInputMatch) > 1 {
					i++
					// model thoughts
					thoughtAndAction := firstResponse.Content
					ai.MessageStore.AddForAssistant(thoughtAndAction)

					action := actionMatch[1]
					actionInput := actionInputMatch[1]

					var observation string
					switch action {
					case functionTools.CreateTool.Name:
						var param tools.CreateToolParam
						err := json.Unmarshal([]byte(actionInput), &param)
						if err != nil {
							fmt.Println("Error parsing CreateTool parameters:", err)
							observation = fmt.Sprintf("Error parsing CreateTool parameters: %v", err)
						} else {
							output := functionTools.CreateTool.Run(param.Prompt, param.Resource)
							observation = fmt.Sprintf("Observation: %s", output)
						}
					case functionTools.ListTool.Name:
						var param tools.ListToolParam
						err := json.Unmarshal([]byte(actionInput), &param)
						if err != nil {
							fmt.Println("Error parsing ListTool parameters:", err)
							observation = fmt.Sprintf("Error parsing ListTool parameters: %v", err)
						} else {
							output := functionTools.ListTool.Run(param.Resource, param.Namespace)
							observation = fmt.Sprintf("Observation: %s", output)
						}
					case functionTools.DeleteTool.Name:
						var param tools.DeleteToolParam
						err := json.Unmarshal([]byte(actionInput), &param)
						if err != nil {
							fmt.Println("Error parsing DeleteTool parameters:", err)
							observation = fmt.Sprintf("Error parsing DeleteTool parameters: %v", err)
						} else {
							output := functionTools.DeleteTool.Run(param.Resource, param.Name, param.Namespace)
							observation = fmt.Sprintf("Observation: %s", output)
						}
					case functionTools.HumanTool.Name:
						var param tools.HumanToolParam
						err := json.Unmarshal([]byte(actionInput), &param)
						if err != nil {
							fmt.Println("Error parsing HumanTool parameters:", err)
							observation = fmt.Sprintf("Error parsing HumanTool parameters: %v", err)
						} else {
							output := functionTools.HumanTool.Run(param.Prompt)
							observation = fmt.Sprintf("Observation: %s", output)
						}
					default:
						observation = fmt.Sprintf("Unknown action: %s", action)
					}
					fmt.Printf("========工具执行结果========\n%s\n", observation)

					// 将 Observation 添加到历史记录，让模型进行下一步思考
					ai.MessageStore.AddForUser(observation)

				} else {
					// 如果模型没有按预期格式返回 Action，则直接将回复作为最终答案并结束
					fmt.Println("模型未返回有效 Action，对话结束。")
					fmt.Println(firstResponse.Content)
					break
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

type HTTPFunctionTools struct {
	DeleteTool *tools.DeleteTool
	CreateTool *tools.CreateTool
	ListTool   *tools.ListTool
	HumanTool  *tools.HumanTool
}

func initHTTPFunctionTools() *HTTPFunctionTools {
	return &HTTPFunctionTools{
		DeleteTool: tools.NewDeleteTool(),
		CreateTool: tools.NewCreateTool(),
		ListTool:   tools.NewListTool(),
		HumanTool:  tools.NewHumanTool(),
	}
}

func buildHTTPPrompt(deleteTool *tools.DeleteTool, createTool *tools.CreateTool, listTool *tools.ListTool, humanTool *tools.HumanTool, query string) string {
	deleteToolDef := "Name: " + deleteTool.Name + "\nDescription: " + deleteTool.Description + "\nArgsSchema: " + deleteTool.ArgsSchema + "\n"
	createToolDef := "Name: " + createTool.Name + "\nDescription: " + createTool.Description + "\nArgsSchema: " + createTool.ArgsSchema + "\n"
	listToolDef := "Name: " + listTool.Name + "\nDescription: " + listTool.Description + "\nArgsSchema: " + listTool.ArgsSchema + "\n"
	humanToolDef := "Name: " + humanTool.Name + "\nDescription: " + humanTool.Description + "\nArgsSchema: " + humanTool.ArgsSchema + "\n"

	toolsList := make([]string, 0)
	toolsList = append(toolsList, createToolDef, listToolDef, deleteToolDef, humanToolDef)

	toolNames := make([]string, 0)
	toolNames = append(toolNames, createTool.Name, listTool.Name, deleteTool.Name, humanTool.Name)

	prompt := fmt.Sprintf(prompttpl.Template, toolsList, toolNames, "", query)

	return prompt
}
