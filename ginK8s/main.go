package main

import (
	"ginK8s/pkg/config"
	"ginK8s/pkg/controller"
	"ginK8s/pkg/services"

	"github.com/gin-gonic/gin"
)

const (
	KUBECONFIG_PATH = "/Users/I765226/develop/sap/kubeconfigs/white-demo-kubeconfig.yaml"
)

func main() {
	K8sConfig := config.NewK8sConfig().InitResetConfig(KUBECONFIG_PATH)
	restMapper := K8sConfig.InitRESTMapper()
	dynamicClient := K8sConfig.InitDynamicClient()
	informer := K8sConfig.InitInformer()

	// clinetSet := K8sConfig.InitClientSet()

	resourceContrller := controller.NewResourceController(
		services.NewResourceService(&restMapper, dynamicClient, informer),
	)

	r := gin.New()

	r.GET("/:resourceName", resourceContrller.GetResourceList)
	r.POST("/:resourceName", resourceContrller.CreateResource)
	r.DELETE("/resource/:resourceName", resourceContrller.DeleteResource)
	r.GET("/get/gvr", resourceContrller.GetGVR)

	r.Run(":8080")
	
}