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
	"context"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	roclient "github.com/apecloud/kubeblocks/internal/controller/client"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/model"
	"github.com/apecloud/kubeblocks/internal/controllerutil"
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

	// read the underlying sts
	sts := &apps.StatefulSet{}
	if err := transCtx.Client.Get(transCtx.Context, client.ObjectKeyFromObject(csSet), sts); err != nil {
		return err
	}
	switch {
	case model.IsObjectUpdating(origCSSet):
		// use consensus set's generation instead of sts's
		csSet.Status.ObservedGeneration = csSet.Generation
	case model.IsObjectStatusUpdating(origCSSet):
		generation := csSet.Status.ObservedGeneration
		csSet.Status.StatefulSetStatus = sts.Status
		csSet.Status.ObservedGeneration = generation
		// read all pods belong to the sts, hence belong to our consensus set
		pods, err := getPodsOfStatefulSet(transCtx.Context, transCtx.Client, sts)
		if err != nil {
			return err
		}
		// update role fields
		setConsensusSetStatusRoles(csSet, pods)
	}

	// TODO: handle Update(i.e. pods deletion)

	return nil
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
		if controllerutil.IsMemberOf(stsObj, &pod) {
			pods = append(pods, pod)
		}
	}
	return pods, nil
}

var _ graph.Transformer = &CSSetStatusTransformer{}
