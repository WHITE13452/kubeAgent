package main

import (
	"fmt"
	"function-calling/ai"
)

func main() {
	ai.MessageStore.AddFor(ai.RoleSystem, "你是一个k8s的开发专家，请尽可能地帮我回答与k8s相关的问题。")
	ai.MessageStore.AddFor(ai.RoleUser, "Pod是什么？请给我一个简单的解释。")
	ai.MessageStore.AddFor(ai.RoleAssistant, "Pod是Kubernetes中最小的可部署单元，它可以包含一个或多个容器。Pod中的容器共享网络和存储资源。")
	ai.MessageStore.AddFor(ai.RoleUser, "Pod和容器有什么区别？请给我一个简单的解释。")

	response := ai.Chat(ai.MessageStore.ToMessage())	
	fmt.Println("Assistant:", response.Content)
	
}