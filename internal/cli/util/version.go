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
	"context"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"

	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/version"
)

type AppName string

const (
	KubernetesApp AppName = "Kubernetes"
	KubeBlocksApp AppName = "KubeBlocks"
	KBCLIApp      AppName = "kbcli"
)

// GetVersionInfo get application version include KubeBlocks, CLI and kubernetes
func GetVersionInfo(client kubernetes.Interface) (map[AppName]string, error) {
	var err error
	versionInfo := map[AppName]string{}
	versionInfo[KBCLIApp] = version.GetVersion()

	if versionInfo[KubernetesApp], err = getK8sVersion(client.Discovery()); err != nil {
		return versionInfo, err
	}

	if versionInfo[KubeBlocksApp], err = getKubeBlocksVersion(client); err != nil {
		return versionInfo, err
	}

	return versionInfo, nil
}

// getKubeBlocksVersion get KubeBlocks version
func getKubeBlocksVersion(client kubernetes.Interface) (string, error) {
	kubeBlocksDeploys, err := client.AppsV1().Deployments(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=" + types.KubeBlocksChartName,
	})
	if err != nil {
		return "", err
	}

	var versions []string
	for _, deploy := range kubeBlocksDeploys.Items {
		labels := deploy.GetLabels()
		if labels == nil {
			continue
		}
		if v, ok := labels["app.kubernetes.io/version"]; ok {
			versions = append(versions, v)
		}
	}
	return strings.Join(versions, " "), nil
}

// getK8sVersion get k8s server version
func getK8sVersion(discoveryClient discovery.DiscoveryInterface) (string, error) {
	if discoveryClient == nil {
		return "", nil
	}

	serverVersion, err := discoveryClient.ServerVersion()
	if err != nil {
		return "", err
	}

	if serverVersion != nil {
		return serverVersion.GitVersion, nil
	}
	return "", nil
}
