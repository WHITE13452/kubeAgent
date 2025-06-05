package main

import (
	"encoding/json"
	"fmt"
	"function-calling/ai"
	"function-calling/tools"
	"log"

	"github.com/sashabaranov/go-openai"
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
	toolList := make([]openai.Tool, 0)
	toolList = append(toolList, tools.AddToolDefine, tools.SubToolDefine)

	prmopt := "1+2-3+4-5+6=? Just return the result number , no other text."
	ai.MessageStore.AddFor(ai.RoleUser, prmopt, nil)

	resp := ai.ToolChat(ai.MessageStore.ToMessage(), toolList)
	toolCall := resp.ToolCalls

	for {
		if toolCall != nil {
			var res int
			var args tools.InputArgs
			err := json.Unmarshal([]byte(toolCall[0].Function.Arguments), &args)
			if err != nil {
				log.Fatalln("Error unmarshalling tool call arguments:", err.Error())
			}

			if toolCall[0].Function.Name == tools.AddToolDefine.Function.Name {
				res = tools.AddTool(args)
			} else if toolCall[0].Function.Name == tools.SubToolDefine.Function.Name {
				res = tools.SubTool(args)
			}

			fmt.Println("Tool call result:", res)
			ai.MessageStore.AddFor(ai.RoleAssistant, resp.Content, toolCall)
			ai.MessageStore.AddForTool(fmt.Sprintf("%d", res), toolCall[0].Function.Name, toolCall[0].ID)

			resp = ai.ToolChat(ai.MessageStore.ToMessage(), toolList)
			toolCall = resp.ToolCalls
		} else {
			fmt.Println("Final response from AI:", resp.Content)
			break
		}
	}

}
