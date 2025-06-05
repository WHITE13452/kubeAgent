package tools

import "github.com/sashabaranov/go-openai"

var AddToolDefine = openai.Tool{
	Type: "function",
	Function: &openai.FunctionDefinition{
		Name: "AddTool",
		Description: `
		Use this tool for addition calculations.
			example:
				1+2 = ?
			then Action Input is : 1, 2
		`,
		Parameters: `{
		"type": "object",
		"properties": {
			"numbers": {
				"type": "array",
				"items": {
					"type": "number"
				},`,
	},
}

var SubToolDefine = openai.Tool{
	Type: "function",
	Function: &openai.FunctionDefinition{
		Name: "SubTool",
		Description: `
		Use this tool for subtraction calculations.
			example:
				1-2 = ?
			then Action Input is : 1, 2
		`,
		Parameters: `{
		"type": "object",
		"properties": {
			"numbers": {
				"type": "array",
				"items": {	
					"type": "number"
				},`,
	},
}
