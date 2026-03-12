package k8s

import (
	"fmt"
	"os"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

// AgentType represents the type of agent that needs a K8s client
type AgentType string

const (
	AgentTypeCoordinator   AgentType = "coordinator"
	AgentTypeDiagnostician AgentType = "diagnostician"
	AgentTypeRemediator   AgentType = "remediator"
	AgentTypeSecurity     AgentType = "security"
	AgentTypeDefault      AgentType = "default"
)

// AgentServiceAccount holds the service account info for each agent type
var AgentServiceAccounts = map[AgentType]string{
	AgentTypeCoordinator:   "kubeagent-coordinator",
	AgentTypeDiagnostician: "kubeagent-diagnostician",
	AgentTypeRemediator:    "kubeagent-remediator",
	AgentTypeSecurity:      "kubeagent-security",
	AgentTypeDefault:       "kubeagent",
}

// GetServiceAccountName returns the service account name for a given agent type
func GetServiceAccountName(agentType AgentType) string {
	if sa, ok := AgentServiceAccounts[agentType]; ok {
		return sa
	}
	return AgentServiceAccounts[AgentTypeDefault]
}

// NewClientForAgent creates a K8s client configured for a specific agent type.
// It uses the service account token mounted in the pod for authentication.
// 
// In production, each agent pod should have its own ServiceAccount with appropriate
// RBAC permissions. This client reads the token from the mounted service account
// secret and uses it for authentication.
func NewClientForAgent(agentType AgentType) (*Client, error) {
	namespace := os.Getenv("KUBEAGENT_NAMESPACE")
	if namespace == "" {
		namespace = "kubeagent-system"
	}

	config, err := getRestConfigForAgent(agentType, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes config for agent %s: %w", agentType, err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset for agent %s: %w", agentType, err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client for agent %s: %w", agentType, err)
	}

	gr, err := restmapper.GetAPIGroupResources(clientset.Discovery())
	if err != nil {
		return nil, fmt.Errorf("failed to get API group resources: %w", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(gr)

	return &Client{
		clientset:     clientset,
		dynamicClient: dynamicClient,
		restMapper:    mapper,
	}, nil
}

// getRestConfigForAgent creates a REST config that uses the service account token
// mounted in the pod. 
//
// The standard approach is to:
// 1. Use the token from the mounted service account secret
// 2. Let Kubernetes handle the RBAC based on that SA's permissions
//
// For more advanced scenarios (single pod with multiple SAs), you can use
// Impersonation, but that requires additional RBAC setup.
func getRestConfigForAgent(agentType AgentType, namespace string) (*rest.Config, error) {
	// First try standard in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig for local development
		return getRestConfig()
	}

	// Get the service account name for this agent type
	saName := GetServiceAccountName(agentType)

	// Try to read the service account token
	// In production, each agent should run with its own SA
	// The token path is: /var/run/secrets/kubernetes.io/serviceaccount/<sa-name>/token
	// But we use the default mounted token which is for the pod's SA
	
	// For production deployment with separate pods per agent:
	// - Each agent runs in its own Pod with its own ServiceAccount
	// - The mounted token is for that specific ServiceAccount
	// - RBAC is automatically applied based on the SA's roles/clusterroles
	
	// For the default service account, use the mounted token
	tokenFile := "/var/run/secrets/kubernetes.io/serviceaccount/token"
	caFile := "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

	// Check if we're running in cluster with a dedicated SA
	if _, err := os.Stat(tokenFile); err == nil {
		// Token file exists, use it
		tokenData, err := os.ReadFile(tokenFile)
		if err == nil {
			config.BearerToken = string(tokenData)
			// Clear the token file to avoid using it instead of the explicit token
			config.BearerTokenFile = ""
		}
	}

	// Impersonation: if we want to use a different SA's permissions
	// This requires the pod's SA to have permission to impersonate other SAs
	// Uncomment if you need this advanced feature:
	/*
	config.Impersonate = rest.ImpersonationConfig{
		UserName: fmt.Sprintf("system:serviceaccount:%s:%s", namespace, saName),
	}
	*/

	_ = saName // saName available for future use with impersonation
	_ = caFile // caFile available for future use

	return config, nil
}
