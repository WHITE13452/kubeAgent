package tools

import "fmt"

type HumanToolOaram struct {
	Prompt string `json:"prompt"`
}

type HumanTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ArgsSchema  string `json:"args_schema"`
}

func NewHumanTool() *HumanTool {
	return &HumanTool{
		Name:        "HumanTool",
		Description: "当你判断出要执行一些不可逆的危险操作时，比如删除动作，需要先用本工具向人类发起确认",
		ArgsSchema:  `{"type":"object","properties":{"prompt":{"type":"string", "description": "你要向人类寻求帮助的内容", "example": "请确认是否要删除 default 命名空间下的 foo-app pod"}}}`,
	}
}

func (h *HumanTool) Run(prompt string) string {
	fmt.Println("HumanTool Run called with prompt:", prompt)
	var input string
	fmt.Scanln(&input)
	return input
}
