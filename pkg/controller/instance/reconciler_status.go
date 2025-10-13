/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package instance

import (
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func NewStatusReconciler() kubebuilderx.Reconciler {
	return &statusReconciler{}
}

type statusReconciler struct{}

var _ kubebuilderx.Reconciler = &statusReconciler{}

func (r *statusReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || !model.IsObjectStatusUpdating(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (r *statusReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	inst := tree.GetRoot().(*workloads.Instance)

	obj, err := tree.Get(podObj(inst))
	if err != nil {
		return kubebuilderx.Continue, err
	}
	if obj == nil {
		return kubebuilderx.Continue, nil
	}
	pod := obj.(*corev1.Pod)

	ready, available, updated := false, false, false
	notReadyName, notAvailableName := "", ""

	if isCreated(pod) {
		notReadyName = pod.Name
	}
	if isImageMatched(pod) && intctrlutil.IsPodReady(pod) {
		ready = true
		notReadyName = ""
		if intctrlutil.IsPodAvailable(pod, inst.Spec.MinReadySeconds) {
			available = true
		} else {
			notAvailableName = pod.Name
		}
	}
	if isCreated(pod) && !isTerminating(pod) {
		updated, err = isPodUpdated(inst, pod)
		if err != nil {
			return kubebuilderx.Continue, err
		}
	}

	inst.Status.CurrentRevision = getPodRevision(pod)
	if updated {
		inst.Status.CurrentRevision = inst.Status.UpdateRevision
	}

	readyCondition := r.buildReadyCondition(inst, ready, notReadyName)
	meta.SetStatusCondition(&inst.Status.Conditions, *readyCondition)

	availableCondition := r.buildAvailableCondition(inst, available, notAvailableName)
	meta.SetStatusCondition(&inst.Status.Conditions, *availableCondition)

	failureCondition := r.buildFailureCondition(inst, pod)
	if failureCondition != nil {
		meta.SetStatusCondition(&inst.Status.Conditions, *failureCondition)
	} else {
		meta.RemoveStatusCondition(&inst.Status.Conditions, string(workloads.InstanceFailure))
	}

	inst.Status.UpToDate = updated
	inst.Status.Ready = ready
	inst.Status.Available = available
	inst.Status.Role = r.observedRoleOfPod(inst, pod)
	r.buildLifecycleStatus(inst, pod)
	inst.Status.InVolumeExpansion = r.hasRunningVolumeExpansion(tree, inst)

	if inst.Spec.MinReadySeconds > 0 && !available {
		return kubebuilderx.RetryAfter(time.Second), nil
	}
	return kubebuilderx.Continue, nil
}

func (r *statusReconciler) buildReadyCondition(inst *workloads.Instance, ready bool, notReadyName string) *metav1.Condition {
	condition := &metav1.Condition{
		Type:               string(workloads.InstanceReady),
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inst.Generation,
		Reason:             workloads.ReasonReady,
	}
	if !ready {
		condition.Status = metav1.ConditionFalse
		condition.Reason = workloads.ReasonNotReady
		condition.Message = notReadyName
	}
	return condition
}

func (r *statusReconciler) buildAvailableCondition(inst *workloads.Instance, available bool, notAvailableName string) *metav1.Condition {
	condition := &metav1.Condition{
		Type:               string(workloads.InstanceAvailable),
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inst.Generation,
		Reason:             workloads.ReasonAvailable,
	}
	if !available {
		condition.Status = metav1.ConditionFalse
		condition.Reason = workloads.ReasonNotAvailable
		condition.Message = notAvailableName
	}
	return condition
}

func (r *statusReconciler) buildFailureCondition(inst *workloads.Instance, pod *corev1.Pod) *metav1.Condition {
	if isTerminating(pod) {
		return nil
	}
	var failureName string
	// Kubernetes says the Pod is 'Failed'
	if pod.Status.Phase == corev1.PodFailed {
		failureName = pod.Name
	}
	// KubeBlocks says the Pod is 'Failed'
	isFailed, isTimedOut, _ := intctrlutil.IsPodFailedAndTimedOut(pod)
	if len(failureName) == 0 && isFailed && isTimedOut {
		failureName = pod.Name
	}
	if len(failureName) == 0 {
		return nil
	}
	return &metav1.Condition{
		Type:               string(workloads.InstanceFailure),
		Status:             metav1.ConditionTrue,
		ObservedGeneration: inst.Generation,
		Reason:             workloads.ReasonInstanceFailure,
		Message:            failureName,
	}
}

func (r *statusReconciler) observedRoleOfPod(inst *workloads.Instance, pod *corev1.Pod) string {
	if inst.Spec.Roles != nil && intctrlutil.PodIsReadyWithLabel(*pod) {
		roleMap := composeRoleMap(inst)
		roleName := getRoleName(pod)
		role, ok := roleMap[roleName]
		if ok {
			return role.Name
		}
	}
	return ""
}

func (r *statusReconciler) buildLifecycleStatus(inst *workloads.Instance, pod *corev1.Pod) {
	dataLoaded := func() *bool {
		if inst.Spec.LifecycleActions == nil || inst.Spec.LifecycleActions.DataLoad == nil {
			return nil
		}
		if inst.Status.DataLoaded == nil || *inst.Status.DataLoaded {
			return inst.Status.DataLoaded
		}
		loaded, ok := pod.Annotations[constant.LifeCycleDataLoadedAnnotationKey]
		if !ok {
			return ptr.To(false)
		}
		return ptr.To(strings.ToLower(loaded) == "true")
	}

	inst.Status.Provisioned = true
	inst.Status.DataLoaded = dataLoaded()
}

func (r *statusReconciler) hasRunningVolumeExpansion(tree *kubebuilderx.ObjectTree, inst *workloads.Instance) bool {
	pvcs := tree.List(&corev1.PersistentVolumeClaim{})
	var pvcList []*corev1.PersistentVolumeClaim
	for _, obj := range pvcs {
		pvc, _ := obj.(*corev1.PersistentVolumeClaim)
		pvcList = append(pvcList, pvc)
	}
	for _, vct := range inst.Spec.VolumeClaimTemplates {
		prefix := fmt.Sprintf("%s-%s", vct.Name, inst.Name)
		for _, pvc := range pvcList {
			if !strings.HasPrefix(pvc.Name, prefix) {
				continue
			}
			if pvc.Status.Capacity == nil || pvc.Status.Capacity.Storage().Cmp(pvc.Spec.Resources.Requests[corev1.ResourceStorage]) >= 0 {
				continue
			}
			instName := ""
			if pvc.Labels != nil {
				instName = pvc.Labels[constant.KBAppPodNameLabelKey]
			}
			if len(instName) > 0 {
				return true
			}
		}
	}
	return false
}
