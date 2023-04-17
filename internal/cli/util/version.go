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
	"fmt"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"

	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/version"
)

type VersionInfo struct {
	KubeBlocks string
	Kubernetes string
	Cli        string
}

// GetVersionInfo get application version include KubeBlocks, CLI and kubernetes
func GetVersionInfo(client kubernetes.Interface) (VersionInfo, error) {
	var err error
	version := VersionInfo{
		Cli: version.GetVersion(),
	}

	if client == nil || reflect.ValueOf(client).IsNil() {
		return version, nil
	}

	if version.Kubernetes, err = GetK8sVersion(client.Discovery()); err != nil {
		return version, err
	}

	if version.KubeBlocks, err = getKubeBlocksVersion(client); err != nil {
		return version, err
	}

	return version, nil
}

// getKubeBlocksVersion get KubeBlocks version
func getKubeBlocksVersion(client kubernetes.Interface) (string, error) {
	deploy, err := getKubeBlocksDeploy(client)
	if err != nil || deploy == nil {
		return "", err
	}

	labels := deploy.GetLabels()
	if labels == nil {
		return "", fmt.Errorf("KubeBlocks deployment has no labels")
	}

	v, ok := labels["app.kubernetes.io/version"]
	if !ok {
		return "", fmt.Errorf("KubeBlocks deployment has no version label")
	}
	return v, nil
}

// GetK8sVersion get k8s server version
func GetK8sVersion(discoveryClient discovery.DiscoveryInterface) (string, error) {
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

// getKubeBlocksDeploy get KubeBlocks deployments, now one kubernetes cluster
// only support one KubeBlocks
func getKubeBlocksDeploy(client kubernetes.Interface) (*appsv1.Deployment, error) {
	deploys, err := client.AppsV1().Deployments(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=" + types.KubeBlocksChartName,
	})
	if err != nil {
		return nil, err
	}
	if deploys == nil || len(deploys.Items) == 0 {
		return nil, nil
	}
	if len(deploys.Items) > 1 {
		return nil, fmt.Errorf("found multiple KubeBlocks deployments, please check your cluster")
	}
	return &deploys.Items[0], nil
}
