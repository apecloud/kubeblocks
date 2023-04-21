/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

type Version struct {
	KubeBlocks string
	Kubernetes string
	Cli        string
}

// GetVersionInfo get version include KubeBlocks, CLI and kubernetes
func GetVersionInfo(client kubernetes.Interface) (Version, error) {
	var err error
	version := Version{
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
	deploy, err := GetKubeBlocksDeploy(client)
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

// GetKubeBlocksDeploy gets KubeBlocks deployments, now one kubernetes cluster
// only support one KubeBlocks
func GetKubeBlocksDeploy(client kubernetes.Interface) (*appsv1.Deployment, error) {
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
