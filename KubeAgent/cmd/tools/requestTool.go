package tools

import (
	"fmt"
	"kubeagent/cmd/utils"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type RequestTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ArgsSchema  string `json:"args_schema"`
}

func NewRequestTool() *RequestTool {
	return &RequestTool{
		Name: "RequestsTool",
		Description: `
		A portal to the internet. Use this when you need to get specific
    content from a website. Input should be a url (i.e. https://www.kubernetes.io/releases).
    The output will be the text response of the GET request.
		`,
		ArgsSchema: `description: "要访问的website，格式是字符串" example: "https://www.kubernetes.io/releases"`,
	}
}

func (r *RequestTool) Run(url string) (string, error) {
	responseBody, err := utils.GetHTTP(url)
	if err != nil {
		return "", fmt.Errorf("NewRequestTool error making request: %w", err)
	}
	

	return r.parseHTML(responseBody), nil
}

func (r *RequestTool) parseHTML(htmlContent string) string { 
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return fmt.Sprintf("Error parsing HTML: %v", err)
	}

	doc.Find("header, footer, script, style").Each(func(i int, s *goquery.Selection) {
		s.Remove()
	})

	return doc.Find("body").Text()
}