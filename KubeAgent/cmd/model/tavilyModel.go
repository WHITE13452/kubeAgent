package model

// RequestParams 定义请求参数
type TavilyRequestParams struct {
	APIKey                   string   `json:"api_key"`
	Query                    string   `json:"query"`
	SearchDepth              string   `json:"search_depth,omitempty"`
	Topic                    string   `json:"topic,omitempty"`
	Days                     int      `json:"days,omitempty"`
	MaxResults               int      `json:"max_results,omitempty"`
	IncludeImages            bool     `json:"include_images,omitempty"`
	IncludeImageDescriptions bool     `json:"include_image_descriptions,omitempty"`
	IncludeAnswer            bool     `json:"include_answer,omitempty"`
	IncludeRawContent        bool     `json:"include_raw_content,omitempty"`
	IncludeDomains           []string `json:"include_domains,omitempty"`
	ExcludeDomains           []string `json:"exclude_domains,omitempty"`
}

type TavilyResponse struct {
	Query        string         `json:"query"`
	Answer       string         `json:"answer,omitempty"`
	ResponseTime float64        `json:"response_time"`
	Images       []Image        `json:"images,omitempty"`
	Results      []SearchResult `json:"results,omitempty"`
}

// Image 定义图像结构
type Image struct {
	URL         string `json:"url"`
	Description string `json:"description"`
}

// SearchResult 定义搜索结果结构
type SearchResult struct {
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	Content       string  `json:"content"`
	RawContent    string  `json:"raw_content,omitempty"`
	Score         float64 `json:"score"`
	PublishedDate string  `json:"published_date,omitempty"`
}

type FinalResult struct {
	Title string
	Link  string
	//Snippet string
}