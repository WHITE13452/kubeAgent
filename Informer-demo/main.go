package main

import (
	"Informer-demo/pkg/config"
	"Informer-demo/pkg/handlers"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

const (
	KUBECONFIG_PATH = "/Users/I765226/develop/sap/kubeconfigs/white-demo-kubeconfig.yaml"
)

func informer(lw *cache.ListWatch) {
	options := cache.InformerOptions{
		ListerWatcher: lw,
		ResyncPeriod:  0, // No resync
		ObjectType:    &v1.Pod{},
		Handler:       &handlers.PodHandler{},
	}

	_, informer := cache.NewInformerWithOptions(options)
	informer.Run(wait.NeverStop)
}

func sharedInformer(lw *cache.ListWatch) {

	sharedInformer := cache.NewSharedInformer(lw, &v1.Pod{}, 0)
	sharedInformer.AddEventHandler(&handlers.PodHandler{})
	sharedInformer.AddEventHandler(&handlers.NewPodHandler{})
	sharedInformer.Run(wait.NeverStop)
}

func sharedInformerWithFactory(lw *cache.ListWatch, client *kubernetes.Clientset) {
	fact := informers.NewSharedInformerFactoryWithOptions(
		client, 0, informers.WithNamespace(v1.NamespaceDefault),
	)
	podInformer := fact.Core().V1().Pods()
	podInformer.Informer().AddEventHandler(&handlers.PodHandler{})

	svcInformer := fact.Core().V1().Services()
	svcInformer.Informer().AddEventHandler(&handlers.SvcHandler{})

	fact.Start(wait.NeverStop)

}

func sharedInformerWithFactoryLister(lw *cache.ListWatch, client *kubernetes.Clientset) {
	fact := informers.NewSharedInformerFactoryWithOptions(
		client, 0, informers.WithNamespace(v1.NamespaceDefault),
	)
	podInformer := fact.Core().V1().Pods()
	podInformer.Informer().AddEventHandler(&handlers.PodHandler{})

	// ch := make(chan struct{})
	// fact.Start(ch)
	// fact.WaitForCacheSync(ch)
	fact.Start(wait.NeverStop)	
	pods, err := podInformer.Lister().List(labels.Everything())
	if err != nil {
		panic(err)
	}
	for _, pod := range pods {
		fmt.Println(pod)
	}
}

func SharedInformerFactoryListerWithGVR(lw *cache.ListWatch, client *kubernetes.Clientset) {
	fact := informers.NewSharedInformerFactoryWithOptions(
		client, 0, informers.WithNamespace(v1.NamespaceDefault),
	)

	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}
	informer, err := fact.ForResource(gvr)
	if err != nil {
		panic(err)
	}
	informer.Informer().AddEventHandler(&cache.ResourceEventHandlerFuncs{})

	ch := make(chan struct{})
	fact.Start(ch)
	fact.WaitForCacheSync(ch)
	pods, err := informer.Lister().List(labels.Everything())
	if err!= nil {
		panic(err)
	}

	for _, pod := range pods {
		fmt.Println(pod)
	}
}

func main() {
	client := config.NewK8sConfig().InitResetConfig(KUBECONFIG_PATH).InitClientSet()

	lw := cache.NewListWatchFromClient(client.CoreV1().RESTClient(), "pods", v1.NamespaceDefault, fields.Everything())

	// informer(lw)
	// sharedInformer(lw)
	// sharedInformerWithFactory(lw, client)
	sharedInformerWithFactoryLister(lw, client)
	// SharedInformerFactoryWithGVR(lw, client)
	select {}
}
