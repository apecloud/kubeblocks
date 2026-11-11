/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package instancestatus

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// TemplateAssignment associates an instance identity with a template.
type TemplateAssignment struct {
	InstanceName string
	TemplateName string
}

// Observation contains runtime fields observed from a Pod or Instance.
type Observation struct {
	InstanceName    string
	State           workloads.InstanceCurrentState
	Revision        string
	UpToDate        bool
	Ready           bool
	Available       bool
	Failed          bool
	Role            string
	Configs         []workloads.InstanceConfigStatus
	VolumeExpansion bool
}

// Input contains the independently produced desired and observed dimensions used to build InstanceStatus.
type Input struct {
	Previous           []workloads.InstanceStatus
	DesiredAssignments []TemplateAssignment
	Offline            []string
	Observations       []Observation
	TemplateHints      []TemplateAssignment
	UpdateRevisions    map[string]string
}

// Health is one trusted observation of an identity's runtime health.
type Health struct {
	Ready     bool
	Available bool
}

// PodHealth observes runtime health using the current parent's availability policy.
func PodHealth(pod *corev1.Pod, minReadySeconds int32) Health {
	if pod == nil || !intctrlutil.IsPodReady(pod) {
		return Health{}
	}
	for _, container := range pod.Spec.Containers {
		index := slices.IndexFunc(pod.Status.ContainerStatuses, func(status corev1.ContainerStatus) bool {
			return status.Name == container.Name
		})
		if index < 0 {
			continue
		}
		status := pod.Status.ContainerStatuses[index]
		if !intctrlutil.MatchContainerImageInStatus(container.Image, status.Image, status.ImageID) {
			return Health{}
		}
	}
	return Health{Ready: true, Available: intctrlutil.IsPodAvailable(pod, minReadySeconds)}
}

// SetReadinessConditions evaluates configured identities independently of extra runtime objects.
func SetReadinessConditions(its *workloads.InstanceSet, assignments []instancetemplate.Assignment, health map[string]Health, assignmentErr error) error {
	if assignmentErr != nil {
		if !instancetemplate.IsAssignmentIncomplete(assignmentErr) {
			return assignmentErr
		}
		for _, kind := range []workloads.ConditionType{workloads.InstanceReady, workloads.InstanceAvailable} {
			meta.SetStatusCondition(&its.Status.Conditions, metav1.Condition{
				Type: string(kind), Status: metav1.ConditionUnknown, ObservedGeneration: its.Generation,
				Reason: "AssignmentIncomplete", Message: assignmentErr.Error(),
			})
		}
		return nil
	}
	var notReady, notAvailable []string
	for _, assignment := range assignments {
		member := health[assignment.InstanceName]
		if !member.Ready {
			notReady = append(notReady, assignment.InstanceName)
		}
		if !member.Ready || !member.Available {
			notAvailable = append(notAvailable, assignment.InstanceName)
		}
	}
	for _, result := range []struct {
		kind          workloads.ConditionType
		unhealthy     []string
		successReason string
		failureReason string
	}{
		{workloads.InstanceReady, notReady, workloads.ReasonReady, workloads.ReasonNotReady},
		{workloads.InstanceAvailable, notAvailable, workloads.ReasonAvailable, workloads.ReasonNotAvailable},
	} {
		condition := metav1.Condition{Type: string(result.kind), Status: metav1.ConditionTrue, ObservedGeneration: its.Generation, Reason: result.successReason}
		if len(result.unhealthy) > 0 {
			condition.Status = metav1.ConditionFalse
			condition.Reason = result.failureReason
			message, err := json.Marshal(result.unhealthy)
			if err != nil {
				return err
			}
			condition.Message = string(message)
		}
		meta.SetStatusCondition(&its.Status.Conditions, condition)
	}
	return nil
}

// ReadinessConditionsChanged identifies observations that must persist before child changes.
func ReadinessConditionsChanged(previous, current []metav1.Condition) bool {
	for _, kind := range []string{string(workloads.InstanceReady), string(workloads.InstanceAvailable)} {
		condition := meta.FindStatusCondition(current, kind)
		old := meta.FindStatusCondition(previous, kind)
		if condition != nil && (old == nil || old.Status != condition.Status || old.ObservedGeneration != condition.ObservedGeneration) {
			return true
		}
	}
	return false
}

// ConfigsApplied reports whether every desired config hash has been observed for an instance.
// Extra observed entries do not make the desired config stale; they may belong to configuration
// that is no longer managed by the current InstanceSet spec.
func ConfigsApplied(desired []workloads.ConfigTemplate, observed []workloads.InstanceConfigStatus) bool {
	for _, config := range desired {
		found := false
		for _, status := range observed {
			if status.Name == config.Name && ptr.Deref(status.ConfigHash, "") == ptr.Deref(config.ConfigHash, "") {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Build merges InstanceStatus by PodName. It carries only retained template identity from Previous;
// all observed revision, health, and runtime fields are rebuilt from Observations.
func Build(input Input) ([]workloads.InstanceStatus, error) {
	previousByName, err := indexPrevious(input.Previous)
	if err != nil {
		return nil, err
	}
	desiredByName, err := indexAssignments("desired assignment", input.DesiredAssignments, true)
	if err != nil {
		return nil, err
	}
	// The desired assignment is authoritative. An observed object may still carry the previous template while an
	// identity is moving between templates, so its hint must not veto the desired assignment.
	nonDesiredHints := make([]TemplateAssignment, 0, len(input.TemplateHints))
	for _, hint := range input.TemplateHints {
		if _, ok := desiredByName[hint.InstanceName]; !ok {
			nonDesiredHints = append(nonDesiredHints, hint)
		}
	}
	templateHintsByName, err := indexAssignments("template hint", nonDesiredHints, false)
	if err != nil {
		return nil, err
	}
	offlineNames := indexOfflineNames(input.Offline)
	for name := range desiredByName {
		if offlineNames[name] {
			return nil, fmt.Errorf("instance %q is both desired to run and Offline", name)
		}
	}

	observationsByName := make(map[string]*Observation, len(input.Observations))
	for i := range input.Observations {
		observation := &input.Observations[i]
		if observation.InstanceName == "" {
			return nil, fmt.Errorf("observation has an empty instance name")
		}
		if _, ok := observationsByName[observation.InstanceName]; ok {
			return nil, fmt.Errorf("duplicate observation for %q", observation.InstanceName)
		}
		if observation.State != workloads.InstanceCurrentStatePresent && observation.State != workloads.InstanceCurrentStateTerminating {
			return nil, fmt.Errorf("observation for %q has invalid state %q", observation.InstanceName, observation.State)
		}
		observationsByName[observation.InstanceName] = observation
	}

	names := make(map[string]struct{}, len(desiredByName)+len(offlineNames)+len(observationsByName))
	for name := range desiredByName {
		names[name] = struct{}{}
	}
	for name := range offlineNames {
		names[name] = struct{}{}
	}
	for name := range observationsByName {
		names[name] = struct{}{}
	}

	// Previous is intentionally excluded from the output identity set. It may retain template identity for a
	// desired or observed instance, but must not keep a fully released and disappeared instance alive forever.
	statuses := make([]workloads.InstanceStatus, 0, len(names))
	for name := range names {
		status := workloads.InstanceStatus{PodName: name, CurrentState: workloads.InstanceCurrentStateAbsent}
		observation := observationsByName[name]
		if observation != nil {
			status.CurrentState = observation.State
			status.CurrentRevision = observation.Revision
		}

		switch {
		case desiredByName[name] != nil:
			status.DesiredState = workloads.InstanceDesiredStateActive
			status.TemplateName = ptr.To(*desiredByName[name])
			status.UpdateRevision = input.UpdateRevisions[name]
		case offlineNames[name]:
			status.DesiredState = workloads.InstanceDesiredStateOffline
			status.TemplateName = retainedTemplateName(name, previousByName, templateHintsByName)
		default:
			status.DesiredState = workloads.InstanceDesiredStateReleased
			status.TemplateName = retainedTemplateName(name, previousByName, templateHintsByName)
		}

		if status.DesiredState != workloads.InstanceDesiredStateActive {
			if old := previousByName[name]; old != nil && old.TemplateName != nil && status.TemplateName != nil && *old.TemplateName != *status.TemplateName {
				return nil, fmt.Errorf("instance %q has conflicting template assignments %q and %q", name, *old.TemplateName, *status.TemplateName)
			}
			if hint := templateHintsByName[name]; hint != nil && status.TemplateName != nil && *hint != *status.TemplateName {
				return nil, fmt.Errorf("instance %q has conflicting template assignments %q and %q", name, *hint, *status.TemplateName)
			}
		}

		// Terminating observations retain only lifecycle state and revision. Runtime health belongs to a usable,
		// present instance and must be cleared rather than inherited from its previous status.
		if observation != nil && observation.State == workloads.InstanceCurrentStatePresent {
			status.Ready = observation.Ready
			status.Available = observation.Ready && observation.Available
			status.Failed = observation.Failed
			status.Role = observation.Role
			status.Configs = copyConfigs(observation.Configs)
			status.VolumeExpansion = observation.VolumeExpansion
			if status.DesiredState == workloads.InstanceDesiredStateActive {
				status.UpToDate = observation.UpToDate
			}
		}
		statuses = append(statuses, status)
	}

	sortStatuses(statuses)
	return statuses, nil
}

func indexPrevious(statuses []workloads.InstanceStatus) (map[string]*workloads.InstanceStatus, error) {
	result := make(map[string]*workloads.InstanceStatus, len(statuses))
	for i := range statuses {
		status := &statuses[i]
		if status.PodName == "" {
			return nil, fmt.Errorf("previous InstanceStatus has an empty PodName")
		}
		if _, ok := result[status.PodName]; ok {
			return nil, fmt.Errorf("duplicate previous InstanceStatus for %q", status.PodName)
		}
		result[status.PodName] = status
	}
	return result, nil
}

func indexAssignments(kind string, assignments []TemplateAssignment, rejectDuplicate bool) (map[string]*string, error) {
	result := make(map[string]*string, len(assignments))
	for _, assignment := range assignments {
		if assignment.InstanceName == "" {
			return nil, fmt.Errorf("%s has an empty instance name", kind)
		}
		if template, ok := result[assignment.InstanceName]; ok {
			if *template != assignment.TemplateName {
				return nil, fmt.Errorf("instance %q has conflicting %s templates %q and %q", assignment.InstanceName, kind, *template, assignment.TemplateName)
			}
			if rejectDuplicate {
				return nil, fmt.Errorf("duplicate %s for %q", kind, assignment.InstanceName)
			}
			continue
		}
		result[assignment.InstanceName] = ptr.To(assignment.TemplateName)
	}
	return result, nil
}

func indexOfflineNames(names []string) map[string]bool {
	result := make(map[string]bool, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		result[name] = true
	}
	return result
}

func retainedTemplateName(name string, previous map[string]*workloads.InstanceStatus, hints map[string]*string) *string {
	if old := previous[name]; old != nil && old.TemplateName != nil {
		return ptr.To(*old.TemplateName)
	}
	if hint := hints[name]; hint != nil {
		return ptr.To(*hint)
	}
	return nil
}

func copyConfigs(configs []workloads.InstanceConfigStatus) []workloads.InstanceConfigStatus {
	if configs == nil {
		return nil
	}
	result := make([]workloads.InstanceConfigStatus, len(configs))
	copy(result, configs)
	for i := range result {
		if configs[i].ConfigHash != nil {
			result[i].ConfigHash = ptr.To(*configs[i].ConfigHash)
		}
	}
	return result
}

func sortStatuses(statuses []workloads.InstanceStatus) {
	sort.Slice(statuses, func(i, j int) bool {
		leftParent, leftOrdinal := parentAndOrdinal(statuses[i].PodName)
		rightParent, rightOrdinal := parentAndOrdinal(statuses[j].PodName)
		if leftParent != rightParent {
			return leftParent < rightParent
		}
		if leftOrdinal != rightOrdinal {
			return leftOrdinal < rightOrdinal
		}
		return statuses[i].PodName < statuses[j].PodName
	})
}

func parentAndOrdinal(name string) (string, int) {
	index := strings.LastIndex(name, "-")
	if index < 0 {
		return name, -1
	}
	ordinal, err := strconv.Atoi(name[index+1:])
	if err != nil {
		return name, -1
	}
	return name[:index], ordinal
}
