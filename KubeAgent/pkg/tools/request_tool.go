package tools

import (
	"fmt"
	"kubeagent/cmd/utils"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// RequestTool fetches a web page and returns its text content
type RequestTool struct{}

func NewRequestTool() *RequestTool {
	return &RequestTool{}
}

func (r *RequestTool) Name() string {
	return "RequestTool"
}

func (r *RequestTool) Description() string {
	return "用于访问指定 URL 并获取网页的文本内容，适合查阅在线文档"
}

func (r *RequestTool) ArgsSchema() string {
	return `{"type":"object","properties":{"url":{"type":"string","description":"要访问的网页 URL，例如 https://kubernetes.io/docs"}},"required":["url"]}`
}

func (r *RequestTool) Execute(params map[string]any) (string, error) {
	url, ok := params["url"].(string)
	if !ok || url == "" {
		return "", fmt.Errorf("url is required")
	}

	responseBody, err := utils.GetHTTP(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}

	return r.parseHTML(responseBody), nil
}

func (r *RequestTool) parseHTML(htmlContent string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent
	}
	doc.Find("header, footer, script, style").Each(func(i int, s *goquery.Selection) {
		s.Remove()
	})
	return doc.Find("body").Text()
}
