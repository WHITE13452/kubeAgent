package handlers

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
)


type SvcHandler struct {
	
}

func (s *SvcHandler) OnAdd(obj interface{}, isInInitialList bool) {
	// Handle service addition logic here
	fmt.Println("Service added: ", obj.(*v1.Service).Name)
}
func (s *SvcHandler) OnUpdate(oldObj, newObj interface{}) {
	// Handle service update logic here
	fmt.Println("Service updated: ", oldObj.(*v1.Service).Name, " -> ", newObj.(*v1.Service).Name)
}
func (s *SvcHandler) OnDelete(obj interface{}) {
	// Handle service deletion logic here
	fmt.Println("Service deleted: ", obj.(*v1.Service).Name)
}