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

package statefulreplicaset

import (
	apps "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
)

// SRSStatusTransformer computes the current status:
// 1. read the underlying sts's status and copy them to stateful_replica_set's status
// 2. read pod role label and update stateful_replica_set's status role fields
type SRSStatusTransformer struct{}

func (t *SRSStatusTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*SRSTransformContext)
	srs := transCtx.srs
	srsOrig := transCtx.srsOrig

	// fast return
	if model.IsObjectDeleting(srsOrig) {
		return nil
	}

	switch {
	case model.IsObjectUpdating(srsOrig):
		// use srs's generation instead of sts's
		srs.Status.ObservedGeneration = srs.Generation
		// hack for sts initialization error: is invalid: status.replicas: Required value
		if srs.Status.Replicas == 0 {
			srs.Status.Replicas = srs.Spec.Replicas
		}
	case model.IsObjectStatusUpdating(srsOrig):
		// read the underlying sts
		sts := &apps.StatefulSet{}
		if err := transCtx.Client.Get(transCtx.Context, client.ObjectKeyFromObject(srs), sts); err != nil {
			return err
		}
		// keep srs's ObservedGeneration to avoid override by sts's ObservedGeneration
		generation := srs.Status.ObservedGeneration
		srs.Status.StatefulSetStatus = sts.Status
		srs.Status.ObservedGeneration = generation
		// read all pods belong to the sts, hence belong to the srs
		pods, err := getPodsOfStatefulSet(transCtx.Context, transCtx.Client, sts)
		if err != nil {
			return err
		}
		// update role fields
		setMembersStatus(srs, pods)
	}

	if err := model.PrepareRootStatus(dag); err != nil {
		return err
	}

	return nil
}

var _ graph.Transformer = &SRSStatusTransformer{}
