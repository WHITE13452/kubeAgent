# KubeAgent 快速开始指南

## 环境准备

### 1. 安装依赖

```bash
cd /Users/I765226/develop/go-workspace/kubeAgent/KubeAgent
go mod download
```

### 2. 配置环境变量

```bash
# 设置通义千问 API Key
export DASHSCOPE_API_KEY="your-api-key-here"
```

## 运行示例

### 方式一：运行完整示例

```bash
cd /Users/I765226/develop/go-workspace/kubeAgent/KubeAgent
go run examples/multi_agent_demo.go
```

**示例输出**：
```
=== KubeAgent Multi-Agent Framework Demo ===

✓ Coordinator and specialist agents initialized
✓ Registered agents: Diagnostician, Remediator

=== Example 1: Simple Diagnosis Task ===
[2026-01-07 10:30:00] KubeAgent INFO: Coordinator executing task {task_id=task-001, task_type=diagnose}
✓ Diagnosis completed
  Task ID: task-001
  Status: completed
  Output:
  {
    "pod_name": "nginx-deployment-7d5c8b9f4d-x8k2l",
    "namespace": "production",
    "root_cause": "Pod is in CrashLoopBackOff due to...",
    "recommendations": ["Recommendation 1", "Recommendation 2"]
  }

...
```

### 方式二：使用现有 CLI (逐步迁移)

当前 CLI 仍然可用：

```bash
# 诊断模式
go run main.go analyze

# 资源管理模式
go run main.go chat

# 集群检查模式
go run main.go kubecheck
```

## 运行测试

```bash
# 运行所有测试
go test ./pkg/agent/... -v

# 运行单个测试
go test ./pkg/agent/ -v -run TestCoordinatorBasics
```

**测试输出**：
```
=== RUN   TestCoordinatorBasics
--- PASS: TestCoordinatorBasics (0.00s)
=== RUN   TestCoordinatorExecution
--- PASS: TestCoordinatorExecution (0.00s)
=== RUN   TestStateSaving
--- PASS: TestStateSaving (0.00s)
PASS
```

## 代码示例

### 示例 1: 简单的诊断任务

```go
package main

import (
    "context"
    "fmt"
    "kubeagent/pkg/agent"
    "kubeagent/pkg/agent/specialists"
)

func main() {
    // 初始化
    logger := agent.NewSimpleLogger("KubeAgent")
    stateStore := agent.NewMemoryStateStore()
    llmClient, _ := agent.NewOpenAILLMClient(nil)

    // 创建 Coordinator
    coordinator := agent.NewCoordinator(nil, llmClient, stateStore, logger)

    // 注册 Diagnostician
    diagnostician := specialists.NewDiagnosticianAgent(llmClient, logger)
    coordinator.RegisterAgent(diagnostician)

    // 创建诊断任务
    ctx := agent.NewAgentContext(
        context.Background(),
        "req-001",
        "user@example.com",
        "trace-001",
    )

    task := &agent.Task{
        ID:            "task-001",
        Type:          agent.TaskTypeDiagnose,
        Description:   "nginx pod is in CrashLoopBackOff",
        AssignedAgent: agent.AgentTypeDiagnostician,
        Input: map[string]interface{}{
            "pod_name":  "nginx-7d5c8b9f4d-x8k2l",
            "namespace": "production",
        },
    }

    // 执行任务
    result, err := coordinator.Execute(ctx, task)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Diagnosis result: %v\n", result.Output)
}
```

### 示例 2: 自动规划执行

```go
// 用户请求
request := &agent.Request{
    ID:    "req-001",
    User:  "user@example.com",
    Input: "My nginx pod keeps restarting. Can you diagnose and fix it?",
}

// 自动规划
plan, _ := coordinator.Plan(ctx, request)
fmt.Printf("Created plan with %d tasks\n", len(plan.Tasks))

// 执行计划
response, _ := coordinator.ExecutePlan(ctx, plan)
fmt.Printf("Result: %s\n", response.Result)
```

## 项目结构

```
kubeAgent/
├── KubeAgent/                      # 主项目
│   ├── main.go                     # CLI 入口
│   ├── cmd/                        # Cobra 命令
│   │   ├── analyze.go              # 诊断模式（旧）
│   │   ├── chat.go                 # 资源管理模式（旧）
│   │   └── kubecheck.go            # 集群检查模式（旧）
│   ├── pkg/agent/                  # 新多 Agent 框架 ✨
│   │   ├── types.go                # 核心类型定义
│   │   ├── interface.go            # 接口定义
│   │   ├── coordinator.go          # Coordinator 实现
│   │   ├── base_agent.go           # BaseAgent 实现
│   │   ├── state_store.go          # 状态存储
│   │   ├── llm_client.go           # LLM 客户端
│   │   ├── logger.go               # 日志系统
│   │   └── specialists/            # 专家 Agent
│   │       ├── diagnostician.go    # 诊断 Agent
│   │       └── remediator.go       # 修复 Agent
│   ├── examples/                   # 示例代码
│   │   ├── multi_agent_demo.go     # 完整示例
│   │   └── README.md               # 示例文档
│   └── cmd/tools/                  # 工具集（旧）
├── ginK8s/                         # K8s API 后端
├── REQUIREMENTS.md                 # 产品需求文档
├── MULTI_AGENT_FRAMEWORK.md        # 框架实现总结
└── QUICKSTART.md                   # 本文档
```

## 下一步

### 迁移现有功能到新框架

1. **将现有工具迁移到新框架**
   - 将 `cmd/tools/` 中的工具适配为 `Tool` 接口
   - 注册到相应的 Specialist Agent

2. **重构现有命令**
   - 将 `analyze`、`chat`、`kubecheck` 迁移到新框架
   - 使用 Coordinator 统一编排

3. **添加新 Agent**
   - Security Agent
   - Cost Optimizer Agent
   - Knowledge Agent

### 开发新功能

查看 [REQUIREMENTS.md](REQUIREMENTS.md) 了解完整的功能规划。

**第二阶段重点**：
- ServiceAccount 和 RBAC 集成
- 沙箱执行环境
- OpenTelemetry 追踪
- Prometheus 指标

**第三阶段重点**：
- Web UI 开发
- Slack/钉钉集成
- Kubernetes Operator
- Helm Chart

## 常见问题

### Q: 如何添加新的 Specialist Agent?

```go
// 1. 定义 Agent 类型（在 types.go）
const AgentTypeMyAgent AgentType = "my_agent"

// 2. 实现 Agent（新建文件）
type MyAgent struct {
    *agent.BaseAgent
}

func NewMyAgent(llmClient agent.LLMClient, logger agent.Logger) *MyAgent {
    config := &agent.AgentConfig{
        Name:        "my_agent",
        Type:        AgentTypeMyAgent,
        Description: "My custom agent",
    }
    return &MyAgent{BaseAgent: agent.NewBaseAgent(config, llmClient, logger)}
}

func (m *MyAgent) CanHandle(taskType agent.TaskType) bool {
    return taskType == agent.TaskTypeMyTask
}

func (m *MyAgent) Execute(ctx *agent.AgentContext, task *agent.Task) (*agent.Task, error) {
    // 实现逻辑
    task.Status = agent.TaskStatusCompleted
    task.Output = map[string]interface{}{
        "result": "My agent result",
    }
    return task, nil
}

// 3. 注册
myAgent := NewMyAgent(llmClient, logger)
coordinator.RegisterAgent(myAgent)
```

### Q: 如何添加工具 (Tool)?

```go
type MyTool struct{}

func (t *MyTool) Name() string {
    return "my_tool"
}

func (t *MyTool) Description() string {
    return "My custom tool"
}

func (t *MyTool) ArgsSchema() string {
    return `{"type": "object", "properties": {"param": {"type": "string"}}}`
}

func (t *MyTool) Execute(params map[string]interface{}) (string, error) {
    param := params["param"].(string)
    return fmt.Sprintf("Executed with: %s", param), nil
}

// 添加到 Agent
myAgent.AddTool(&MyTool{})
```

### Q: 如何切换到 Redis StateStore?

```go
// TODO: 将在第二阶段实现
// redisStore := agent.NewRedisStateStore(redisClient)
// coordinator := agent.NewCoordinator(nil, llmClient, redisStore, logger)
```

### Q: 如何集成现有的 K8s 工具?

参考 `cmd/tools/` 中的现有工具，将它们适配为 `Tool` 接口：

```go
// 原有工具
type LogTool struct {
    Name        string
    Description string
    ArgsSchema  string
}

// 适配为新接口（添加方法）
func (t *LogTool) Execute(params map[string]interface{}) (string, error) {
    podName := params["pod_name"].(string)
    namespace := params["namespace"].(string)
    return t.Run(podName, namespace, "")
}

// 使用
diagnostician.AddTool(tools.NewLogTool())
```

## 资源链接

- [需求文档](REQUIREMENTS.md) - 完整的产品规划
- [框架实现总结](MULTI_AGENT_FRAMEWORK.md) - 技术细节
- [示例文档](KubeAgent/examples/README.md) - 更多示例

## 联系方式

如有问题，请在 GitHub Issues 中提出。
