/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"kubeagent/cmd/ai"
	prompttpl "kubeagent/cmd/promptTpl"
	"kubeagent/cmd/tools"
	"regexp"

	"github.com/spf13/cobra"
)

// analyzeCmd represents the analyze command
var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Initialize tools
		functionTools := initAnalyzeFunctionTools()

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
			prompt := buildAnalyzePrompt(functionTools.LogTool, functionTools.EventTool, functionTools.HumanTool, input)
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
					case functionTools.LogTool.Name:
						var param tools.LogToolParam
						err := json.Unmarshal([]byte(actionInput), &param)
						if err != nil {
							fmt.Println("Error parsing LogTool parameters:", err)
							observation = fmt.Sprintf("Error parsing LogTool parameters: %v", err)
						} else {
							output, err := functionTools.LogTool.Run(param.PodName, param.Namespace, param.Container)
							if err != nil {
								observation = fmt.Sprintf("Error fetching logs: %v", err)
							} else {
								observation = fmt.Sprintf("Observation: %s", output)
							}

						}
					case functionTools.EventTool.Name:
						var param tools.EventToolParam
						err := json.Unmarshal([]byte(actionInput), &param)
						if err != nil {
							fmt.Println("Error parsing EventTool parameters:", err)
							observation = fmt.Sprintf("Error parsing EventTool parameters: %v", err)
						} else {
							output, err := functionTools.EventTool.Run(param.PodName, param.Namespace)
							if err != nil {
								observation = fmt.Sprintf("Error fetching events: %v", err)
							} else {
								observation = fmt.Sprintf("Observation: %s", output)
							}
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
	rootCmd.AddCommand(analyzeCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// analyzeCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// analyzeCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

type AnalyzeFunctionTools struct {
	LogTool   *tools.LogTool
	EventTool *tools.EventTool
	HumanTool *tools.HumanTool
}

func initAnalyzeFunctionTools() *AnalyzeFunctionTools {
	return &AnalyzeFunctionTools{
		LogTool:   tools.NewLogTool(),
		EventTool: tools.NewEventTool(),
		HumanTool: tools.NewHumanTool(),
	}
}

func buildAnalyzePrompt(logTool *tools.LogTool, eventTool *tools.EventTool, humanTool *tools.HumanTool, query string) string {
	logToolDef := "Name: " + logTool.Name + "\nDescription: " + logTool.Description + "\nArgsSchema: " + logTool.ArgsSchema + "\n"
	eventToolDef := "Name: " + eventTool.Name + "\nDescription: " + eventTool.Description + "\nArgsSchema: " + eventTool.ArgsSchema + "\n"
	humanToolDef := "Name: " + humanTool.Name + "\nDescription: " + humanTool.Description + "\nArgsSchema: " + humanTool.ArgsSchema + "\n"

	toolsList := make([]string, 0)
	toolsList = append(toolsList, logToolDef, eventToolDef, humanToolDef)

	toolNames := make([]string, 0)
	toolNames = append(toolNames, logTool.Name, eventTool.Name, humanTool.Name)

	prompt := fmt.Sprintf(prompttpl.Template, toolsList, toolNames, "", query)

	return prompt
}
