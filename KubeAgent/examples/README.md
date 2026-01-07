# KubeAgent Multi-Agent Framework Examples

本目录包含 KubeAgent 多 Agent 框架的使用示例。

## 运行示例

### 前置条件

1. 设置 DashScope API Key 环境变量：
```bash
export DASHSCOPE_API_KEY="your-api-key-here"
```

2. 安装依赖：
```bash
cd /Users/I765226/develop/go-workspace/kubeAgent/KubeAgent
go mod tidy
```

### 运行 Multi-Agent Demo

```bash
cd /Users/I765226/develop/go-workspace/kubeAgent/KubeAgent
go run examples/multi_agent_demo.go
```

## 示例说明

### Example 1: Simple Diagnosis Task (简单诊断任务)

演示如何使用 Diagnostician Agent 诊断单个 Pod 故障：

```go
task := &agent.Task{
    Type:          agent.TaskTypeDiagnose,
    Description:   "nginx pod is in CrashLoopBackOff state",
    AssignedAgent: agent.AgentTypeDiagnostician,
    Input: map[string]interface{}{
        "pod_name":  "nginx-deployment-7d5c8b9f4d-x8k2l",
        "namespace": "production",
    },
}

result, err := coordinator.Execute(ctx, task)
```

**输出**：
- 根本原因分析
- 错误类型分类
- 修复建议
- 置信度评分

### Example 2: Diagnosis + Remediation Workflow (诊断+修复工作流)

演示完整的诊断和修复流程：

1. **诊断阶段**: Diagnostician Agent 分析问题
2. **修复阶段**: Remediator Agent 生成修复方案

```go
// Step 1: Diagnosis
diagnosisResult, _ := coordinator.Execute(ctx, diagnosisTask)

// Step 2: Remediation (使用诊断结果)
remediationTask.Input = map[string]interface{}{
    "diagnosis":  diagnosisResult.Output,
    "root_cause": diagnosisResult.Output["root_cause"],
}
remediationResult, _ := coordinator.Execute(ctx, remediationTask)
```

**输出**：
- 诊断报告
- 修复类型 (patch/config_change/restart)
- 风险等级 (low/medium/high)
- 是否需要人工审批

### Example 3: Full Request with Automatic Planning (自动规划执行)

演示 Coordinator Agent 如何自动分解任务并编排执行：

```go
request := &agent.Request{
    Input: "My nginx pod keeps restarting. Can you diagnose and fix it?",
}

// Coordinator 自动规划
plan, _ := coordinator.Plan(ctx, request)

// 执行计划
response, _ := coordinator.ExecutePlan(ctx, plan)
```

**核心功能**：
- **意图识别**: LLM 解析用户请求，识别意图
- **任务分解**: 自动分解为诊断、修复等子任务
- **Agent 路由**: 根据任务类型选择合适的 Agent
- **执行编排**: 支持串行、并行、条件分支执行
- **结果聚合**: LLM 生成最终用户友好的响应

## 架构说明

### Coordinator Agent (协调器)

负责：
- 解析用户请求意图
- 分解复杂任务
- 路由到合适的 Specialist Agent
- 编排执行顺序 (串行/并行)
- 聚合结果并生成最终响应

### Specialist Agents (专家 Agent)

#### Diagnostician Agent (诊断专家)
- **任务类型**: `diagnose`, `query`
- **能力**:
  - 分析 Pod 日志和事件
  - 识别错误类型 (OOMKilled, CrashLoopBackOff, ImagePullBackOff)
  - 根因分析
  - 生成诊断报告

#### Remediator Agent (修复专家)
- **任务类型**: `remediate`
- **能力**:
  - 生成 Kubernetes Patch
  - 提供修复建议
  - 评估风险等级
  - 判断是否需要人工审批

### 执行模式

1. **Sequential (串行)**: 任务按顺序执行 (适用于有依赖关系的任务)
2. **Parallel (并行)**: 任务并发执行 (适用于独立任务)
3. **Conditional (条件)**: 根据前一个任务的结果决定下一步 (TODO)

## 扩展框架

### 添加新的 Specialist Agent

1. 创建新的 Agent 类型：
```go
const AgentTypeMyAgent AgentType = "my_agent"
```

2. 实现 Agent 接口：
```go
type MyAgent struct {
    *agent.BaseAgent
}

func (m *MyAgent) CanHandle(taskType agent.TaskType) bool {
    return taskType == agent.TaskTypeMyTask
}

func (m *MyAgent) Execute(ctx *agent.AgentContext, task *agent.Task) (*agent.Task, error) {
    // 实现你的逻辑
}
```

3. 注册到 Coordinator：
```go
myAgent := NewMyAgent(llmClient, logger)
coordinator.RegisterAgent(myAgent)
```

### 添加工具 (Tools)

```go
type MyTool struct {
    name        string
    description string
    argsSchema  string
}

func (t *MyTool) Name() string { return t.name }
func (t *MyTool) Description() string { return t.description }
func (t *MyTool) ArgsSchema() string { return t.argsSchema }

func (t *MyTool) Execute(params map[string]interface{}) (string, error) {
    // 实现工具逻辑
    return "Tool result", nil
}

// 添加到 Agent
myAgent.AddTool(myTool)
```

## 下一步计划

- [ ] 添加 Security Agent (安全审计)
- [ ] 添加 Cost Optimizer Agent (成本优化)
- [ ] 添加 Knowledge Agent (知识检索)
- [ ] 集成 Tool Registry (工具注册中心)
- [ ] 实现 Redis StateStore (持久化状态)
- [ ] 添加 OpenTelemetry 追踪
- [ ] 集成 Prometheus 指标
- [ ] Web UI 界面

## 参考文档

- [需求文档](../REQUIREMENTS.md)
- [架构设计](../docs/ARCHITECTURE.md) (TODO)
- [API 文档](../docs/API.md) (TODO)
