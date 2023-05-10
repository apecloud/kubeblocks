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

package testing

import (
	"bytes"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

// NewTestFactory like cmdtesting.NewTestFactory, register KubeBlocks custom objects
func NewTestFactory(namespace string) *cmdtesting.TestFactory {
	tf := cmdtesting.NewTestFactory()
	mapper := restmapper.NewDiscoveryRESTMapper(testDynamicResources())
	clientConfig := testClientConfig()
	cf := genericclioptions.NewTestConfigFlags().WithRESTMapper(mapper).
		WithClientConfig(clientConfig).WithNamespace(namespace)
	tf.Factory = cmdutil.NewFactory(cf)

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		panic(fmt.Sprintf("unable to create a fake restclient config: %v", err))
	}
	tf.ClientConfigVal = restConfig

	return tf.WithClientConfig(clientConfig).WithNamespace(namespace)
}

func testClientConfig() clientcmd.ClientConfig {
	tmpFile, err := os.CreateTemp(os.TempDir(), "cmdtests_temp")
	if err != nil {
		panic(fmt.Sprintf("unable to create a fake client config: %v", err))
	}

	loadingRules := &clientcmd.ClientConfigLoadingRules{
		Precedence:     []string{tmpFile.Name()},
		MigrationRules: map[string]string{},
	}

	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmdapi.Cluster{Server: "http://localhost:8080"}}
	fallbackReader := bytes.NewBuffer([]byte{})
	return clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, overrides, fallbackReader)
}

func testDynamicResources() []*restmapper.APIGroupResources {
	return []*restmapper.APIGroupResources{
		{
			Group: metav1.APIGroup{
				Versions: []metav1.GroupVersionForDiscovery{
					{Version: "v1"},
				},
				PreferredVersion: metav1.GroupVersionForDiscovery{Version: "v1"},
			},
			VersionedResources: map[string][]metav1.APIResource{
				"v1": {
					{Name: "pods", Namespaced: true, Kind: "Pod"},
					{Name: "services", Namespaced: true, Kind: "Service"},
					{Name: "nodes", Namespaced: false, Kind: "Node"},
					{Name: "secrets", Namespaced: true, Kind: "Secret"},
					{Name: "configmaps", Namespaced: true, Kind: "ConfigMap"},
					{Name: "namespaces", Namespaced: false, Kind: "Namespace"},
				},
			},
		},
		{
			Group: metav1.APIGroup{
				Name: "apps",
				Versions: []metav1.GroupVersionForDiscovery{
					{Version: "v1"},
				},
				PreferredVersion: metav1.GroupVersionForDiscovery{Version: "v1"},
			},
			VersionedResources: map[string][]metav1.APIResource{
				"v1": {
					{Name: "deployments", Namespaced: true, Kind: "Deployment"},
					{Name: "statefulsets", Namespaced: true, Kind: "StatefulSet"},
				},
			},
		},
		// KubeBlocks objects
		{
			Group: metav1.APIGroup{
				Name: "apps.kubeblocks.io",
				Versions: []metav1.GroupVersionForDiscovery{
					{GroupVersion: "apps.kubeblocks.io/v1alpha1", Version: "v1alpha1"},
				},
				PreferredVersion: metav1.GroupVersionForDiscovery{
					GroupVersion: "apps.kubeblocks.io/v1alpha1",
					Version:      "v1alpha1"},
			},
			VersionedResources: map[string][]metav1.APIResource{
				"v1alpha1": {
					{Name: "clusters", Namespaced: true, Kind: "Cluster"},
					{Name: "clusterdefinitions", Namespaced: false, Kind: "clusterdefinition"},
					{Name: "clusterversions", Namespaced: false, Kind: "clusterversion"},
				},
			},
		},
	}
}
