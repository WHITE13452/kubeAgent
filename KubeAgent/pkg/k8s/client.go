package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	yamlutil "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Client wraps Kubernetes client-go for direct cluster access.
// Supports both in-cluster (ServiceAccount) and out-of-cluster (kubeconfig) modes.
type Client struct {
	clientset     kubernetes.Interface
	dynamicClient dynamic.Interface
	restMapper    meta.RESTMapper
}

// NewClient creates a K8s client. It tries in-cluster config first,
// then falls back to KUBECONFIG env var or ~/.kube/config.
func NewClient() (*Client, error) {
	config, err := getRestConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
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

func getRestConfig() (*rest.Config, error) {
	// Try in-cluster config first (when running inside K8s)
	if config, err := rest.InClusterConfig(); err == nil {
		log.Println("[K8sClient] Using in-cluster config")
		return config, nil
	}

	// Fall back to kubeconfig file
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		kubeconfigPath = filepath.Join(homedir.HomeDir(), ".kube", "config")
	}
	log.Printf("[K8sClient] Using kubeconfig: %s\n", kubeconfigPath)
	return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
}

// ListResources lists K8s resources by type and namespace.
func (c *Client) ListResources(resource, namespace string) (string, error) {
	mapping, err := c.mappingFor(resource)
	if err != nil {
		return "", fmt.Errorf("failed to resolve resource '%s': %w", resource, err)
	}

	var ri dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		ri = c.dynamicClient.Resource(mapping.Resource).Namespace(namespace)
	} else {
		ri = c.dynamicClient.Resource(mapping.Resource)
	}

	list, err := ri.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list %s: %w", resource, err)
	}

	data, err := json.Marshal(list.Items)
	if err != nil {
		return "", fmt.Errorf("failed to marshal resource list: %w", err)
	}
	return string(data), nil
}

// GetPodLogs returns logs for a pod. For non-running pods, it attempts
// to retrieve logs from the previous container instance.
func (c *Client) GetPodLogs(podName, namespace, containerName string) (string, error) {
	if podName == "" {
		return "", fmt.Errorf("podName cannot be empty")
	}

	pod, err := c.clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get pod %s in namespace %s: %w", podName, namespace, err)
	}

	tailLines := int64(100)
	logOptions := &v1.PodLogOptions{
		Follow:    false,
		TailLines: &tailLines,
	}
	if containerName != "" {
		logOptions.Container = containerName
	}

	// For non-running pods, try previous container logs (useful for CrashLoopBackOff)
	if pod.Status.Phase == v1.PodPending {
		return fmt.Sprintf("Pod '%s' is in Pending state. Reason: %s. Use EventTool to check events.",
			podName, getPodStatusReason(pod)), nil
	}
	if pod.Status.Phase != v1.PodRunning {
		logOptions.Previous = true
	}

	req := c.clientset.CoreV1().Pods(namespace).GetLogs(podName, logOptions)
	stream, err := req.Stream(context.TODO())
	if err != nil {
		return "", fmt.Errorf("failed to get log stream for pod %s: %w", podName, err)
	}
	defer stream.Close()

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, stream); err != nil {
		return "", fmt.Errorf("failed to read log stream: %w", err)
	}
	return buf.String(), nil
}

// GetPodEvents returns warning events for a pod.
func (c *Client) GetPodEvents(podName, namespace string) (string, error) {
	listOptions := metav1.ListOptions{}
	if podName != "" {
		listOptions.FieldSelector = "involvedObject.kind=Pod,involvedObject.name=" + podName
	} else {
		listOptions.FieldSelector = "involvedObject.kind=Pod"
	}

	events, err := c.clientset.CoreV1().Events(namespace).List(context.TODO(), listOptions)
	if err != nil {
		return "", fmt.Errorf("failed to list events: %w", err)
	}

	var eventMessages []string
	for _, event := range events.Items {
		if event.Type == "Warning" {
			message := fmt.Sprintf("[%s] %s - %s",
				event.LastTimestamp.Format("2006-01-02 15:04:05"),
				event.InvolvedObject.Name,
				event.Message)
			eventMessages = append(eventMessages, message)
		}
	}

	data, err := json.Marshal(eventMessages)
	if err != nil {
		return "", fmt.Errorf("failed to marshal events: %w", err)
	}
	return string(data), nil
}

// CreateResource creates a K8s resource from YAML content.
func (c *Client) CreateResource(yamlContent string) (string, error) {
	obj := &unstructured.Unstructured{}
	dec := yamlutil.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	_, gvk, err := dec.Decode([]byte(yamlContent), nil, obj)
	if err != nil {
		return "", fmt.Errorf("failed to decode YAML: %w", err)
	}

	mapping, err := c.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return "", fmt.Errorf("failed to get REST mapping for %v: %w", gvk, err)
	}

	namespace := obj.GetNamespace()
	if namespace == "" {
		namespace = "default"
	}

	var ri dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		ri = c.dynamicClient.Resource(mapping.Resource).Namespace(namespace)
	} else {
		ri = c.dynamicClient.Resource(mapping.Resource)
	}

	created, err := ri.Create(context.TODO(), obj, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create resource: %w", err)
	}

	return fmt.Sprintf("Created %s/%s in namespace %s",
		created.GetKind(), created.GetName(), namespace), nil
}

// DeleteResource deletes a K8s resource.
func (c *Client) DeleteResource(resource, name, namespace string) (string, error) {
	mapping, err := c.mappingFor(resource)
	if err != nil {
		return "", fmt.Errorf("failed to resolve resource '%s': %w", resource, err)
	}

	var ri dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		ri = c.dynamicClient.Resource(mapping.Resource).Namespace(namespace)
	} else {
		ri = c.dynamicClient.Resource(mapping.Resource)
	}

	err = ri.Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to delete %s/%s: %w", resource, name, err)
	}
	return fmt.Sprintf("Deleted %s/%s in namespace %s", resource, name, namespace), nil
}

// ResourceState is a structured snapshot of a single resource, returned
// by GetResourceState. It is the input the harness Verifier consumes to
// decide whether an action's intended state has been reached.
type ResourceState struct {
	Kind      string
	Name      string
	Namespace string
	Exists    bool
	// Phase mirrors pod.status.phase for Pods, or a derived condition for
	// other kinds (e.g. "Available" for Deployments). Empty when not
	// applicable.
	Phase string
	// Reason carries a short machine-readable reason when the resource is
	// in a non-ready state (e.g. "CrashLoopBackOff").
	Reason string
	// Ready is a coarse boolean rollup: true when the resource is in its
	// healthy steady state.
	Ready bool
	// Raw exposes the underlying object for callers that need richer info.
	Raw *unstructured.Unstructured
}

// GetResourceState fetches a single resource and returns a structured
// state snapshot. When the resource does not exist, it returns
// (state, nil) with Exists=false rather than an error, so callers
// verifying a delete operation can treat "not found" as success.
func (c *Client) GetResourceState(resource, name, namespace string) (*ResourceState, error) {
	if resource == "" || name == "" {
		return nil, fmt.Errorf("resource and name are required")
	}

	mapping, err := c.mappingFor(resource)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve resource '%s': %w", resource, err)
	}

	var ri dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		ri = c.dynamicClient.Resource(mapping.Resource).Namespace(namespace)
	} else {
		ri = c.dynamicClient.Resource(mapping.Resource)
	}

	obj, err := ri.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		// Map "not found" to a structured non-existence rather than error,
		// so verification of a delete can succeed by reading absence.
		if isNotFoundError(err) {
			return &ResourceState{
				Kind:      mapping.GroupVersionKind.Kind,
				Name:      name,
				Namespace: namespace,
				Exists:    false,
			}, nil
		}
		return nil, fmt.Errorf("failed to get %s/%s: %w", resource, name, err)
	}

	state := &ResourceState{
		Kind:      obj.GetKind(),
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Exists:    true,
		Raw:       obj,
	}
	deriveReadiness(state, obj)
	return state, nil
}

// isNotFoundError loosely detects 404-style errors without pulling in the
// full apierrors package at every call site.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return bytes.Contains([]byte(msg), []byte("not found")) ||
		bytes.Contains([]byte(msg), []byte("NotFound"))
}

// deriveReadiness fills Phase/Reason/Ready on the given state based on
// the kind of object. Currently knows about Pod and Deployment; other
// kinds are reported as Ready=true if they exist (the verifier will
// fall back to Inconclusive when it can't say more).
func deriveReadiness(state *ResourceState, obj *unstructured.Unstructured) {
	switch state.Kind {
	case "Pod":
		phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
		state.Phase = phase
		state.Ready = phase == "Running"
		// Surface waiting/terminated reason for non-ready pods.
		statuses, _, _ := unstructured.NestedSlice(obj.Object, "status", "containerStatuses")
		for _, s := range statuses {
			cs, ok := s.(map[string]interface{})
			if !ok {
				continue
			}
			if waiting, ok, _ := unstructured.NestedMap(cs, "state", "waiting"); ok {
				if reason, _, _ := unstructured.NestedString(waiting, "reason"); reason != "" {
					state.Reason = reason
					state.Ready = false
					return
				}
			}
		}
	case "Deployment":
		// Ready when status.readyReplicas == spec.replicas.
		desired, _, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas")
		ready, _, _ := unstructured.NestedInt64(obj.Object, "status", "readyReplicas")
		state.Phase = fmt.Sprintf("%d/%d", ready, desired)
		state.Ready = desired > 0 && ready == desired
		if !state.Ready {
			state.Reason = "ReplicasNotReady"
		}
	default:
		// Unknown kinds: existence implies a best-effort "ready".
		state.Ready = true
	}
}

// mappingFor resolves a resource or kind name to a RESTMapping.
// Handles both resource names ("pods") and kind names ("Pod").
func (c *Client) mappingFor(resourceOrKind string) (*meta.RESTMapping, error) {
	fullySpecifiedGVR, groupResource := schema.ParseResourceArg(resourceOrKind)
	gvk := schema.GroupVersionKind{}

	if fullySpecifiedGVR != nil {
		gvk, _ = c.restMapper.KindFor(*fullySpecifiedGVR)
	}
	if gvk.Empty() {
		gvk, _ = c.restMapper.KindFor(groupResource.WithVersion(""))
	}
	if !gvk.Empty() {
		return c.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	}

	fullySpecifiedGVK, groupKind := schema.ParseKindArg(resourceOrKind)
	if fullySpecifiedGVK == nil {
		gvk := groupKind.WithVersion("")
		fullySpecifiedGVK = &gvk
	}

	if !fullySpecifiedGVK.Empty() {
		if mapping, err := c.restMapper.RESTMapping(fullySpecifiedGVK.GroupKind(), fullySpecifiedGVK.Version); err == nil {
			return mapping, nil
		}
	}

	return c.restMapper.RESTMapping(groupKind, gvk.Version)
}

func getPodStatusReason(pod *v1.Pod) string {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			return cs.State.Waiting.Reason
		}
	}
	return "Unknown"
}
