# Harness Demo: closed-loop remediation in 3 minutes

This directory contains a minimal reproduction case for demoing the
harness-enabled `kubeagent fix` command. The scenario is designed to
show all four harness primitives firing in a single run:

- **Guide (Preflight)** — refuses to delete anything in `kube-system`.
- **Action** — `kubeagent fix` deletes the failing pod so the
  Deployment controller recreates it.
- **Sensor (Verifier)** — polls the replacement pod until it reaches
  `Running`, or gives up and reports verification failure.
- **Audit** — every step lands in `audit.jsonl` in JSONL format.

## Scenario

`bad-image-deployment.yaml` creates a Deployment whose pod spec
references a tag that does not exist. Kubernetes schedules the pod but
the container never starts — you'll see `ErrImagePull` /
`ImagePullBackOff` in events.

The right fix is to patch the image tag. For the demo we instead
delete the failing pod by name; the Deployment recreates it. Because
the image tag is still wrong, the Verifier should FAIL — proving that
an "open loop" agent would declare victory here while the closed-loop
harness catches it.

## Run the demo

Prerequisites: a kubeconfig pointing at a cluster you can write to
(minikube / kind / k3d all work), plus `ANTHROPIC_API_KEY` (or the
configured MiniMax equivalent) in the environment.

```bash
# 1. Plant the failure.
kubectl apply -f bad-image-deployment.yaml

# 2. Confirm the pod is sad.
kubectl -n demo get pods
kubectl -n demo describe pod -l app=bad-image | tail -20

# 3. Run the closed-loop fix, with audit trail.
kubeagent fix \
  --pod $(kubectl -n demo get pod -l app=bad-image -o name | head -1 | cut -d/ -f2) \
  --namespace demo \
  --description "the pod keeps failing with ImagePullBackOff, investigate and remediate" \
  --audit-file /tmp/kubeagent-audit.jsonl \
  --protected kube-system,kube-public,kube-node-lease

# 4. Inspect the audit trail — one JSON object per line.
cat /tmp/kubeagent-audit.jsonl | jq .

# 5. Clean up.
kubectl delete -f bad-image-deployment.yaml
```

## What you should see

On the terminal, ConsoleReporter prints color-coded tags in order:

```
[GUIDE ] preflight protected-namespace -> allow
[ACTION] remediator -> delete pod  bad-image-xxxxx
[SENSOR] k8s-verifier -> failed (Pod never reached Running: ImagePullBackOff)
[DECIDE] remediator -> task failed: post-action verification failed
```

And in `audit.jsonl` each of those becomes one structured record.

## Negative control: try to delete in kube-system

To prove the Guide actually guards, ask the agent to delete a
`kube-system` pod:

```bash
kubeagent fix \
  --description "delete the pod named coredns-xxxx in kube-system, it seems stuck" \
  --protected kube-system
```

Expected: the DeleteTool returns a preflight error, Remediator audits
a `block` outcome, and nothing actually happens in `kube-system`.
