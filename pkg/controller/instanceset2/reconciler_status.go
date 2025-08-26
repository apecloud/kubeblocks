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

package instanceset2

import (
	"encoding/json"
	"slices"
	"sort"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset/instancetemplate"
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
	its, _ := tree.GetRoot().(*workloads.InstanceSet)

	instances := tree.List(&workloads.Instance{})
	var instanceList []*workloads.Instance
	for _, object := range instances {
		inst, _ := object.(*workloads.Instance)
		instanceList = append(instanceList, inst)
	}

	replicas := int32(0)
	ordinals := make([]int32, 0)
	currentReplicas, updatedReplicas := int32(0), int32(0)
	readyReplicas, availableReplicas := int32(0), int32(0)
	notReadyNames := sets.New[string]()
	notAvailableNames := sets.New[string]()
	// currentRevisions := map[string]string{}

	template2TemplatesStatus := map[string]*workloads.InstanceTemplateStatus{}
	template2TotalReplicas := map[string]int32{}
	for _, template := range its.Spec.Instances {
		templateReplicas := int32(1)
		if template.Replicas != nil {
			templateReplicas = *template.Replicas
		}
		template2TotalReplicas[template.Name] = templateReplicas
	}

	// podToNodeMapping, err := ParseNodeSelectorOnceAnnotation(its)
	// if err != nil {
	//	return kubebuilderx.Continue, err
	// }

	for _, inst := range instanceList {
		_, ordinal := parseParentNameAndOrdinal(inst.Name)
		templateName := inst.Labels[instancetemplate.TemplateNameLabelKey]
		if template2TemplatesStatus[templateName] == nil {
			template2TemplatesStatus[templateName] = &workloads.InstanceTemplateStatus{
				Name:     templateName,
				Ordinals: make([]int32, 0),
			}
		}
		{
			notReadyNames.Insert(inst.Name)
			replicas++
			if len(templateName) == 0 {
				ordinals = append(ordinals, int32(ordinal))
			}
			template2TemplatesStatus[templateName].Replicas++
			template2TemplatesStatus[templateName].Ordinals = append(template2TemplatesStatus[templateName].Ordinals, int32(ordinal))
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
		if !intctrlutil.IsInstanceTerminating(inst) {
			if isInstanceUpdated(its, inst) {
				updatedReplicas++
				template2TemplatesStatus[templateName].UpdatedReplicas++
			} else {
				currentReplicas++
				template2TemplatesStatus[templateName].CurrentReplicas++
			}
		}

		// TODO: ???
		// if nodeName, ok := podToNodeMapping[inst.Name]; ok {
		//	// there's chance that a pod is currently running and wait to be deleted so that it can be rescheduled
		//	if inst.Spec.NodeName == nodeName {
		//		if err := deleteNodeSelectorOnceAnnotation(its, inst.Name); err != nil {
		//			return kubebuilderx.Continue, err
		//		}
		//	}
		// }
	}
	its.Status.Replicas = replicas
	its.Status.Ordinals = ordinals
	slices.Sort(its.Status.Ordinals)
	its.Status.ReadyReplicas = readyReplicas
	its.Status.AvailableReplicas = availableReplicas
	its.Status.CurrentReplicas = currentReplicas
	its.Status.UpdatedReplicas = updatedReplicas
	// its.Status.CurrentRevisions, _ = buildRevisions(currentRevisions)
	its.Status.TemplatesStatus = buildTemplatesStatus(template2TemplatesStatus)
	// all pods have been updated
	totalReplicas := int32(1)
	if its.Spec.Replicas != nil {
		totalReplicas = *its.Spec.Replicas
	}
	if its.Status.Replicas == totalReplicas && its.Status.UpdatedReplicas == totalReplicas {
		// its.Status.CurrentRevision = its.Status.UpdateRevision
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

	// 4. set members status
	setMembersStatus(its, instanceList)

	// 5. set instance status
	setInstanceStatus(its, instanceList)

	if its.Spec.MinReadySeconds > 0 && availableReplicas != readyReplicas {
		return kubebuilderx.RetryAfter(time.Second), nil
	}
	return kubebuilderx.Continue, nil
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
		slices.Sort(templateStatus.Ordinals)
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

func setMembersStatus(its *workloads.InstanceSet, instances []*workloads.Instance) {
	// no roles defined
	if its.Spec.Roles == nil {
		return
	}
	// compose new status
	newMembersStatus := make([]workloads.MemberStatus, 0)
	roleMap := composeRoleMap(*its)
	for _, inst := range instances {
		if !intctrlutil.IsInstanceReadyWithRole(inst) {
			continue
		}
		roleName := getInstanceRoleName(inst)
		role, ok := roleMap[roleName]
		if !ok {
			continue
		}
		memberStatus := workloads.MemberStatus{
			PodName:     inst.Name,
			ReplicaRole: &role,
		}
		newMembersStatus = append(newMembersStatus, memberStatus)
	}

	// sort and set
	rolePriorityMap := composeRolePriorityMap(its.Spec.Roles)
	sortMembersStatus(newMembersStatus, rolePriorityMap)
	its.Status.MembersStatus = newMembersStatus
}

func sortMembersStatus(membersStatus []workloads.MemberStatus, rolePriorityMap map[string]int) {
	getRolePriorityFunc := func(i int) int {
		role := membersStatus[i].ReplicaRole.Name
		return rolePriorityMap[role]
	}
	getNameNOrdinalFunc := func(i int) (string, int) {
		return parseParentNameAndOrdinal(membersStatus[i].PodName)
	}
	baseSort(membersStatus, getNameNOrdinalFunc, getRolePriorityFunc, true)
}

func setInstanceStatus(its *workloads.InstanceSet, instances []*workloads.Instance) {
	// compose new instance status
	newInstanceStatus := make([]workloads.InstanceStatus, 0)
	for _, inst := range instances {
		instanceStatus := workloads.InstanceStatus{
			PodName: inst.Name,
		}
		newInstanceStatus = append(newInstanceStatus, instanceStatus)
	}

	syncInstanceConfigStatus(its, newInstanceStatus)

	sortInstanceStatus(newInstanceStatus)
	its.Status.InstanceStatus = newInstanceStatus
}

func syncInstanceConfigStatus(its *workloads.InstanceSet, instanceStatus []workloads.InstanceStatus) {
	if its.Status.InstanceStatus == nil {
		// initialize
		configs := make([]workloads.InstanceConfigStatus, 0)
		for _, config := range its.Spec.Configs {
			configs = append(configs, workloads.InstanceConfigStatus{
				Name:       config.Name,
				Generation: config.Generation,
			})
		}
		for i := range instanceStatus {
			instanceStatus[i].Configs = configs
		}
	} else {
		// HACK: copy the existing config status from the current its.status.instanceStatus
		configs := sets.New[string]()
		for _, config := range its.Spec.Configs {
			configs.Insert(config.Name)
		}
		for i, newStatus := range instanceStatus {
			for _, status := range its.Status.InstanceStatus {
				if status.PodName == newStatus.PodName {
					if instanceStatus[i].Configs == nil {
						instanceStatus[i].Configs = make([]workloads.InstanceConfigStatus, 0)
					}
					for j, config := range status.Configs {
						if configs.Has(config.Name) {
							instanceStatus[i].Configs = append(instanceStatus[i].Configs, status.Configs[j])
						}
					}
					break
				}
			}
		}
	}
}

func sortInstanceStatus(instanceStatus []workloads.InstanceStatus) {
	getNameNOrdinalFunc := func(i int) (string, int) {
		return parseParentNameAndOrdinal(instanceStatus[i].PodName)
	}
	baseSort(instanceStatus, getNameNOrdinalFunc, nil, true)
}
