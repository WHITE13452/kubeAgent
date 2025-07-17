/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"kubeagent/cmd/ai"
	"kubeagent/cmd/tools"
	"regexp"
	"strings"

	prompttpl "kubeagent/cmd/promptTpl"

	"github.com/spf13/cobra"
)

// kubecheckCmd represents the kubecheck command
var kubecheckCmd = &cobra.Command{
	Use:   "kubecheck",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Initialize tools
		functionTools := initKubeCheckFunctionTools()

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
			prompt := buildKubeCheckPrompt(functionTools.KubeTool, functionTools.RequestTool, functionTools.TavilyTool, functionTools.HumanTool, input)

			ai.MessageStore.AddForUser(prompt)
			i := 1
			maxIteration := 10
			consecutiveFailures := 0    // 连续失败计数
			maxConsecutiveFailures := 3 // 最大连续失败次数

			for i <= maxIteration {
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
					case functionTools.KubeTool.Name:
						actionInputProcessed := strings.Trim(actionInput, "\"")
						actionInputProcessed = cleanMarkdownCodeBlock(actionInputProcessed)
						fmt.Println("actionInputProcessed: ", actionInputProcessed)
						output, toolErr := functionTools.KubeTool.Run(actionInputProcessed)
						if toolErr != nil {
							observation = fmt.Sprintf("Observation: Error executing command: %v", toolErr)
							consecutiveFailures++
						} else {
							fmt.Println("========函数返回结果========")
							fmt.Println("output: ", output)
							observation = fmt.Sprintf("Observation: %s", output)
							consecutiveFailures = 0
						}

					case functionTools.TavilyTool.Name:
						output, toolErr := functionTools.TavilyTool.Run(actionInput)
						if toolErr != nil {
							observation = fmt.Sprintf("Observation: Error executing Tavily command: %v", toolErr)
							consecutiveFailures++
						} else {
							fmt.Println("========函数返回结果========")
							fmt.Println("output: ", output)
							observation = fmt.Sprintf("Observation: %s", output)
							consecutiveFailures = 0
						}
					case functionTools.RequestTool.Name:
						fmt.Println("actionInput: ", actionInput)
						actionInputProcessed := strings.Trim(actionInput, "\"")
						fmt.Println("actionInputProcessed: ", actionInputProcessed)
						output, toolErr := functionTools.RequestTool.Run(actionInputProcessed)
						if toolErr != nil {
							observation = fmt.Sprintf("Observation: Error executing Request command: %v", toolErr)
							consecutiveFailures++
						} else {
							observation = fmt.Sprintf("Observation: %s", output)
							consecutiveFailures = 0
						}
					case "HumanTool":
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

					if consecutiveFailures >= maxConsecutiveFailures {
						fmt.Printf("连续失败 %d 次，停止执行\n", maxConsecutiveFailures)
                    	break
					}

					// 将 Observation 添加到历史记录，让模型进行下一步思考
					ai.MessageStore.AddForUser(observation)

				} else {
					// 如果模型没有按预期格式返回 Action，则直接将回复作为最终答案并结束
					fmt.Println("模型未返回有效 Action，对话结束。")
					fmt.Println(firstResponse.Content)
					break
				}
			}
			if i > maxIteration {
				 fmt.Printf("达到最大迭代次数 %d，强制结束对话\n", maxIteration)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(kubecheckCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// kubecheckCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// kubecheckCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

type KubeCheckFunctionTools struct {
	KubeTool    *tools.KubeTool
	RequestTool *tools.RequestTool
	TavilyTool  *tools.TavilyTool
	HumanTool   *tools.HumanTool
}

func initKubeCheckFunctionTools() *KubeCheckFunctionTools {
	return &KubeCheckFunctionTools{
		KubeTool:    tools.NewKubeTool(),
		RequestTool: tools.NewRequestTool(),
		TavilyTool:  tools.NewTavilyTool(),
		HumanTool:   tools.NewHumanTool(),
	}
}

func buildKubeCheckPrompt(kubeTool *tools.KubeTool, requestTool *tools.RequestTool, tavilyTool *tools.TavilyTool, humanTool *tools.HumanTool, query string) string {

	kubeToolDef := "Name: " + kubeTool.Name + "\nDescription: " + kubeTool.Description + "\nArgsSchema: " + fmt.Sprintf(kubeTool.ArgsSchema.Commands) + "\n"
	requestToolDef := "Name: " + requestTool.Name + "\nDescription: " + requestTool.Description + "\nArgsSchema: " + requestTool.ArgsSchema + "\n"
	tavilyToolDef := "Name: " + tavilyTool.Name + "\nDescription: " + tavilyTool.Description + "\nArgsSchema: " + tavilyTool.ArgsSchema + "\n"
	humanToolDef := "Name: " + humanTool.Name + "\nDescription: " + humanTool.Description + "\nArgsSchema: " + humanTool.ArgsSchema + "\n"

	toolsList := make([]string, 0)
	toolsList = append(toolsList, kubeToolDef, requestToolDef, tavilyToolDef, humanToolDef)

	toolNames := make([]string, 0)
	toolNames = append(toolNames, kubeTool.Name, requestTool.Name, tavilyTool.Name, humanTool.Name)

	prompt := fmt.Sprintf(prompttpl.Template, toolsList, toolNames, "", query)

	return prompt
}

func cleanMarkdownCodeBlock(input string) string {
	// 移除各种 markdown 代码块标记
	input = strings.TrimSpace(input)
	input = strings.TrimPrefix(input, "```sh")
	input = strings.TrimPrefix(input, "```bash")
	input = strings.TrimPrefix(input, "```")
	input = strings.TrimSuffix(input, "```")
	input = strings.TrimSpace(input)

	// 移除多行命令中的换行符，替换为 &&
	lines := strings.Split(input, "\n")
	var cleanLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			cleanLines = append(cleanLines, line)
		}
	}

	if len(cleanLines) > 1 {
		return strings.Join(cleanLines, " && ")
	}

	return input
}
