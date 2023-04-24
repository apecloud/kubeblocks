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
	if model.IsObjectDeleting(origCSSet) {
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
	plan := generateConsensusUpdatePlan(stsObj, pods, *csSet, dag)
	// execute plan
	if _, err := plan.WalkOneStep(); err != nil {
		return err
	}
	return nil
}

var _ graph.Transformer = &UpdateStrategyTransformer{}
