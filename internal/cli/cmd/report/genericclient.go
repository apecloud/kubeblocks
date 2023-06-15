package report

import (
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	kubeblocks "github.com/apecloud/kubeblocks/pkg/client/clientset/versioned"
)

type genericClientSet struct {
	client      kubernetes.Interface
	dynamic     dynamic.Interface
	kbClientSet kubeblocks.Interface
}

func NewGenericClientSet(f cmdutil.Factory) (*genericClientSet, error) {
	client, err := f.KubernetesClientSet()
	if err != nil {
		return nil, err
	}
	dynamic, err := f.DynamicClient()
	if err != nil {
		return nil, err
	}
	config, err := f.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	kbClientSet, err := kubeblocks.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &genericClientSet{
		client:      client,
		dynamic:     dynamic,
		kbClientSet: kbClientSet,
	}, nil
}
