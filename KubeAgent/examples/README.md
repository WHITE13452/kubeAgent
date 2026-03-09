# KubeAgent Multi-Agent Framework Examples

本目录包含 KubeAgent 多 Agent 框架的使用示例。

## 运行示例

### 前置条件

1. 设置环境变量：
```bash
export DASHSCOPE_API_KEY="your-api-key-here"
```

2. 确保有可用的 Kubernetes 集群访问（kubeconfig 或 InCluster）。

3. 安装依赖：
```bash
cd KubeAgent
go mod tidy
```

### 运行 Multi-Agent Demo

```bash
cd KubeAgent
go run examples/multi_agent_demo.go
```

## 示例说明

### Example 1: Simple Diagnosis Task

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

输出：
- 根本原因分析
- 错误类型分类
- 修复建议
- 置信度评分

### Example 2: Diagnosis + Remediation Workflow

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

输出：
- 诊断报告
- 修复类型 (patch/config_change/restart)
- 风险等级 (low/medium/high)
- 是否需要人工审批

### Example 3: Full Request with Automatic Planning

演示 Coordinator Agent 如何自动分解任务并编排执行：

```go
request := &agent.Request{
    Input: "My nginx pod keeps restarting. Can you diagnose and fix it?",
}

plan, _ := coordinator.Plan(ctx, request)
response, _ := coordinator.ExecutePlan(ctx, plan)
```

核心功能：
- **意图识别**: LLM 解析用户请求，识别意图
- **任务分解**: 自动分解为诊断、修复等子任务
- **Agent 路由**: 根据任务类型选择合适的 Agent
- **DAG 执行**: 基于依赖图的并行/串行执行
- **结果聚合**: LLM 生成最终用户友好的响应

## 扩展框架

### 添加新的 Specialist Agent

1. 在 `pkg/agent/types.go` 中定义新的 Agent 类型
2. 实现 Agent 接口
3. 注册到 Coordinator

### 添加工具 (Tools)

实现 `Tool` 接口（Name, Description, ArgsSchema, Execute），然后通过 `agent.AddTool()` 注册到对应的 Agent。

如果工具需要访问 K8s API，构造函数接收 `*k8s.Client` 参数：

```go
func NewMyTool(client *k8s.Client) *MyTool {
    return &MyTool{client: client}
}
```
