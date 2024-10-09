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

package experimental

import (
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	experimental "github.com/apecloud/kubeblocks/apis/experimental/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type scaleTargetClusterReconciler struct{}

func (r *scaleTargetClusterReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (r *scaleTargetClusterReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	scaler, _ := tree.GetRoot().(*experimental.NodeCountScaler)
	clusterKey := builder.NewClusterBuilder(scaler.Namespace, scaler.Spec.TargetClusterName).GetObject()
	object, err := tree.Get(clusterKey)
	if err != nil {
		return kubebuilderx.Continue, err
	}
	cluster, _ := object.(*appsv1.Cluster)
	nodes := tree.List(&corev1.Node{})
	// TODO(free6om): filter nodes that satisfy pod template spec of each component (by nodeSelector, nodeAffinity&nodeAntiAffinity, tolerations)
	desiredReplicas := int32(len(nodes))
	scaled := false
	for i := range cluster.Spec.ComponentSpecs {
		spec := &cluster.Spec.ComponentSpecs[i]
		if slices.IndexFunc(scaler.Spec.TargetComponentNames, func(name string) bool {
			return name == spec.Name
		}) < 0 {
			continue
		}
		if spec.Replicas != desiredReplicas {
			spec.Replicas = desiredReplicas
			scaled = true
		}
	}
	if !scaled {
		return kubebuilderx.Continue, nil
	}

	scaler.Status.LastScaleTime = metav1.Time{Time: time.Now()}
	if err = tree.Update(cluster); err != nil {
		return kubebuilderx.Continue, err
	}

	return kubebuilderx.Continue, nil
}

func scaleTargetCluster() kubebuilderx.Reconciler {
	return &scaleTargetClusterReconciler{}
}

var _ kubebuilderx.Reconciler = &scaleTargetClusterReconciler{}
