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

package instanceset2

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	instctrl "github.com/apecloud/kubeblocks/pkg/controller/instance"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/revisionmap"
	"github.com/apecloud/kubeblocks/pkg/controller/workloads/instancestatus"
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
	its, _ := tree.GetRoot().(*workloads.InstanceSet)

	instances := tree.List(&workloads.Instance{})
	var instanceList []*workloads.Instance
	for _, object := range instances {
		inst, _ := object.(*workloads.Instance)
		instanceList = append(instanceList, inst)
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
	replicas := int32(0)
	currentReplicas, updatedReplicas := int32(0), int32(0)
	readyReplicas, availableReplicas := int32(0), int32(0)
	notReadyNames := sets.New[string]()
	notAvailableNames := sets.New[string]()
	currentRevisions := map[string]string{}
	updateRevisions, err := revisionmap.Decode(its.Status.UpdateRevisions)
	if err != nil {
		return kubebuilderx.Continue, err
	}

	template2TemplatesStatus := map[string]*workloads.InstanceTemplateStatus{}
	template2TotalReplicas := map[string]int32{}
	for _, template := range its.Spec.Instances {
		templateReplicas := int32(1)
		if template.Replicas != nil {
			templateReplicas = *template.Replicas
		}
		template2TotalReplicas[template.Name] = templateReplicas
	}

	for _, inst := range instanceList {
		templateName := getInstanceTemplateName(inst)
		if template2TemplatesStatus[templateName] == nil {
			template2TemplatesStatus[templateName] = &workloads.InstanceTemplateStatus{
				Name: templateName,
			}
		}
		{
			notReadyNames.Insert(inst.Name)
			replicas++
			template2TemplatesStatus[templateName].Replicas++
		}
		if intctrlutil.IsInstanceReady(inst) {
			readyReplicas++
			template2TemplatesStatus[templateName].ReadyReplicas++
			notReadyNames.Delete(inst.Name)
			if intctrlutil.IsInstanceAvailable(inst) {
				availableReplicas++
				template2TemplatesStatus[templateName].AvailableReplicas++
			} else {
				notAvailableNames.Insert(inst.Name)
			}
		}
		currentRevisions[inst.Name] = getInstanceRevision(inst)
		if !intctrlutil.IsInstanceTerminating(inst) {
			if isInstanceUpdatedWithRevisions(inst, currentRevisions[inst.Name], updateRevisions) {
				updatedReplicas++
				template2TemplatesStatus[templateName].UpdatedReplicas++
			} else {
				currentReplicas++
				template2TemplatesStatus[templateName].CurrentReplicas++
			}
		}
	}
	its.Status.Replicas = replicas
	its.Status.ReadyReplicas = readyReplicas
	its.Status.AvailableReplicas = availableReplicas
	its.Status.CurrentReplicas = currentReplicas
	its.Status.UpdatedReplicas = updatedReplicas
	its.Status.CurrentRevisions, _ = revisionmap.Encode(currentRevisions)
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
	failureCondition, err := buildFailureCondition(its, instanceList)
	if err != nil {
		return kubebuilderx.Continue, err
	}
	if failureCondition != nil {
		meta.SetStatusCondition(&its.Status.Conditions, *failureCondition)
	} else {
		meta.RemoveStatusCondition(&its.Status.Conditions, string(workloads.InstanceFailure))
	}

	if err = r.reconcileRestoreCondition(tree, its, instanceList); err != nil {
		return kubebuilderx.Continue, err
	}

	// 4. set instance status
	if err := setInstanceStatus(tree, its, instanceList); err != nil {
		return kubebuilderx.Continue, err
	}

	if its.Spec.MinReadySeconds > 0 && availableReplicas != readyReplicas {
		return kubebuilderx.RetryAfter(time.Second), nil
	}
	return kubebuilderx.Continue, nil
}

func (r *statusReconciler) reconcileRestoreCondition(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, instances []*workloads.Instance) error {
	restoreCond := meta.FindStatusCondition(its.Status.Conditions, string(workloads.InstanceRestore))
	if restoreCond != nil && (restoreCond.Status == metav1.ConditionTrue || restoreCond.Status == metav1.ConditionFalse) {
		return nil
	}
	condition, err := buildRestoreCondition(tree, its, instances)
	if err != nil {
		return err
	}
	if condition == nil {
		meta.RemoveStatusCondition(&its.Status.Conditions, string(workloads.InstanceRestore))
		return nil
	}
	meta.SetStatusCondition(&its.Status.Conditions, *condition)
	return nil
}

func buildRestoreCondition(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, instances []*workloads.Instance) (*metav1.Condition, error) {
	desiredInstances, _, err := buildDesiredInstancesByName(tree, its)
	if err != nil {
		return nil, err
	}
	expectedNames := sets.New[string]()
	for name, inst := range desiredInstances {
		for i := range inst.Spec.VolumeClaimTemplates {
			if inst.Spec.VolumeClaimTemplates[i].Annotations[constant.RestoreSourceKindAnnotationKey] != "" {
				expectedNames.Insert(name)
				break
			}
		}
	}
	if expectedNames.Len() == 0 {
		return nil, nil
	}

	instancesByName := make(map[string]*workloads.Instance, len(instances))
	for _, inst := range instances {
		instancesByName[inst.Name] = inst
	}
	completed := 0
	waiting := sets.New[string]()
	for name := range expectedNames {
		inst := instancesByName[name]
		if inst == nil {
			waiting.Insert(name)
			continue
		}
		cond := meta.FindStatusCondition(inst.Status.Conditions, string(workloads.InstanceRestore))
		if cond == nil || cond.Status == metav1.ConditionUnknown {
			waiting.Insert(name)
			continue
		}
		if cond.Status == metav1.ConditionFalse {
			return &metav1.Condition{
				Type:               string(workloads.InstanceRestore),
				Status:             metav1.ConditionFalse,
				ObservedGeneration: its.Generation,
				Reason:             workloads.ReasonRestoreFailed,
				Message:            fmt.Sprintf("Instance %s restore failed: %s", inst.Name, cond.Message),
			}, nil
		}
		if cond.Status == metav1.ConditionTrue {
			completed++
		}
	}
	if completed == expectedNames.Len() {
		return &metav1.Condition{
			Type:               string(workloads.InstanceRestore),
			Status:             metav1.ConditionTrue,
			ObservedGeneration: its.Generation,
			Reason:             workloads.ReasonRestoreCompleted,
			Message:            "All initial restore Instances have completed",
		}, nil
	}
	message, err := buildConditionMessageWithNames(waiting.UnsortedList())
	if err != nil {
		return nil, err
	}
	return &metav1.Condition{
		Type:               string(workloads.InstanceRestore),
		Status:             metav1.ConditionUnknown,
		ObservedGeneration: its.Generation,
		Reason:             workloads.ReasonRestoreRunning,
		Message:            fmt.Sprintf("Waiting for initial restore Instances to complete: %s", message),
	}, nil
}

func getInstanceTemplateName(inst *workloads.Instance) string {
	if inst.Labels == nil {
		return ""
	}
	if templateName := inst.Labels[instancetemplate.TemplateNameLabelKey]; templateName != "" {
		return templateName
	}
	return inst.Labels[constant.KBAppInstanceTemplateLabelKey]
}

func buildConditionMessageWithNames(instanceNames []string) ([]byte, error) {
	baseSort(instanceNames, func(i int) (string, int) {
		return parseParentNameAndOrdinal(instanceNames[i])
	}, nil, true)
	return json.Marshal(instanceNames)
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

func buildFailureCondition(its *workloads.InstanceSet, instances []*workloads.Instance) (*metav1.Condition, error) {
	var failureNames []string
	for _, inst := range instances {
		if intctrlutil.IsInstanceFailure(inst) {
			failureNames = append(failureNames, inst.Name)
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

func setInstanceStatus(tree *kubebuilderx.ObjectTree, its *workloads.InstanceSet, instances []*workloads.Instance) error {
	desiredAssignments, templateNames, err := instancetemplate.BuildAssignments(tree, its)
	if err != nil {
		return err
	}
	desiredTemplateAssignments := make([]instancestatus.TemplateAssignment, 0, len(desiredAssignments))
	desiredNames := make(map[string]struct{}, len(desiredAssignments))
	for _, assignment := range desiredAssignments {
		desiredTemplateAssignments = append(desiredTemplateAssignments, instancestatus.TemplateAssignment{InstanceName: assignment.InstanceName, TemplateName: assignment.TemplateName})
		desiredNames[assignment.InstanceName] = struct{}{}
	}
	instanceSpecUpdateRevisions, err := revisionmap.Decode(its.Status.UpdateRevisions)
	if err != nil {
		return err
	}
	desiredInstances, _, err := buildDesiredInstancesByName(tree, its)
	if err != nil {
		return err
	}
	podUpdateRevisions := make(map[string]string, len(desiredInstances))
	for name, desired := range desiredInstances {
		podRevision, err := instctrl.BuildPodRevision(desired)
		if err != nil {
			return err
		}
		podUpdateRevisions[name] = podRevision
	}
	offlineNames := append([]string(nil), its.Spec.OfflineInstances...)
	templateHints := make([]instancestatus.TemplateAssignment, 0, len(desiredTemplateAssignments)+len(instances)+len(offlineNames))
	if its.Spec.Stop != nil && *its.Spec.Stop {
		// Stop removes the desired Pod assignments, but they remain useful for retaining template identity
		// while the corresponding Instances are draining or have already disappeared.
		for _, assignment := range desiredTemplateAssignments {
			offlineNames = append(offlineNames, assignment.InstanceName)
			templateHints = append(templateHints, assignment)
		}
		desiredTemplateAssignments = nil
	}

	observations := make([]instancestatus.Observation, 0, len(instances))
	seenInstances := make(map[string]struct{}, len(instances))
	roleMap := composeRoleMap(*its)
	for _, inst := range instances {
		if _, ok := seenInstances[inst.Name]; ok {
			return fmt.Errorf("duplicate Instance object for %q", inst.Name)
		}
		seenInstances[inst.Name] = struct{}{}
		templateHints = append(templateHints, instancestatus.TemplateAssignment{InstanceName: inst.Name, TemplateName: inst.Spec.InstanceTemplateName})
		if templateName, ok := instancetemplate.TemplateNameFromLabels(inst.Labels); ok {
			templateHints = append(templateHints, instancestatus.TemplateAssignment{InstanceName: inst.Name, TemplateName: templateName})
		}
		switch inst.Status.CurrentState {
		case workloads.InstanceCurrentStatePresent, workloads.InstanceCurrentStateTerminating:
			// InstanceSet revisions identify desired Instance specs, while Instance CurrentRevision identifies the
			// actual Pod revision. Keep the two revision domains separate.
			instanceSpecRevision := getInstanceRevision(inst)
			observation := instancestatus.Observation{
				InstanceName:    inst.Name,
				State:           inst.Status.CurrentState,
				Revision:        inst.Status.CurrentRevision,
				UpToDate:        isInstanceUpdatedWithRevisions(inst, instanceSpecRevision, instanceSpecUpdateRevisions),
				Ready:           inst.Status.Ready,
				Available:       inst.Status.Available,
				Failed:          meta.IsStatusConditionTrue(inst.Status.Conditions, string(workloads.InstanceFailure)),
				Configs:         inst.Status.Configs,
				VolumeExpansion: inst.Status.VolumeExpansion,
			}
			if inst.Status.Ready {
				if role, ok := roleMap[getInstanceRoleName(inst)]; ok {
					observation.Role = role.Name
				}
			}
			observations = append(observations, observation)
		case workloads.InstanceCurrentStateAbsent, "":
		default:
			return fmt.Errorf("instance %q has invalid current state %q", inst.Name, inst.Status.CurrentState)
		}
	}

	for _, name := range append(append([]string(nil), offlineNames...), observationNames(observations)...) {
		if _, ok := desiredNames[name]; ok {
			continue
		}
		if templateName, ok, err := instancetemplate.ResolveHistoricalTemplate(its, name, templateNames); err != nil {
			return err
		} else if ok {
			templateHints = append(templateHints, instancestatus.TemplateAssignment{InstanceName: name, TemplateName: templateName})
		}
	}

	statuses, err := instancestatus.Build(instancestatus.Input{
		Previous:           its.Status.InstanceStatus,
		DesiredAssignments: desiredTemplateAssignments,
		Offline:            offlineNames,
		Observations:       observations,
		TemplateHints:      templateHints,
		UpdateRevisions:    podUpdateRevisions,
	})
	if err != nil {
		return err
	}
	its.Status.InstanceStatus = statuses
	return nil
}

func observationNames(observations []instancestatus.Observation) []string {
	names := make([]string, 0, len(observations))
	for _, observation := range observations {
		names = append(names, observation.InstanceName)
	}
	return names
}
