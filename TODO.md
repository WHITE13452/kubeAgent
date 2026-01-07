# KubeAgent 开发任务列表

> 最后更新: 2026-01-07
> 当前状态: 第一阶段 MVP 已完成 ✅

## ✅ 已完成 (Phase 1 - MVP)

- [x] 设计 Agent 接口和核心数据结构
- [x] 实现 Coordinator Agent 核心框架
- [x] 实现任务分解和 Agent 路由逻辑
- [x] 实现状态存储（内存版本）
- [x] 创建 Specialist Agent 基础框架
- [x] 实现 Diagnostician Agent（诊断专家）
- [x] 实现 Remediator Agent（修复专家）
- [x] 添加 LLM Client 封装
- [x] 添加 Logger 系统
- [x] 编写单元测试（全部通过）
- [x] 编写完整示例和文档
- [x] 添加依赖包（google/uuid）

## 🔥 第二阶段: 安全和可观测性 (2-3周)

### P0 - 核心安全功能

#### 1. ServiceAccount 和 RBAC 集成 (3-4天)
**优先级**: 🔴 最高
**依赖**: 需要 K8s 集群测试环境

- [ ] **为每个 Agent 创建 ServiceAccount**
  - [ ] 创建 YAML 配置模板 (`deploy/rbac/`)
  - [ ] Diagnostician ServiceAccount (只读权限)
    - `get, list, watch` pods, events, nodes
  - [ ] Remediator ServiceAccount (写权限 + 审批)
    - `patch, update` deployments, statefulsets
    - `delete` pods (需要人工确认)
  - [ ] Security ServiceAccount (RBAC 审计权限)
    - 读取 Roles, RoleBindings, ClusterRoles

- [ ] **集成 ServiceAccount 到 Agent**
  - [ ] 修改 `pkg/agent/interface.go` 添加 `K8sClient` 接口
  - [ ] 实现 `pkg/k8s/client.go` 封装 client-go
  - [ ] 为每个 Agent 注入对应的 K8sClient
  - [ ] 实现 `GetK8sClientForAgent(agentType)` 工厂方法

- [ ] **权限审计日志**
  - [ ] 扩展 `StateStore` 添加审计日志保存
  - [ ] 实现 `AuditLogger` 记录所有 K8s API 调用
  - [ ] 添加 `pkg/audit/logger.go` 和数据模型

**验收标准**:
```bash
# 部署到 K8s 后
kubectl get sa -n kubeagent-system
# 应该看到: kubeagent-diagnostician, kubeagent-remediator, kubeagent-security

# 测试权限
kubectl auth can-i get pods --as=system:serviceaccount:kubeagent-system:kubeagent-diagnostician
# 应该返回: yes

kubectl auth can-i delete pods --as=system:serviceaccount:kubeagent-system:kubeagent-diagnostician
# 应该返回: no
```

#### 2. 沙箱执行环境 (2-3天)
**优先级**: 🔴 最高
**依赖**: ServiceAccount 集成完成

- [ ] **实现 Dry-run 模式**
  - [ ] 在 `RemediatorAgent` 中添加 `DryRunPatch()` 方法
  - [ ] 使用 K8s Server-side Dry Run (`--dry-run=server`)
  - [ ] 返回 diff 结果给用户审批

- [ ] **沙箱 Namespace**
  - [ ] 创建独立的 `kubeagent-sandbox` Namespace
  - [ ] 实现 `pkg/sandbox/executor.go`
    - `CloneToSandbox()`: 复制资源到沙箱
    - `TestPatch()`: 在沙箱中应用 Patch
    - `VerifyStability()`: 观察 30 秒验证稳定性
    - `CleanupSandbox()`: 清理沙箱资源

- [ ] **Rollback 机制**
  - [ ] 在 `StateStore` 中保存资源快照
  - [ ] 实现 `RollbackTool` 恢复到上一个版本
  - [ ] 添加自动 Rollback 逻辑（修复失败时）

**验收标准**:
```go
// 测试代码
patch := generatePatch()
result, err := sandbox.TestPatch(patch)
// 应该在沙箱中测试成功，不影响生产环境
```

#### 3. Security Agent 实现 (3-4天)
**优先级**: 🟡 高
**依赖**: RBAC 集成完成

- [ ] **创建 Security Agent**
  - [ ] 实现 `pkg/agent/specialists/security.go`
  - [ ] 支持任务类型: `audit`, `scan`

- [ ] **RBAC 审计功能**
  - [ ] `CheckOverPrivilegedRoles()`: 检测过度授权
  - [ ] `FindUnusedServiceAccounts()`: 发现未使用的 SA
  - [ ] `RecommendMinimalRoles()`: 推荐最小权限 Role

- [ ] **镜像安全扫描 (可选)**
  - [ ] 集成 Trivy API
  - [ ] 实现 `ImageScanTool` 扫描镜像漏洞
  - [ ] 生成安全报告

**文件位置**:
```
pkg/agent/specialists/security.go
pkg/tools/rbac_analyzer.go
pkg/tools/image_scanner.go
```

### P1 - 可观测性

#### 4. OpenTelemetry 分布式追踪 (2-3天)
**优先级**: 🟡 高
**依赖**: 无

- [ ] **集成 OpenTelemetry**
  - [ ] 添加依赖: `go.opentelemetry.io/otel`
  - [ ] 创建 `pkg/telemetry/tracer.go`
  - [ ] 初始化 Jaeger Exporter

- [ ] **在 Agent 中添加追踪**
  - [ ] 修改 `AgentContext` 添加 `span` 字段
  - [ ] 在 `Coordinator.ExecutePlan()` 创建根 Span
  - [ ] 在每个 Agent.Execute() 创建子 Span
  - [ ] 记录 LLM 调用、工具执行的 Span

- [ ] **部署 Jaeger**
  - [ ] 创建 `deploy/jaeger/` 部署文件
  - [ ] Helm Chart 或 K8s YAML

**示例代码位置**:
```go
// pkg/telemetry/tracer.go
func (c *Coordinator) ExecutePlan(ctx *AgentContext, plan *ExecutionPlan) (*Response, error) {
    tracer := otel.Tracer("kubeagent.coordinator")
    ctx.ctx, span := tracer.Start(ctx.Context(), "ExecutePlan")
    defer span.End()

    span.SetAttributes(
        attribute.String("plan_id", plan.ID),
        attribute.Int("task_count", len(plan.Tasks)),
    )
    // ...
}
```

**验收标准**:
- 访问 Jaeger UI (`http://localhost:16686`) 可以看到完整调用链
- 每个任务执行都有对应的 Span

#### 5. Prometheus 指标暴露 (1-2天)
**优先级**: 🟢 中
**依赖**: 无

- [ ] **定义核心指标**
  - [ ] 在 `pkg/metrics/metrics.go` 中定义 Prometheus 指标
    ```go
    var (
        AgentRequestDuration = prometheus.NewHistogramVec(...)
        DiagnosisSuccessRate = prometheus.NewCounterVec(...)
        LLMTokenUsage = prometheus.NewCounterVec(...)
        K8sAPICallsTotal = prometheus.NewCounterVec(...)
    )
    ```

- [ ] **暴露指标端点**
  - [ ] 创建 HTTP Server (`/metrics` 端点)
  - [ ] 在 `main.go` 中启动 metrics server

- [ ] **在 Agent 中收集指标**
  - [ ] 在 `BaseAgent.updateMetrics()` 中记录到 Prometheus

**验收标准**:
```bash
curl http://localhost:9090/metrics | grep kubeagent
# 应该看到:
# kubeagent_request_duration_seconds{agent="diagnostician",action="diagnose"} 1.234
# kubeagent_diagnosis_total{result="success"} 10
```

#### 6. Grafana Dashboard (1天)
**优先级**: 🟢 中
**依赖**: Prometheus 指标

- [ ] **创建 Dashboard JSON**
  - [ ] 在 `deploy/grafana/dashboards/` 创建 `kubeagent-overview.json`
  - [ ] Panel 1: Agent 执行延迟 (P50/P95/P99)
  - [ ] Panel 2: 诊断成功率趋势
  - [ ] Panel 3: LLM Token 消耗
  - [ ] Panel 4: 实时任务数

- [ ] **部署 Grafana**
  - [ ] 添加到 Helm Chart
  - [ ] 配置 Prometheus 数据源

---

## 🚀 第三阶段: 易用性和落地 (2-3周)

### P0 - 核心用户界面

#### 7. Web UI 开发 (5-7天)
**优先级**: 🔴 最高
**技术栈**: React 18 + Ant Design + TypeScript

- [ ] **项目初始化**
  - [ ] 创建 `web-ui/` 目录
  - [ ] `npx create-react-app web-ui --template typescript`
  - [ ] 安装依赖: `antd`, `axios`, `react-router-dom`

- [ ] **后端 API 实现**
  - [ ] 创建 `pkg/api/server.go` (Gin 或 Echo)
  - [ ] 实现 REST API:
    ```
    POST   /api/v1/requests           # 提交用户请求
    GET    /api/v1/requests/:id       # 查询请求状态
    GET    /api/v1/tasks              # 任务列表
    GET    /api/v1/agents             # Agent 状态
    GET    /api/v1/metrics            # 指标摘要
    POST   /api/v1/approvals/:id      # 审批修复方案
    ```

- [ ] **前端页面开发**
  - [ ] Dashboard (首页)
    - 集群健康状态卡片
    - 最近任务列表
    - Agent 活动图表
  - [ ] 任务列表页
    - 所有诊断/修复任务
    - 状态过滤（Pending/Running/Completed/Failed）
    - 详情查看
  - [ ] 交互式诊断页
    - 聊天界面
    - 实时显示 Agent 执行过程
    - 审批修复方案按钮
  - [ ] 资源视图页
    - Pod/Deployment/Node 状态
    - 集成 K8s Dashboard 风格

**文件结构**:
```
web-ui/
├── src/
│   ├── pages/
│   │   ├── Dashboard.tsx
│   │   ├── TaskList.tsx
│   │   ├── Chat.tsx
│   │   └── Resources.tsx
│   ├── components/
│   │   ├── TaskCard.tsx
│   │   ├── AgentStatus.tsx
│   │   └── ApprovalModal.tsx
│   └── api/
│       └── client.ts
pkg/api/
├── server.go
├── handlers.go
└── middleware.go
```

#### 8. Slack/钉钉集成 (2-3天)
**优先级**: 🟡 高

- [ ] **Slack Bot 实现**
  - [ ] 创建 `pkg/integrations/slack/bot.go`
  - [ ] 实现 Slash Commands:
    - `/kubeagent diagnose <pod-name>`
    - `/kubeagent status`
    - `/kubeagent approve <task-id>`
  - [ ] 实现交互式审批按钮 (Block Kit)
  - [ ] 故障告警推送

- [ ] **钉钉 Bot 实现**
  - [ ] 创建 `pkg/integrations/dingtalk/bot.go`
  - [ ] 类似 Slack 功能
  - [ ] 使用钉钉机器人 Webhook

**验收标准**:
```
用户在 Slack 输入: /kubeagent diagnose nginx-pod
Bot 回复:
  🔍 开始诊断 nginx-pod...

  根因: OOMKilled - 容器超过内存限制
  建议: 增加 memory limit 到 512Mi

  是否应用修复? [批准] [拒绝]
```

### P1 - 部署和配置

#### 9. Kubernetes Operator (4-5天)
**优先级**: 🟡 高
**技术**: kubebuilder v3

- [ ] **初始化 Operator 项目**
  - [ ] `kubebuilder init --domain kubeagent.io --repo kubeagent/operator`
  - [ ] 创建 API: `kubebuilder create api --group kubeagent --version v1 --kind DiagnosisTask`

- [ ] **定义 CRD**
  - [ ] `DiagnosisTask` CRD
    ```yaml
    apiVersion: kubeagent.io/v1
    kind: DiagnosisTask
    spec:
      target:
        kind: Pod
        name: nginx-pod
      agents: [diagnostician, remediator]
      autoRemediate: true
      approvalRequired: true
    status:
      phase: Diagnosing
      rootCause: "..."
      remediationPlan: "..."
    ```

- [ ] **实现 Controller**
  - [ ] `controllers/diagnosistask_controller.go`
  - [ ] Reconcile 逻辑调用 Coordinator

- [ ] **测试部署**
  - [ ] `make install` 安装 CRD
  - [ ] `make run` 本地运行
  - [ ] 创建 CR 测试

#### 10. Helm Chart 打包 (1-2天)
**优先级**: 🟢 中

- [ ] **创建 Helm Chart**
  - [ ] `helm create charts/kubeagent`
  - [ ] 定义 Values:
    ```yaml
    coordinator:
      replicas: 1
      image: kubeagent/coordinator:latest

    agents:
      diagnostician:
        enabled: true
      remediator:
        enabled: true
      security:
        enabled: false

    llm:
      provider: qwen
      apiKey: ""
    ```

- [ ] **打包资源**
  - [ ] Deployment, Service
  - [ ] ServiceAccount, RBAC
  - [ ] ConfigMap
  - [ ] Ingress (可选)

**验收标准**:
```bash
helm install kubeagent ./charts/kubeagent \
  --set llm.apiKey=$DASHSCOPE_API_KEY

kubectl get pods -n kubeagent-system
# 应该看到 coordinator 和 agents 运行中
```

#### 11. 配置管理 (Viper) (1天)
**优先级**: 🟢 中

- [ ] **集成 Viper**
  - [ ] 创建 `pkg/config/config.go`
  - [ ] 支持读取:
    - 环境变量
    - 配置文件 (`config.yaml`)
    - K8s ConfigMap

- [ ] **配置热更新**
  - [ ] 监听 ConfigMap 变化
  - [ ] 动态更新 Agent 配置

**配置示例**:
```yaml
# config.yaml
coordinator:
  max_retries: 3
  timeout: 5m

agents:
  diagnostician:
    timeout: 2m
  remediator:
    requires_approval: true

llm:
  provider: qwen
  model: qwen-max
  base_url: https://dashscope.aliyuncs.com/compatible-mode/v1
```

---

## 🌟 第四阶段: 高级特性 (2-3周)

### P1 - 新 Agent 实现

#### 12. Cost Optimizer Agent (3-4天)
**优先级**: 🟡 高

- [ ] **实现 Cost Optimizer**
  - [ ] 创建 `pkg/agent/specialists/cost_optimizer.go`
  - [ ] 任务类型: `optimize`

- [ ] **核心功能**
  - [ ] 分析 Pod 实际 CPU/内存使用率（查询 Prometheus）
  - [ ] 推荐合理的 requests/limits
  - [ ] 识别闲置资源（使用率 < 10%）
  - [ ] 生成成本报告

- [ ] **工具集**
  - [ ] `ResourceAnalyzerTool`: 分析资源使用
  - [ ] `HPARecommenderTool`: HPA 配置推荐
  - [ ] `CostCalculatorTool`: 成本计算

**验收标准**:
```
用户请求: "优化我的集群资源使用"
输出:
  发现 5 个过度配置的 Pod:
  - api-server: 建议 memory 从 2Gi 降到 512Mi (节省 75%)
  - worker: 建议删除（30 天未使用）

  预计月度节省: $120
```

#### 13. Knowledge Agent (2-3天)
**优先级**: 🟢 中

- [ ] **实现 Knowledge Agent**
  - [ ] 创建 `pkg/agent/specialists/knowledge.go`
  - [ ] 任务类型: `query`

- [ ] **向量数据库集成**
  - [ ] 选择: pgvector (PostgreSQL 插件) 或 Pinecone
  - [ ] 创建 `pkg/knowledge/vector_store.go`
  - [ ] 向量化 K8s 文档、Runbook

- [ ] **功能实现**
  - [ ] 文档检索
  - [ ] Runbook 推荐
  - [ ] 最佳实践搜索

#### 14. 多集群管理 (3-4天)
**优先级**: 🟢 中

- [ ] **Multi-Cluster Manager**
  - [ ] 创建 `pkg/cluster/manager.go`
  - [ ] 支持管理多个 K8s 集群

- [ ] **跨集群功能**
  - [ ] 跨集群资源查询
  - [ ] 跨集群诊断
  - [ ] 多集群故障关联分析

**配置示例**:
```yaml
clusters:
  - name: dev
    kubeconfig: /path/to/dev-kubeconfig
  - name: staging
    kubeconfig: /path/to/staging-kubeconfig
  - name: production
    kubeconfig: /path/to/prod-kubeconfig
```

### P2 - 高级集成

#### 15. GitOps 集成 (2-3天)
**优先级**: 🟡 高

- [ ] **实现 GitOps Client**
  - [ ] 创建 `pkg/gitops/client.go`
  - [ ] 支持 GitHub/GitLab API

- [ ] **核心功能**
  - [ ] 生成修复 Patch
  - [ ] 自动创建 PR
  - [ ] 生成 PR Description (包含诊断报告)
  - [ ] 等待审批后合并
  - [ ] 触发 ArgoCD/Flux 部署

**工作流**:
```
诊断 → 生成 Patch → 创建 PR → 人工审批 → 合并 → ArgoCD 部署 → 验证
```

#### 16. 预设 Runbook 库 (2天)
**优先级**: 🟢 中

- [ ] **定义 Runbook CRD**
  - [ ] 创建 `api/v1/runbook_types.go`
  - [ ] 支持 YAML 定义 Runbook

- [ ] **实现常见 Runbook**
  - [ ] OOMKilled 修复
  - [ ] CrashLoopBackOff 诊断
  - [ ] ImagePullBackOff 修复
  - [ ] Node NotReady 处理

**Runbook 示例**:
```yaml
apiVersion: kubeagent.io/v1
kind: Runbook
metadata:
  name: oomkilled-remediation
spec:
  trigger:
    errorType: OOMKilled
  steps:
  - name: check-memory-usage
    action: query_metrics
  - name: increase-memory-limit
    action: patch_resource
    patch: |
      spec:
        containers:
        - name: app
          resources:
            limits:
              memory: "{{.NewMemoryLimit}}"
```

---

## 🔧 技术债务和优化 (持续进行)

### 代码质量

#### 17. 工具迁移到新框架 (1-2天)
**优先级**: 🟡 高

- [ ] **适配现有工具**
  - [ ] 将 `cmd/tools/logTool.go` 适配为 `Tool` 接口
  - [ ] 将 `cmd/tools/eventTool.go` 适配
  - [ ] 将 `cmd/tools/createTool.go` 适配
  - [ ] 其他工具...

- [ ] **注册到 Agent**
  - [ ] Diagnostician 添加: LogTool, EventTool, MetricQueryTool
  - [ ] Remediator 添加: PatchGeneratorTool, GitOpsTool

#### 18. 重构现有命令 (2-3天)
**优先级**: 🟢 中

- [ ] **迁移 CLI 命令到新框架**
  - [ ] 重构 `cmd/analyze.go` 使用 Coordinator
  - [ ] 重构 `cmd/chat.go` 使用 Coordinator
  - [ ] 重构 `cmd/kubecheck.go` 使用 Coordinator

- [ ] **保持向后兼容**
  - [ ] 保留旧命令参数
  - [ ] 添加 `--use-new-framework` 标志

#### 19. LLM 输出稳定性改进 (1天)
**优先级**: 🟡 高

- [ ] **JSON Schema 验证**
  - [ ] 在 Prompt 中添加 JSON Schema
  - [ ] 使用 `github.com/xeipuuv/gojsonschema` 验证输出
  - [ ] 失败时自动重试（最多 3 次）

- [ ] **Structured Output**
  - [ ] 使用 Function Calling 模式
  - [ ] 定义明确的输出格式

#### 20. Redis StateStore 实现 (1-2天)
**优先级**: 🟢 中

- [ ] **实现 Redis 版本**
  - [ ] 创建 `pkg/agent/redis_state_store.go`
  - [ ] 实现 `StateStore` 接口
  - [ ] 使用 `go-redis/redis`

- [ ] **数据序列化**
  - [ ] 使用 JSON 或 MessagePack
  - [ ] 设置合理的 TTL

#### 21. 错误处理和重试机制 (1天)
**优先级**: 🟢 中

- [ ] **统一错误类型**
  - [ ] 定义错误码
  - [ ] 创建 `pkg/errors/types.go`

- [ ] **重试策略**
  - [ ] 实现指数退避重试
  - [ ] 使用 `github.com/cenkalti/backoff`

- [ ] **Circuit Breaker**
  - [ ] 防止雪崩
  - [ ] 使用 `github.com/sony/gobreaker`

### 性能优化

#### 22. LLM 调用优化 (1天)
**优先级**: 🟢 中

- [ ] **缓存相似请求**
  - [ ] 使用向量相似度
  - [ ] 缓存到 Redis

- [ ] **使用小模型处理简单任务**
  - [ ] 简单查询: qwen-turbo
  - [ ] 复杂诊断: qwen-max

- [ ] **并行调用**
  - [ ] 多个独立 Agent 并行调用 LLM

#### 23. 增加测试覆盖率 (持续)
**优先级**: 🟢 中

- [ ] **单元测试**
  - [ ] 目标: 80% 覆盖率
  - [ ] 为每个 Agent 添加测试

- [ ] **集成测试**
  - [ ] 端到端测试场景
  - [ ] 使用 Kind (Kubernetes in Docker) 创建测试集群

- [ ] **性能测试**
  - [ ] 使用 `go test -bench`
  - [ ] 压力测试 Coordinator

---

## 📊 项目管理

### 里程碑

**M1: Phase 1 完成 ✅** (已完成)
- Multi-Agent 框架
- Diagnostician & Remediator
- 示例和测试

**M2: Phase 2 完成** (预计 2-3 周)
- ServiceAccount & RBAC
- Security Agent
- OpenTelemetry & Prometheus

**M3: Phase 3 完成** (预计 4-6 周)
- Web UI
- Slack/钉钉集成
- Kubernetes Operator
- Helm Chart

**M4: Phase 4 完成** (预计 8-10 周)
- Cost Optimizer & Knowledge Agent
- GitOps 集成
- 多集群管理
- 生产就绪

### 工作量估算

| 阶段 | 任务数 | 预计时间 | 优先级分布 |
|------|--------|----------|------------|
| Phase 2 | 6 个任务 | 2-3 周 | P0: 3, P1: 3 |
| Phase 3 | 5 个任务 | 2-3 周 | P0: 2, P1: 3 |
| Phase 4 | 5 个任务 | 2-3 周 | P1: 2, P2: 3 |
| 技术债务 | 7 个任务 | 持续进行 | P0: 0, P1: 3, P2: 4 |

**总计**: 23 个主要任务，预计 8-10 周完成全部功能

---

## 🚦 下次开发从这里开始

### 立即可以开始的任务 (按优先级)

1. **ServiceAccount 和 RBAC 集成** (P0, 3-4 天)
   - 从这里开始最合适
   - 是后续所有功能的基础
   - 需要 K8s 集群环境

2. **沙箱执行环境** (P0, 2-3 天)
   - 依赖 ServiceAccount
   - 安全关键功能

3. **Security Agent** (P1, 3-4 天)
   - 面试重点
   - 展示技术深度

4. **OpenTelemetry 追踪** (P1, 2-3 天)
   - 独立任务，无依赖
   - 可以并行开发

5. **Web UI** (P0, 5-7 天)
   - 用户体验关键
   - 需要前端技能

### 开发环境准备

#### 需要的工具
```bash
# K8s 集群 (选择一个)
kind create cluster --name kubeagent-dev
# 或
minikube start

# Jaeger (用于追踪)
kubectl create namespace observability
kubectl apply -f https://raw.githubusercontent.com/jaegertracing/jaeger-operator/main/deploy/crds/jaegertracing.io_jaegers_crd.yaml

# Prometheus (用于指标)
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install prometheus prometheus-community/kube-prometheus-stack -n observability
```

#### 环境变量
```bash
export DASHSCOPE_API_KEY="your-key"
export KUBECONFIG="$HOME/.kube/config"
```

### 推荐的开发顺序

**Week 1-2: 安全基础**
- Day 1-4: ServiceAccount & RBAC
- Day 5-7: 沙箱执行
- Day 8-10: Security Agent

**Week 3-4: 可观测性**
- Day 1-2: OpenTelemetry
- Day 3-4: Prometheus 指标
- Day 5: Grafana Dashboard

**Week 5-7: 用户界面**
- Day 1-7: Web UI 开发
- Day 8-10: Slack/钉钉集成

**Week 8-10: 部署和高级特性**
- Day 1-5: Kubernetes Operator
- Day 6-7: Helm Chart
- Day 8-10: Cost Optimizer Agent

---

## 📝 笔记

### 当前技术栈
- **语言**: Go 1.24.3
- **LLM**: 通义千问 (qwen-max)
- **CLI**: Cobra
- **K8s**: client-go v0.33.1
- **测试**: Go testing (100% 通过)

### 已知问题
1. LLM JSON 输出不稳定 → 需要 Schema 验证
2. StateStore 只有内存版 → 需要 Redis 实现
3. 缺少真实的 K8s 工具集成 → 需要适配现有工具
4. 没有速率限制 → 需要添加 Circuit Breaker

### 有用的命令
```bash
# 运行测试
go test ./pkg/agent/... -v

# 运行示例
go run examples/multi_agent_demo.go

# 检查代码覆盖率
go test ./pkg/agent/... -cover

# 构建
go build -o kubeagent main.go

# 格式化
go fmt ./...
```

---

**最后更新**: 2026-01-07
**下次更新**: 完成 Phase 2 任务后更新此文档
