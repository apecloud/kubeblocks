package kubernetes

import (
	"k8s.io/client-go/kubernetes"
	ctlruntime "sigs.k8s.io/controller-runtime"
)

// GetKubeClient returns a kubernetes client.
func GetClientSet() (*kubernetes.Clientset, error) {
	restConfig := ctlruntime.GetConfigOrDie()
	clientset, err := kubernetes.NewForConfig(conf)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}
