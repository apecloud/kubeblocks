/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package custom

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// TaskPodRequests maps Custom task Pod events back to their OpsRequest.
func TaskPodRequests(ctx context.Context, cli client.Client, pod *corev1.Pod) []reconcile.Request {
	opsName := pod.Labels[constant.OpsRequestNameLabelKey]
	opsNamespace := pod.Labels[constant.OpsRequestNamespaceLabelKey]
	if opsName == "" || opsNamespace == "" {
		return nil
	}
	key := types.NamespacedName{Namespace: opsNamespace, Name: opsName}
	opsRequest := &opsv1alpha1.OpsRequest{}
	if err := cli.Get(ctx, key, opsRequest); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return []reconcile.Request{{NamespacedName: key}}
	}
	if opsRequest.Spec.Type != opsv1alpha1.CustomType {
		return nil
	}
	return []reconcile.Request{{NamespacedName: key}}
}

// TaskJobRequests maps directly owned Custom task Job events back to their OpsRequest.
func TaskJobRequests(ctx context.Context, cli client.Client, job *batchv1.Job) []reconcile.Request {
	owner := metav1.GetControllerOf(job)
	if owner == nil || owner.APIVersion != opsv1alpha1.GroupVersion.String() || owner.Kind != "OpsRequest" {
		return nil
	}
	key := types.NamespacedName{Namespace: job.Namespace, Name: owner.Name}
	opsRequest := &opsv1alpha1.OpsRequest{}
	if err := cli.Get(ctx, key, opsRequest); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return []reconcile.Request{{NamespacedName: key}}
	}
	if opsRequest.UID != owner.UID || opsRequest.Spec.Type != opsv1alpha1.CustomType {
		return nil
	}
	return []reconcile.Request{{NamespacedName: key}}
}

// DeleteJobTasks deletes the in-namespace Jobs created by Custom workload actions.
func DeleteJobTasks(ctx context.Context, cli client.Client, opsRequest *opsv1alpha1.OpsRequest) error {
	jobs := &batchv1.JobList{}
	if err := cli.List(ctx, jobs,
		client.InNamespace(opsRequest.Namespace),
		client.MatchingLabels{constant.OpsRequestNameLabelKey: opsRequest.Name}); err != nil {
		return err
	}
	for i := range jobs.Items {
		if err := intctrlutil.BackgroundDeleteObject(cli, ctx, &jobs.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// DeleteManagerNamespacePodTasks deletes Custom exec Pods that cannot be owned by
// an OpsRequest because they run in the controller manager namespace.
func DeleteManagerNamespacePodTasks(ctx context.Context, cli client.Client, opsRequest *opsv1alpha1.OpsRequest) error {
	namespace := viper.GetString(constant.CfgKeyCtrlrMgrNS)
	if namespace == "" {
		return nil
	}
	pods := &corev1.PodList{}
	if err := cli.List(ctx, pods,
		client.InNamespace(namespace),
		client.MatchingLabels{
			constant.OpsRequestNameLabelKey:      opsRequest.Name,
			constant.OpsRequestNamespaceLabelKey: opsRequest.Namespace,
		}); err != nil {
		return err
	}
	for i := range pods.Items {
		if err := intctrlutil.BackgroundDeleteObject(cli, ctx, &pods.Items[i]); err != nil {
			return err
		}
	}
	return nil
}
