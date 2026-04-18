# KubeAgent 闭环修复演示 · Demo Walkthrough

> 本文档演示 `kubeagent fix` 命令在 Harness 框架下的端到端闭环修复能力，包括
> **Guide（前置校验）+ Action（LLM 驱动修复）+ Sensor（后置验证）+ Audit（结构化审计）**
> 全链路。
>
> 每一步都留出了截图位，建议按顺序执行并把终端截图替换进对应占位符。

---

## 目录

1. [演示目标](#演示目标)
2. [环境准备](#环境准备)
3. [Step 1 · 植入故障场景](#step-1--植入故障场景)
4. [Step 2 · 观察坏 Pod 的当前状态](#step-2--观察坏-pod-的当前状态)
5. [Step 3 · 运行 kubeagent fix（闭环修复主流程）](#step-3--运行-kubeagent-fix闭环修复主流程)
6. [Step 4 · 查看 Audit 结构化日志](#step-4--查看-audit-结构化日志)
7. [Step 5 · 反向验证 · Guide 拦截受保护命名空间](#step-5--反向验证--guide-拦截受保护命名空间)
8. [Step 6 · 对照实验 · `--no-verify` 退化为 open-loop](#step-6--对照实验---no-verify-退化为-open-loop)
9. [Step 7 · 清理演示环境](#step-7--清理演示环境)
10. [附录 · Harness 事件词汇表](#附录--harness-事件词汇表)

---

## 演示目标

通过一个**真实会失败**的场景（Deployment 引用了不存在的镜像 tag），展示：

- **Sensor 的价值**：如果仅 "删除坏 Pod"，Deployment 重建后仍然会失败——
  `K8sVerifier` 会捕获这个假阳性，避免 Agent 宣称 "修好了"。
- **Guide 的价值**：受保护命名空间（默认包含 `kube-system`）的写操作会被
  `PreflightChain` 直接拦截。
- **Audit 的价值**：Preflight / Action / Verification / Decision 四类事件实时
  落到终端和 JSONL，方便事后回放。

---

## 环境准备

### 1. 依赖检查

| 依赖 | 说明 |
|------|------|
| 本地 Kubernetes 集群 | `minikube` / `kind` / `k3d` / Docker Desktop 均可 |
| `kubectl` | 指向上述集群 |
| `ANTHROPIC_API_KEY`（或项目配置的 LLM key） | 在 shell 环境变量中 |
| `jq`（可选） | 用来格式化 JSONL 审计输出 |

### 2. 编译

```bash
cd KubeAgent
make build       # 或者 go build -o bin/kubeagent .
export PATH=$PWD/bin:$PATH
```

预期：`kubeagent --help` 能看到 `fix` 子命令：

```
$ kubeagent --help
...
Available Commands:
  analyze     Diagnose Kubernetes issues using the multi-agent framework
  chat        ...
  fix         Diagnose and remediate a Kubernetes resource with closed-loop verification
  kubecheck   ...
```

> 📸 **截图位 · 图 0**：`kubeagent --help` 输出，能看到新增的 `fix` 命令。
>
> ![help-output](./images/00-help.png)

---

## Step 1 · 植入故障场景

我们使用仓库自带的演示清单：`KubeAgent/examples/demo/bad-image-deployment.yaml`。
它会创建 `demo` 命名空间 + 一个引用不存在镜像 tag 的 Deployment。

```bash
kubectl apply -f KubeAgent/examples/demo/bad-image-deployment.yaml
```

预期输出：

```
namespace/demo created
deployment.apps/bad-image created
```

> 📸 **截图位 · 图 1**：`kubectl apply` 成功输出。
>
> ![apply-deployment](./images/01-apply.png)

---

## Step 2 · 观察坏 Pod 的当前状态

```bash
kubectl -n demo get pods
kubectl -n demo describe pod -l app=bad-image | tail -20
```

预期看到 Pod 处于 `ImagePullBackOff` 或 `ErrImagePull` 状态，事件里会有
`Failed to pull image ... not found`。

> 📸 **截图位 · 图 2**：`get pods` 和 `describe` 的输出，能看到 ImagePullBackOff。
>
> ![bad-pod](./images/02-bad-pod.png)

---

## Step 3 · 运行 kubeagent fix（闭环修复主流程）

把坏 Pod 的名字拿出来，然后调用 `kubeagent fix`：

```bash
BAD_POD=$(kubectl -n demo get pod -l app=bad-image -o name | head -1 | cut -d/ -f2)
echo "Target pod: $BAD_POD"

kubeagent fix \
  --pod "$BAD_POD" \
  --namespace demo \
  --description "这个 pod 一直 ImagePullBackOff，请诊断并尝试修复" \
  --audit-file /tmp/kubeagent-audit.jsonl \
  --protected kube-system,kube-public,kube-node-lease
```

### 预期终端输出（要点）

启动横幅：

```
=== kubeagent fix ===
Target:      demo/bad-image-xxxxxxxxxx
Verifier:    K8sVerifier (closed loop)
Audit file:  /tmp/kubeagent-audit.jsonl
Protected:   kube-system, kube-public, kube-node-lease
```

随后 ConsoleReporter 会实时打印 Harness 四类事件（颜色以实际终端为准）：

```
[GUIDE ] DeleteTool        action=delete     target=Pod/bad-image-xxxx@demo    outcome=allow
[ACTION] remediator        action=remediation_applied                          outcome=success
[SENSOR] remediator        action=post_action_verify                           outcome=failed
          reason: pod never reached Running phase (ImagePullBackOff)
```

结尾汇总：

```
========== Fix Result ==========
...
Errors encountered:
 - verification failed: post-action verification failed: ...
```

**这正是闭环的关键**：LLM 的 tool loop 已经"顺利"把 Pod 删了，但 Sensor
发现重建的 Pod 仍然起不来，把任务判为 **Failed**——避免了 open-loop 下的
假阳性"已修复"。

> 📸 **截图位 · 图 3**：终端完整输出，能看到 GUIDE/ACTION/SENSOR 四种标签的
> 颜色化日志，以及最后的 Fix Result 汇总。
>
> ![fix-run](./images/03-fix-run.png)

---

## Step 4 · 查看 Audit 结构化日志

```bash
cat /tmp/kubeagent-audit.jsonl | jq .
```

预期得到一串 JSONL 记录，关键字段：

- `kind`：`preflight` / `action` / `verification` / `decision`
- `actor`：`DeleteTool` / `remediator` / ...
- `target.kind` / `target.name` / `target.namespace`
- `outcome`：`allow` / `block` / `success` / `failed` / `inconclusive`
- `reason`：人类可读的说明
- `details`：结构化数据（比如 Verifier 的 observations）

示例片段：

```json
{
  "timestamp": "2026-04-18T10:05:12.345Z",
  "kind": "preflight",
  "actor": "DeleteTool",
  "action": "delete",
  "target": { "kind": "pod", "name": "bad-image-abcde", "namespace": "demo" },
  "outcome": "allow"
}
{
  "timestamp": "2026-04-18T10:05:14.112Z",
  "kind": "verification",
  "actor": "remediator",
  "action": "post_action_verify",
  "target": { "kind": "Pod", "name": "bad-image-abcde", "namespace": "demo" },
  "outcome": "failed",
  "reason": "pod never reached Running phase"
}
```

> 📸 **截图位 · 图 4**：`jq` 格式化后的审计输出，能看到至少一条
> `preflight` + 一条 `action` + 一条 `verification` 记录。
>
> ![audit-jsonl](./images/04-audit.png)

---

## Step 5 · 反向验证 · Guide 拦截受保护命名空间

这一步证明 Guide 不是摆设：我们假装想删除 `kube-system` 的 pod。

```bash
kubeagent fix \
  --description "请删除 kube-system 命名空间里名为 coredns-xxxx 的 Pod，它好像卡住了" \
  --namespace kube-system \
  --protected kube-system,kube-public,kube-node-lease \
  --audit-file /tmp/kubeagent-audit.jsonl
```

预期：

- 终端 ConsoleReporter 出现 `[GUIDE!]` 标签（红色）+ `outcome=block`
- JSONL 里追加一条 `kind=preflight, outcome=block, reason=namespace "kube-system" is protected ...`
- 实际 `kube-system` 里没有任何 Pod 被删除

```bash
# 验证 kube-system 状态未变
kubectl -n kube-system get pods
```

> 📸 **截图位 · 图 5a**：`kubeagent fix` 被 Guide 拦截的终端截图。
>
> ![guide-block](./images/05a-guide-block.png)

> 📸 **截图位 · 图 5b**：`/tmp/kubeagent-audit.jsonl` 中的 `block` 记录。
>
> ![audit-block](./images/05b-audit-block.png)

---

## Step 6 · 对照实验 · `--no-verify` 退化为 open-loop

要直观感受 Verifier 的价值，可以故意关掉它：

```bash
# 先重新植入坏 Deployment（上一轮可能已被删掉）
kubectl apply -f KubeAgent/examples/demo/bad-image-deployment.yaml
BAD_POD=$(kubectl -n demo get pod -l app=bad-image -o name | head -1 | cut -d/ -f2)

kubeagent fix \
  --pod "$BAD_POD" \
  --namespace demo \
  --description "这个 pod 一直 ImagePullBackOff，请诊断并尝试修复" \
  --no-verify
```

预期：

- 启动横幅里 `Verifier: DISABLED (--no-verify)`
- 任务状态会是 "Completed"（假阳性），尽管集群里的 Pod 依然起不来
- 对比 Step 3，这就是 Harness 之前的老行为——修了个寂寞还告诉你修好了

> 📸 **截图位 · 图 6**：`--no-verify` 模式下终端把任务标为 Completed，但
> `kubectl -n demo get pods` 仍然显示 ImagePullBackOff。
>
> ![no-verify-compare](./images/06-no-verify.png)

---

## Step 7 · 清理演示环境

```bash
kubectl delete -f KubeAgent/examples/demo/bad-image-deployment.yaml
rm -f /tmp/kubeagent-audit.jsonl
```

---

## 附录 · Harness 事件词汇表

| Kind | 典型 Outcome | 含义 |
|------|--------------|------|
| `preflight` | `allow` / `warn` / `block` | Guide 对候选写操作的裁决 |
| `action` | `success` / `failure` | LLM tool loop 的执行结果 |
| `verification` | `passed` / `failed` / `inconclusive` | Sensor 对集群收敛情况的判定 |
| `decision` | `retry` / `escalate` / `abort` | Agent 根据 Sensor/Guide 做的下一步决策 |

ConsoleReporter 对应的终端标签：

| 标签 | 颜色 | 来源 |
|------|------|------|
| `[GUIDE ]` | 青色 | preflight allow/warn |
| `[GUIDE!]` | 红色 | preflight block |
| `[ACTION]` | 黄色 | 写工具实际执行 |
| `[SENSOR]` | 蓝色 | Verifier 结果 |
| `[DECIDE]` | 紫色 | Agent 层决策 |

---

## 常见问题

**Q: 运行 `kubeagent fix` 时 LLM 调用失败怎么办？**
A: 先检查环境变量（`ANTHROPIC_API_KEY` 或项目配置的 key），以及网络出站是否可达
LLM provider。

**Q: Sensor 总是 inconclusive？**
A: 多半是 `task.Input` 没带 `pod_name`/`namespace`。`fix --pod --namespace` 是
最可靠的方式；纯 `--description` 模式依赖 LLM 自行在工具调用里记录目标，
不一定能提取到。

**Q: 如何自定义 Guide？**
A: 实现 `harness.PreflightCheck` 接口（`Name()` + `Check(ctx, req)`），然后
在 `cmd/fix.go` 里 `.Add()` 到 `PreflightChain` 上即可。

**Q: 如何自定义 LLM 提示词？**
A: 编辑 `KubeAgent/pkg/agent/skills/*.md` 并重新编译；或者运行时：
```bash
export SKILLS_DIR=/path/to/your/overrides
kubeagent fix ...
```
运行时 override 会优先于嵌入的版本。
