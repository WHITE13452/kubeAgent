package tools

func AddTool(args InputArgs) int {
	sum := 0
	for _, num := range args.Numbsers {
		sum += num
	}
	return sum
}

func SubTool(args InputArgs) int {
	result := args.Numbsers[0]
	for _, num := range args.Numbsers[1:] {
		result -= num
	}
	return result
}