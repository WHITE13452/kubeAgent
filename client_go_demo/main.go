package main

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

func InitRestMapper(clientSet *kubernetes.Clientset)  meta.RESTMapper{
	gr, err := restmapper.GetAPIGroupResources(clientSet.Discovery())
	fmt.Println("gr",gr)
	if err != nil {
		panic(err)
	}

	mapper := restmapper.NewDiscoveryRESTMapper(gr)
	fmt.Println("mapper",mapper)
	return mapper
}

func mappingFor(resourceOrKindArg string, restMapper *meta.RESTMapper) (*meta.RESTMapping, error) {
	// resourceOrKindArg做为kind
	fullySpecifiedGVK, groupKind := schema.ParseKindArg(resourceOrKindArg)
	fmt.Println("Trying as Kind - fullySpecifiedGVK:", fullySpecifiedGVK)
    fmt.Println("Trying as Kind - groupKind:", groupKind)
	if fullySpecifiedGVK != nil {
		return (*restMapper).RESTMapping(fullySpecifiedGVK.GroupKind(), fullySpecifiedGVK.Version)
	}
	if !groupKind.Empty() {
		mapping, err := (*restMapper).RESTMapping(groupKind)
		fmt.Println("Trying as Kind - mapping:", mapping)
		if err != nil && mapping != nil {
			return mapping, nil
		}
	}

	// resourceOrKindArg做为resource
	fullySpecifiedGVR, groupResource := schema.ParseResourceArg(resourceOrKindArg)
	gvk := schema.GroupVersionKind{}
	fmt.Println("fullySpecifiedGVR", fullySpecifiedGVR)
	fmt.Println("groupResource", groupResource)
	if fullySpecifiedGVR != nil {
		gvk, _ = (*restMapper).KindFor(*fullySpecifiedGVR)
	}
	if gvk.Empty() {
		gvk, _ = (*restMapper).KindFor(groupResource.WithVersion(""))
	}
	if !gvk.Empty() {
		return (*restMapper).RESTMapping(gvk.GroupKind(), gvk.Version)
	}
	return nil, nil
}

func main() {
	config, err := clientcmd.BuildConfigFromFlags("", "/Users/I765226/develop/sap/kubeconfigs/white-demo-kubeconfig.yaml")
	if err != nil {
		panic(err)
	}
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	restMapper := InitRestMapper(clientSet)
	resourceOrKind := "Deployment.v1.apps" 
	
	dynamicClient, err := dynamic.NewForConfig(config)
	fmt.Println("dynamicClient",dynamicClient)
	if err != nil {
		panic(err)
	}

	var ri dynamic.ResourceInterface

	restmapping, err := mappingFor(resourceOrKind, &restMapper)
	fmt.Println("Resource Mapping:", restmapping)
	if err != nil {
		panic(err)
	}
	if restmapping.Scope.Name() == "namespace" {
		ri = dynamicClient.Resource(restmapping.Resource).Namespace("default")
	} else {
		ri = dynamicClient.Resource(restmapping.Resource)
	}

	resources, err := ri.List(context.TODO(), metav1.ListOptions{})
	fmt.Println("Resources:", resources)
	if err != nil {
		panic(err)
	}
	for _, item := range resources.Items {
		fmt.Printf("Resource Name: %s\n", item.GetName())
	}
}