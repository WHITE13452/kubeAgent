package tools

import (
	"encoding/json"
	"fmt"
	"kubeagent/cmd/ai"
	prompttpl "kubeagent/cmd/promptTpl"
	"kubeagent/cmd/utils"
	"log"

	"github.com/sashabaranov/go-openai"
)

type CreateTool struct {
	Name string
	Description string
	ArgsSchema string
}

type CreateToolParam struct {
	Prompt string `json:"prompt"` // Prompt is the prompt to create a tool
	Resource string `json:"resource"` 
}

// unmarshall json response to this struct
type CreateToolResponse struct {
	Data string `json:"data"` 
}

func NewCreateTool() *CreateTool {
	return &CreateTool{
		Name:        "CreateTool",
		Description: "用于在指定命名空间创建指定 Kubernetes 资源，例如创建某 pod 等等",
		ArgsSchema:  `{"type":"object","properties":{"prompt":{"type":"string", "description": "把用户提出的创建资源的prompt原样放在这，不要做任何改变"},"resource":{"type":"string", "description": "指定的 k8s 资源类型，例如 pod, service等等"}}}`,
	}
}

func (c *CreateTool) Run(prompt, resource string) string {
	messages := make([]openai.ChatCompletionMessage, 2)

	messages[0] = openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: prompttpl.SystemPrompt,
	}
	messages[1] = openai.ChatCompletionMessage{
		Role:   openai.ChatMessageRoleUser,
		Content: prompt,
	}

	resp := ai.NormalChat(messages)
	fmt.Println("CreateTool response:", resp.Content)

	body := map[string]string{"yaml": resp.Content}
	requestJsonBody, err := json.Marshal(body)
	if err != nil {
		log.Println("Error marshalling JSON:", err)
		return "Error creating tool: " + err.Error()
	}

	url := "http://localhost:8080/" + resource
	responseBody ,err := utils.PostHTTP(url, requestJsonBody)
	if err != nil {
		log.Println("Error making HTTP request:", err)
		return "Error creating tool: " + err.Error()
	}
	fmt.Println("HTTP response body:", responseBody)
	var response CreateToolResponse
	err = json.Unmarshal([]byte(responseBody), &response)
	if err != nil {
		log.Println("Error unmarshalling JSON:", err)
		return "Error creating tool: " + err.Error()
	}

	return response.Data
}