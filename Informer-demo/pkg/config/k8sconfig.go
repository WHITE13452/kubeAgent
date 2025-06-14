package config

import (
	"path/filepath"

	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type K8sConfig struct {
	*rest.Config
	*kubernetes.Clientset
	e error
}

func NewK8sConfig() *K8sConfig {
	return &K8sConfig{}
}

func (k *K8sConfig) InitResetConfig(kubeconfigPath string) *K8sConfig {
	if kubeconfigPath == "" {
        kubeconfigPath = filepath.Join(homedir.HomeDir(), ".kube", "config")
    }
    
    config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
    if err != nil {
        k.e = errors.Wrap(err, "failed to build kubeconfig")
        return k
    }

    k.Config = config
    return k
}

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

func (k *K8sConfig) Error() error {
	return k.e
}