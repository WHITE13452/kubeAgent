package main

import (
	"ginK8s/pkg/config"
	"ginK8s/pkg/controller"
	"ginK8s/pkg/services"

	"github.com/gin-gonic/gin"
)

const (
	WHITE_KUBECONFIG_PATH = "/Users/I765226/develop/sap/kubeconfigs/white-demo-kubeconfig.yaml"
	DEV_KUBECONFIG_PATH    = "/Users/I765226/develop/go-workspace/kubeAgent/kubeconfig/rgm-dev.yaml"
)

func main() {
	K8sConfig := config.NewK8sConfig().InitResetConfig(DEV_KUBECONFIG_PATH)
	restMapper := K8sConfig.InitRESTMapper()
	dynamicClient := K8sConfig.InitDynamicClient()
	informer := K8sConfig.InitInformer()
	clinetSet := K8sConfig.InitClientSet()

	resourceContrller := controller.NewResourceController(
		services.NewResourceService(&restMapper, dynamicClient, informer),
	)
	podLogController := controller.NewPodLogEventController(
		services.NewPodLogEventService(clinetSet),
	)

	r := gin.New()

	r.GET("/:resourceName", resourceContrller.GetResourceList)
	r.POST("/:resourceName", resourceContrller.CreateResource)
	r.DELETE("/resource/:resourceName", resourceContrller.DeleteResource)
	r.GET("/get/gvr", resourceContrller.GetGVR)

	r.GET("/pods/logs", podLogController.GetPodLog)
	r.GET("/pods/events", podLogController.GetPodEventList)

	r.Run(":8080")
	
}