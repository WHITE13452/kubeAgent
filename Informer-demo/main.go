package main

import (
	"Informer-demo/pkg/config"
	"Informer-demo/pkg/handlers"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/apimachinery/pkg/fields"
)

func informer(lw *cache.ListWatch) {
	options := cache.InformerOptions{
		ListerWatcher: lw,
		ResyncPeriod: 0, // No resync
		ObjectType: &v1.Pod{},
		Handler: &handlers.PodHandler{},
	}

	_, informer := cache.NewInformerWithOptions(options)
	informer.Run(wait.NeverStop)

}

func main() {
	client := config.NewK8sConfig().InitResetConfig("/Users/I765226/develop/sap/kubeconfigs/white-demo-kubeconfig.yaml").InitClientSet()

	lw := cache.NewListWatchFromClient(client.CoreV1().RESTClient(), "pods", v1.NamespaceDefault, fields.Everything())

	informer(lw)
	select{}
}