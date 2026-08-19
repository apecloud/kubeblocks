/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package instanceset

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/workloads/instancestatus"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// statusReconciler computes the current status
type statusReconciler struct{}

var _ kubebuilderx.Reconciler = &statusReconciler{}

func NewStatusReconciler() kubebuilderx.Reconciler {
	return &statusReconciler{}
}

func (r *statusReconciler) PreCondition(tree *kubebuilderx.ObjectTree) *kubebuilderx.CheckResult {
	if tree.GetRoot() == nil || !model.IsObjectStatusUpdating(tree.GetRoot()) {
		return kubebuilderx.ConditionUnsatisfied
	}
	return kubebuilderx.ConditionSatisfied
}

func (r *statusReconciler) Reconcile(tree *kubebuilderx.ObjectTree) (kubebuilderx.Result, error) {
	its, _ := tree.GetRoot().(*workloads.InstanceSet)
	// 1. get all pods
	pods := tree.List(&corev1.Pod{})
	var podList []*corev1.Pod
	for _, object := range pods {
		pod, _ := object.(*corev1.Pod)
		podList = append(podList, pod)
	}
	if its.Spec.FlatInstanceOrdinal {
		if _, _, err := instancetemplate.BuildActiveAllocations(tree, its); err != nil {
			if instancetemplate.IsActiveAllocationIncomplete(err) {
				return kubebuilderx.Continue, nil
			}
			return kubebuilderx.Continue, err
		}
	}
	// 2. calculate status summary
	updateRevisions, err := GetRevisions(its.Status.UpdateRevisions)
	if err != nil {
		return kubebuilderx.Continue, err
	}
	replicas := int32(0)
	currentReplicas, updatedReplicas := int32(0), int32(0)
	readyReplicas, availableReplicas := int32(0), int32(0)
	notReadyNames := sets.New[string]()
	notAvailableNames := sets.New[string]()
	currentRevisions := map[string]string{}

	template2TemplatesStatus := map[string]*workloads.InstanceTemplateStatus{}
	template2TotalReplicas := map[string]int32{}
	for _, template := range its.Spec.Instances {
		templateReplicas := int32(1)
		if template.Replicas != nil {
			templateReplicas = *template.Replicas
		}
		template2TotalReplicas[template.Name] = templateReplicas
	}

	podToNodeMapping, err := ParseNodeSelectorOnceAnnotation(its)
	if err != nil {
		return kubebuilderx.Continue, err
	}

	for _, pod := range podList {
		templateName := pod.Labels[instancetemplate.TemplateNameLabelKey]
		if template2TemplatesStatus[templateName] == nil {
			template2TemplatesStatus[templateName] = &workloads.InstanceTemplateStatus{
				Name: templateName,
			}
		}
		currentRevisions[pod.Name] = getPodRevision(pod)
		if isCreated(pod) {
			notReadyNames.Insert(pod.Name)
			replicas++
			template2TemplatesStatus[templateName].Replicas++
		}
		if isImageMatched(pod) && intctrlutil.IsPodReady(pod) {
			readyReplicas++
			template2TemplatesStatus[templateName].ReadyReplicas++
			notReadyNames.Delete(pod.Name)
			if intctrlutil.IsPodAvailable(pod, its.Spec.MinReadySeconds) {
				availableReplicas++
				template2TemplatesStatus[templateName].AvailableReplicas++
			} else {
				notAvailableNames.Insert(pod.Name)
			}
		}
		if isCreated(pod) && !isTerminating(pod) {
			isPodUpdated, err := isPodUpdated(its, pod)
			if err != nil {
				return kubebuilderx.Continue, err
			}
			switch _, ok := updateRevisions[pod.Name]; {
			case !ok, !isPodUpdated:
				currentReplicas++
				template2TemplatesStatus[templateName].CurrentReplicas++
			default:
				updatedReplicas++
				template2TemplatesStatus[templateName].UpdatedReplicas++
			}
		}

		if nodeName, ok := podToNodeMapping[pod.Name]; ok {
			// there's chance that a pod is currently running and wait to be deleted so that it can be rescheduled
			if pod.Spec.NodeName == nodeName {
				if err := deleteNodeSelectorOnceAnnotation(its, pod.Name); err != nil {
					return kubebuilderx.Continue, err
				}
			}
		}
	}
	its.Status.Replicas = replicas
	its.Status.ReadyReplicas = readyReplicas
	its.Status.AvailableReplicas = availableReplicas
	its.Status.CurrentReplicas = currentReplicas
	its.Status.UpdatedReplicas = updatedReplicas
	its.Status.CurrentRevisions, _ = buildRevisions(currentRevisions)
	its.Status.TemplatesStatus = buildTemplatesStatus(template2TemplatesStatus)
	// all pods have been updated
	totalReplicas := int32(1)
	if its.Spec.Replicas != nil {
		totalReplicas = *its.Spec.Replicas
	}
	if its.Status.Replicas == totalReplicas && its.Status.UpdatedReplicas == totalReplicas {
		its.Status.CurrentRevision = its.Status.UpdateRevision
		its.Status.CurrentReplicas = totalReplicas
	}
	for idx, templateStatus := range its.Status.TemplatesStatus {
		templateTotalReplicas := template2TotalReplicas[templateStatus.Name]
		if templateStatus.Replicas == templateTotalReplicas && templateStatus.UpdatedReplicas == templateTotalReplicas {
			its.Status.TemplatesStatus[idx].CurrentReplicas = templateTotalReplicas
		}
	}

	readyCondition, err := buildReadyCondition(its, readyReplicas >= replicas, notReadyNames)
	if err != nil {
		return kubebuilderx.Continue, err
	}
	meta.SetStatusCondition(&its.Status.Conditions, *readyCondition)

	availableCondition, err := buildAvailableCondition(its, availableReplicas >= replicas, notAvailableNames)
	if err != nil {
		return kubebuilderx.Continue, err
	}
	meta.SetStatusCondition(&its.Status.Conditions, *availableCondition)

	// 3. set InstanceFailure condition
	failureCondition, err := buildFailureCondition(its, podList)
	if err != nil {
		return kubebuilderx.Continue, err
	}
	if failureCondition != nil {
		meta.SetStatusCondition(&its.Status.Conditions, *failureCondition)
	} else {
		meta.RemoveStatusCondition(&its.Status.Conditions, string(workloads.InstanceFailure))
	}

	// 4. set instance status
	if err = setInstanceStatus(tree, its, podList); err != nil {
		return kubebuilderx.Continue, err
	}

	if its.Spec.MinReadySeconds > 0 && availableReplicas != readyReplicas {
		return kubebuilderx.RetryAfter(time.Second), nil
	}

	// serviceaccount name migration process
	proposedRevisions, err := GetRevisions(its.Status.DeferredUpdatedRevisions)
	if err != nil {
		return kubebuilderx.Continue, err
	}

	updateDone := true
	for podName, revision := range proposedRevisions {
		if currentRevisions[podName] != revision {
			updateDone = false
			break
		}
	}
	if updateDone {
		_, ok1 := its.Annotations[constant.ServiceAccountInUseAnnotationKey]
		proposedSA, ok2 := its.Annotations[constant.ProposedServiceAccountNameAnnotationKey]
		if !ok1 && ok2 {
			its.Annotations[constant.ServiceAccountInUseAnnotationKey] = proposedSA
		}
	}

	return kubebuilderx.Continue, nil
}

func buildConditionMessageWithNames(podNames []string) ([]byte, error) {
	baseSort(podNames, func(i int) (string, int) {
		return parseParentNameAndOrdinal(podNames[i])
	}, nil, true)
	return json.Marshal(podNames)
}

func buildTemplatesStatus(template2TemplatesStatus map[string]*workloads.InstanceTemplateStatus) []workloads.InstanceTemplateStatus {
	var templatesStatus []workloads.InstanceTemplateStatus
	for templateName, templateStatus := range template2TemplatesStatus {
		if len(templateName) == 0 {
			continue
		}
		templatesStatus = append(templatesStatus, *templateStatus)
	}
	sort.Slice(templatesStatus, func(i, j int) bool {
		return templatesStatus[i].Name < templatesStatus[j].Name
	})
	return templatesStatus
}

func buildReadyCondition(its *workloads.InstanceSet, ready bool, notReadyNames sets.Set[string]) (*metav1.Condition, error) {
	condition := &metav1.Condition{
		Type:               string(workloads.InstanceReady),
		Status:             metav1.ConditionTrue,
		ObservedGeneration: its.Generation,
		Reason:             workloads.ReasonReady,
	}
	if !ready {
		condition.Status = metav1.ConditionFalse
		condition.Reason = workloads.ReasonNotReady
		message, err := buildConditionMessageWithNames(notReadyNames.UnsortedList())
		if err != nil {
			return nil, err
		}
		condition.Message = string(message)
	}
	return condition, nil
}

func buildAvailableCondition(its *workloads.InstanceSet, available bool, notAvailableNames sets.Set[string]) (*metav1.Condition, error) {
	condition := &metav1.Condition{
		Type:               string(workloads.InstanceAvailable),
		Status:             metav1.ConditionTrue,
		ObservedGeneration: its.Generation,
		Reason:             workloads.ReasonAvailable,
	}
	if !available {
		condition.Status = metav1.ConditionFalse
		condition.Reason = workloads.ReasonNotAvailable
		message, err := buildConditionMessageWithNames(notAvailableNames.UnsortedList())
		if err != nil {
			return nil, err
		}
		condition.Message = string(message)
	}
	return condition, nil
}

func buildFailureCondition(its *workloads.InstanceSet, pods []*corev1.Pod) (*metav1.Condition, error) {
	var failureNames []string
	for _, pod := range pods {
		if instancePodFailed(pod) {
			failureNames = append(failureNames, pod.Name)
		}
	}
	if len(failureNames) == 0 {
		return nil, nil
	}
	message, err := buildConditionMessageWithNames(failureNames)
	if err != nil {
		return nil, err
	}
	return &metav1.Condition{
		Type:               string(workloads.InstanceFailure),
		Status:             metav1.ConditionTrue,
		ObservedGeneration: its.Generation,
		Reason:             workloads.ReasonInstanceFailure,
		Message:            string(message),
	}, nil
}

func instancePodFailed(pod *corev1.Pod) bool {
	if isTerminating(pod) {
		return false
	}
	if pod.Status.Phase == corev1.PodFailed {
		return true
	}
	isFailed, isTimedOut, _ := intctrlutil.IsPodFailedAndTimedOut(pod)
	return isFailed && isTimedOut
}

func setInstanceStatus(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, pods []*corev1.Pod) error {
	activeAllocations, templateNames, err := instancetemplate.BuildActiveAllocations(tree, its)
	if err != nil {
		return err
	}
	active := make([]instancestatus.Allocation, 0, len(activeAllocations))
	activeNames := make(map[string]struct{}, len(activeAllocations))
	for _, allocation := range activeAllocations {
		active = append(active, instancestatus.Allocation{PodName: allocation.PodName, TemplateName: allocation.TemplateName})
		activeNames[allocation.PodName] = struct{}{}
	}
	updateRevisions, err := GetRevisions(its.Status.UpdateRevisions)
	if err != nil {
		return err
	}
	offline := append([]string(nil), its.Spec.OfflineInstances...)
	hints := make([]instancestatus.Allocation, 0, len(active)+len(pods)+len(offline))
	if isStopRequested(its) {
		for _, allocation := range active {
			offline = append(offline, allocation.PodName)
			hints = append(hints, allocation)
		}
		active = nil
	}
	currentActiveNames := make(map[string]struct{}, len(active))
	for _, allocation := range active {
		currentActiveNames[allocation.PodName] = struct{}{}
	}

	observations := make([]instancestatus.CurrentObservation, 0, len(pods))
	roleMap := composeRoleMap(*its)
	for _, pod := range pods {
		state := workloads.InstanceCurrentStatePresent
		if !pod.DeletionTimestamp.IsZero() {
			state = workloads.InstanceCurrentStateTerminating
		}
		if templateName, ok := instancetemplate.TemplateNameFromLabels(pod.Labels); ok {
			hints = append(hints, instancestatus.Allocation{PodName: pod.Name, TemplateName: templateName})
		}
		ready := state == workloads.InstanceCurrentStatePresent && isImageMatched(pod) && intctrlutil.IsPodReady(pod)
		observation := instancestatus.CurrentObservation{
			InstanceName:    pod.Name,
			State:           state,
			CurrentRevision: getPodRevision(pod),
			Ready:           ready,
			Available:       ready && intctrlutil.IsPodAvailable(pod, its.Spec.MinReadySeconds),
			Failed:          instancePodFailed(pod),
		}
		if state == workloads.InstanceCurrentStatePresent && isCreated(pod) {
			updated, err := isPodUpdated(its, pod)
			if err != nil {
				return err
			}
			observation.UpToDate = updated
		}
		if state == workloads.InstanceCurrentStatePresent && intctrlutil.PodIsReadyWithLabel(*pod) {
			if role, ok := roleMap[getRoleName(pod)]; ok {
				observation.Role = role.Name
			}
		}
		_, isActive := currentActiveNames[pod.Name]
		observation.Configs = instanceConfigStatus(its, pod.Name, isActive)
		observations = append(observations, observation)
	}

	for _, name := range append(append([]string(nil), offline...), podObservationNames(observations)...) {
		if _, ok := activeNames[name]; ok {
			continue
		}
		if templateName, ok, err := instancetemplate.HistoricalTemplateHint(its, name, templateNames); err != nil {
			return err
		} else if ok {
			hints = append(hints, instancestatus.Allocation{PodName: name, TemplateName: templateName})
		}
	}
	syncObservationPVCStatus(tree, its, observations)

	result, err := instancestatus.Build(instancestatus.BuildInput{
		Previous:        its.Status.InstanceStatus,
		Active:          active,
		Offline:         offline,
		Current:         observations,
		TemplateHints:   hints,
		UpdateRevisions: updateRevisions,
	})
	if err != nil {
		return err
	}
	its.Status.InstanceStatus = result.Statuses
	return nil
}

func instanceConfigStatus(its *workloads.InstanceSet, podName string, isActive bool) []workloads.InstanceConfigStatus {
	if its.Status.InstanceStatus == nil {
		return instanceConfigStatusFromSpec(its)
	}

	configNames := sets.New[string]()
	for _, config := range its.Spec.Configs {
		configNames.Insert(config.Name)
	}
	for _, status := range its.Status.InstanceStatus {
		if status.PodName != podName {
			continue
		}
		if isActive && status.EffectiveCurrentState() == workloads.InstanceCurrentStateAbsent {
			return instanceConfigStatusFromSpec(its)
		}
		configs := make([]workloads.InstanceConfigStatus, 0, len(status.Configs))
		for _, config := range status.Configs {
			if configNames.Has(config.Name) {
				configs = append(configs, config)
			}
		}
		return configs
	}
	return nil
}

func instanceConfigStatusFromSpec(its *workloads.InstanceSet) []workloads.InstanceConfigStatus {
	configs := make([]workloads.InstanceConfigStatus, 0, len(its.Spec.Configs))
	for _, config := range its.Spec.Configs {
		configs = append(configs, workloads.InstanceConfigStatus{Name: config.Name, Generation: config.Generation})
	}
	return configs
}

func podObservationNames(observations []instancestatus.CurrentObservation) []string {
	names := make([]string, 0, len(observations))
	for _, observation := range observations {
		names = append(names, observation.InstanceName)
	}
	return names
}

func syncObservationPVCStatus(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, observations []instancestatus.CurrentObservation) {
	if tree == nil {
		return
	}
	pvcs := tree.List(&corev1.PersistentVolumeClaim{})
	var pvcList []*corev1.PersistentVolumeClaim
	for _, obj := range pvcs {
		pvc, _ := obj.(*corev1.PersistentVolumeClaim)
		pvcList = append(pvcList, pvc)
	}
	for _, vct := range its.Spec.VolumeClaimTemplates {
		prefix := fmt.Sprintf("%s-%s", vct.Name, its.Name)
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
				for i := range observations {
					if observations[i].InstanceName == instName && observations[i].State == workloads.InstanceCurrentStatePresent {
						// TODO: how to check the expansion failed?
						observations[i].VolumeExpansion = true
						break
					}
				}
			}
		}
	}
}
