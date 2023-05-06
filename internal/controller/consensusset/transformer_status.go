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

package consensusset

import (
	"context"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	roclient "github.com/apecloud/kubeblocks/internal/controller/client"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
)

// CSSetStatusTransformer computes the current status:
// 1. read the underlying sts's status and copy them to consensus set's status
// 2. read pod role label and update consensus set's status role fields
type CSSetStatusTransformer struct{}

func (t *CSSetStatusTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*CSSetTransformContext)
	csSet := transCtx.CSSet
	origCSSet := transCtx.OrigCSSet

	// fast return
	if model.IsObjectDeleting(origCSSet) {
		return nil
	}

	switch {
	case model.IsObjectUpdating(origCSSet):
		// use consensus set's generation instead of sts's
		csSet.Status.ObservedGeneration = csSet.Generation
		csSet.Status.Replicas = csSet.Spec.Replicas
	case model.IsObjectStatusUpdating(origCSSet):
		// read the underlying sts
		sts := &apps.StatefulSet{}
		if err := transCtx.Client.Get(transCtx.Context, client.ObjectKeyFromObject(csSet), sts); err != nil {
			return err
		}
		copyStatus(csSet, *sts)
		// read all pods belong to the sts, hence belong to our consensus set
		pods, err := getPodsOfStatefulSet(transCtx.Context, transCtx.Client, sts)
		if err != nil {
			return err
		}
		// update role fields
		setConsensusSetStatusRoles(csSet, pods)
	}

	// get root vertex(i.e. consensus set)
	root, err := model.FindRootVertex(dag)
	if err != nil {
		return err
	}
	root.Action = model.ActionPtr(model.STATUS)

	return nil
}

func copyStatus(csSet *workloads.ConsensusSet, sts apps.StatefulSet) {
	csSet.Status.Replicas = sts.Status.Replicas
	csSet.Status.ReadyReplicas = sts.Status.ReadyReplicas
	csSet.Status.CurrentReplicas = sts.Status.CurrentReplicas
	csSet.Status.UpdatedReplicas = sts.Status.UpdatedReplicas
	csSet.Status.CurrentRevision = sts.Status.CurrentRevision
	csSet.Status.UpdateRevision = sts.Status.UpdateRevision
	csSet.Status.CollisionCount = sts.Status.CollisionCount
	csSet.Status.Conditions = sts.Status.Conditions
	csSet.Status.AvailableReplicas = sts.Status.AvailableReplicas
}

func getPodsOfStatefulSet(ctx context.Context, cli roclient.ReadonlyClient, stsObj *apps.StatefulSet) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	if err := cli.List(ctx, podList,
		&client.ListOptions{Namespace: stsObj.Namespace},
		client.MatchingLabels{
			model.KBManagedByKey:      stsObj.Labels[model.KBManagedByKey],
			model.AppInstanceLabelKey: stsObj.Labels[model.AppInstanceLabelKey],
		}); err != nil {
		return nil, err
	}
	var pods []corev1.Pod
	for _, pod := range podList.Items {
		if util.IsMemberOf(stsObj, &pod) {
			pods = append(pods, pod)
		}
	}
	return pods, nil
}

var _ graph.Transformer = &CSSetStatusTransformer{}
