package agent

import (
	"context"
	"fmt"
	"os"

	"github.com/sashabaranov/go-openai"
)

// OpenAILLMClient implements LLMClient using OpenAI-compatible API
type OpenAILLMClient struct {
	client *openai.Client
	config *LLMConfig
}

// NewOpenAILLMClient creates a new OpenAI LLM client
func NewOpenAILLMClient(config *LLMConfig) (*OpenAILLMClient, error) {
	if config == nil {
		// Default configuration for Qwen
		apiKey := os.Getenv("DASHSCOPE_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("DASHSCOPE_API_KEY environment variable not set")
		}

		config = &LLMConfig{
			Provider:    "qwen",
			Model:       "qwen-max",
			APIKey:      apiKey,
			BaseURL:     "https://dashscope.aliyuncs.com/compatible-mode/v1",
			Temperature: 0.7,
			MaxTokens:   2000,
		}
	}

	clientConfig := openai.DefaultConfig(config.APIKey)
	if config.BaseURL != "" {
		clientConfig.BaseURL = config.BaseURL
	}

	client := openai.NewClientWithConfig(clientConfig)

	return &OpenAILLMClient{
		client: client,
		config: config,
	}, nil
}

// Complete sends a prompt and returns the completion
func (c *OpenAILLMClient) Complete(ctx context.Context, messages []Message) (string, error) {
	// Convert messages to OpenAI format
	openaiMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, msg := range messages {
		openaiMessages[i] = openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	resp, err := c.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:       c.config.Model,
			Messages:    openaiMessages,
			Temperature: c.config.Temperature,
			MaxTokens:   c.config.MaxTokens,
		},
	)

	if err != nil {
		return "", fmt.Errorf("LLM API call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	return resp.Choices[0].Message.Content, nil
}

// CompleteWithTools sends a prompt with available tools
func (c *OpenAILLMClient) CompleteWithTools(ctx context.Context, messages []Message, tools []Tool) (*LLMResponse, error) {
	// Convert messages to OpenAI format
	openaiMessages := make([]openai.ChatCompletionMessage, len(messages))
	for i, msg := range messages {
		openaiMessages[i] = openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Convert tools to OpenAI format
	openaiTools := make([]openai.Tool, len(tools))
	for i, tool := range tools {
		openaiTools[i] = openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  tool.ArgsSchema(),
			},
		}
	}

	resp, err := c.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:       c.config.Model,
			Messages:    openaiMessages,
			Tools:       openaiTools,
			Temperature: c.config.Temperature,
			MaxTokens:   c.config.MaxTokens,
		},
	)

	if err != nil {
		return nil, fmt.Errorf("LLM API call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from LLM")
	}

	choice := resp.Choices[0]
	llmResponse := &LLMResponse{
		Content:      choice.Message.Content,
		FinishReason: string(choice.FinishReason),
	}

	// Convert tool calls if present
	if len(choice.Message.ToolCalls) > 0 {
		llmResponse.ToolCalls = make([]ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			llmResponse.ToolCalls[i] = ToolCall{
				ID:   tc.ID,
				Name: tc.Function.Name,
				// Arguments would need to be parsed from JSON string
				// Arguments: parseArgs(tc.Function.Arguments),
			}
		}
	}

	return llmResponse, nil
}

// MockLLMClient is a mock implementation for testing
type MockLLMClient struct {
	CompleteFunc         func(ctx context.Context, messages []Message) (string, error)
	CompleteWithToolsFunc func(ctx context.Context, messages []Message, tools []Tool) (*LLMResponse, error)
}

// Complete implements LLMClient
func (m *MockLLMClient) Complete(ctx context.Context, messages []Message) (string, error) {
	if m.CompleteFunc != nil {
		return m.CompleteFunc(ctx, messages)
	}
	return "Mock response", nil
}

// CompleteWithTools implements LLMClient
func (m *MockLLMClient) CompleteWithTools(ctx context.Context, messages []Message, tools []Tool) (*LLMResponse, error) {
	if m.CompleteWithToolsFunc != nil {
		return m.CompleteWithToolsFunc(ctx, messages, tools)
	}
	return &LLMResponse{
		Content:      "Mock response with tools",
		FinishReason: "stop",
	}, nil
}
