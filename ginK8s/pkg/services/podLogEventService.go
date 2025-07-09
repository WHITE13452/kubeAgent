package services

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type PodLogEventService struct {
	client *kubernetes.Clientset 
}

func NewPodLogEventService(client *kubernetes.Clientset) *PodLogEventService {
	return &PodLogEventService{
		client: client,
	}
}

func (p *PodLogEventService) GetPodLog(podName, namespace, containerName string) (*rest.Request, error) {

	if podName == "" {
		return nil, fmt.Errorf("podName cannot be empty")
	}

	pod, err := p.client.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod %s in namespace %s: %v", podName, namespace, err)
	}
	if pod.Status.Phase != v1.PodRunning {
		return nil, fmt.Errorf("pod '%s' is in '%s' state, not running. Reason: %s", 
            podName, pod.Status.Phase, getPodStatusReason(pod))
	}

	tailLine := int64(100)
    logOptions := &v1.PodLogOptions{
        Follow:    false, 
        TailLines: &tailLine,
    }

	if containerName != "" {
        logOptions.Container = containerName
    }
    
    req := p.client.CoreV1().Pods(namespace).GetLogs(podName, logOptions)
    return req, nil
}

func getPodStatusReason(pod *v1.Pod) string {
    for _, containerStatus := range pod.Status.ContainerStatuses {
        if containerStatus.State.Waiting != nil {
            return containerStatus.State.Waiting.Reason
        }
    }
    return "Unknown"
}

func (p *PodLogEventService) GetEventList(podName, namespace string) ([]string, error) {
	listOptions := metav1.ListOptions{}
	if podName != "" {
		fieldSelector := "involvedObject.kind=Pod,involvedObject.name=" + podName
		listOptions.FieldSelector = fieldSelector
	} else {
		listOptions.FieldSelector = "involvedObject.kind=Pod"
	}

	events, err := p.client.CoreV1().Events(namespace).List(context.TODO(), listOptions)
	if err != nil {
		return nil, err
	}
	fmt.Println("Event List:", events)

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
	return eventMessages, nil
}

