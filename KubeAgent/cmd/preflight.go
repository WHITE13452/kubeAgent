package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"kubeagent/pkg/agent/harness"
	"kubeagent/pkg/k8s"
)

// Flags for `kubeagent preflight`. Package-private; read by Run after
// Cobra populates them from argv.
var (
	preflightVerb      string
	preflightKind      string
	preflightName      string
	preflightNamespace string
	preflightProtected []string
	preflightAuditFile string
	preflightNoCluster bool
)

// preflightCmd exists purely for demoing / smoke-testing the Harness
// Guide layer. It does NOT spin up an LLM, does NOT register agents,
// and does NOT touch the cluster's write path. All it does is:
//
//   1. Build a PreflightChain (ProtectedNamespaceCheck + optionally
//      ResourceExistsCheck if a k8s client is reachable).
//   2. Feed one synthetic PreflightRequest through the chain.
//   3. Print the decision + reason + warnings.
//   4. Optionally append a single audit event to a JSONL file so the
//      audit trail matches the `fix` command's format.
//
// Why it exists:
//
//   The `fix` command exercises the full multi-agent loop, which
//   means demo-ing Guide-level enforcement (e.g. "writes into
//   kube-system are blocked") requires the LLM to first finish a
//   diagnosis that can take minutes and occasionally times out before
//   any Guide fires. For a demo audience that just needs to SEE the
//   Guide say "no", this shortcut is much more reliable: sub-second
//   feedback, zero LLM cost, deterministic output.
var preflightCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Evaluate the Harness PreflightChain against a hypothetical action (no LLM, no writes)",
	Long: `Run the Harness PreflightChain directly, without invoking the LLM or
executing any cluster write. Intended for fast, deterministic demos
and smoke tests of the Guide layer.

Examples:
  # Block path: pretend we want to delete a pod in kube-system.
  kubeagent preflight --verb delete --kind pod --name coredns-xxxx -n kube-system

  # Allow path: same request in a non-protected namespace.
  kubeagent preflight --verb delete --kind pod --name nginx-1 -n default

  # Also persist the preflight event to a JSONL audit file.
  kubeagent preflight --verb delete --kind pod --name nginx-1 -n kube-system \
    --audit-file /tmp/kubeagent-audit.jsonl

  # Skip cluster lookup (ResourceExistsCheck) when you don't have a cluster handy.
  kubeagent preflight --verb delete --kind pod --name foo -n kube-system --no-cluster`,
	Run: runPreflight,
}

func init() {
	preflightCmd.Flags().StringVar(&preflightVerb, "verb", "delete",
		"Action verb to evaluate (create/delete/patch/update/scale/apply)")
	preflightCmd.Flags().StringVar(&preflightKind, "kind", "pod",
		"Resource kind, e.g. pod, deployment, service")
	preflightCmd.Flags().StringVar(&preflightName, "name", "",
		"Resource name (may be empty for generic create flows)")
	preflightCmd.Flags().StringVarP(&preflightNamespace, "namespace", "n", "default",
		"Namespace the hypothetical action targets")
	preflightCmd.Flags().StringSliceVar(&preflightProtected,
		"protected",
		[]string{"kube-system", "kube-public", "kube-node-lease"},
		"Namespaces the ProtectedNamespaceCheck should block writes against")
	preflightCmd.Flags().StringVar(&preflightAuditFile, "audit-file", "",
		"Optional JSONL file to append the preflight audit event to")
	preflightCmd.Flags().BoolVar(&preflightNoCluster, "no-cluster", false,
		"Skip k8s client initialization (disables ResourceExistsCheck)")

	rootCmd.AddCommand(preflightCmd)
}

func runPreflight(_ *cobra.Command, _ []string) {
	// Build the chain. ProtectedNamespaceCheck is always on; it's the
	// whole point of this command in the demo. ResourceExistsCheck is
	// wired only when we have a working k8s client — otherwise the
	// chain's fail-closed default would turn the "not reachable"
	// cluster itself into a block, confusing the demo.
	chain := harness.NewPreflightChain().
		Add(harness.NewProtectedNamespaceCheck(preflightProtected...))

	if !preflightNoCluster {
		k8sClient, err := k8s.NewClient()
		if err != nil {
			fmt.Printf("(info) k8s client unavailable: %v\n", err)
			fmt.Println("(info) falling back to ProtectedNamespaceCheck only (pass --no-cluster to silence this).")
		} else {
			chain.Add(harness.NewResourceExistsCheck(k8sClient))
		}
	}

	// Console reporter so the output matches what operators see during
	// a real `fix` run — important for demo continuity.
	consoleSink := harness.NewConsoleReporter(os.Stdout)
	sinks := []harness.AuditLogger{consoleSink}

	if preflightAuditFile != "" {
		f, err := os.OpenFile(preflightAuditFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			fmt.Printf("Failed to open audit file %q: %v\n", preflightAuditFile, err)
			os.Exit(1)
		}
		defer f.Close()
		sinks = append(sinks, harness.NewJSONLogAuditor(f))
	}
	auditor := harness.NewTee(sinks...)

	req := harness.PreflightRequest{
		Verb:         preflightVerb,
		ResourceKind: preflightKind,
		ResourceName: preflightName,
		Namespace:    preflightNamespace,
	}

	fmt.Printf("\n=== kubeagent preflight ===\n")
	fmt.Printf("Verb:        %s\n", req.Verb)
	fmt.Printf("Target:      %s/%s in namespace %q\n",
		nonEmpty(req.ResourceKind, "<any>"),
		nonEmpty(req.ResourceName, "<unnamed>"),
		nonEmpty(req.Namespace, "default"))
	fmt.Printf("Protected:   %v\n", preflightProtected)
	fmt.Println()

	res := chain.Run(context.Background(), req)

	// Emit the preflight event via the same audit pipeline the real
	// write tools use, so the console output and the JSONL file line
	// up with what a `fix` run would have produced.
	event := harness.AuditEvent{
		Kind:    harness.AuditPreflight,
		Actor:   "preflight-cli",
		Action:  req.Verb,
		Outcome: string(res.Decision),
		Reason:  res.Reason,
		Target: harness.AuditTarget{
			Kind:      req.ResourceKind,
			Name:      req.ResourceName,
			Namespace: req.Namespace,
		},
	}
	if len(res.Warnings) > 0 {
		event.Details = map[string]interface{}{"warnings": res.Warnings}
	}
	if err := auditor.Record(context.Background(), event); err != nil {
		fmt.Fprintf(os.Stderr, "[audit] record failed: %v\n", err)
	}

	fmt.Println("\n========== Preflight Result ==========")
	fmt.Printf("Decision: %s\n", res.Decision)
	if res.Reason != "" {
		fmt.Printf("Reason:   %s\n", res.Reason)
	}
	if len(res.Warnings) > 0 {
		fmt.Println("Warnings:")
		for _, w := range res.Warnings {
			fmt.Println("  -", w)
		}
	}
	fmt.Println()

	// Exit code signals the decision so scripts can assert easily.
	//   0 = allow
	//   2 = block
	//   3 = warn-with-allow (only when someone pipes the output)
	switch res.Decision {
	case harness.PreflightBlock:
		os.Exit(2)
	case harness.PreflightWarn:
		os.Exit(3)
	}
}

// nonEmpty returns fallback when s is the empty string. Used for
// printing placeholders in summary lines.
func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
