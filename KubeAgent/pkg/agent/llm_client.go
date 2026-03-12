package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// AnthropicLLMClient implements LLMClient using Anthropic-compatible API
// Supports both Anthropic (Claude) and MiniMax models via the same SDK
type AnthropicLLMClient struct {
	client anthropic.Client
	config *LLMConfig
}

// NewAnthropicLLMClient creates a new Anthropic LLM client
// If config is nil, auto-detects provider from environment variables:
//   - ANTHROPIC_API_KEY → Anthropic (Claude)
//   - MINIMAX_API_KEY   → MiniMax (via Anthropic-compatible API)
func NewAnthropicLLMClient(config *LLMConfig) (*AnthropicLLMClient, error) {
	if config == nil {
		config = detectLLMConfig()
		if config == nil {
			return nil, fmt.Errorf("no API key found: set ANTHROPIC_API_KEY or MINIMAX_API_KEY")
		}
	}

	opts := []option.RequestOption{
		option.WithAPIKey(config.APIKey),
	}
	if config.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(config.BaseURL))
	}

	client := anthropic.NewClient(opts...)

	return &AnthropicLLMClient{
		client: client,
		config: config,
	}, nil
}

// detectLLMConfig auto-detects LLM provider from environment variables
func detectLLMConfig() *LLMConfig {
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		return &LLMConfig{
			Provider:    "anthropic",
			Model:       "claude-sonnet-4-20250514",
			APIKey:      apiKey,
			Temperature: 0.7,
			MaxTokens:   2000,
		}
	}

	if apiKey := os.Getenv("MINIMAX_API_KEY"); apiKey != "" {
		return &LLMConfig{
			Provider:    "minimax",
			Model:       "MiniMax-M2.5",
			APIKey:      apiKey,
			BaseURL:     "https://api.minimaxi.com/anthropic",
			Temperature: 0.7,
			MaxTokens:   2000,
		}
	}

	return nil
}

// Complete sends a prompt and returns the completion
func (c *AnthropicLLMClient) Complete(ctx context.Context, messages []Message) (string, error) {
	systemBlocks, anthropicMessages := c.convertMessages(messages)

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(c.config.Model),
		MaxTokens: int64(c.config.MaxTokens),
		Messages:  anthropicMessages,
	}
	if len(systemBlocks) > 0 {
		params.System = systemBlocks
	}
	if c.config.Temperature > 0 {
		params.Temperature = anthropic.Float(float64(c.config.Temperature))
	}

	resp, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("LLM API call failed: %w", err)
	}

	return c.extractTextContent(resp), nil
}

// CompleteWithTools sends a prompt with available tools
func (c *AnthropicLLMClient) CompleteWithTools(ctx context.Context, messages []Message, tools []Tool) (*LLMResponse, error) {
	systemBlocks, anthropicMessages := c.convertMessages(messages)
	anthropicTools := c.convertTools(tools)

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(c.config.Model),
		MaxTokens: int64(c.config.MaxTokens),
		Messages:  anthropicMessages,
		Tools:     anthropicTools,
	}
	if len(systemBlocks) > 0 {
		params.System = systemBlocks
	}
	if c.config.Temperature > 0 {
		params.Temperature = anthropic.Float(float64(c.config.Temperature))
	}

	resp, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("LLM API call failed: %w", err)
	}

	llmResponse := &LLMResponse{
		Content:      c.extractTextContent(resp),
		FinishReason: string(resp.StopReason),
	}

	// Extract tool calls from response
	for _, block := range resp.Content {
		if toolUse, ok := block.AsAny().(anthropic.ToolUseBlock); ok {
			var args map[string]interface{}
			json.Unmarshal(toolUse.Input, &args)
			llmResponse.ToolCalls = append(llmResponse.ToolCalls, ToolCall{
				ID:        toolUse.ID,
				Name:      toolUse.Name,
				Arguments: args,
			})
		}
	}

	return llmResponse, nil
}

// convertMessages separates system messages and converts to Anthropic format
// Handles plain text messages, assistant messages with tool calls, and tool result messages
func (c *AnthropicLLMClient) convertMessages(messages []Message) ([]anthropic.TextBlockParam, []anthropic.MessageParam) {
	var systemBlocks []anthropic.TextBlockParam
	var anthropicMessages []anthropic.MessageParam

	for i := 0; i < len(messages); i++ {
		msg := messages[i]
		switch msg.Role {
		case "system":
			systemBlocks = append(systemBlocks, anthropic.TextBlockParam{Text: msg.Content})
		case "user":
			anthropicMessages = append(anthropicMessages, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)))
		case "assistant":
			if len(msg.ToolCalls) > 0 {
				// Assistant message with tool use blocks
				var blocks []anthropic.ContentBlockParamUnion
				if msg.Content != "" {
					blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
				}
				for _, tc := range msg.ToolCalls {
					blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, tc.Arguments, tc.Name))
				}
				anthropicMessages = append(anthropicMessages, anthropic.NewAssistantMessage(blocks...))
			} else {
				anthropicMessages = append(anthropicMessages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Content)))
			}
		case "tool":
			// Batch consecutive tool result messages into a single user message
			var toolResults []anthropic.ContentBlockParamUnion
			for i < len(messages) && messages[i].Role == "tool" {
				toolResults = append(toolResults, anthropic.NewToolResultBlock(
					messages[i].ToolCallID,
					messages[i].Content,
					messages[i].IsError,
				))
				i++
			}
			i-- // compensate for outer loop increment
			anthropicMessages = append(anthropicMessages, anthropic.NewUserMessage(toolResults...))
		}
	}

	return systemBlocks, anthropicMessages
}

// convertTools converts Tool interface to Anthropic tool format
func (c *AnthropicLLMClient) convertTools(tools []Tool) []anthropic.ToolUnionParam {
	result := make([]anthropic.ToolUnionParam, len(tools))
	for i, tool := range tools {
		var schema map[string]interface{}
		json.Unmarshal([]byte(tool.ArgsSchema()), &schema)
		properties, _ := schema["properties"]

		toolParam := anthropic.ToolParam{
			Name:        tool.Name(),
			Description: anthropic.String(tool.Description()),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: properties,
			},
		}
		result[i] = anthropic.ToolUnionParam{OfTool: &toolParam}
	}
	return result
}

// extractTextContent extracts text from Anthropic response content blocks
func (c *AnthropicLLMClient) extractTextContent(resp *anthropic.Message) string {
	var result string
	for _, block := range resp.Content {
		if textBlock, ok := block.AsAny().(anthropic.TextBlock); ok {
			result += textBlock.Text
		}
	}
	return result
}

// MockLLMClient is a mock implementation for testing
type MockLLMClient struct {
	CompleteFunc          func(ctx context.Context, messages []Message) (string, error)
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
