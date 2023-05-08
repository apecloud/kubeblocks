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

package lifecycle

import (
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

type StsPodsTransformer struct{}

func (t *StsPodsTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	origCluster := transCtx.OrigCluster

	if isClusterDeleting(*origCluster) {
		return nil
	}

	handlePodsUpdate := func(vertex *lifecycleVertex) error {
		stsObj, _ := vertex.oriObj.(*appsv1.StatefulSet)
		stsProto, _ := vertex.obj.(*appsv1.StatefulSet)

		if stsObj.Spec.Replicas != stsProto.Spec.Replicas {
			ml := client.MatchingLabels{
				constant.AppInstanceLabelKey:    stsObj.Labels[constant.AppInstanceLabelKey],
				constant.KBAppComponentLabelKey: stsObj.Labels[constant.KBAppComponentLabelKey],
			}
			podList := corev1.PodList{}
			if err := transCtx.Client.List(transCtx.Context, &podList, ml); err != nil {
				return err
			}
			for _, pod := range podList.Items {
				obj := pod.DeepCopy()
				if obj.Annotations == nil {
					obj.Annotations = make(map[string]string)
				}
				obj.Annotations[constant.ComponentReplicasAnnotationKey] = strconv.Itoa(int(*stsProto.Spec.Replicas))
				v := &lifecycleVertex{
					obj:    obj,
					oriObj: &pod,
					action: actionPtr(UPDATE),
				}
				dag.AddVertex(v)
				dag.Connect(vertex, v)
			}
		}
		return nil
	}

	vertices := findAll[*appsv1.StatefulSet](dag)
	for _, vertex := range vertices {
		v, _ := vertex.(*lifecycleVertex)
		if v.obj != nil && v.oriObj != nil && v.action != nil && *v.action == UPDATE {
			if err := handlePodsUpdate(v); err != nil {
				return err
			}
		}
	}
	return nil
}

var _ graph.Transformer = &StsPodsTransformer{}
