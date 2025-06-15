package handlers

import (
	"fmt"
	v1 "k8s.io/api/core/v1"
)

type PodHandler struct {
}

func (p *PodHandler) OnAdd(obj interface{}, isInInitialList bool) {
	// Handle pod addition logic here
	fmt.Println("PodHandler OnAdd: ", obj.(*v1.Pod).Name)
}
func (p *PodHandler) OnUpdate(oldObj, newObj interface{}) {
	// Handle pod update logic here
	fmt.Println("PodHandler OnUpdate: ", oldObj.(*v1.Pod).Name, " -> ", newObj.(*v1.Pod).Name)
}
func (p *PodHandler) OnDelete(obj interface{}) {
	// Handle pod deletion logic here
	fmt.Println("PodHandler OnDelete: ", obj.(*v1.Pod).Name)
}

type NewPodHandler struct {

}

func (n *NewPodHandler) OnAdd(obj interface{}, isInInitialList bool) {
	// Handle pod addition logic here
	fmt.Println("NewPodHandler OnAdd: ", obj.(*v1.Pod).Name)
}
func (n *NewPodHandler) OnUpdate(oldObj, newObj interface{}) {
	// Handle pod update logic here
	fmt.Println("NewPodHandler OnUpdate: ", oldObj.(*v1.Pod).Name, " -> ", newObj.(*v1.Pod).Name)
}
func (n *NewPodHandler) OnDelete(obj interface{}) {
	// Handle pod deletion logic here
	fmt.Println("NewPodHandler OnDelete: ", obj.(*v1.Pod).Name)
}
