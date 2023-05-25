/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
