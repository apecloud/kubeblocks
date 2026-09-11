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
	// Flat-ordinal reassignment can temporarily expose fewer authoritative names than spec.replicas.
	// Validate before mutating any status fields so a partial view cannot replace the last complete status.
	if its.Spec.FlatInstanceOrdinal {
		if _, _, err := instancetemplate.BuildAssignments(tree, its); err != nil {
			if instancetemplate.IsAssignmentIncomplete(err) {
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
			updated, err1 := isPodUpdated(its, pod)
			if err1 != nil {
				return kubebuilderx.Continue, err1
			}
			switch _, ok := updateRevisions[pod.Name]; {
			case !ok, !updated:
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

	if err = r.reconcileRestoreCondition(tree, its); err != nil {
		return kubebuilderx.Continue, err
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

func (r *statusReconciler) reconcileRestoreCondition(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet) error {
	restoreCond := meta.FindStatusCondition(its.Status.Conditions, string(workloads.Restore))
	if restoreCond != nil && (restoreCond.Status == metav1.ConditionTrue || restoreCond.Status == metav1.ConditionFalse) {
		return nil
	}
	condition, err := buildRestoreCondition(tree, its)
	if err != nil {
		return err
	}
	if condition == nil {
		meta.RemoveStatusCondition(&its.Status.Conditions, string(workloads.Restore))
		return nil
	}
	meta.SetStatusCondition(&its.Status.Conditions, *condition)
	return nil
}

func buildRestoreCondition(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet) (*metav1.Condition, error) {
	expectedPVCNames, err := expectedRestorePVCNames(tree, its)
	if err != nil {
		return nil, err
	}
	if expectedPVCNames.Len() == 0 {
		return nil, nil
	}

	completedPVCNames := sets.New[string]()
	missingPVCNames := expectedPVCNames.Clone()
	for _, obj := range tree.List(&corev1.PersistentVolumeClaim{}) {
		pvc, _ := obj.(*corev1.PersistentVolumeClaim)
		if !expectedPVCNames.Has(pvc.Name) {
			continue
		}
		missingPVCNames.Delete(pvc.Name)
		cond := findPVCRestoreCondition(pvc)
		if cond == nil {
			continue
		}
		switch cond.Status {
		case corev1.ConditionTrue:
			completedPVCNames.Insert(pvc.Name)
		case corev1.ConditionFalse:
			return &metav1.Condition{
				Type:               string(workloads.Restore),
				Status:             metav1.ConditionFalse,
				ObservedGeneration: its.Generation,
				Reason:             workloads.ReasonRestoreFailed,
				Message:            fmt.Sprintf("PVC %s restore failed: %s", pvc.Name, cond.Message),
			}, nil
		}
	}
	if missingPVCNames.Len() > 0 {
		return &metav1.Condition{
			Type:               string(workloads.Restore),
			Status:             metav1.ConditionUnknown,
			ObservedGeneration: its.Generation,
			Reason:             workloads.ReasonRestoreRunning,
			Message:            fmt.Sprintf("Waiting for restore PVCs to be created: %s", restoreConditionNamesMessage(missingPVCNames)),
		}, nil
	}
	if completedPVCNames.Len() == expectedPVCNames.Len() {
		return &metav1.Condition{
			Type:               string(workloads.Restore),
			Status:             metav1.ConditionTrue,
			ObservedGeneration: its.Generation,
			Reason:             workloads.ReasonRestoreCompleted,
			Message:            "All initial restore PVCs have completed",
		}, nil
	}
	return &metav1.Condition{
		Type:               string(workloads.Restore),
		Status:             metav1.ConditionUnknown,
		ObservedGeneration: its.Generation,
		Reason:             workloads.ReasonRestoreRunning,
		Message:            "Waiting for initial restore PVCs to complete",
	}, nil
}

func expectedRestorePVCNames(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet) (sets.Set[string], error) {
	itsExt, err := instancetemplate.BuildInstanceSetExt(its, tree)
	if err != nil {
		return nil, err
	}
	nameBuilder, err := instancetemplate.NewPodNameBuilder(itsExt, nil)
	if err != nil {
		return nil, err
	}
	nameToTemplateMap, err := nameBuilder.BuildInstanceName2TemplateMap()
	if err != nil {
		return nil, err
	}
	pvcNames := sets.New[string]()
	for instanceName, template := range nameToTemplateMap {
		for i := range template.VolumeClaimTemplates {
			vct := &template.VolumeClaimTemplates[i]
			if vct.Annotations[constant.RestoreSourceKindAnnotationKey] == "" {
				continue
			}
			pvcNames.Insert(intctrlutil.ComposePVCName(*vct, its.Name, instanceName))
		}
	}
	return pvcNames, nil
}

func findPVCRestoreCondition(pvc *corev1.PersistentVolumeClaim) *corev1.PersistentVolumeClaimCondition {
	for i := range pvc.Status.Conditions {
		if string(pvc.Status.Conditions[i].Type) == string(workloads.Restore) {
			return &pvc.Status.Conditions[i]
		}
	}
	return nil
}

func restoreConditionNamesMessage(names sets.Set[string]) string {
	sortedNames := sets.List(names)
	sort.Strings(sortedNames)
	return strings.Join(sortedNames, ",")
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
	desiredAssignments, templateNames, err := instancetemplate.BuildAssignments(tree, its)
	if err != nil {
		return err
	}
	desiredTemplates, err := desiredInstanceTemplates(its, tree)
	if err != nil {
		return err
	}
	pvcsByName := persistentVolumeClaimsByName(tree)
	desiredTemplateAssignments := make([]instancestatus.TemplateAssignment, 0, len(desiredAssignments))
	desiredNames := make(map[string]struct{}, len(desiredAssignments))
	for _, assignment := range desiredAssignments {
		desiredTemplateAssignments = append(desiredTemplateAssignments, instancestatus.TemplateAssignment{InstanceName: assignment.InstanceName, TemplateName: assignment.TemplateName})
		desiredNames[assignment.InstanceName] = struct{}{}
	}
	updateRevisions, err := GetRevisions(its.Status.UpdateRevisions)
	if err != nil {
		return err
	}
	offlineNames := append([]string(nil), its.Spec.OfflineInstances...)
	templateHints := make([]instancestatus.TemplateAssignment, 0, len(desiredTemplateAssignments)+len(pods)+len(offlineNames))
	if isStopRequested(its) {
		// Stop removes the desired Pod assignments, but they remain useful for retaining template identity
		// while the corresponding Pods are draining or have already disappeared.
		for _, assignment := range desiredTemplateAssignments {
			offlineNames = append(offlineNames, assignment.InstanceName)
			templateHints = append(templateHints, assignment)
		}
		desiredTemplateAssignments = nil
	}

	observations := make([]instancestatus.Observation, 0, len(pods))
	roleMap := composeRoleMap(*its)
	for _, pod := range pods {
		state := workloads.InstanceCurrentStatePresent
		if !pod.DeletionTimestamp.IsZero() {
			state = workloads.InstanceCurrentStateTerminating
		}
		if templateName, ok := instancetemplate.TemplateNameFromLabels(pod.Labels); ok {
			templateHints = append(templateHints, instancestatus.TemplateAssignment{InstanceName: pod.Name, TemplateName: templateName})
		}
		ready := state == workloads.InstanceCurrentStatePresent && isImageMatched(pod) && intctrlutil.IsPodReady(pod)
		observation := instancestatus.Observation{
			InstanceName: pod.Name,
			State:        state,
			Revision:     getPodRevision(pod),
			Ready:        ready,
			Available:    ready && intctrlutil.IsPodAvailable(pod, its.Spec.MinReadySeconds),
			Failed:       instancePodFailed(pod),
		}
		if state == workloads.InstanceCurrentStatePresent && intctrlutil.PodIsReadyWithLabel(*pod) {
			if role, ok := roleMap[getRoleName(pod)]; ok {
				observation.Role = role.Name
			}
		}
		configs, err := configsFromPod(pod)
		if err != nil {
			return err
		}
		for _, config := range configs {
			observation.Configs = append(observation.Configs, workloads.InstanceConfigStatus{Name: config.Name, ConfigHash: config.ConfigHash})
		}
		if state == workloads.InstanceCurrentStatePresent && isCreated(pod) {
			template := desiredTemplates[pod.Name]
			if _, active := desiredNames[pod.Name]; active && template != nil {
				podApplied, err := isDesiredPodApplied(its, pod, template)
				if err != nil {
					return err
				}
				observation.UpToDate = podApplied &&
					instancestatus.ConfigsApplied(its.Spec.Configs, observation.Configs) &&
					!hasPendingPVCExpansion(its.Name, pod.Name, template.VolumeClaimTemplates, pvcsByName)
			}
		}
		observations = append(observations, observation)
	}

	for _, name := range append(append([]string(nil), offlineNames...), podObservationNames(observations)...) {
		if _, ok := desiredNames[name]; ok {
			continue
		}
		if templateName, ok, err := instancetemplate.ResolveHistoricalTemplate(its, name, templateNames); err != nil {
			return err
		} else if ok {
			templateHints = append(templateHints, instancestatus.TemplateAssignment{InstanceName: name, TemplateName: templateName})
		}
	}
	syncObservationPVCStatus(tree, observations)

	statuses, err := instancestatus.Build(instancestatus.Input{
		Previous:           its.Status.InstanceStatus,
		DesiredAssignments: desiredTemplateAssignments,
		Offline:            offlineNames,
		Observations:       observations,
		TemplateHints:      templateHints,
		UpdateRevisions:    updateRevisions,
	})
	if err != nil {
		return err
	}
	its.Status.InstanceStatus = statuses
	return nil
}

func podObservationNames(observations []instancestatus.Observation) []string {
	names := make([]string, 0, len(observations))
	for _, observation := range observations {
		names = append(names, observation.InstanceName)
	}
	return names
}

func syncObservationPVCStatus(tree *kubebuilderx.ObjectTree, observations []instancestatus.Observation) {
	if tree == nil {
		return
	}
	for _, obj := range tree.List(&corev1.PersistentVolumeClaim{}) {
		pvc, _ := obj.(*corev1.PersistentVolumeClaim)
		if !isPVCExpansionRunning(pvc) {
			continue
		}
		instName := pvc.Labels[constant.KBAppPodNameLabelKey]
		for i := range observations {
			if observations[i].InstanceName == instName && observations[i].State == workloads.InstanceCurrentStatePresent {
				// TODO: how to check the expansion failed?
				observations[i].VolumeExpansion = true
				break
			}
		}
	}
}

func desiredInstanceTemplates(its *workloads.InstanceSet, tree *kubebuilderx.ObjectTree) (map[string]*instancetemplate.InstanceTemplateExt, error) {
	itsExt, err := instancetemplate.BuildInstanceSetExt(its, tree)
	if err != nil {
		return nil, err
	}
	nameBuilder, err := instancetemplate.NewPodNameBuilder(itsExt, nil)
	if err != nil {
		return nil, err
	}
	return nameBuilder.BuildInstanceName2TemplateMap()
}

func persistentVolumeClaimsByName(tree *kubebuilderx.ObjectTree) map[string]*corev1.PersistentVolumeClaim {
	result := map[string]*corev1.PersistentVolumeClaim{}
	if tree == nil {
		return result
	}
	for _, obj := range tree.List(&corev1.PersistentVolumeClaim{}) {
		pvc, _ := obj.(*corev1.PersistentVolumeClaim)
		result[pvc.Name] = pvc
	}
	return result
}

func isDesiredPodApplied(its *workloads.InstanceSet, pod *corev1.Pod, template *instancetemplate.InstanceTemplateExt) (bool, error) {
	updated, err := isPodUpdated(its, pod)
	if err != nil || !updated {
		return updated, err
	}
	desiredPod, err := buildInstancePodByTemplateForUpdate(pod, template, its)
	if err != nil {
		return false, err
	}
	// Resource requests are intentionally excluded from the legacy revision hash and may also be
	// ignored by the update-policy feature gate. They remain part of InstanceStatus.UpToDate.
	return equalResourcesInPlaceFields(pod, desiredPod), nil
}

func hasPendingPVCExpansion(itsName, instanceName string, templates []corev1.PersistentVolumeClaim,
	pvcsByName map[string]*corev1.PersistentVolumeClaim) bool {
	for _, template := range templates {
		desired, desiredOK := template.Spec.Resources.Requests[corev1.ResourceStorage]
		if !desiredOK || desired.IsZero() {
			continue
		}
		pvc := pvcsByName[intctrlutil.ComposePVCName(template, itsName, instanceName)]
		if pvc == nil {
			continue
		}
		capacity, capacityOK := pvc.Status.Capacity[corev1.ResourceStorage]
		if capacityOK && capacity.Cmp(desired) < 0 {
			return true
		}
	}
	return false
}

func isPVCExpansionRunning(pvc *corev1.PersistentVolumeClaim) bool {
	if pvc == nil {
		return false
	}
	requested, requestedOK := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	capacity, capacityOK := pvc.Status.Capacity[corev1.ResourceStorage]
	return requestedOK && capacityOK && capacity.Cmp(requested) < 0
}
