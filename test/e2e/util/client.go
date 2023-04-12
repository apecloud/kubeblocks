/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"os"

	"github.com/vmware-tanzu/velero/pkg/client"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
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

func GetConfig() (*rest.Config, error) {
	kubeConfigPath, exists := os.LookupEnv("KUBECONFIG")
	if !exists {
		kubeConfigPath = os.ExpandEnv("$HOME/.kube/config")
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, err
	}
	return config, nil
}
