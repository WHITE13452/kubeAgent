package tools

import (
	"strconv"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

func AddToolTpl() openai.Tool {
	fundefine := openai.FunctionDefinition{
		Name:        AddToolName,
		Description: AddToolDescription,
		Parameters:  AddToolParameters,
	}

	tool := openai.Tool{
		Type : openai.ToolTypeFunction,
		Function: &fundefine,
	}
	return tool
}

func SubToolTpl() openai.Tool {
	fundefine := openai.FunctionDefinition{
		Name:        SubToolName,
		Description: SubToolDescription,
		Parameters:  SubToolParameters,
	}

	tool := openai.Tool{
		Type : openai.ToolTypeFunction,
		Function: &fundefine,
	}
	return tool
}

func AddTool(numbsers string) int {
	nums := strings.Split(numbsers, ",")
	inum0, _ := strconv.Atoi(nums[0])
	inum1, _ := strconv.Atoi(nums[1])
	return inum0 + inum1
}

func SubTool(numbsers string) int {
	nums := strings.Split(numbsers, ",")
	inum0, _ := strconv.Atoi(nums[0])
	inum1, _ := strconv.Atoi(nums[1])
	return inum0 - inum1
}
