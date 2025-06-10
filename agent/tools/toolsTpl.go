package tools

const AddToolName = "AddTool"
const SubToolName = "SubTool"

const AddToolDescription = `
Use this tool for addition calculations.
	example:
		1+2 = ?
	then Action Input is : 1, 2
`
const SubToolDescription = `
Use this tool for subtraction calculations.	
	example:
		1-2 = ?
	then Action Input is : 1, 2
`
const AddToolParameters = `{
"type": "object",
"properties": {
	"numbers": {	
		"type": "array",
		"items": {
		"type": "integer"
		}
	}
}
}`

const SubToolParameters = `{
"type": "object",
"properties": {
	"numbers": {
		"type": "array",
		"items": {
		"type": "integer"
		}	
	}
}
}`
