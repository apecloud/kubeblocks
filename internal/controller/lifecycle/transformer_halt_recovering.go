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
	"encoding/json"
	"fmt"

	"github.com/authzed/controller-idioms/hash"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	"github.com/apecloud/kubeblocks/internal/controller/types"
)

type HaltRecoveryTransformer struct{}

func (t *HaltRecoveryTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*ClusterTransformContext)
	cluster := transCtx.Cluster

	if cluster.Status.ObservedGeneration != 0 {
		// skip handling for cluster.status.observedGeneration > 0
		return nil
	}

	listOptions := []client.ListOption{
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{
			constant.AppInstanceLabelKey: cluster.Name,
		},
	}

	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := transCtx.Client.List(transCtx.Context, pvcList, listOptions...); err != nil {
		return types.NewRequeueError(types.RequeueDuration, err.Error())
	}

	if len(pvcList.Items) == 0 {
		return nil
	}

	emitError := func(newCondition metav1.Condition) error {
		if newCondition.LastTransitionTime.IsZero() {
			newCondition.LastTransitionTime = metav1.Now()
		}
		newCondition.Status = metav1.ConditionFalse
		oldCondition := meta.FindStatusCondition(cluster.Status.Conditions, newCondition.Type)
		if oldCondition == nil {
			cluster.Status.Conditions = append(cluster.Status.Conditions, newCondition)
		} else {
			*oldCondition = newCondition
		}
		transCtx.EventRecorder.Event(transCtx.Cluster, corev1.EventTypeWarning, newCondition.Reason, newCondition.Message)
		return graph.ErrPrematureStop
	}

	// halt recovering from last applied record stored in pvc's annotation
	l, ok := pvcList.Items[0].Annotations[constant.LastAppliedClusterAnnotationKey]
	if !ok || l == "" {
		return emitError(metav1.Condition{
			Type:   appsv1alpha1.ConditionTypeHaltRecovery,
			Reason: "UncleanedResources",
			Message: fmt.Sprintf("found uncleaned resources, requires manual deletion, check with `kubectl -n %s get pvc,secret,cm -l %s=%s`",
				cluster.Namespace, constant.AppInstanceLabelKey, cluster.Name),
		})
	}

	lc := &appsv1alpha1.Cluster{}
	if err := json.Unmarshal([]byte(l), lc); err != nil {
		return types.NewRequeueError(types.RequeueDuration, err.Error())
	}

	// skip if same cluster UID
	if lc.UID == cluster.UID {
		return nil
	}

	// check clusterDefRef equality
	if cluster.Spec.ClusterDefRef != lc.Spec.ClusterDefRef {
		return emitError(metav1.Condition{
			Type:    appsv1alpha1.ConditionTypeHaltRecovery,
			Reason:  "HaltRecoveryFailed",
			Message: fmt.Sprintf("not equal to last applied cluster.spec.clusterDefRef %s", lc.Spec.ClusterDefRef),
		})
	}

	// check clusterVersionRef equality but allow clusters.apps.kubeblocks.io/allow-inconsistent-cv=true annotation override
	if cluster.Spec.ClusterVersionRef != lc.Spec.ClusterVersionRef &&
		cluster.Annotations[constant.HaltRecoveryAllowInconsistentCVAnnotKey] != trueVal {
		return emitError(metav1.Condition{
			Type:   appsv1alpha1.ConditionTypeHaltRecovery,
			Reason: "HaltRecoveryFailed",
			Message: fmt.Sprintf("not equal to last applied cluster.spec.clusterVersionRef %s; add '%s=true' annotation if void this check",
				lc.Spec.ClusterVersionRef, constant.HaltRecoveryAllowInconsistentCVAnnotKey),
		})
	}

	// check component len equality
	if l := len(lc.Spec.ComponentSpecs); l != len(cluster.Spec.ComponentSpecs) {
		return emitError(metav1.Condition{
			Type:    appsv1alpha1.ConditionTypeHaltRecovery,
			Reason:  "HaltRecoveryFailed",
			Message: fmt.Sprintf("inconsistent spec.componentSpecs counts to last applied cluster.spec.componentSpecs (len=%d)", l),
		})
	}

	// check every components' equality
	for _, comp := range cluster.Spec.ComponentSpecs {
		found := false
		for _, lastUsedComp := range lc.Spec.ComponentSpecs {
			// only need to verify [name, componentDefRef, replicas] for equality
			if comp.Name != lastUsedComp.Name {
				continue
			}
			if comp.ComponentDefRef != lastUsedComp.ComponentDefRef {
				return emitError(metav1.Condition{
					Type:   appsv1alpha1.ConditionTypeHaltRecovery,
					Reason: "HaltRecoveryFailed",
					Message: fmt.Sprintf("not equal to last applied cluster.spec.componetSpecs[%s].componentDefRef=%s",
						comp.Name, lastUsedComp.ComponentDefRef),
				})
			}
			if comp.Replicas != lastUsedComp.Replicas {
				return emitError(metav1.Condition{
					Type:   appsv1alpha1.ConditionTypeHaltRecovery,
					Reason: "HaltRecoveryFailed",
					Message: fmt.Sprintf("not equal to last applied cluster.spec.componetSpecs[%s].replicas=%d",
						comp.Name, lastUsedComp.Replicas),
				})
			}

			// following only check resource related spec., will skip check if HaltRecoveryAllowInconsistentResAnnotKey
			// annotation is specified
			if cluster.Annotations[constant.HaltRecoveryAllowInconsistentResAnnotKey] == trueVal {
				found = true
				break
			}
			if hash.Object(comp.VolumeClaimTemplates) != hash.Object(lastUsedComp.VolumeClaimTemplates) {
				objJSON, _ := json.Marshal(&lastUsedComp.VolumeClaimTemplates)
				return emitError(metav1.Condition{
					Type:   appsv1alpha1.ConditionTypeHaltRecovery,
					Reason: "HaltRecoveryFailed",
					Message: fmt.Sprintf("not equal to last applied cluster.spec.componetSpecs[%s].volumeClaimTemplates=%s; add '%s=true' annotation to void this check",
						comp.Name, objJSON, constant.HaltRecoveryAllowInconsistentResAnnotKey),
				})
			}

			if lastUsedComp.ClassDefRef != nil {
				if comp.ClassDefRef == nil || hash.Object(*comp.ClassDefRef) != hash.Object(*lastUsedComp.ClassDefRef) {
					objJSON, _ := json.Marshal(lastUsedComp.ClassDefRef)
					return emitError(metav1.Condition{
						Type:   appsv1alpha1.ConditionTypeHaltRecovery,
						Reason: "HaltRecoveryFailed",
						Message: fmt.Sprintf("not equal to last applied cluster.spec.componetSpecs[%s].classDefRef=%s; add '%s=true' annotation to void this check",
							comp.Name, objJSON, constant.HaltRecoveryAllowInconsistentResAnnotKey),
					})
				}
			} else if comp.ClassDefRef != nil {
				return emitError(metav1.Condition{
					Type:   appsv1alpha1.ConditionTypeHaltRecovery,
					Reason: "HaltRecoveryFailed",
					Message: fmt.Sprintf("not equal to last applied cluster.spec.componetSpecs[%s].classDefRef=null; add '%s=true' annotation to void this check",
						comp.Name, constant.HaltRecoveryAllowInconsistentResAnnotKey),
				})
			}

			if hash.Object(comp.Resources) != hash.Object(lastUsedComp.Resources) {
				objJSON, _ := json.Marshal(&lastUsedComp.Resources)
				return emitError(metav1.Condition{
					Type:   appsv1alpha1.ConditionTypeHaltRecovery,
					Reason: "HaltRecoveryFailed",
					Message: fmt.Sprintf("not equal to last applied cluster.spec.componetSpecs[%s].resources=%s; add '%s=true' annotation to void this check",
						comp.Name, objJSON, constant.HaltRecoveryAllowInconsistentResAnnotKey),
				})
			}
			found = true
			break
		}
		if !found {
			return emitError(metav1.Condition{
				Type:   appsv1alpha1.ConditionTypeHaltRecovery,
				Reason: "HaltRecoveryFailed",
				Message: fmt.Sprintf("cluster.spec.componetSpecs[%s] not found in last applied cluster",
					comp.Name),
			})
		}
	}
	return nil
}

var _ graph.Transformer = &HaltRecoveryTransformer{}
