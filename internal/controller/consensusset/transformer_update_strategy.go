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
	apps "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
)

type UpdateStrategyTransformer struct{}

func (t *UpdateStrategyTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*CSSetTransformContext)
	csSet := transCtx.CSSet
	origCSSet := transCtx.OrigCSSet
	if !model.IsObjectStatusUpdating(origCSSet) {
		return nil
	}

	// read the underlying sts
	stsObj := &apps.StatefulSet{}
	if err := transCtx.Client.Get(transCtx.Context, client.ObjectKeyFromObject(csSet), stsObj); err != nil {
		return err
	}
	// read all pods belong to the sts, hence belong to our consensus set
	pods, err := getPodsOfStatefulSet(transCtx.Context, transCtx.Client, stsObj)
	if err != nil {
		return err
	}

	// prepare to do pods Deletion, that's the only thing we should do,
	// the stateful_set reconciler will do the others.
	// to simplify the process, we do pods Deletion after stateful_set reconcile done,
	// that is stsObj.Generation == stsObj.Status.ObservedGeneration
	if stsObj.Generation != stsObj.Status.ObservedGeneration {
		return nil
	}

	// then we wait all pods' presence, that is len(pods) == stsObj.Spec.Replicas
	// only then, we have enough info about the previous pods before delete the current one
	if len(pods) != int(*stsObj.Spec.Replicas) {
		return nil
	}

	// we don't check whether pod role label present: prefer stateful set's Update done than role probing ready

	// generate the pods Deletion plan
	plan := newUpdatePlan(*csSet, pods)
	podsToBeUpdated, err := plan.execute()
	if err != nil {
		return err
	}
	// get root vertex(i.e. consensus set)
	root, err := model.FindRootVertex(dag)
	if err != nil {
		return err
	}
	for _, pod := range podsToBeUpdated {
		vertex := &model.ObjectVertex{Obj: pod, Action: model.ActionPtr(model.DELETE)}
		dag.AddConnect(root, vertex)
	}

	return nil
}

var _ graph.Transformer = &UpdateStrategyTransformer{}
