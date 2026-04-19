package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"kubeagent/pkg/agent"
	"kubeagent/pkg/agent/harness"
	"kubeagent/pkg/agent/skills"
	"kubeagent/pkg/agent/specialists"
	"kubeagent/pkg/k8s"
	pkgtools "kubeagent/pkg/tools"
)

// Flags for `kubeagent fix`. Kept package-private; cobra's Run closure
// reads them after Cobra has populated them from argv.
var (
	fixPod         string
	fixNamespace   string
	fixDescription string
	fixAuditFile   string
	fixProtected   []string
	fixNoVerify    bool
)

// fixCmd is the closed-loop remediation entry point.
//
// Differences vs `analyze`:
//   - Wires both Diagnostician (read-only) and Remediator (write) into
//     the Coordinator, so a single request can flow diagnose → remediate.
//   - Plugs the harness Sensors (Verifier + AuditLogger) into the
//     Remediator: every action is followed by a real cluster-state check
//     and an entry in the audit trail.
//   - Renders progress to the operator's terminal via ConsoleReporter
//     while simultaneously persisting structured JSONL via JSONLogAuditor
//     when --audit-file is set. Tee fans out to both.
//   - Loads system prompts from pkg/agent/skills/*.md (overridable at
//     runtime via the SKILLS_DIR env var).
//
// Demo intent: a single command an operator can run during a live
// failure that shows Guides → Action → Sensors in real time.
var fixCmd = &cobra.Command{
	Use:   "fix",
	Short: "Diagnose and remediate a Kubernetes resource with closed-loop verification",
	Long: `Run the full diagnose → remediate → verify loop on a Kubernetes resource.

Examples:
  # One-shot fix of a known pod, with audit trail:
  kubeagent fix --pod nginx-1 -n default --audit-file /tmp/audit.jsonl

  # Free-form description (let the LLM figure out the target):
  kubeagent fix --description "the redis pod in cache namespace keeps crashing"

  # Dry-run style: skip post-action verification (not recommended for prod):
  kubeagent fix --pod nginx-1 --no-verify`,
	Run: runFix,
}

func init() {
	fixCmd.Flags().StringVar(&fixPod, "pod", "", "Pod name to remediate (optional if --description is rich enough)")
	fixCmd.Flags().StringVarP(&fixNamespace, "namespace", "n", "default", "Namespace of the target resource")
	fixCmd.Flags().StringVar(&fixDescription, "description", "", "Free-form description of the issue (defaults to a generic prompt when --pod is set)")
	fixCmd.Flags().StringVar(&fixAuditFile, "audit-file", "", "Path to a JSONL audit log file (also tees to console)")
	fixCmd.Flags().StringSliceVar(&fixProtected, "protected", []string{"kube-system", "kube-public", "kube-node-lease"}, "Namespaces that must never be mutated")
	fixCmd.Flags().BoolVar(&fixNoVerify, "no-verify", false, "Skip post-action verification (debug only)")

	rootCmd.AddCommand(fixCmd)
}

func runFix(cmd *cobra.Command, args []string) {
	if fixPod == "" && fixDescription == "" {
		fmt.Println("Either --pod or --description is required.")
		os.Exit(1)
	}

	logger := agent.NewSimpleLogger("KubeAgent")
	stateStore := agent.NewMemoryStateStore()

	llmClient, err := agent.NewAnthropicLLMClient(nil)
	if err != nil {
		fmt.Printf("Failed to initialize LLM client: %v\n", err)
		os.Exit(1)
	}

	k8sClient, err := k8s.NewClient()
	if err != nil {
		fmt.Printf("Failed to initialize K8s client: %v\n", err)
		os.Exit(1)
	}

	// Skills: load embedded markdown prompts, allow runtime override via
	// SKILLS_DIR (operators can drop a tweaked diagnose.md without
	// rebuilding).
	skillRegistry, err := harness.NewSkillsFromFS(skills.FS())
	if err != nil {
		fmt.Printf("Failed to load skills: %v\n", err)
		os.Exit(1)
	}
	if dir := os.Getenv("SKILLS_DIR"); dir != "" {
		skillRegistry = skillRegistry.WithOverrideDir(dir)
	}

	// Audit sink: ConsoleReporter is always on (operator-facing), JSON
	// auditor only when --audit-file is set. Tee fans out to both.
	consoleSink := harness.NewConsoleReporter(os.Stdout)
	var auditSinks []harness.AuditLogger
	auditSinks = append(auditSinks, consoleSink)

	if fixAuditFile != "" {
		f, err := os.OpenFile(fixAuditFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			fmt.Printf("Failed to open audit file %q: %v\n", fixAuditFile, err)
			os.Exit(1)
		}
		defer f.Close()
		auditSinks = append(auditSinks, harness.NewJSONLogAuditor(f))
	}
	auditor := harness.NewTee(auditSinks...)

	// Verifier: nil when --no-verify so the remediator falls back to its
	// noop default. The point of --no-verify is to make this *visible*
	// rather than silent.
	var verifier harness.Verifier
	if !fixNoVerify {
		verifier = harness.NewK8sVerifier(k8sClient)
	}

	coordinator := agent.NewCoordinator(nil, llmClient, stateStore, logger)

	// Diagnostician: read-only tools.
	diagnostician := specialists.NewDiagnosticianAgent(llmClient, logger).
		WithSkills(skillRegistry)
	diagnostician.AddTool(pkgtools.NewLogTool(k8sClient))
	diagnostician.AddTool(pkgtools.NewEventTool(k8sClient))
	diagnostician.AddTool(pkgtools.NewListTool(k8sClient))
	diagnostician.AddTool(pkgtools.NewKubeTool())
	if err := coordinator.RegisterAgent(diagnostician); err != nil {
		fmt.Printf("Failed to register diagnostician: %v\n", err)
		os.Exit(1)
	}

	// Preflight chain: protected namespaces + resource-existence invariants.
	// Fail-closed so a flaky cluster read never silently allows a write.
	preflight := harness.NewPreflightChain().
		Add(harness.NewProtectedNamespaceCheck(fixProtected...)).
		Add(harness.NewResourceExistsCheck(k8sClient))

	// Remediator: write tools + closed-loop sensors.
	remediator := specialists.NewRemediatorAgent(llmClient, logger).
		WithSkills(skillRegistry).
		WithAuditor(auditor).
		WithVerifier(verifier)
	// Remediator tool set is deliberately NARROW:
	//   - HumanTool     : approvals for dangerous writes
	//   - CreateTool    : submit full YAML (also the "patch" path)
	//   - DeleteTool    : delete a resource, letting controllers rebuild
	//
	// KubeTool is intentionally NOT registered here. Including it
	// tempted the LLM to reach for `kubectl patch` — which KubeTool
	// rejects (read-only whitelist) — causing the tool loop to spin
	// until iteration cap. Read-only state lookup is Diagnostician's
	// job; by this point its report is already in `task.Input`.
	remediator.AddTool(pkgtools.NewHumanTool())
	remediator.AddTool(pkgtools.NewCreateTool(k8sClient).
		WithPreflight(preflight).
		WithAuditor(auditor))
	remediator.AddTool(pkgtools.NewDeleteTool(k8sClient).
		WithPreflight(preflight).
		WithAuditor(auditor))
	if err := coordinator.RegisterAgent(remediator); err != nil {
		fmt.Printf("Failed to register remediator: %v\n", err)
		os.Exit(1)
	}

	// Build the user request. We synthesize a description when the
	// operator only supplied --pod / -n, so the Coordinator's planner
	// has enough material to decompose the work.
	input := buildFixDescription(fixPod, fixNamespace, fixDescription)

	ctx := agent.NewAgentContext(
		context.Background(),
		uuid.New().String(),
		"cli-user",
		uuid.New().String(),
	)
	request := &agent.Request{
		ID:    uuid.New().String(),
		User:  "cli-user",
		Input: input,
	}

	fmt.Printf("\n=== kubeagent fix ===\n")
	fmt.Printf("Target:      %s/%s\n", defaultIfEmpty(fixNamespace, "default"), defaultIfEmpty(fixPod, "(from description)"))
	fmt.Printf("Verifier:    %s\n", verifierLabel(verifier))
	fmt.Printf("Audit file:  %s\n", defaultIfEmpty(fixAuditFile, "(console only)"))
	fmt.Printf("Protected:   %s\n", strings.Join(fixProtected, ", "))
	fmt.Println()

	plan, err := coordinator.Plan(ctx, request)
	if err != nil {
		fmt.Printf("Planning failed: %v\n", err)
		os.Exit(1)
	}

	response, err := coordinator.ExecutePlan(ctx, plan)
	if err != nil {
		fmt.Printf("Execution failed: %v\n", err)
		// Don't exit non-zero immediately — we still want to print the
		// partial result so the operator can see what got tried.
	}

	fmt.Println("\n========== Fix Result ==========")
	if response != nil {
		fmt.Println(response.Result)
		if len(response.Errors) > 0 {
			fmt.Println("\nErrors encountered:")
			for _, e := range response.Errors {
				fmt.Println(" -", e)
			}
		}
	}
	fmt.Println()
}

// buildFixDescription composes a user-facing input string. Three modes:
//   - Description only: pass through verbatim.
//   - Pod only: synthesize a generic ask so the planner has a target.
//   - Both: prepend the pod context to the description so the LLM keeps
//     the concrete identifier in its working memory.
func buildFixDescription(pod, namespace, description string) string {
	switch {
	case pod == "" && description != "":
		return description
	case pod != "" && description == "":
		return fmt.Sprintf("Diagnose and remediate the issue with pod %q in namespace %q.",
			pod, defaultIfEmpty(namespace, "default"))
	default:
		return fmt.Sprintf("Pod %q in namespace %q. %s",
			pod, defaultIfEmpty(namespace, "default"), description)
	}
}

func defaultIfEmpty(s, dflt string) string {
	if s == "" {
		return dflt
	}
	return s
}

func verifierLabel(v harness.Verifier) string {
	if v == nil {
		return "DISABLED (--no-verify)"
	}
	return "K8sVerifier (closed loop)"
}
