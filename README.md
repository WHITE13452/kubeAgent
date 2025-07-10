# KubeAgent

🚀 **基于大模型的 Kubernetes 智能运维助手** | **AI-Powered Kubernetes Operations Assistant**

[![Go Version](https://img.shields.io/badge/Go-1.24+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Status](https://img.shields.io/badge/Status-Active%20Development-orange.svg)](https://github.com/yourusername/kubeagent)

> ⚠️ **项目状态**: 该项目正在积极开发和重构中，API 和功能可能会发生变化。
> 
> **Project Status**: This project is under active development and refactoring. APIs and features may change.

## 📖 项目简介 | Overview

KubeAgent 是一个集成大模型能力的 Kubernetes 运维工具，通过自然语言交互简化 K8s 集群管理和问题诊断。基于 Function Calling 技术，让运维人员可以用自然语言描述需求，AI 助手自动调用相应的工具完成操作。

KubeAgent is an intelligent Kubernetes operations tool that integrates large language models to simplify cluster management and troubleshooting through natural language interactions.

## ✨ 核心特性 | Key Features

- 🤖 **智能对话**: 基于大模型的自然语言交互，支持中英文
- 🔧 **工具系统**: 可扩展的工具插件架构，支持动态注册
- 📊 **多模式**: 支持问题分析、资源管理、集群检查三种模式
- 🚀 **ReAct推理**: 采用 ReAct 模式提升问题解决的准确性
- 🌐 **多数据源**: 集成 kubectl、K8s API、网络搜索等数据源
- 🔄 **实时交互**: 支持流式对话和实时反馈

## 🏗️ 系统架构 | Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   CLI Frontend  │────│   Tool Manager  │────│  K8s API Server │
│   (Cobra)       │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │              ┌─────────────────┐              │
         └──────────────│  LLM Service    │──────────────┘
                        │  (Qwen/OpenAI)  │
                        └─────────────────┘
                                 │
                        ┌─────────────────┐
                        │  HTTP Backend   │
                        │  (Gin Server)   │
                        └─────────────────┘
```

## 🚀 快速开始 | Quick Start

### 前置要求 | Prerequisites

- Go 1.24+
- Kubernetes 集群访问权限
- 阿里云 DashScope API Key 或 OpenAI API Key

### 安装 | Installation

```bash
# 克隆项目
git clone https://github.com/yourusername/kubeagent.git
cd kubeagent

# 编译项目
go build -o kubeagent main.go

# 或者直接运行
go run main.go
```

### 配置 | Configuration

设置环境变量：

```bash
# 阿里云通义千问 API
export DASHSCOPE_API_KEY="your-dashscope-api-key"

# 或者 OpenAI API (可选)
export OPENAI_API_KEY="your-openai-api-key"

# Kubernetes 配置文件路径 (可选，默认使用 ~/.kube/config)
export KUBECONFIG="/path/to/your/kubeconfig"

# Tavily 搜索 API (可选，用于网络搜索功能)
export TAVILY_API_KEY="your-tavily-api-key"
```

### 启动后端服务 | Start Backend Service

```bash
# 启动 HTTP API 服务
cd ginK8s
go run main.go
# 服务将在 http://localhost:8080 启动
```

### 使用 | Usage

#### 1. 问题分析模式 | Analysis Mode
```bash
./kubeagent analyze
>>> 帮我查看 default 命名空间下 nginx pod 的日志
```

#### 2. 资源管理模式 | Resource Management Mode
```bash
./kubeagent chat
>>> 创建一个 nginx deployment
```

#### 3. 集群检查模式 | Cluster Check Mode
```bash
./kubeagent kubecheck
>>> 检查集群状态并搜索最新的 Kubernetes 最佳实践
```

## 🛠️ 可用工具 | Available Tools

| 工具名称 | 功能描述 | 支持模式 |
|---------|---------|---------|
| LogTool | 查看 Pod 日志 | analyze |
| EventTool | 查看 Pod 事件 | analyze |
| CreateTool | 创建 K8s 资源 | chat |
| ListTool | 列出 K8s 资源 | chat |
| DeleteTool | 删除 K8s 资源 | chat |
| KubeTool | 执行 kubectl 命令 | kubecheck |
| RequestTool | 网络请求工具 | kubecheck |
| TavilyTool | 网络搜索工具 | kubecheck |
| HumanTool | 人工确认工具 | 所有模式 |

## 📝 示例对话 | Example Conversations

### 故障诊断 | Troubleshooting
```
用户: nginx pod 启动失败了，帮我看看是什么问题
助手: 我来帮你检查 nginx pod 的状态。首先让我查看相关的事件信息...

[调用 EventTool 查看事件]
[调用 LogTool 查看日志]

根据日志和事件分析，问题是镜像拉取失败，建议检查镜像名称和网络连接。
```

### 资源创建 | Resource Creation
```
用户: 创建一个 nginx deployment，3个副本
助手: 我来为你创建一个包含3个副本的 nginx deployment...

[调用 CreateTool 生成并创建资源]

nginx deployment 已成功创建，包含3个副本。
```

## 🔄 项目重构计划 | Refactoring Roadmap

> **当前重构进展 | Current Refactoring Progress**

- [x] 统一工具管理系统设计
- [ ] 工具注册系统重构
- [ ] 配置管理模块优化
- [ ] 错误处理和日志系统改进
- [ ] 单元测试完善
- [ ] API 文档生成
- [ ] 性能优化
- [ ] 插件系统实现

### 即将到来的功能 | Upcoming Features

- 🔌 **插件系统**: 支持第三方工具插件
- 🌐 **Web UI**: 图形化界面
- 📊 **监控集成**: Prometheus/Grafana 集成
- 🔐 **权限控制**: RBAC 权限管理
- 🌍 **多集群**: 管理多个 K8s 集群
- 📝 **配置文件**: YAML/JSON 配置支持


## 🙏 致谢 | Acknowledgments

- [Cobra](https://github.com/spf13/cobra) - 强大的 CLI 框架
- [Gin](https://github.com/gin-gonic/gin) - 高性能 Web 框架
- [Kubernetes Client-go](https://github.com/kubernetes/client-go) - Kubernetes Go 客户端
- [阿里云通义千问](https://dashscope.aliyun.com/) - 大模型服务

---

⭐ 如果这个项目对你有帮助，请给我们一个 Star！

**联系方式 | Contact**: baijie0219@gmail.com
