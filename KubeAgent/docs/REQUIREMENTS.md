# KubeAgent 产品级改造需求文档

## 一、项目背景与定位

### 1.1 当前项目现状
- **架构**: 单一 Agent + 基础工具集 + ginK8s API 后端
- **能力**: 3 种模式（analyze、chat、kubecheck），9 个工具
- **问题**:
  - 缺少多 Agent 协作能力
  - 无 K8s RBAC 和安全机制
  - 硬编码配置，难以扩展
  - 缺少可观测性和审计
  - 无自动化修复能力

### 1.2 目标定位
将 KubeAgent 从 **PoC 级别**升级为**企业级 K8s 智能运维平台**，对标 K8sGPT、Kagent 等 CNCF 项目，具备：
- 多 Agent 协同决策
- 企业级安全和鉴权
- 自动化诊断和修复
- 生产级可观测性
- 真正可落地使用

## 二、核心痛点分析（基于行业调研）

### 2.1 K8s 运维的核心痛点

| 痛点 | 当前手动处理 | KubeAgent 应解决方案 |
|------|--------------|---------------------|
| **故障诊断效率低** | 人工排查日志、事件、指标 | AI Agent 自动分析根因 |
| **YAML 配置复杂** | 手写 YAML 易出错 | 自然语言生成配置 + 安全校验 |
| **跨团队权限管理** | 分散的 RBAC 配置 | 统一权限中心 + Agent 权限审计 |
| **故障修复依赖专家** | 等待 SRE 介入 | 自动修复建议 + GitOps 流程 |
| **成本优化无感知** | 手动分析资源使用 | 智能资源推荐 + 成本预测 |
| **多集群管理混乱** | 切换 kubeconfig 操作多个集群 | 统一视图 + 跨集群编排 |
| **安全隐患检测滞后** | 定期扫描 + 人工审查 | 实时安全检测 + 自动加固 |

### 2.2 技术深度要求（面试官关注点）

**多 Agent 调度**：
- 不同专业领域的 Agent（诊断、修复、安全、成本）协同工作
- 任务分解和 Agent 选择策略
- Agent 间通信和状态共享

**K8s 安全和鉴权**：
- Service Account 和 RBAC 集成
- 危险操作的沙箱隔离
- 审计日志和操作回溯
- 最小权限原则实施

**生产落地**：
- 与现有工具链集成（GitOps、监控、告警）
- 分布式追踪和可观测性
- 故障恢复和降级策略
- 性能和成本优化

## 三、技术架构设计

### 3.1 多 Agent 架构（核心亮点）

```
┌─────────────────────────────────────────────────────────┐
│                   用户接口层                              │
│   CLI / Web UI / Slack Bot / API Gateway                │
└────────────────────┬────────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────────┐
│              协调 Agent (Coordinator)                    │
│  - 任务分解与路由                                         │
│  - Agent 选择与调度                                       │
│  - 执行计划生成                                          │
│  - 冲突检测与解决                                         │
└────┬────────┬────────┬────────┬────────┬────────────────┘
     │        │        │        │        │
     ▼        ▼        ▼        ▼        ▼
┌─────────┬─────────┬─────────┬─────────┬─────────────┐
│ 诊断     │ 修复     │ 安全     │ 成本     │ 知识        │
│ Agent   │ Agent   │ Agent   │ Agent   │ Agent      │
│         │         │         │         │            │
│ - 日志   │ - 自动   │ - RBAC  │ - 资源   │ - 文档检索 │
│ - 事件   │   修复   │   审计  │   优化   │ - 最佳实践 │
│ - 指标   │ - GitOps│ - 镜像   │ - 成本   │ - Runbook │
│ - 追踪   │   集成  │   扫描  │   预测   │   推荐    │
└────┬────┴────┬────┴────┬────┴────┬────┴─────┬──────┘
     │         │         │         │          │
     └─────────┴─────────┴─────────┴──────────┘
                         │
              ┌──────────▼──────────┐
              │   共享能力层         │
              │ - Agent 通信总线    │
              │ - 状态存储（Redis） │
              │ - 工具池管理        │
              │ - 追踪与日志        │
              └──────────┬──────────┘
                         │
              ┌──────────▼──────────┐
              │   K8s 抽象层        │
              │ - Client-go         │
              │ - Dynamic Client    │
              │ - Informer Cache    │
              │ - Operator SDK      │
              └──────────┬──────────┘
                         │
              ┌──────────▼──────────┐
              │  Kubernetes 集群    │
              └─────────────────────┘
```

### 3.2 Agent 协作模式

采用 **Hierarchical Multi-Agent System (层级式多 Agent 系统)**：

**1. Coordinator Agent（协调者）**
- **职责**: 任务理解、Agent 路由、执行编排
- **实现**: LangGraph 状态图
- **决策流程**:
  ```
  用户请求 → 意图识别 → 任务分解 → Agent 选择 →
  执行计划 → 并行/串行调度 → 结果聚合 → 返回用户
  ```

**2. Specialist Agents（专家 Agent）**
- **Diagnostician Agent**: 故障诊断（日志、事件、指标、追踪）
- **Remediator Agent**: 自动修复（Patch 生成、GitOps 提交、Rollback）
- **Security Agent**: 安全审计（RBAC 检查、镜像扫描、合规性）
- **Cost Optimizer Agent**: 成本优化（资源推荐、水平扩展建议）
- **Knowledge Agent**: 知识检索（文档搜索、Runbook、最佳实践）

**3. 协作模式**
- **Sequential（顺序）**: 诊断 → 修复 → 验证
- **Parallel（并行）**: 同时检查多个资源的健康状态
- **Iterative（迭代）**: 修复失败后重新诊断
- **Consensus（共识）**: 多个 Agent 投票决策（如删除生产资源）

### 3.3 技术栈升级

| 模块 | 当前技术 | 升级方案 | 理由 |
|------|----------|----------|------|
| **Agent 框架** | 手写 ReAct | **LangGraph** | 状态管理、持久化、断点续执 |
| **多 Agent 协作** | 无 | **CrewAI 模式** | 角色分工、任务编排 |
| **配置管理** | 硬编码 | **Viper + ConfigMap** | 动态配置、热更新 |
| **认证授权** | 无 | **K8s RBAC + OIDC** | 企业级身份集成 |
| **沙箱执行** | 无 | **gVisor + Pod Sandbox** | 隔离执行危险操作 |
| **可观测性** | 无 | **Prometheus + Jaeger + Grafana** | 分布式追踪、指标监控 |
| **状态存储** | 内存 | **Redis + PostgreSQL** | 持久化、多实例共享 |
| **API 网关** | 直接调用 ginK8s | **Envoy/Nginx** | 鉴权、限流、路由 |
| **部署方式** | 二进制 | **Kubernetes Operator** | 云原生部署、自愈 |

## 四、详细功能需求

### 4.1 多 Agent 调度系统（P0 - 核心亮点）

#### 4.1.1 Coordinator Agent

**功能需求**:
- **任务理解**: 使用 LLM 解析用户自然语言意图
- **任务分解**: 将复杂任务拆解为子任务（DAG 图）
- **Agent 路由**: 根据任务类型选择合适的 Specialist Agent
- **执行编排**: 支持串行、并行、条件分支、循环
- **冲突检测**: 检测多个操作是否冲突（如同时扩容和缩容）
- **状态管理**: 追踪任务执行状态、支持断点续执

**技术实现**:
```go
// Coordinator 使用 LangGraph 状态图
type CoordinatorAgent struct {
    graph        *langgraph.StateGraph
    specialists  map[AgentType]Agent
    taskQueue    *redis.Queue
    stateStore   StateStore
}

// 状态节点
- ParseIntent: 意图识别
- DecomposeTask: 任务分解
- SelectAgents: Agent 选择
- ExecutePlan: 执行编排
- AggregateResults: 结果聚合
- HandleError: 错误处理
```

**案例**:
```
用户: "我的 nginx pod 一直重启，帮我诊断并修复"

Coordinator 决策流程:
1. ParseIntent: 识别为 "故障诊断 + 自动修复"
2. DecomposeTask:
   - 子任务1: 诊断 Pod 故障原因
   - 子任务2: 生成修复方案
   - 子任务3: 执行修复
   - 子任务4: 验证修复结果
3. SelectAgents:
   - 子任务1 → Diagnostician Agent
   - 子任务2 → Remediator Agent
   - 子任务3 → Remediator Agent（需要用户确认）
   - 子任务4 → Diagnostician Agent
4. ExecutePlan: 串行执行（诊断 → 修复 → 验证）
5. AggregateResults: 生成诊断报告和修复记录
```

#### 4.1.2 Diagnostician Agent（诊断专家）

**功能需求**:
- **多维度数据收集**:
  - Pod 日志（stdout/stderr，容器日志）
  - K8s 事件（Warning、Error）
  - 指标（CPU、内存、网络、磁盘）
  - 分布式追踪（如 Jaeger traces）
  - 节点状态（Kubelet 日志、Node Conditions）

- **根因分析**:
  - OOMKilled → 内存不足
  - CrashLoopBackOff → 启动命令错误 / 健康检查失败
  - ImagePullBackOff → 镜像拉取失败（认证、网络、镜像不存在）
  - Pending → 资源不足 / 亲和性规则不满足 / PV 绑定失败

- **智能诊断**:
  - 使用 LLM 分析日志和事件，提取关键错误信息
  - 与历史故障库对比，查找相似案例
  - 生成结构化诊断报告（原因、影响范围、修复建议）

**工具集**:
- `PodLogTool`: 获取 Pod 日志（支持多容器、previous logs）
- `EventAnalyzerTool`: 分析 K8s 事件
- `MetricQueryTool`: 查询 Prometheus 指标
- `TracingTool`: 查询 Jaeger/Tempo 追踪
- `ResourceInspectorTool`: 检查资源配置（limits/requests、健康检查）

**技术实现**:
```go
type DiagnosticianAgent struct {
    llm          LLMClient
    tools        []Tool
    knowledgeDB  KnowledgeBase  // 历史故障库
    prometheus   PromClient
    jaeger       JaegerClient
}

// 诊断流程
func (a *DiagnosticianAgent) Diagnose(ctx context.Context, pod PodInfo) (*DiagnosisReport, error) {
    // 1. 收集数据
    logs := a.tools.PodLogTool.GetLogs(pod)
    events := a.tools.EventAnalyzerTool.GetEvents(pod)
    metrics := a.tools.MetricQueryTool.GetMetrics(pod)

    // 2. LLM 分析
    analysis := a.llm.Analyze(logs, events, metrics)

    // 3. 知识库匹配
    similarCases := a.knowledgeDB.FindSimilar(analysis)

    // 4. 生成报告
    return &DiagnosisReport{
        RootCause:      analysis.RootCause,
        ImpactScope:    analysis.ImpactScope,
        Recommendations: analysis.Recommendations,
        SimilarCases:   similarCases,
        Confidence:     analysis.Confidence,
    }
}
```

#### 4.1.3 Remediator Agent（修复专家）

**功能需求**:
- **自动修复策略**:
  - OOMKilled → 增加内存 limits
  - ImagePullBackOff → 修复镜像地址 / 添加 ImagePullSecrets
  - 配置错误 → 生成正确的 ConfigMap/Secret
  - 健康检查失败 → 调整 livenessProbe/readinessProbe 参数

- **GitOps 集成**:
  - 生成修复 Patch（JSON Patch/Strategic Merge Patch）
  - 提交 PR 到 GitOps 仓库（Flux/ArgoCD）
  - 等待审批后自动合并
  - 触发重新部署

- **安全保护**:
  - 危险操作需要人工确认（删除资源、修改生产配置）
  - 沙箱执行（在测试环境先验证修复方案）
  - 自动 Rollback（修复失败后恢复原状态）

- **修复验证**:
  - 应用修复后监控 Pod 状态
  - 验证健康检查通过
  - 确认没有新错误

**工具集**:
- `PatchGeneratorTool`: 生成 K8s Patch
- `GitOpsTool`: 提交 PR 到 GitOps 仓库
- `SandboxTool`: 在隔离环境测试修复
- `RollbackTool`: 回滚到上一个稳定版本
- `VerificationTool`: 验证修复结果

**技术实现**:
```go
type RemediatorAgent struct {
    llm         LLMClient
    k8sClient   *kubernetes.Clientset
    gitClient   GitOpsClient
    sandbox     SandboxExecutor
}

// 修复流程（带 GitOps）
func (a *RemediatorAgent) Remediate(ctx context.Context, diagnosis *DiagnosisReport) error {
    // 1. 生成修复 Patch
    patch := a.generatePatch(diagnosis)

    // 2. 沙箱验证（可选）
    if diagnosis.Risk == "high" {
        if err := a.sandbox.TestPatch(patch); err != nil {
            return fmt.Errorf("sandbox test failed: %w", err)
        }
    }

    // 3. 提交 GitOps PR
    pr := a.gitClient.CreatePR(patch, diagnosis)

    // 4. 等待审批（危险操作）
    if diagnosis.RequiresApproval {
        a.waitForApproval(pr)
    }

    // 5. 应用修复
    if err := a.k8sClient.Patch(patch); err != nil {
        a.rollback(patch)
        return err
    }

    // 6. 验证修复
    return a.verifyRemediation(ctx, diagnosis.Pod)
}
```

#### 4.1.4 Security Agent（安全专家）

**功能需求**:
- **RBAC 审计**:
  - 检测过度授权（ClusterAdmin 滥用）
  - 发现未使用的 ServiceAccount
  - 推荐最小权限 Role

- **镜像安全扫描**:
  - 集成 Trivy/Grype 扫描镜像漏洞
  - 检测过期镜像和未签名镜像
  - 推荐安全基础镜像

- **合规性检查**:
  - PodSecurityStandard 合规性（restricted/baseline）
  - NetworkPolicy 是否配置
  - Secret 加密状态检查

- **实时监控**:
  - 检测异常 API 调用（如深夜创建特权容器）
  - 监控 Secret 访问模式

**工具集**:
- `RBACAnalyzerTool`: RBAC 权限分析
- `ImageScannerTool`: 镜像漏洞扫描
- `ComplianceCheckerTool`: 合规性检查
- `AuditLogAnalyzerTool`: 审计日志分析

#### 4.1.5 Cost Optimizer Agent（成本优化专家）

**功能需求**:
- **资源优化建议**:
  - 分析 Pod 实际 CPU/内存使用率
  - 推荐合理的 requests/limits
  - 识别闲置资源（低利用率的 Pod）

- **自动扩缩容**:
  - 基于历史数据预测流量模式
  - 推荐 HPA 配置
  - 检测过度扩容

- **成本预测**:
  - 计算集群月度成本
  - 预测变更后的成本影响
  - 生成成本报告

**工具集**:
- `ResourceAnalyzerTool`: 资源使用分析
- `HPARecommenderTool`: HPA 配置推荐
- `CostCalculatorTool`: 成本计算

#### 4.1.6 Knowledge Agent（知识专家）

**功能需求**:
- **文档检索**:
  - 向量化 K8s 官方文档
  - 搜索相关解决方案

- **Runbook 推荐**:
  - 维护常见故障的 Runbook 库
  - 根据诊断结果推荐 Runbook

- **最佳实践**:
  - 网络搜索最新的 K8s 最佳实践
  - 集成 Kubernetes Patterns 知识库

**工具集**:
- `VectorSearchTool`: 向量检索
- `RunbookMatcherTool`: Runbook 匹配
- `WebSearchTool`: 网络搜索（Tavily）

### 4.2 安全和鉴权系统（P0 - 面试重点）

#### 4.2.1 K8s RBAC 集成

**功能需求**:
- **多租户支持**:
  - 每个团队有独立的 Namespace
  - KubeAgent 使用团队特定的 ServiceAccount
  - 根据用户身份自动选择 ServiceAccount

- **最小权限原则**:
  - 为每个 Agent 创建专用 ServiceAccount
  - Diagnostician: 只读权限（get, list, watch）
  - Remediator: 写权限（patch, update）但需审批
  - Security Agent: RBAC 读取权限

- **权限审计**:
  - 记录每个 Agent 的 K8s API 调用
  - 检测权限提升尝试
  - 定期审查 ServiceAccount 权限

**技术实现**:
```yaml
# Diagnostician ServiceAccount（只读）
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kubeagent-diagnostician
  namespace: kubeagent-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kubeagent-diagnostician
rules:
- apiGroups: [""]
  resources: ["pods", "pods/log", "events"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["get", "list"]
---
# Remediator ServiceAccount（需审批的写权限）
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kubeagent-remediator
  namespace: kubeagent-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kubeagent-remediator
rules:
- apiGroups: ["apps"]
  resources: ["deployments", "statefulsets"]
  verbs: ["get", "list", "patch", "update"]
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["delete"]  # 需要人工确认
```

#### 4.2.2 沙箱执行环境

**功能需求**:
- **隔离执行危险操作**:
  - 使用 gVisor 或 Kata Containers 隔离
  - 在独立 Namespace 中测试修复
  - 限制网络访问（NetworkPolicy）

- **Dry-run 模式**:
  - K8s Server-side Dry Run（`kubectl apply --dry-run=server`）
  - 预览变更内容（diff）
  - 验证 YAML 合法性

- **快照和回滚**:
  - 执行操作前保存资源快照
  - 失败时自动回滚

**技术实现**:
```go
type SandboxExecutor struct {
    k8sClient   *kubernetes.Clientset
    sandboxNS   string  // 专用沙箱 Namespace
}

func (s *SandboxExecutor) TestPatch(patch *Patch) error {
    // 1. 在沙箱 NS 中创建资源副本
    testResource := s.cloneToSandbox(patch.Resource)

    // 2. 应用 Patch（Dry Run）
    result, err := s.k8sClient.Patch(
        context.TODO(),
        testResource,
        patch.Data,
        metav1.PatchOptions{DryRun: []string{"All"}},
    )

    // 3. 验证结果
    if err != nil {
        return fmt.Errorf("dry run failed: %w", err)
    }

    // 4. 真实应用到沙箱（可选）
    if patch.RequiresRealTest {
        s.applyToSandbox(patch)
        time.Sleep(30 * time.Second)  // 观察稳定性
        s.cleanupSandbox()
    }

    return nil
}
```

#### 4.2.3 审计日志系统

**功能需求**:
- **操作记录**:
  - 记录所有 Agent 决策和执行
  - 包含用户身份、时间戳、操作类型、资源信息

- **可追溯性**:
  - 支持按用户、时间、资源类型查询
  - 导出审计日志到外部系统（Elasticsearch）

- **告警**:
  - 危险操作立即告警（删除生产资源）
  - 失败操作聚合告警

**数据模型**:
```go
type AuditLog struct {
    ID            string    `json:"id"`
    Timestamp     time.Time `json:"timestamp"`
    User          string    `json:"user"`           // 用户身份
    Agent         string    `json:"agent"`          // Agent 名称
    Action        string    `json:"action"`         // 操作类型（diagnose, remediate, delete）
    Resource      string    `json:"resource"`       // 资源类型和名称
    Namespace     string    `json:"namespace"`
    Details       string    `json:"details"`        // 详细信息
    Result        string    `json:"result"`         // success/failure
    Error         string    `json:"error,omitempty"`
    ApprovedBy    string    `json:"approved_by,omitempty"` // 审批人
}
```

### 4.3 自动化诊断和修复系统（P0 - 核心功能）

#### 4.3.1 故障检测

**数据源集成**:
- **Prometheus 指标**:
  - Pod 重启次数（`kube_pod_container_status_restarts_total`）
  - OOMKill 事件（`container_oom_events_total`）
  - CPU/内存使用率

- **K8s 事件**:
  - Warning 级别事件
  - Informer 实时监听

- **日志聚合**:
  - 集成 Loki/Elasticsearch
  - 检测错误日志模式（如 "OutOfMemory", "Connection refused"）

**技术实现**:
```go
// 使用 Informer 实时监听 Pod 事件
type FaultDetector struct {
    informer    cache.SharedIndexInformer
    eventQueue  workqueue.RateLimitingInterface
    agents      *AgentPool
}

func (f *FaultDetector) Start(ctx context.Context) {
    f.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
        UpdateFunc: func(oldObj, newObj interface{}) {
            newPod := newObj.(*corev1.Pod)
            oldPod := oldObj.(*corev1.Pod)

            // 检测 Pod 状态变化
            if f.isFaulty(newPod, oldPod) {
                f.eventQueue.Add(newPod)
            }
        },
    })

    // 工作队列处理
    go f.processQueue(ctx)
}

func (f *FaultDetector) isFaulty(newPod, oldPod *corev1.Pod) bool {
    // CrashLoopBackOff
    if newPod.Status.ContainerStatuses[0].State.Waiting != nil &&
       newPod.Status.ContainerStatuses[0].State.Waiting.Reason == "CrashLoopBackOff" {
        return true
    }

    // 重启次数增加
    if newPod.Status.ContainerStatuses[0].RestartCount > oldPod.Status.ContainerStatuses[0].RestartCount {
        return true
    }

    return false
}

func (f *FaultDetector) processQueue(ctx context.Context) {
    for {
        obj, shutdown := f.eventQueue.Get()
        if shutdown {
            return
        }

        pod := obj.(*corev1.Pod)

        // 触发诊断 Agent
        go f.agents.Diagnostician.Diagnose(ctx, pod)

        f.eventQueue.Done(obj)
    }
}
```

#### 4.3.2 根因分析增强

**LLM 驱动的日志分析**:
```go
func (a *DiagnosticianAgent) analyzeLogsWithLLM(logs string, events []Event) (*RootCauseAnalysis, error) {
    prompt := fmt.Sprintf(`
你是 Kubernetes 专家。分析以下 Pod 日志和事件，找出根本原因。

日志:
%s

事件:
%s

请以 JSON 格式返回分析结果:
{
  "root_cause": "根本原因描述",
  "error_type": "OOMKilled/CrashLoopBackOff/ImagePullBackOff/ConfigError/HealthCheckFailed",
  "key_errors": ["关键错误信息1", "关键错误信息2"],
  "recommendations": ["修复建议1", "修复建议2"],
  "confidence": 0.95
}
`, logs, eventsToString(events))

    response := a.llm.Complete(prompt)

    var analysis RootCauseAnalysis
    json.Unmarshal([]byte(response), &analysis)

    return &analysis, nil
}
```

**相似案例检索**:
```go
// 向量化历史故障，使用 pgvector 或 Pinecone
type KnowledgeBase struct {
    vectorDB VectorDB
}

func (kb *KnowledgeBase) FindSimilar(analysis *RootCauseAnalysis) []HistoricalCase {
    // 1. 将当前故障向量化
    embedding := kb.vectorDB.Embed(analysis.Description)

    // 2. 相似度搜索
    cases := kb.vectorDB.Search(embedding, topK=5)

    // 3. 返回历史修复方案
    return cases
}
```

#### 4.3.3 GitOps 集成

**GitOps 流程**:
```
故障检测 → 诊断 → 生成修复 Patch → 创建 PR →
(可选) 人工审批 → 合并 PR → ArgoCD/Flux 自动部署 → 验证
```

**技术实现**:
```go
type GitOpsClient struct {
    repoURL     string
    branchName  string
    gitProvider GitProvider  // GitHub/GitLab/Gitea
}

func (g *GitOpsClient) CreateRemediationPR(patch *Patch, diagnosis *DiagnosisReport) (*PullRequest, error) {
    // 1. Clone 仓库
    repo := g.cloneRepo()

    // 2. 创建分支
    branchName := fmt.Sprintf("fix/%s-%s", diagnosis.Pod.Name, time.Now().Unix())
    repo.CreateBranch(branchName)

    // 3. 修改 YAML 文件
    manifestPath := g.findManifest(diagnosis.Pod)
    g.applyPatch(manifestPath, patch)

    // 4. Commit
    repo.Commit(fmt.Sprintf("Fix %s: %s", diagnosis.ErrorType, diagnosis.RootCause))

    // 5. Push
    repo.Push(branchName)

    // 6. 创建 PR
    pr := g.gitProvider.CreatePR(PullRequestOptions{
        Title:       fmt.Sprintf("[AutoRemediation] Fix %s", diagnosis.Pod.Name),
        Description: g.generatePRDescription(diagnosis, patch),
        Base:        "main",
        Head:        branchName,
        Labels:      []string{"auto-remediation", "kubeagent"},
    })

    return pr, nil
}

func (g *GitOpsClient) generatePRDescription(diagnosis *DiagnosisReport, patch *Patch) string {
    return fmt.Sprintf(`
## 故障诊断报告

**Pod**: %s
**Namespace**: %s
**根本原因**: %s
**错误类型**: %s

## 修复方案

\`\`\`yaml
%s
\`\`\`

## 验证步骤

1. 合并此 PR
2. 等待 ArgoCD 自动部署
3. 检查 Pod 状态: kubectl get pod %s -n %s
4. 验证日志无错误: kubectl logs %s -n %s

---
🤖 由 KubeAgent 自动生成
`, diagnosis.Pod.Name, diagnosis.Pod.Namespace, diagnosis.RootCause, diagnosis.ErrorType,
   patch.YAML, diagnosis.Pod.Name, diagnosis.Pod.Namespace, diagnosis.Pod.Name, diagnosis.Pod.Namespace)
}
```

### 4.4 可观测性系统（P1）

#### 4.4.1 分布式追踪

**需求**:
- 追踪 Agent 决策链路（Coordinator → Specialist Agents → Tools）
- 记录每个步骤的耗时和输入输出
- 集成 Jaeger/Tempo

**技术实现**:
```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
)

func (c *CoordinatorAgent) HandleRequest(ctx context.Context, req *Request) (*Response, error) {
    // 创建根 Span
    tracer := otel.Tracer("kubeagent.coordinator")
    ctx, span := tracer.Start(ctx, "HandleRequest")
    defer span.End()

    span.SetAttributes(
        attribute.String("user", req.User),
        attribute.String("intent", req.Intent),
    )

    // 子任务追踪
    diagnosisResult := c.diagnose(ctx, req)  // 自动创建子 Span
    remediationResult := c.remediate(ctx, diagnosisResult)

    return &Response{...}, nil
}
```

**可视化**:
- Jaeger UI 查看完整调用链
- 识别性能瓶颈（如 LLM 调用耗时过长）

#### 4.4.2 指标监控

**核心指标**:
- Agent 处理延迟（P50/P95/P99）
- 故障诊断成功率
- 自动修复成功率
- LLM Token 消耗
- K8s API 调用次数

**Prometheus 指标**:
```go
var (
    agentRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "kubeagent_request_duration_seconds",
            Help:    "Agent request duration",
            Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
        },
        []string{"agent", "action"},
    )

    diagnosisSuccessRate = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "kubeagent_diagnosis_total",
            Help: "Total diagnosis attempts",
        },
        []string{"result"},  // success/failure
    )

    llmTokenUsage = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "kubeagent_llm_tokens_total",
            Help: "LLM tokens consumed",
        },
        []string{"model", "agent"},
    )
)
```

**Grafana Dashboard**:
- 实时诊断任务数
- Agent 健康状态
- 成本趋势（LLM Token 消耗）

#### 4.4.3 结构化日志

**日志格式**:
```json
{
  "timestamp": "2026-01-07T10:30:00Z",
  "level": "info",
  "agent": "diagnostician",
  "trace_id": "abc123",
  "span_id": "def456",
  "user": "user@example.com",
  "action": "diagnose_pod",
  "pod": "nginx-deployment-7d5c8b9f4d-x8k2l",
  "namespace": "production",
  "root_cause": "OOMKilled",
  "duration_ms": 1234
}
```

### 4.5 易用性改进（P1）

#### 4.5.1 Web UI

**功能**:
- **Dashboard**: 集群健康状态、Agent 活动、最近诊断
- **任务列表**: 查看所有诊断/修复任务及其状态
- **资源视图**: 可视化 Pod、Deployment、Node 状态
- **交互式诊断**: Web 聊天界面与 Agent 对话
- **审批流程**: 审批待修复的 PR

**技术栈**: React + Ant Design / Vue3 + Element Plus

#### 4.5.2 Slack/钉钉集成

**功能**:
- **告警通知**: 故障检测后自动发送通知
- **交互式修复**: 在 Slack 中审批修复方案
- **状态查询**: `/kubeagent status pod nginx` 查询 Pod 状态

**技术实现**:
```go
type SlackBot struct {
    client      *slack.Client
    agentPool   *AgentPool
}

func (s *SlackBot) HandleCommand(cmd *SlackCommand) error {
    switch cmd.Command {
    case "/kubeagent diagnose":
        // 触发诊断 Agent
        result := s.agentPool.Diagnostician.Diagnose(...)
        s.client.PostMessage(cmd.ChannelID, result.ToSlackMessage())

    case "/kubeagent approve":
        // 审批修复
        s.agentPool.Remediator.ApplyRemediation(cmd.Args...)
    }
    return nil
}
```

#### 4.5.3 预设 Runbook

**功能**:
- 维护常见故障的 Runbook（如 OOMKilled、CrashLoopBackOff）
- 用户可自定义 Runbook
- Agent 自动匹配并推荐 Runbook

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
    params:
      query: "container_memory_usage_bytes{pod='{{.PodName}}'}"
  - name: increase-memory-limit
    action: patch_resource
    params:
      patch: |
        spec:
          template:
            spec:
              containers:
              - name: app
                resources:
                  limits:
                    memory: "{{.NewMemoryLimit}}"
  - name: verify
    action: wait_healthy
    params:
      timeout: 5m
```

### 4.6 Kubernetes Operator 部署（P1）

**需求**:
- 将 KubeAgent 部署为 K8s Operator
- 使用 CRD 定义诊断任务和修复策略
- 自愈和高可用

**CRD 设计**:
```yaml
apiVersion: kubeagent.io/v1
kind: DiagnosisTask
metadata:
  name: nginx-crashloop-diagnosis
spec:
  target:
    kind: Pod
    name: nginx-deployment-7d5c8b9f4d-x8k2l
    namespace: production
  agents:
  - diagnostician
  - remediator
  autoRemediate: true  # 自动修复
  approvalRequired: true  # 需要审批
status:
  phase: Diagnosing
  rootCause: "OOMKilled: Container exceeded memory limit"
  remediationPlan: "Increase memory limit to 512Mi"
  approvalStatus: Pending
```

**Operator 实现**:
```go
// 使用 kubebuilder 或 operator-sdk
type DiagnosisTaskReconciler struct {
    client.Client
    Scheme    *runtime.Scheme
    AgentPool *AgentPool
}

func (r *DiagnosisTaskReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    var task kubeagentv1.DiagnosisTask
    if err := r.Get(ctx, req.NamespacedName, &task); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // 执行诊断
    if task.Status.Phase == "" {
        diagnosis := r.AgentPool.Diagnostician.Diagnose(ctx, task.Spec.Target)
        task.Status.RootCause = diagnosis.RootCause
        task.Status.Phase = "Diagnosed"
        r.Status().Update(ctx, &task)
    }

    // 执行修复
    if task.Spec.AutoRemediate && task.Status.ApprovalStatus == "Approved" {
        r.AgentPool.Remediator.Remediate(ctx, task.Status.RemediationPlan)
        task.Status.Phase = "Remediated"
        r.Status().Update(ctx, &task)
    }

    return ctrl.Result{}, nil
}
```

### 4.7 多集群管理（P2）

**需求**:
- 统一管理多个 K8s 集群（开发、测试、生产）
- 跨集群资源查询和诊断
- 多集群故障关联分析

**技术实现**:
```go
type MultiClusterManager struct {
    clusters map[string]*K8sClient
}

func (m *MultiClusterManager) DiagnoseAcrossClusters(ctx context.Context, query string) ([]*DiagnosisReport, error) {
    var reports []*DiagnosisReport

    for clusterName, client := range m.clusters {
        // 并行诊断多个集群
        go func(name string, c *K8sClient) {
            report := m.agentPool.Diagnostician.DiagnoseCluster(ctx, c)
            report.ClusterName = name
            reports = append(reports, report)
        }(clusterName, client)
    }

    return reports, nil
}
```

## 五、技术亮点总结（面试话术）

### 5.1 多 Agent 调度（核心亮点）

**面试话术**:
> "KubeAgent 采用**层级式多 Agent 架构**，通过 **Coordinator Agent** 实现任务分解和路由。我设计了 5 个专业 Agent（诊断、修复、安全、成本、知识），每个 Agent 有独立的工具集和决策逻辑。
>
> 我使用 **LangGraph** 实现状态图编排，支持串行、并行、条件分支等复杂流程。例如，诊断故障时，Diagnostician Agent 收集日志、事件、指标后，LLM 分析根因，然后 Remediator Agent 生成修复方案并提交 GitOps PR。整个过程通过 **OpenTelemetry** 追踪，可以在 Jaeger 中看到完整的决策链路。
>
> 在 Agent 协作上，我实现了**共识机制**：删除生产资源需要 Security Agent 和 Remediator Agent 共同投票，降低误操作风险。"

### 5.2 K8s 安全和鉴权（面试重点）

**面试话术**:
> "安全是生产落地的关键。我为每个 Agent 创建了**专用 ServiceAccount**，严格遵循**最小权限原则**：Diagnostician 只有只读权限，Remediator 需要审批才能修改资源。
>
> 我实现了**沙箱执行环境**，使用 K8s Server-side Dry Run 和独立 Namespace 测试修复方案，避免直接在生产环境试错。所有危险操作（删除、修改生产配置）都需要**人工审批**，并通过 **GitOps 流程**提交 PR，保证可追溯。
>
> 我还集成了**审计日志系统**，记录每个 Agent 的决策和执行，支持按用户、时间、资源查询，并导出到 Elasticsearch 做长期存储和分析。"

### 5.3 自动化诊断和修复（核心功能）

**面试话术**:
> "我使用 **Informer 机制**实时监听 Pod 状态变化，检测到 CrashLoopBackOff、OOMKilled 等故障后，立即触发 Diagnostician Agent。
>
> 诊断时，我整合了**多维度数据源**：Pod 日志、K8s 事件、Prometheus 指标、Jaeger 追踪。LLM 分析这些数据后，生成**结构化诊断报告**（根因、影响范围、修复建议）。我还实现了**相似案例检索**，使用向量数据库（pgvector）查找历史故障，提高诊断准确率。
>
> 修复方案通过 **GitOps 集成**实现：生成 K8s Patch 后提交 PR 到 GitOps 仓库，等待审批后由 ArgoCD 自动部署。如果修复失败，**自动 Rollback** 到上一个稳定版本。"

### 5.4 可观测性（生产必备）

**面试话术**:
> "我集成了**完整的可观测性栈**：OpenTelemetry + Jaeger（分布式追踪）+ Prometheus（指标）+ Grafana（可视化）。
>
> 每个 Agent 决策都会创建 Trace Span，记录耗时、输入输出，可以清楚看到哪个步骤是瓶颈（比如 LLM 调用占 80% 耗时）。我定义了关键指标如**诊断成功率**、**自动修复成功率**、**LLM Token 消耗**，并在 Grafana 实时展示。
>
> 我还实现了**结构化日志**，包含 trace_id、用户、操作类型等上下文信息，方便问题排查。"

### 5.5 易用性和落地（差异化优势）

**面试话术**:
> "为了真正落地，我开发了 **Web UI**（React + Ant Design），提供 Dashboard、任务列表、资源视图、交互式诊断等功能，降低使用门槛。
>
> 我集成了 **Slack/钉钉机器人**，故障检测后自动发送告警，用户可以直接在 IM 工具中审批修复方案，无需登录 K8s。
>
> 我还维护了**预设 Runbook 库**，覆盖 OOMKilled、CrashLoopBackOff 等常见故障，Agent 自动匹配并推荐 Runbook，用户也可以自定义。
>
> 部署方面，我使用 **Kubernetes Operator** 模式，定义了 DiagnosisTask CRD，用户可以声明式创建诊断任务，Operator 自动执行并更新状态。"

## 六、实施路线图

### 6.1 第一阶段（2-3 周）- MVP（最小可行产品）

**目标**: 实现多 Agent 协作框架 + 基础诊断修复

**任务**:
- [ ] 重构为 Coordinator-Specialist 架构
- [ ] 实现 Coordinator Agent（使用 LangGraph）
- [ ] 实现 Diagnostician Agent（日志、事件、指标分析）
- [ ] 实现 Remediator Agent（Patch 生成、GitOps PR）
- [ ] ServiceAccount 和 RBAC 配置
- [ ] 沙箱执行环境（Dry Run）
- [ ] 基础审计日志
- [ ] 单元测试和集成测试

**验收标准**:
- 能够自动诊断 OOMKilled、CrashLoopBackOff 故障
- 生成修复 Patch 并提交 GitOps PR
- 所有 Agent 使用独立 ServiceAccount
- 危险操作需要人工确认

### 6.2 第二阶段（2-3 周）- 安全和可观测性

**目标**: 完善安全机制 + 可观测性

**任务**:
- [ ] Security Agent 实现（RBAC 审计、镜像扫描）
- [ ] 完善审计日志系统（导出到 Elasticsearch）
- [ ] OpenTelemetry 集成（分布式追踪）
- [ ] Prometheus 指标暴露
- [ ] Grafana Dashboard 开发
- [ ] 告警规则配置（Alertmanager）

**验收标准**:
- 可以在 Jaeger 中查看完整 Agent 决策链路
- Grafana 实时展示诊断成功率、修复成功率等指标
- 检测到过度授权的 RBAC 配置
- 镜像漏洞扫描集成

### 6.3 第三阶段（2-3 周）- 易用性和落地

**目标**: Web UI + IM 集成 + Operator 部署

**任务**:
- [ ] Web UI 开发（React）
  - Dashboard（集群健康、Agent 活动）
  - 任务列表（诊断/修复任务）
  - 资源视图（Pod、Deployment）
  - 交互式诊断（聊天界面）
- [ ] Slack/钉钉 Bot 开发
- [ ] Kubernetes Operator 实现
  - DiagnosisTask CRD
  - Controller 逻辑
- [ ] Helm Chart 打包
- [ ] 文档编写（安装、使用、架构设计）

**验收标准**:
- Web UI 可用，支持交互式诊断
- Slack 中可以审批修复方案
- 使用 Helm 一键部署到 K8s
- 完整的用户文档和 API 文档

### 6.4 第四阶段（2-3 周）- 高级特性

**目标**: Cost Optimizer + Knowledge Agent + 多集群

**任务**:
- [ ] Cost Optimizer Agent（资源优化建议、HPA 推荐）
- [ ] Knowledge Agent（文档检索、Runbook 推荐）
- [ ] 向量数据库集成（pgvector）
- [ ] 多集群管理
- [ ] 预设 Runbook 库
- [ ] 性能优化（Agent 并行执行、缓存）

**验收标准**:
- 生成资源优化报告（节省 XX% 成本）
- 推荐合理的 HPA 配置
- 相似故障检索准确率 > 80%
- 支持管理 3+ 个集群

## 七、成功指标

### 7.1 技术指标

- **诊断准确率**: > 90%
- **自动修复成功率**: > 80%（需人工确认的修复）
- **平均诊断时间**: < 30 秒
- **平均修复时间**: < 5 分钟
- **Agent 可用性**: > 99.9%
- **LLM Token 成本**: < $10/月（单集群）

### 7.2 面试展示指标

- **多 Agent 协作**: 5 个专业 Agent 协同工作
- **安全机制**: RBAC + 沙箱 + 审计日志
- **可观测性**: 分布式追踪 + 指标监控 + 结构化日志
- **自动化率**: 80% 的常见故障自动修复
- **代码质量**: 80% 单元测试覆盖率

## 八、技术选型总结

| 模块 | 技术选型 | 理由 |
|------|----------|------|
| **Agent 框架** | LangGraph | 状态管理、持久化、断点续执 |
| **多 Agent 协作** | CrewAI 模式 | 角色分工、任务编排清晰 |
| **LLM** | 通义千问 + Claude（备选） | 中文支持好、成本低 |
| **K8s 客户端** | client-go + Dynamic Client | 官方支持、功能完整 |
| **配置管理** | Viper | 支持多种格式、动态加载 |
| **认证授权** | K8s RBAC + ServiceAccount | 原生集成、企业级 |
| **状态存储** | Redis（会话）+ PostgreSQL（持久化） | 高性能、可靠性高 |
| **可观测性** | OpenTelemetry + Jaeger + Prometheus + Grafana | CNCF 标准栈 |
| **日志** | Zap（结构化日志） | 高性能、易于索引 |
| **GitOps** | GitHub API / GitLab API | 代码即基础设施 |
| **Web UI** | React + Ant Design | 组件丰富、开发效率高 |
| **Operator** | kubebuilder / operator-sdk | 官方脚手架、最佳实践 |
| **向量数据库** | pgvector（PostgreSQL 插件） | 低成本、易集成 |
| **沙箱** | K8s Server-side Dry Run + 独立 Namespace | 无需额外运行时 |

## 九、参考资源

### 9.1 开源项目对标

- **K8sGPT** (CNCF Sandbox): https://github.com/k8sgpt-ai/k8sgpt
  - 借鉴：自动诊断、多 LLM 支持、Operator 模式

- **Kagent** (CNCF): https://www.cncf.io/blog/2025/04/15/kagent-bringing-agentic-ai-to-cloud-native/
  - 借鉴：Agent 工具集、K8s 集成

- **Komodor**: https://komodor.com
  - 借鉴：可视化、Change Intelligence

### 9.2 技术框架

- **LangGraph**: https://github.com/langchain-ai/langgraph
- **CrewAI**: https://github.com/crewAIInc/crewAI
- **OpenTelemetry**: https://opentelemetry.io/
- **Kubebuilder**: https://book.kubebuilder.io/

### 9.3 K8s 最佳实践

- **Kubernetes Patterns**: https://www.oreilly.com/library/view/kubernetes-patterns/9781492050278/
- **Production Kubernetes**: https://www.oreilly.com/library/view/production-kubernetes/9781492092292/

---

## 附录：面试问答准备

### Q1: 为什么选择多 Agent 架构而不是单一 Agent？

**答**:
> "单一 Agent 的问题是**职责过重**，难以维护和扩展。多 Agent 架构的优势：
> 1. **专业化分工**：每个 Agent 专注一个领域（诊断、修复、安全），提高准确率
> 2. **并行处理**：多个 Agent 可以并行工作，提升效率
> 3. **易于扩展**：新增功能只需添加新 Agent，不影响现有逻辑
> 4. **降低复杂度**：每个 Agent 的 Prompt 和工具集更简洁，易于调试
> 5. **容错性**：一个 Agent 失败不影响其他 Agent
>
> 我参考了 CrewAI 的设计理念，采用 Coordinator-Specialist 模式，Coordinator 负责编排，Specialist 负责执行。"

### Q2: 如何保证 Agent 执行的安全性？

**答**:
> "我从三个层面保证安全性：
> 1. **权限控制**：每个 Agent 使用独立 ServiceAccount，遵循最小权限原则。Diagnostician 只读，Remediator 需审批
> 2. **沙箱隔离**：危险操作先在沙箱环境测试（Dry Run + 独立 Namespace），验证无误后才应用到生产
> 3. **审计追溯**：所有操作记录审计日志，包含用户、时间、操作类型、资源，导出到 Elasticsearch 做长期存储
> 4. **人工确认**：删除资源、修改生产配置等高风险操作需要人工审批（Slack/Web UI）
> 5. **GitOps 流程**：修复通过 PR 提交，代码审查后再部署，保证可追溯和回滚"

### Q3: 如何评估 Agent 的诊断准确率？

**答**:
> "我设计了**准确率评估系统**：
> 1. **Ground Truth 数据集**：收集历史故障案例，人工标注根因
> 2. **自动化测试**：定期用测试集评估 Diagnostician Agent，计算准确率
> 3. **用户反馈**：Web UI 中让用户对诊断结果打分（有用/无用），持续优化
> 4. **A/B 测试**：对比不同 Prompt 或 LLM 的诊断效果
> 5. **监控指标**：追踪诊断成功率、误报率、漏报率，设定告警阈值
>
> 目前我的目标是诊断准确率 > 90%，通过**知识库检索**和 **Few-shot Learning** 持续提升。"

### Q4: 项目最大的技术挑战是什么？

**答**:
> "最大的挑战是 **Agent 决策的可靠性和可解释性**：
> 1. **LLM 输出不稳定**：同样的问题可能返回不同结果。我通过 **Structured Output**（JSON Schema 约束）和**多次采样投票**提高稳定性
> 2. **工具调用错误**：Agent 可能调用不存在的工具或传错参数。我实现了**工具参数校验**和**错误重试机制**
> 3. **决策链路长**：Coordinator → Specialist → Tool 链路长，难以调试。我引入 **OpenTelemetry 追踪**，可视化每一步
> 4. **成本控制**：频繁调用 LLM 成本高。我通过**缓存相似请求**、**使用小模型处理简单任务**（Haiku vs Sonnet）降低成本
>
> 另一个挑战是**多 Agent 协作的一致性**：需要设计好任务分解和 Agent 通信协议，避免冲突。"

### Q5: 如何与现有工具链（监控、告警、GitOps）集成？

**答**:
> "我设计了**插件化架构**，支持多种集成：
> 1. **监控告警**：
>    - Prometheus：通过 Informer 监听 Pod 状态，主动诊断
>    - Alertmanager：接收告警后触发 Agent（Webhook）
> 2. **GitOps**：
>    - ArgoCD/Flux：修复通过 PR 提交到 Git 仓库，自动部署
>    - 支持 GitHub/GitLab/Gitea API
> 3. **日志聚合**：
>    - Loki/Elasticsearch：诊断时查询聚合日志，检测错误模式
> 4. **IM 工具**：
>    - Slack/钉钉：告警通知 + 交互式审批
> 5. **CI/CD**：
>    - 在 Pipeline 中调用 KubeAgent API，自动检测部署失败原因
>
> 我提供了 **REST API** 和 **Kubernetes CRD** 两种集成方式，方便与现有工具对接。"

---

**总结**: 这份需求文档涵盖了多 Agent 调度、K8s 安全、自动化修复、可观测性、易用性等企业级特性，能够充分展现技术深度和工程能力，适合在面试中讨论。建议按照实施路线图逐步实现，每个阶段都有明确的验收标准。
