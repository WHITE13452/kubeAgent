package ai

import (
	"context"
	"log"
	"os"

	"github.com/sashabaranov/go-openai"
)

const (
	// Environment variable for the DashScope API key
	ENV_DASHSCOPE_API_KEY = "DASHSCOPE_API_KEY"
	QwenBaseURL           = "https://dashscope.aliyuncs.com/compatible-mode/v1"

	// Roles for chat messages
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

func NewOpenAIClient() *openai.Client {
	token := os.Getenv(ENV_DASHSCOPE_API_KEY)
	baseUrl := QwenBaseURL

	config := openai.DefaultConfig(token)
	config.BaseURL = baseUrl
	return openai.NewClientWithConfig(config)
}

func Chat(message []openai.ChatCompletionMessage) openai.ChatCompletionMessage {
	client := NewOpenAIClient()
	resp, err := client.CreateChatCompletion(
		context.TODO(),
		openai.ChatCompletionRequest{
			Model:    "qwen-plus",
			Messages: message,
		},
	)
	if err != nil {
		log.Println(err)
		return openai.ChatCompletionMessage{}
	}
	return resp.Choices[0].Message
}

func ToolChat(message []openai.ChatCompletionMessage, tools []openai.Tool) openai.ChatCompletionMessage {
	client := NewOpenAIClient()
	resp, err := client.CreateChatCompletion(
		context.TODO(),
		openai.ChatCompletionRequest{
			Model:      "qwen-plus",
			Messages:   message,
			Tools:      tools,
			ToolChoice: "auto",
		},
	)
	if err != nil {
		log.Println(err)
		return openai.ChatCompletionMessage{}
	}
	return resp.Choices[0].Message
}

// MessageStore is a global variable to store chat messages
var MessageStore ChatMessages

type ChatMessages []openai.ChatCompletionMessage

// AddFor adds a new message to the chat messages with the specified role
func (cm *ChatMessages) AddFor(role string, msg string, toolCalls []openai.ToolCall) {
	*cm = append(*cm, openai.ChatCompletionMessage{
		Role:      role,
		Content:   msg,
		ToolCalls: toolCalls,
	})
}
func (cm *ChatMessages) AddForTool(msg, name, toolCallsID string) {
	*cm = append(*cm, openai.ChatCompletionMessage{
		Role:         RoleTool,
		Content:      msg,
		Name:         name,
		ToolCallID:   toolCallsID,
	})
}

// ToMessage converts the ChatMessages to a slice of openai.ChatCompletionMessage
// This is useful for passing the messages to the OpenAI API
// It returns a copy of the messages to avoid modifying the original slice
func (cm *ChatMessages) ToMessage() []openai.ChatCompletionMessage {
	ret := make([]openai.ChatCompletionMessage, len(*cm))
	copy(ret, *cm)
	return ret
}
