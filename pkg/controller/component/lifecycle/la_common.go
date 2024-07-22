/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package lifecycle

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

const (
	podNameList     = "podNameList"
	podIPList       = "podIPList"
	podHostNameList = "podHostNameList"
	podHostIPList   = "podHostIPList"
	compName        = "compName"
	compReplicas    = "compReplicas"
)

func getClusterPods(ctx context.Context, cli client.Reader, namespace, clusterName string) (map[string]string, error) {
	// TODO
	pods, err := component.ListOwnedPods(ctx, cli, namespace, clusterName, "")
	if err != nil {
		return nil, err
	}
	name := func(pod *corev1.Pod) string {
		return pod.Name
	}
	IP := func(pod *corev1.Pod) string {
		return pod.Status.PodIP
	}
	hostName := func(pod *corev1.Pod) string {
		return pod.Spec.Hostname
	}
	hostIP := func(pod *corev1.Pod) string {
		return pod.Status.HostIP
	}
	return map[string]string{
		podNameList:     strings.Join(traverse(pods, name), ","),
		podIPList:       strings.Join(traverse(pods, IP), ","),
		podHostNameList: strings.Join(traverse(pods, hostName), ","),
		podHostIPList:   strings.Join(traverse(pods, hostIP), ","),
	}, nil
}

func getCompPods(ctx context.Context, cli client.Reader, namespace, clusterName, compName string) (map[string]string, error) {
	pods, err := component.ListOwnedPods(ctx, cli, namespace, clusterName, compName)
	if err != nil {
		return nil, err
	}
	name := func(pod *corev1.Pod) string {
		return pod.Name
	}
	IP := func(pod *corev1.Pod) string {
		return pod.Status.PodIP
	}
	hostName := func(pod *corev1.Pod) string {
		return pod.Spec.Hostname
	}
	hostIP := func(pod *corev1.Pod) string {
		return pod.Status.HostIP
	}
	return map[string]string{
		podNameList:     strings.Join(traverse(pods, name), ","),
		podIPList:       strings.Join(traverse(pods, IP), ","),
		podHostNameList: strings.Join(traverse(pods, hostName), ","),
		podHostIPList:   strings.Join(traverse(pods, hostIP), ","),
	}, nil
}

func getComps(ctx context.Context, cli client.Reader, namespace, clusterName string) (map[string]string, error) {
	cluster := &appsv1alpha1.Cluster{}
	key := types.NamespacedName{
		Namespace: namespace,
		Name:      clusterName,
	}
	if err := cli.Get(ctx, key, cluster); err != nil {
		return nil, err
	}

	name := func(spec appsv1alpha1.ClusterComponentSpec) string {
		return spec.Name
	}
	replicas := func(spec appsv1alpha1.ClusterComponentSpec) string {
		return fmt.Sprintf("%d", spec.Replicas)
	}
	return map[string]string{
		compName:     strings.Join(traverse(cluster.Spec.ComponentSpecs, name), ","),
		compReplicas: strings.Join(traverse(cluster.Spec.ComponentSpecs, replicas), ","),
	}, nil
}

func traverse[T any, R any](items []T, f func(T) R) []R {
	var result []R
	for _, item := range items {
		result = append(result, f(item))
	}
	return result
}
