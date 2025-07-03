package tools

import (
	"encoding/json"
	"fmt"
	"kubeagent/cmd/model"
	"kubeagent/cmd/utils"
)

const (
	url    = "https://api.tavily.io/v1/search"
	apiKey = "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"
)

type TavilyTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ArgsSchema  string `json:"args_schema"`
}

func NewTavilyTool() *TavilyTool {
	return &TavilyTool{
		Name: "TavilyTool",
		Description: `
		Search the web for information on a topic
		`,
		ArgsSchema: `description: "要搜索的内容，格式是字符串" example: "C罗是谁？"`,
	}
}

func (t *TavilyTool) Run(query string) ([]model.FinalResult, error) {
	requestParams := model.TavilyRequestParams{
		APIKey:      apiKey,
		Query:       query,
		Days:        7,
		MaxResults:  5,
		SearchDepth: "basic",
	}
	jsonBody, err := json.Marshal(requestParams)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	response, err := utils.PostHTTP(url, jsonBody)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}

	var tavilyResponse model.TavilyResponse
	if err := json.Unmarshal([]byte(response), &tavilyResponse); err != nil {
		return nil, fmt.Errorf("error unmarshalling response: %w", err)
	}

	var finalResults []model.FinalResult
	for _, result := range tavilyResponse.Results {
		finalResults = append(finalResults, model.FinalResult{
			Title: "title: " + result.Title,
			Link:  " link: " + result.URL,
			// Snippet: result.Content, // Uncomment if you want to include snippet
		})
	}
	return finalResults, nil
}
