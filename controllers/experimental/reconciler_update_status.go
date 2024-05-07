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
	"fmt"
	"strings"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	experimental "github.com/apecloud/kubeblocks/apis/experimental/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type updateStatusReconciler struct{}

func (r *updateStatusReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || model.IsObjectDeleting(tree.GetRoot()) {
		return kubebuilderx.ResultUnsatisfied
	}
	return kubebuilderx.ResultSatisfied
}

func (r *updateStatusReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (*kubebuilderx.ObjectTree, error) {
	scaler, _ := tree.GetRoot().(*experimental.NodeCountScaler)
	itsList := tree.List(&workloads.InstanceSet{})
	nodes := tree.List(&corev1.Node{})
	// TODO(free6om): filter nodes that satisfy pod template spec of each component (by nodeSelector, nodeAffinity&nodeAntiAffinity, tolerations)
	desiredReplicas := int32(len(nodes))
	var statusList []experimental.ComponentStatus
	for _, name := range scaler.Spec.TargetComponentNames {
		index := slices.IndexFunc(itsList, func(object client.Object) bool {
			fullName := constant.GenerateClusterComponentName(scaler.Spec.TargetClusterName, name)
			return fullName == object.GetName()
		})
		if index < 0 {
			continue
		}
		its, _ := itsList[index].(*workloads.InstanceSet)
		status := experimental.ComponentStatus{
			Name:              name,
			CurrentReplicas:   its.Status.CurrentReplicas,
			ReadyReplicas:     its.Status.ReadyReplicas,
			AvailableReplicas: its.Status.AvailableReplicas,
			DesiredReplicas:   desiredReplicas,
		}
		statusList = append(statusList, status)
	}
	instanceset.MergeList(&statusList, &scaler.Status.ComponentStatuses,
		func(item experimental.ComponentStatus) func(experimental.ComponentStatus) bool {
			return func(status experimental.ComponentStatus) bool {
				return item.Name == status.Name
			}
		})

	condition := buildScaleReadyCondition(scaler)
	meta.SetStatusCondition(&scaler.Status.Conditions, *condition)

	return tree, nil
}

func buildScaleReadyCondition(scaler *experimental.NodeCountScaler) *metav1.Condition {
	var (
		ready         = true
		notReadyNames []string
	)
	for _, name := range scaler.Spec.TargetComponentNames {
		index := slices.IndexFunc(scaler.Status.ComponentStatuses, func(status experimental.ComponentStatus) bool {
			return status.Name == name
		})
		if index < 0 {
			ready = false
			notReadyNames = append(notReadyNames, name)
			continue
		}
		status := scaler.Status.ComponentStatuses[index]
		if status.CurrentReplicas != status.DesiredReplicas ||
			status.ReadyReplicas != status.DesiredReplicas ||
			status.AvailableReplicas != status.DesiredReplicas {
			ready = false
			notReadyNames = append(notReadyNames, name)
		}
	}

	if !ready {
		return &metav1.Condition{
			Type:               string(experimental.ScaleReady),
			Status:             metav1.ConditionFalse,
			ObservedGeneration: scaler.Generation,
			Reason:             experimental.ReasonNotReady,
			Message:            fmt.Sprintf("not ready components: %s", strings.Join(notReadyNames, ",")),
		}
	}
	return &metav1.Condition{
		Type:               string(experimental.ScaleReady),
		Status:             metav1.ConditionTrue,
		ObservedGeneration: scaler.Generation,
		Reason:             experimental.ReasonReady,
		Message:            "scale ready",
	}
}

func updateStatus() kubebuilderx.Reconciler {
	return &updateStatusReconciler{}
}

var _ kubebuilderx.Reconciler = &updateStatusReconciler{}
