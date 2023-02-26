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
	"fmt"
	"os"
	"time"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"
)

// GetDefaultKubeconfigPath returns the path of the default kubeconfig, but errors if the KUBECONFIG env var specifies more than one file
func GetDefaultKubeconfigPath() (string, error) {
	defaultKubeConfigLoadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if len(defaultKubeConfigLoadingRules.GetLoadingPrecedence()) > 1 {
		return "", fmt.Errorf("multiple kubeconfigs specified via KUBECONFIG env var: Please reduce to one entry, unset KUBECONFIG or explicitly choose an output")
	}
	return defaultKubeConfigLoadingRules.GetDefaultFilename(), nil
}

// GetDefaultKubeconfigFile loads the default kubeconfig file
func GetDefaultKubeconfigFile() (*clientcmdapi.Config, error) {
	path, err := GetDefaultKubeconfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get default kubeconfig path: %w", err)
	}
	return clientcmd.LoadFromFile(path)
}

// KubeconfigRemoveClusterFromDefaultConfig removes a cluster's details from the default kubeconfig
func KubeconfigRemoveClusterFromDefaultConfig(cluster string) error {
	defaultKubeConfigPath, err := GetDefaultKubeconfigPath()
	if err != nil {
		return fmt.Errorf("failed to get default kubeconfig path: %w", err)
	}
	kubeconfig, err := GetDefaultKubeconfigFile()
	if err != nil {
		return fmt.Errorf("failed to get default kubeconfig file: %w", err)
	}
	kubeconfig = KubeconfigRemoveCluster(cluster, kubeconfig)
	return KubeconfigWrite(kubeconfig, defaultKubeConfigPath)
}

// KubeconfigRemoveCluster removes a cluster's details from a given kubeconfig
func KubeconfigRemoveCluster(cluster string, kubeconfig *clientcmdapi.Config) *clientcmdapi.Config {
	// now, we assume the three names are the same
	clusterName := cluster
	contextName := cluster
	authInfoName := cluster

	// delete elements from kubeconfig if they're present
	delete(kubeconfig.Contexts, contextName)
	delete(kubeconfig.Clusters, clusterName)
	delete(kubeconfig.AuthInfos, authInfoName)

	// set current-context to any other context, if it was set to the given cluster before
	if kubeconfig.CurrentContext == contextName {
		for k := range kubeconfig.Contexts {
			kubeconfig.CurrentContext = k
			break
		}
		// if current-context didn't change, unset it
		if kubeconfig.CurrentContext == contextName {
			kubeconfig.CurrentContext = ""
		}
	}
	return kubeconfig
}

// KubeconfigWrite writes a kubeconfig to a path atomically
func KubeconfigWrite(kubeconfig *clientcmdapi.Config, path string) error {
	tempPath := fmt.Sprintf("%s.kbcli_)%s", path, time.Now().Format("20060102_150405.000000"))
	if err := clientcmd.WriteToFile(*kubeconfig, tempPath); err != nil {
		return fmt.Errorf("failed to write merged kubeconfig to temporary file '%s': %w", tempPath, err)
	}

	// Move temporary file over existing KubeConfig
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("failed to overwrite existing KubeConfig '%s' with new kubeconfig '%s': %w", path, tempPath, err)
	}

	klog.V(1).Infof("Wrote kubeconfig to '%s'", path)
	return nil
}
