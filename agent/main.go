package main

import (
	"agent/ai"
	prompttpl "agent/promptTpl"
	"agent/tools"
	"fmt"
	"regexp"
	"strconv"
)

// function call example
// #1. 将提问存储到MessageStore
// {"role": "user", "content": "1+2=? Just give me a number result"}

// #开始进行第一轮提问....
// #得到大模型返回

// #2. 将大模型的返回，存储到MessageStore
// {"role": "assistant", "content": "", "tool_calls": [{0xc000284520 call_f31f9091de504216a3a84d function {AddTool {"numbers": [1, 2]}}}]
// }

// #3. 将工具调用信息，存储到MessageStore
// {"role": "tool", "content": "3", "name": "AddTool", "tool_call_id": "call_f31f9091de504216a3a84d"}

// #4. 开始进行第二轮提问，将上述所有Mesage，发送给大模型
// #得到大模型返回

func main() {

	query := "1+2-3+4-5+6=? Just return the result number , no other text."

	addTool := tools.AddToolName + ":" + tools.AddToolDescription + "\nparam:\n" + tools.AddToolParameters
	subTool := tools.SubToolName + ":" + tools.SubToolDescription + "\nparam:\n" + tools.SubToolParameters
	toolList := make([]string, 0)
	toolList = append(toolList, addTool, subTool)

	tool_names := make([]string, 0)
	tool_names = append(tool_names, tools.AddToolName, tools.SubToolName)

	prompt := fmt.Sprintf(prompttpl.Template, toolList, tool_names, query)
	fmt.Println("prompt:", prompt)
	ai.MessageStore.AddForUser(prompt)
	i := 1
	for {
		first_response := ai.NormalChat(ai.MessageStore.ToMessage())
		fmt.Printf("========第%d轮回答========\n", i)
		fmt.Println(first_response)
		regaxPattern := regexp.MustCompile(`Final Answer:\s*(.*)`)
		finalAnswer := regaxPattern.FindStringSubmatch(first_response.Content)
		if len(finalAnswer) > 1 {
			fmt.Println("========最终 GPT 回复========")
			fmt.Println(first_response.Content)
			break
		}
		ai.MessageStore.AddForAssistant(first_response)

		regaxAction := regexp.MustCompile(`Action:\s*(.*?)(?:$|\n)`)
		regexActionInput := regexp.MustCompile(`Action Input:\s*(.*?)(?:$|\n)`)
		
		action := regaxAction.FindStringSubmatch(first_response.Content)
		actionInput := regexActionInput.FindStringSubmatch(first_response.Content)
		// 在正则匹配后添加
		fmt.Println("Action regex match:", len(action) > 1, action)
		fmt.Println("Action Input regex match:", len(actionInput) > 1, actionInput)
		if len(action) > 1 && len(actionInput) > 1 {
			i++
			fmt.Println(i)
			res := 0
			if action[1] == tools.AddToolName {
				fmt.Println("calls AddTool")
				res = tools.AddTool(actionInput[1])
			} else if action[1] == tools.SubToolName {
				fmt.Println("calls SubTool")
				res = tools.SubTool(actionInput[1])
			}
			fmt.Println("========函数返回结果========")
			fmt.Println(res)

			Observation := "Observation" + strconv.Itoa(res)
			prompt = first_response.Content + Observation
			fmt.Printf("========第%d轮的prompt========\n", i)
			fmt.Println(prompt)
			ai.MessageStore.AddForUser(prompt)
		}
	}

}
