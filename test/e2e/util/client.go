package util

import (
	"github.com/vmware-tanzu/velero/pkg/client"
	"k8s.io/client-go/kubernetes"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// TestClient defines the desired type of Client
type TestClient struct {
	Kubebuilder    kbclient.Client
	ClientGo       kubernetes.Interface
	dynamicFactory client.DynamicFactory
}

// NewTestClient returns a set of ready-to-use API clients.
func NewTestClient(kubecontext string) (TestClient, error) {
	return InitTestClient(kubecontext)
}

// InitTestClient init different type clients
func InitTestClient(kubecontext string) (TestClient, error) {
	config, err := client.LoadConfig()
	if err != nil {
		return TestClient{}, err
	}
	f := client.NewFactory("e2e", kubecontext, config)
	clientGo, err := f.KubeClient()
	if err != nil {
		return TestClient{}, err
	}
	kb, err := f.KubebuilderClient()
	if err != nil {
		return TestClient{}, err
	}
	dynamicClient, err := f.DynamicClient()
	if err != nil {
		return TestClient{}, err
	}
	factory := client.NewDynamicFactory(dynamicClient)
	return TestClient{
		Kubebuilder:    kb,
		ClientGo:       clientGo,
		dynamicFactory: factory,
	}, nil
}
