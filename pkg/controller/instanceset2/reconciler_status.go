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

	// 4. set instance status
	if err := setInstanceStatus(tree, its, instanceList); err != nil {
		if instancetemplate.IsActiveAllocationIncomplete(err) {
			return kubebuilderx.Continue, nil
		}
		return kubebuilderx.Continue, err
	}

	if its.Spec.MinReadySeconds > 0 && availableReplicas != readyReplicas {
		return kubebuilderx.RetryAfter(time.Second), nil
	}
	return kubebuilderx.Continue, nil
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
	activeAllocations, templateNames, err := instancetemplate.BuildActiveAllocations(tree, its)
	if err != nil {
		return err
	}
	active := make([]instancestatus.Allocation, 0, len(activeAllocations))
	for _, allocation := range activeAllocations {
		active = append(active, instancestatus.Allocation{PodName: allocation.PodName, TemplateName: allocation.TemplateName})
	}
	updateRevisions, err := revisionmap.Decode(its.Status.UpdateRevisions)
	if err != nil {
		return err
	}
	offline := append([]string(nil), its.Spec.OfflineInstances...)
	hints := make([]instancestatus.Allocation, 0, len(active)+len(instances)+len(offline))
	if its.Spec.Stop != nil && *its.Spec.Stop {
		for _, allocation := range active {
			offline = append(offline, allocation.PodName)
			hints = append(hints, allocation)
		}
		active = nil
	}

	observations := make([]instancestatus.CurrentObservation, 0, len(instances))
	seenInstances := make(map[string]struct{}, len(instances))
	roleMap := composeRoleMap(*its)
	for _, inst := range instances {
		if _, ok := seenInstances[inst.Name]; ok {
			return fmt.Errorf("duplicate Instance object for %q", inst.Name)
		}
		seenInstances[inst.Name] = struct{}{}
		hints = append(hints, instancestatus.Allocation{PodName: inst.Name, TemplateName: inst.Spec.InstanceTemplateName})
		if templateName, ok := instancetemplate.TemplateNameFromLabels(inst.Labels); ok {
			hints = append(hints, instancestatus.Allocation{PodName: inst.Name, TemplateName: templateName})
		}
		switch inst.Status.CurrentState {
		case workloads.InstanceCurrentStatePresent, workloads.InstanceCurrentStateTerminating:
			currentRevision := getInstanceRevision(inst)
			observation := instancestatus.CurrentObservation{
				InstanceName:    inst.Name,
				State:           inst.Status.CurrentState,
				CurrentRevision: currentRevision,
				UpToDate:        isInstanceUpdatedWithRevisions(inst, currentRevision, updateRevisions),
				Ready:           intctrlutil.IsInstanceReady(inst),
				Available:       intctrlutil.IsInstanceAvailable(inst),
				Failed: !intctrlutil.IsInstanceTerminating(inst) && inst.Status.ObservedGeneration == inst.Generation &&
					meta.IsStatusConditionTrue(inst.Status.Conditions, string(workloads.InstanceFailure)),
				Configs:         inst.Status.Configs,
				VolumeExpansion: inst.Status.VolumeExpansion,
			}
			if intctrlutil.IsInstanceReadyWithRole(inst) {
				if role, ok := roleMap[getInstanceRoleName(inst)]; ok {
					observation.Role = role.Name
				}
			}
			observations = append(observations, observation)
		case workloads.InstanceCurrentStateAbsent:
		case "":
			return fmt.Errorf("instance %q has not reported current state", inst.Name)
		default:
			return fmt.Errorf("instance %q has invalid current state %q", inst.Name, inst.Status.CurrentState)
		}
	}

	for _, name := range append(append([]string(nil), offline...), observationNames(observations)...) {
		if templateName, ok, err := instancetemplate.HistoricalTemplateHint(its, name, templateNames); err != nil {
			return err
		} else if ok {
			hints = append(hints, instancestatus.Allocation{PodName: name, TemplateName: templateName})
		}
	}

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
	setIncompleteInstanceStatusCondition(its, result.UnknownTemplateNames)
	return nil
}

func observationNames(observations []instancestatus.CurrentObservation) []string {
	names := make([]string, 0, len(observations))
	for _, observation := range observations {
		names = append(names, observation.InstanceName)
	}
	return names
}

func setIncompleteInstanceStatusCondition(its *workloads.InstanceSet, names []string) {
	if len(names) == 0 {
		meta.RemoveStatusCondition(&its.Status.Conditions, string(workloads.InstanceStatusIncomplete))
		return
	}
	message, _ := json.Marshal(names)
	meta.SetStatusCondition(&its.Status.Conditions, metav1.Condition{
		Type:               string(workloads.InstanceStatusIncomplete),
		Status:             metav1.ConditionTrue,
		ObservedGeneration: its.Generation,
		Reason:             workloads.ReasonTemplateNameUnknown,
		Message:            string(message),
	})
}
