# KubeAgent

**基于大模型的 Kubernetes 智能运维助手** | **AI-Powered Kubernetes Operations Assistant**

[![Go Version](https://img.shields.io/badge/Go-1.24+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

> **项目状态**: 该项目正在积极开发中，API 和功能可能会发生变化。

## 项目简介

KubeAgent 是一个集成大模型能力的 Kubernetes 运维工具，通过多 Agent 协作和自然语言交互简化 K8s 集群管理和问题诊断。采用 Coordinator + Specialist Agent 架构，支持任务自动分解、DAG 依赖执行和人工审批流程。

## 核心特性

- **多 Agent 协作**: Coordinator 自动分解任务，路由到 Diagnostician / Remediator 专家 Agent
- **Harness 闭环框架**: Guides（Preflight 前置校验）+ Sensors（Verifier 后置校验 + Audit 结构化审计），避免 open-loop 修复
- **直接 K8s 访问**: 通过 client-go 直接访问集群 API，支持 InCluster 和 kubeconfig 两种模式
- **DAG 任务编排**: 基于依赖图的任务执行，支持并行和条件分支
- **人工审批 + 策略保护**: 危险操作需 HumanTool 确认，并受 ProtectedNamespaceCheck 等 Guide 策略约束
- **可扩展工具系统**: 9 个内置工具，支持自定义 Tool 接口扩展
- **Skills 可热替换**: LLM 系统提示以 Markdown 形式嵌入（`pkg/agent/skills/*.md`），支持运行时通过 `SKILLS_DIR` 覆盖
- **四种交互模式**: 问题诊断 (analyze)、资源管理 (chat)、集群检查 (kubecheck)、闭环修复 (fix)

## 系统架构

```
                         ┌──────────────────────────┐
                         │       CLI (Cobra)        │
                         │ analyze/chat/kubecheck/fix│
                         └────────────┬─────────────┘
                                      │
                         ┌────────────▼─────────────┐
                         │       Coordinator        │
                         │    Plan + DAG Execution  │
                         └───┬──────────────────┬───┘
                             │                  │
                ┌────────────▼──┐       ┌───────▼────────┐
                │ Diagnostician │       │   Remediator   │
                │    Agent      │       │     Agent      │
                │               │       │                │
                │ LogTool       │       │ HumanTool      │
                │ EventTool     │       │ CreateTool ────┼──┐
                │ ListTool      │       │ DeleteTool ────┼──┤
                │ KubeTool      │       │ KubeTool       │  │
                │ TavilyTool    │       └───────┬────────┘  │ Guide
                │ RequestTool   │               │           ▼
                └───────────────┘               │    ┌─────────────────┐
                                                │    │ PreflightChain  │
                                                │    │ - ProtectedNS   │
                                                │    │ - ResourceExists│
                                                │    └─────────────────┘
                                                │
                                                ▼ Sensor
                                        ┌─────────────────┐
                                        │   K8sVerifier   │
                                        │ poll → converge │
                                        └────────┬────────┘
                                                 │
                                                 ▼
                                        ┌─────────────────┐
                                        │  AuditLogger    │
                                        │ Console + JSONL │
                                        └─────────────────┘
                             │
              ┌──────────────▼──────────────┐
              │     K8s API (client-go)     │
              │   InCluster / kubeconfig    │
              └─────────────────────────────┘
```

### Harness 框架（Guides + Sensors）

KubeAgent 参考 Martin Fowler 的 Harness 工程方法将 Agent 从 “open-loop 瞎操作”
升级为 “closed-loop 可审计闭环”：

| 角色 | 实现 | 作用 |
|------|------|------|
| **Guide**（前置） | `PreflightChain` + `ProtectedNamespaceCheck` / `ResourceExistsCheck` | 在写工具执行前拦截违规操作 |
| **Sensor**（后置） | `K8sVerifier` 轮询集群真实状态 | 检查修复动作是否让资源收敛到期望相位 |
| **Audit** | `JSONLogAuditor` + `ConsoleReporter` + `Tee` | 每一次 Preflight / Action / Verification / Decision 都落盘为 JSONL，并在终端实时呈现 |
| **Skills** | `pkg/agent/skills/*.md` + `go:embed` | 把 LLM 提示词与代码解耦，可运行时 override |

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

### 4. 闭环修复 (fix) — Harness 加持

```bash
# 指定 Pod，启用审计落盘 + 命名空间保护
kubeagent fix \
  --pod bad-image-xxxxx \
  --namespace demo \
  --audit-file /tmp/kubeagent-audit.jsonl \
  --protected kube-system,kube-public,kube-node-lease

# 仅给自然语言描述，由 LLM 推断目标
kubeagent fix \
  --description "redis pod 在 cache 命名空间反复重启，帮我查一下并修"

# Debug：跳过 Verifier（不推荐在生产使用，仅用于演示 open-loop 对照）
kubeagent fix --pod foo --no-verify
```

`fix` 把 Diagnostician + Remediator + Verifier + AuditLogger + Skills + Preflight 连接成一条端到端流水线：

1. **Guide** 在每次写操作前检查受保护命名空间、目标资源存在性。
2. **Action** 由 LLM 驱动的 tool loop 执行实际修复。
3. **Sensor** 轮询 K8s API 确认资源是否真的收敛到期望相位。
4. **Audit** 四类事件（preflight / action / verification / decision）实时写入控制台并按需追加到 JSONL 文件。

> 完整演示见 [`docs/DEMO.md`](docs/DEMO.md)。

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
│   ├── cmd/                         # Cobra 命令 (analyze, chat, kubecheck, fix)
│   ├── pkg/
│   │   ├── k8s/client.go            # K8s 客户端 (InCluster + kubeconfig)
│   │   ├── agent/                   # 多 Agent 框架
│   │   │   ├── coordinator.go       # Coordinator: 任务规划 + DAG 执行
│   │   │   ├── interface.go         # Agent, Tool, LLMClient 接口
│   │   │   ├── llm_client.go        # LLM 客户端
│   │   │   ├── specialists/         # Diagnostician, Remediator
│   │   │   ├── harness/             # Harness 框架
│   │   │   │   ├── verifier.go      # 后置验证 Sensor 接口 + Noop
│   │   │   │   ├── k8s_verifier.go  # K8sVerifier 轮询实现
│   │   │   │   ├── preflight.go     # PreflightChain + 内置 Guide
│   │   │   │   ├── audit.go         # AuditLogger + JSONLogAuditor
│   │   │   │   ├── reporter.go      # ConsoleReporter + Tee
│   │   │   │   ├── retry.go         # 带抖动的指数退避
│   │   │   │   └── skills.go        # Skills 注册 + 运行时覆盖
│   │   │   └── skills/              # LLM 提示词 (diagnose/remediate/decompose .md + go:embed)
│   │   └── tools/                   # 9 个 Tool 实现（CreateTool/DeleteTool 支持 Preflight）
│   └── examples/
│       ├── multi_agent_demo.go      # 编码层 demo
│       └── demo/                    # 端到端 CLI demo
│           ├── README.md
│           └── bad-image-deployment.yaml
├── docs/
│   └── DEMO.md                      # 闭环修复演示文档（配图位）
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
