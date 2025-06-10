package ai

import (
	"context"
	"log"
	"os"

	"github.com/sashabaranov/go-openai"
)

// MessageStore is a global variable to store chat messages
var MessageStore ChatMessages

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

func init() {
	MessageStore = make(ChatMessages, 0)
	MessageStore.Clear()
}

func NewOpenAIClient() *openai.Client {
	token := os.Getenv(ENV_DASHSCOPE_API_KEY)
	baseUrl := QwenBaseURL

	config := openai.DefaultConfig(token)
	config.BaseURL = baseUrl
	return openai.NewClientWithConfig(config)
}

func NormalChat(message []openai.ChatCompletionMessage) openai.ChatCompletionMessage {
	client := NewOpenAIClient()
	resp, err := client.CreateChatCompletion(
		context.TODO(),
		openai.ChatCompletionRequest{
			Model:    "qwen-max",
			Messages: message,
		},
	)
	if err != nil {
		log.Println(err)
		return openai.ChatCompletionMessage{}
	}
	return resp.Choices[0].Message
}

func Chat(message []openai.ChatCompletionMessage, tools []openai.Tool) openai.ChatCompletionMessage {
	client := NewOpenAIClient()
	resp, err := client.CreateChatCompletion(
		context.TODO(),
		openai.ChatCompletionRequest{
			Model:      "qwen-max",
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



type ChatMessages []*ChatMessage

type ChatMessage struct {
	Msg openai.ChatCompletionMessage
}

func (cm *ChatMessages) Clear() {
	*cm = make([]*ChatMessage, 0)
	cm.AddForSystem("你是一个数学家")
}

// AddFor adds a new message to the chat messages with the specified role
func (cm *ChatMessages) AddFor(role string, msg string) {
	*cm = append(*cm, &ChatMessage{
		Msg: openai.ChatCompletionMessage{
			Role:      role,
			Content:   msg,
		},
	})
} 

func (cm *ChatMessages) AddForToolCall(role string, resp openai.ChatCompletionMessage) {
	*cm = append(*cm, &ChatMessage{
		Msg: openai.ChatCompletionMessage{
			Role:      role,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
			FunctionCall: resp.FunctionCall,
		},
	})
}
func (cm *ChatMessages) AddForAssistant(resp openai.ChatCompletionMessage) {
	cm.AddForToolCall(RoleAssistant, resp)
}

func (cm *ChatMessages) AddForSystem(msg string) {
	cm.AddFor(RoleSystem, msg)
}

func (cm *ChatMessages) AddForUser(msg string) {
	cm.AddFor(RoleUser, msg)
}

func (cm *ChatMessages) AddForTool(msg, name, toolCallsID string) {
	*cm = append(*cm, &ChatMessage{
		Msg: openai.ChatCompletionMessage{
			Role:       RoleTool,
			Content:    msg,
			Name:       name,
			ToolCallID: toolCallsID,
		},
	})
}

// ToMessage converts the ChatMessages to a slice of openai.ChatCompletionMessage
// This is useful for passing the messages to the OpenAI API
// It returns a copy of the messages to avoid modifying the original slice
func (cm *ChatMessages) ToMessage() []openai.ChatCompletionMessage {
	ret := make([]openai.ChatCompletionMessage, len(*cm))
	for index, c := range *cm {
		ret[index] = c.Msg
	}
	return ret
}

func (cm *ChatMessages) GetLast() string {
	if len(*cm) == 0 {
		return "nothing found"
	}
	return (*cm)[len(*cm)-1].Msg.Content
}
