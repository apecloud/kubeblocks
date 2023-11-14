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

package component

import (
	"context"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

func ListPodOwnedByComponent(ctx context.Context, cli client.Reader, namespace string, labels client.MatchingLabels) ([]*corev1.Pod, error) {
	return ListObjWithLabelsInNamespace(ctx, cli, generics.PodSignature, namespace, labels)
}

// GetComponentPodList gets the pod list by cluster and componentName
func GetComponentPodList(ctx context.Context, cli client.Reader, cluster appsv1alpha1.Cluster, componentName string) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	labels := constant.GetComponentWellKnownLabels(cluster.Name, componentName)
	if err := cli.List(ctx, podList, client.InNamespace(cluster.Namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}
	return podList, nil
}

// GetComponentPodListWithRole gets the pod list with target role by cluster and componentName
func GetComponentPodListWithRole(ctx context.Context, cli client.Reader, cluster appsv1alpha1.Cluster, compSpecName, role string) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	labels := constant.GetComponentWellKnownLabels(cluster.Name, compSpecName)
	labels[constant.RoleLabelKey] = role
	if err := cli.List(ctx, podList, client.InNamespace(cluster.Namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}
	return podList, nil
}

// IsComponentPodsWithLatestRevision checks whether the underlying pod spec matches the one declared in the Cluster/Component.
func IsComponentPodsWithLatestRevision(ctx context.Context, cli client.Reader,
	cluster *appsv1alpha1.Cluster, rsm *workloads.ReplicatedStateMachine) (bool, error) {
	if cluster == nil || rsm == nil {
		return false, nil
	}
	// check whether component spec has been sent to rsm
	rsmComponentGeneration := rsm.GetAnnotations()[constant.KubeBlocksGenerationKey]
	if cluster.Status.ObservedGeneration != cluster.Generation ||
		rsmComponentGeneration != strconv.FormatInt(cluster.Generation, 10) {
		return false, nil
	}
	// check whether rsm spec has been sent to the underlying workload(sts)
	if rsm.Status.ObservedGeneration != rsm.Generation ||
		rsm.Status.CurrentGeneration != rsm.Generation {
		return false, nil
	}
	// check whether the underlying workload(sts) has sent the latest template to pods
	sts := &appsv1.StatefulSet{}
	if err := cli.Get(ctx, client.ObjectKeyFromObject(rsm), sts); err != nil {
		return false, err
	}
	if sts.Status.ObservedGeneration != sts.Generation {
		return false, nil
	}
	pods, err := ListPodOwnedByComponent(ctx, cli, rsm.Namespace, rsm.Spec.Selector.MatchLabels)
	if err != nil {
		return false, err
	}
	for _, pod := range pods {
		if intctrlutil.GetPodRevision(pod) != sts.Status.UpdateRevision {
			return false, nil
		}
	}
	return true, nil
}
