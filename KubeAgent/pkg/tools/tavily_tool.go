package tools

import (
	"encoding/json"
	"fmt"
	"kubeagent/cmd/model"
	"kubeagent/cmd/utils"
	"strings"
)

const tavilyURL = "https://api.tavily.io/v1/search"

// TavilyTool searches the web using Tavily API
type TavilyTool struct {
	APIKey string
}

func NewTavilyTool(apiKey string) *TavilyTool {
	return &TavilyTool{APIKey: apiKey}
}

func (t *TavilyTool) Name() string {
	return "TavilyTool"
}

func (t *TavilyTool) Description() string {
	return "用于在网络上搜索信息，适合查找 Kubernetes 文档、最佳实践、故障解决方案等"
}

func (t *TavilyTool) ArgsSchema() string {
	return `{"type":"object","properties":{"query":{"type":"string","description":"要搜索的内容"}},"required":["query"]}`
}

func (t *TavilyTool) Execute(params map[string]any) (string, error) {
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("query is required")
	}

	jsonBody, err := json.Marshal(model.TavilyRequestParams{
		APIKey:      t.APIKey,
		Query:       query,
		Days:        7,
		MaxResults:  5,
		SearchDepth: "basic",
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	response, err := utils.PostHTTP(tavilyURL, jsonBody)
	if err != nil {
		return "", fmt.Errorf("tavily search failed: %w", err)
	}

	var tavilyResponse model.TavilyResponse
	if err := json.Unmarshal([]byte(response), &tavilyResponse); err != nil {
		return "", fmt.Errorf("failed to parse tavily response: %w", err)
	}

	var sb strings.Builder
	for _, result := range tavilyResponse.Results {
		fmt.Fprintf(&sb, "Title: %s\nURL: %s\n\n", result.Title, result.URL)
	}
	return sb.String(), nil
}
