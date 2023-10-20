package kubernetes

import (
	"errors"

	"github.com/go-logr/logr"
	"k8s.io/client-go/kubernetes"
	ctlruntime "sigs.k8s.io/controller-runtime"

	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

// GetClientSet returns a kubernetes clientset.
func GetClientSet(logger logr.Logger) (*kubernetes.Clientset, error) {
	restConfig, err := ctlruntime.GetConfig()
	if err != nil {
		logger.Error(err, "kubeconfig not found")
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}

// GetRESTClient returns a kubernetes restclient for KubeBlocks types.
func GetRESTClient(logger logr.Logger) (*rest.RESTClient, error) {
	restConfig, err := ctlruntime.GetConfig()
	if err != nil {
		logger.Error(err, "kubeconfig not found")
		return nil, errors.New("kubeconfig not found")
	}
	_ = appsv1alpha1.AddToScheme(clientsetscheme.Scheme)
	restConfig.GroupVersion = &appsv1alpha1.GroupVersion
	restConfig.APIPath = "/apis"
	restConfig.NegotiatedSerializer = clientsetscheme.Codecs.WithoutConversion()
	client, err := rest.RESTClientFor(restConfig)
	if err != nil {
		return nil, err
	}

	return client, nil
}
