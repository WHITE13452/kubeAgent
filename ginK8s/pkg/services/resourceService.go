package services

import (
	"context"
	"fmt"
	"log"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ResourceService struct {
	restMapper *meta.RESTMapper                //	 资源映射器
	client     *dynamic.DynamicClient          // 动态客户端
	fact       informers.SharedInformerFactory // informers工厂
}

func NewResourceService(restMapper *meta.RESTMapper, client *dynamic.DynamicClient, fact informers.SharedInformerFactory) *ResourceService {
	return &ResourceService{
		restMapper: restMapper,
		client:     client,
		fact:       fact,
	}
}

func (r *ResourceService) GetResourceList(resourceOrKindArg string, namespace string) (interface{}, error) {
	restMapping, err := r.mappingFor(resourceOrKindArg, r.restMapper)
	if err != nil {
		return nil, fmt.Errorf("failed to get mapping for resource or kind %s: %v", resourceOrKindArg, err)
	}

	informer, err := r.fact.ForResource(restMapping.Resource)
	if err != nil {
		return nil, fmt.Errorf("failed to get informer for resource %s: %v", restMapping.Resource, err)
	}

	list, err := informer.Lister().ByNamespace(namespace).List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list resources for %s in namespace %s: %v", resourceOrKindArg, namespace, err)
	}

	return list, nil
}

func (r *ResourceService) CreateResource(resourceOrKindArg string, yaml string) error {
	obj := &unstructured.Unstructured{}
	_, _, err := scheme.Codecs.UniversalDeserializer().Decode([]byte(yaml), nil, obj)
	if err != nil {
		return err
	}
	ri, err := r.getResourceInterface(resourceOrKindArg, obj.GetNamespace(), r.client, r.restMapper)
	if err != nil {
		return fmt.Errorf("failed to get resource interface for %s: %v", resourceOrKindArg, err)
	}

	_, err = ri.Create(context.Background(), obj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create resource %s: %v", resourceOrKindArg, err)
	}
	return nil
}

func (r *ResourceService) getResourceInterface(resourceOrKindArg, namespace string, client dynamic.Interface, restMapper *meta.RESTMapper) (dynamic.ResourceInterface, error) {
	var ri dynamic.ResourceInterface
	restMapping, err := r.mappingFor(resourceOrKindArg, restMapper)
	if err != nil {
		return nil, fmt.Errorf("failed to get mapping for resource or kind %s: %v", resourceOrKindArg, err)
	}

	// 判断资源是命名空间级别还是集群级别的
	if restMapping.Scope.Name() == meta.RESTScopeNameNamespace {
		ri = client.Resource(restMapping.Resource).Namespace(namespace)
	} else {
		ri = client.Resource(restMapping.Resource)
	}
	return ri, nil
}

func (r *ResourceService) mappingFor(resourceOrKindArg string, restMapper *meta.RESTMapper) (*meta.RESTMapping, error) {
	// resourceOrKindArg做为resource
	fullySpecifiedGVR, groupResource := schema.ParseResourceArg(resourceOrKindArg)
	gvk := schema.GroupVersionKind{}

	if fullySpecifiedGVR != nil {
		gvk, _ = (*restMapper).KindFor(*fullySpecifiedGVR)
	}
	if gvk.Empty() {
		log.Println("mappingFor: groupResource", groupResource)
		gvk, _ = (*restMapper).KindFor(groupResource.WithVersion(""))
		log.Println("mappingFor: gvk", gvk)
	}
	if !gvk.Empty() {
		return (*restMapper).RESTMapping(gvk.GroupKind(), gvk.Version)
	}

	// resourceOrKindArg做为kind
	fullySpecifiedGVK, groupKind := schema.ParseKindArg(resourceOrKindArg)
	if fullySpecifiedGVK == nil {
		gvk := groupKind.WithVersion("")
		log.Println("mappingFor: gvk", gvk)
		fullySpecifiedGVK = &gvk
	}

	if !fullySpecifiedGVK.Empty() {
		if mapping, err := (*restMapper).RESTMapping(fullySpecifiedGVK.GroupKind(), fullySpecifiedGVK.Version); err == nil && mapping != nil {
			return mapping, nil
		}
	}
	
	mapping, err := (*restMapper).RESTMapping(groupKind, gvk.Version)
	if err != nil {
		if meta.IsNoMatchError(err) {
			log.Printf("mappingFor: the server doesn't have a resource type %q", groupResource.Resource)
			return nil, fmt.Errorf("the server doesn't have a resource type %q", groupResource.Resource)
		}
		return nil, err
	}

	return mapping, nil
}