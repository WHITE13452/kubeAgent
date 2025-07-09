package config

import (
	"path/filepath"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type K8sConfig struct {
	*rest.Config                          // Config 保存连接到 Kubernetes 集群所需的认证信息和服务器配置
	*kubernetes.Clientset                 // Clientset 是标准的 Kubernetes 客户端集合，提供对所有核心 API 资源的访问
	*dynamic.DynamicClient                // DynamicClient 是动态类型客户端，可与任何 API 资源交互，包括自定义资源（CRD）
	meta.RESTMapper                       // RESTMapper 用于在 GroupVersionKind (GVK) 和 GroupVersionResource (GVR) 之间进行映射
	informers.SharedInformerFactory       // SharedInformerFactory 是共享的 Informer 工厂，用于创建和管理资源监听器
	e                               error // e 用于存储在初始化过程中可能发生的错误
}

type K8sConfigOptionFunc func(k *K8sConfig)

// WithQps 是一个配置选项函数生成器，用于设置 Kubernetes 客户端的每秒查询数（QPS）。
// 参数 qps 表示要设置的每秒查询数，类型为 float32。
// 返回一个 K8sConfigOptionFunc 函数，该函数可用于在初始化 K8sConfig 时应用 QPS 设置。
func WithQps(qps float32) K8sConfigOptionFunc {
	return func(k *K8sConfig) {
		if k.Config != nil {
			k.QPS = qps
		}
	}
}

// WithBurst 是一个配置选项函数生成器，用于设置 Kubernetes 客户端的突发请求数（Burst）。
// 参数 burst 表示要设置的突发请求数，类型为 int。
func WithBurst(burst int) K8sConfigOptionFunc {
	return func(k *K8sConfig) {
		if k.Config != nil {
			k.Burst = burst
		}
	}
}

func NewK8sConfig() *K8sConfig {
	return &K8sConfig{}
}

// InitConfig 初始化 Kubernetes 配置，使用默认的 kubeconfig 路径
func (k *K8sConfig) InitResetConfig(kubeconfigPath string, optfuncs ...K8sConfigOptionFunc) *K8sConfig {
	if kubeconfigPath == "" {
		kubeconfigPath = filepath.Join(homedir.HomeDir(), ".kube", "config")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		k.e = errors.Wrap(err, "failed to build kubeconfig")
		return k
	}

	k.Config = config

	for _, optfunc := range optfuncs {
		optfunc(k)
	}

	return k
}

func (k *K8sConfig) InitConfigInCluster() *K8sConfig {
	config, err := rest.InClusterConfig()
	if err != nil {
		k.e = errors.Wrap(errors.New("k8s config is nil"), "init k8s client failed")
		return k
	}

	k.Config = config
	return k
}

// 初始化clientSet客户端
func (k *K8sConfig) InitClientSet() *kubernetes.Clientset {
	if k.Config == nil {
		k.e = errors.Wrap(errors.New("k8s config is nil"), "init k8s client failed")
		return nil
	}
	clientSet, e := kubernetes.NewForConfig(k.Config)
	if e != nil {
		k.e = errors.Wrap(e, "failed to create k8s clientset")
		return nil
	}

	return clientSet
}

func (k *K8sConfig) InitDynamicClient() *dynamic.DynamicClient {
	if k.Config == nil {
		k.e = errors.Wrap(errors.New("k8s config is nil"), "init k8s client failed")
		return nil
	}
	dynamicClient, e := dynamic.NewForConfig(k.Config)
	if e!= nil {
		k.e = errors.Wrap(e, "failed to create k8s dynamic client")
		return nil
	}
	return dynamicClient
}

func (k *K8sConfig) InitRESTMapper() meta.RESTMapper {
	clientSet := k.InitClientSet()
    if clientSet == nil {
        k.e = errors.Wrap(k.e, "InitRESTMapper failed: clientSet is nil")
        return nil
    }
    
    discovery := clientSet.Discovery()
    if discovery == nil {
        k.e = errors.Wrap(errors.New("discovery client is nil"), "InitRESTMapper failed")
        return nil
    }

	gr, err := restmapper.GetAPIGroupResources(discovery)
	if err != nil {
		k.e = errors.Wrap(err, "InitRESTMapper:failed to get API group resources")
		return nil
	}
	mapper := restmapper.NewDiscoveryRESTMapper(gr)
	if mapper == nil {
		k.e = errors.New("InitRESTMapper: failed to create RESTMapper")
		return nil
	}
	return mapper
}

func (k *K8sConfig) InitInformer() informers.SharedInformerFactory {
	if k.Config == nil {
		k.e = errors.Wrap(errors.New("k8s config is nil"), "init k8s client failed")
		return nil
	}
	// 通用informer工厂
	fact := informers.NewSharedInformerFactory(k.InitClientSet(), 0)
	informer := fact.Core().V1().Pods() // 监听Pod资源
	informer.Informer().AddEventHandler(&cache.ResourceEventHandlerFuncs{})

	ch := make(chan struct{})
	fact.Start(ch)
	fact.WaitForCacheSync(ch)
	
	return fact
}

func (k *K8sConfig) Error() error {
	return k.e
}
