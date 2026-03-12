package ai

import (
	"context"
	"log"

	"kubeagent/pkg/agent"
)

type ChatCompletionMessage struct {
	Role    string
	Content string
}

type ChatMessage struct {
	Msg ChatCompletionMessage
}

type ChatMessages []*ChatMessage

var MessageStore ChatMessages

const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

func init() {
	MessageStore = make(ChatMessages, 0)
	MessageStore.Clear()
}

func (cm *ChatMessages) Clear() {
	*cm = make([]*ChatMessage, 0)
	cm.AddForSystem("You are a helpful k8s assistant!")
}

func (cm *ChatMessages) AddFor(role string, msg string) {
	*cm = append(*cm, &ChatMessage{
		Msg: ChatCompletionMessage{
			Role:    role,
			Content: msg,
		},
	})
}

func (cm *ChatMessages) AddForSystem(msg string) {
	cm.AddFor(RoleSystem, msg)
}

func (cm *ChatMessages) AddForUser(msg string) {
	cm.AddFor(RoleUser, msg)
}

func (cm *ChatMessages) AddForAssistant(msg string) {
	cm.AddFor(RoleAssistant, msg)
}

func (cm *ChatMessages) ToMessage() []ChatCompletionMessage {
	ret := make([]ChatCompletionMessage, len(*cm))
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

func NormalChat(messages []ChatCompletionMessage) ChatCompletionMessage {
	agentMessages := make([]agent.Message, len(messages))
	for i, m := range messages {
		agentMessages[i] = agent.Message{Role: m.Role, Content: m.Content}
	}

	client, err := agent.NewAnthropicLLMClient(nil)
	if err != nil {
		log.Println("Failed to create LLM client:", err)
		return ChatCompletionMessage{}
	}

	resp, err := client.Complete(context.TODO(), agentMessages)
	if err != nil {
		log.Println("LLM call failed:", err)
		return ChatCompletionMessage{}
	}

	return ChatCompletionMessage{
		Role:    RoleAssistant,
		Content: resp,
	}
}
