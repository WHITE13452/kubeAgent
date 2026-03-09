# KubeAgent

**基于大模型的 Kubernetes 智能运维助手** | **AI-Powered Kubernetes Operations Assistant**

[![Go Version](https://img.shields.io/badge/Go-1.24+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

> **项目状态**: 该项目正在积极开发中，API 和功能可能会发生变化。

## 项目简介

KubeAgent 是一个集成大模型能力的 Kubernetes 运维工具，通过多 Agent 协作和自然语言交互简化 K8s 集群管理和问题诊断。采用 Coordinator + Specialist Agent 架构，支持任务自动分解、DAG 依赖执行和人工审批流程。

## 核心特性

- **多 Agent 协作**: Coordinator 自动分解任务，路由到 Diagnostician / Remediator 专家 Agent
- **直接 K8s 访问**: 通过 client-go 直接访问集群 API，支持 InCluster 和 kubeconfig 两种模式
- **DAG 任务编排**: 基于依赖图的任务执行，支持并行和条件分支
- **人工审批**: 危险操作（删除、创建资源）需经 HumanTool 确认
- **可扩展工具系统**: 9 个内置工具，支持自定义 Tool 接口扩展
- **三种交互模式**: 问题诊断 (analyze)、资源管理 (chat)、集群检查 (kubecheck)

## 系统架构

```
                         ┌──────────────────┐
                         │   CLI (Cobra)    │
                         │ analyze/chat/    │
                         │ kubecheck        │
                         └────────┬─────────┘
                                  │
                         ┌────────▼─────────┐
                         │   Coordinator    │
                         │   Plan + DAG     │
                         │   Execution      │
                         └───┬─────────┬────┘
                             │         │
                ┌────────────▼──┐  ┌───▼────────────┐
                │ Diagnostician │  │  Remediator    │
                │ Agent         │  │  Agent         │
                │               │  │                │
                │ LogTool       │  │ HumanTool      │
                │ EventTool     │  │ CreateTool     │
                │ ListTool      │  │ DeleteTool     │
                │ KubeTool      │  └────────────────┘
                │ TavilyTool    │
                │ RequestTool   │
                └───────────────┘
                        │
              ┌─────────▼──────────┐
              │  K8s API (client-go)│
              │  InCluster /       │
              │  kubeconfig        │
              └────────────────────┘
```

## 快速开始

### 前置要求

- Go 1.24+
- Kubernetes 集群访问权限
- DashScope API Key（通义千问）

### 本地运行

```bash
# 克隆项目
git clone https://github.com/yourusername/kubeagent.git
cd kubeagent

# 设置环境变量
export DASHSCOPE_API_KEY="your-api-key"

# 可选：指定 kubeconfig 路径（默认 ~/.kube/config）
export KUBECONFIG="/path/to/kubeconfig"

# 可选：启用网络搜索
export TAVILY_API_KEY="your-tavily-api-key"

# 编译并运行
make build
./bin/kubeagent analyze
```

### 部署到 Kubernetes

```bash
# 1. 构建 Docker 镜像
make docker-build

# 2. 推送到镜像仓库（根据你的环境调整）
docker tag kubeagent:latest your-registry/kubeagent:latest
docker push your-registry/kubeagent:latest

# 3. 编辑 Secret（填入你的 API Key）
vim deploy/secret.yaml

# 4. 如使用私有仓库，更新 deployment.yaml 中的 image 地址
vim deploy/deployment.yaml

# 5. 部署
make deploy

# 6. 进入 Pod 使用
make exec MODE=analyze     # 诊断模式
make exec MODE=chat        # 资源管理模式
make exec MODE=kubecheck   # 集群检查模式
```

部署会自动创建 ServiceAccount 和 ClusterRole，KubeAgent 通过 InClusterConfig 访问集群 API，无需额外配置。

## 使用模式

### 1. 问题诊断 (analyze)

```bash
kubeagent analyze
>>> 帮我查看 default 命名空间下 nginx pod 的日志
>>> nginx pod 一直重启，帮我分析原因
```

Diagnostician Agent 会自动调用 LogTool、EventTool 收集信息，通过 LLM 分析根因。

### 2. 资源管理 (chat)

```bash
kubeagent chat
>>> 创建一个 nginx deployment，3个副本
>>> 删除 default 命名空间下的 test-pod
```

写操作会通过 HumanTool 请求确认后再执行。

### 3. 集群检查 (kubecheck)

```bash
kubeagent kubecheck
>>> 检查集群中所有 pod 的状态
>>> 搜索 Kubernetes HPA 最佳实践
```

支持 kubectl 命令执行和 Tavily 网络搜索。

## 可用工具

| 工具 | 说明 | Agent | 类型 |
|------|------|-------|------|
| LogTool | 查看 Pod 日志 | Diagnostician | 只读 |
| EventTool | 查看 Pod 事件 | Diagnostician | 只读 |
| ListTool | 列出 K8s 资源 | Diagnostician | 只读 |
| KubeTool | 执行 kubectl 只读命令 | Diagnostician | 只读 |
| RequestTool | HTTP 请求 + HTML 解析 | Diagnostician | 只读 |
| TavilyTool | 网络搜索 | Diagnostician | 只读 |
| HumanTool | 人工确认 | Remediator | 审批 |
| CreateTool | 创建 K8s 资源 | Remediator | 写入 |
| DeleteTool | 删除 K8s 资源 | Remediator | 写入 |

## 项目结构

```
kubeAgent/
├── KubeAgent/
│   ├── main.go                      # CLI 入口
│   ├── cmd/                         # Cobra 命令 (analyze, chat, kubecheck)
│   ├── pkg/
│   │   ├── k8s/client.go            # K8s 客户端 (InCluster + kubeconfig)
│   │   ├── agent/                   # 多 Agent 框架
│   │   │   ├── coordinator.go       # Coordinator: 任务规划 + DAG 执行
│   │   │   ├── interface.go         # Agent, Tool, LLMClient 接口
│   │   │   ├── llm_client.go        # Qwen LLM 客户端
│   │   │   └── specialists/         # Diagnostician, Remediator
│   │   └── tools/                   # 9 个 Tool 实现
│   └── examples/                    # 使用示例
├── deploy/                          # K8s 部署清单
│   ├── namespace.yaml
│   ├── rbac.yaml                    # ServiceAccount + ClusterRole
│   ├── secret.yaml                  # API Key Secret
│   └── deployment.yaml
├── Dockerfile
└── Makefile
```

## 扩展开发

### 添加自定义 Tool

```go
type MyTool struct{}

func (t *MyTool) Name() string        { return "MyTool" }
func (t *MyTool) Description() string { return "工具描述" }
func (t *MyTool) ArgsSchema() string  { return `{"type":"object","properties":{...}}` }
func (t *MyTool) Execute(params map[string]interface{}) (string, error) {
    // 实现工具逻辑
    return "result", nil
}

// 注册到 Agent
diagnostician.AddTool(&MyTool{})
```

### 添加自定义 Specialist Agent

```go
type MyAgent struct {
    *agent.BaseAgent
}

func (m *MyAgent) CanHandle(taskType agent.TaskType) bool {
    return taskType == "my_task"
}

func (m *MyAgent) Execute(ctx *agent.AgentContext, task *agent.Task) (*agent.Task, error) {
    // 实现 Agent 逻辑
}

coordinator.RegisterAgent(myAgent)
```

## 后续计划

- Security Agent（安全审计）
- OpenTelemetry 追踪
- Prometheus 指标
- Web UI / API Server 模式
- 多集群支持

## 致谢

- [Cobra](https://github.com/spf13/cobra) - CLI 框架
- [Kubernetes Client-go](https://github.com/kubernetes/client-go) - K8s Go 客户端
- [通义千问](https://dashscope.aliyun.com/) - 大模型服务

---

**联系方式**: baijie0219@gmail.com
