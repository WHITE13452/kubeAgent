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
	client *kubernetes.Clientset // Kubernetes client
}

func NewPodLogEventService(client *kubernetes.Clientset) *PodLogEventService {
	return &PodLogEventService{
		client: client,
	}
}

func (p *PodLogEventService) GetPodLog(podName, namespace string) *rest.Request {
	tailLine := int64(100) // Tail the last 100 lines of logs
	req := p.client.CoreV1().Pods(namespace).GetLogs(podName, &v1.PodLogOptions{Follow: false, TailLines: &tailLine})
	return req
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