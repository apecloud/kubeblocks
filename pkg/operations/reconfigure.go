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

package operations

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	parameterscore "github.com/apecloud/kubeblocks/pkg/parameters/core"
)

type reconfigureAction struct {
}

type reconfigureTarget struct {
	requestName string
	component   string
	reconfigure opsv1alpha1.Reconfigure
	parameter   *parametersv1alpha1.ComponentParameter
}

type parameterAssignmentState struct {
	present bool
	value   *string
}

const invalidParameterActionResultCode appsv1.ActionResultCode = "InvalidParameter"

const (
	reconfigureRolledBackMessage      = "ActionRejectedRolledBack"
	reconfigureManualMessage          = "ManualCleanupRequired"
	defaultReconfigureRollbackTimeout = 10 * time.Minute
)

func init() {
	reAction := reconfigureAction{}
	opsManager := GetOpsManager()
	reconfigureBehaviour := OpsBehaviour{
		// REVIEW: can do opsrequest if not running?
		FromClusterPhases: appsv1.GetReconfiguringRunningPhases(),
		// TODO: add cluster reconcile Reconfiguring phase.
		ToClusterPhase: appsv1.UpdatingClusterPhase,
		QueueByCluster: true,
		OpsHandler:     &reAction,
	}
	opsManager.RegisterOps(opsv1alpha1.ReconfiguringType, reconfigureBehaviour)
}

var noRequeueAfter time.Duration = 0

// ActionStartedCondition the started condition when handle the reconfiguring request.
func (r *reconfigureAction) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return opsv1alpha1.NewReconfigureCondition(opsRes.OpsRequest), nil
}

func (r *reconfigureAction) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	targets, err := r.listReconfigureTargets(reqCtx.Ctx, cli, opsRes)
	if err != nil {
		return err
	}
	if opsRes.OpsRequest.Status.LastConfiguration.Components == nil {
		opsRes.OpsRequest.Status.LastConfiguration.Components = map[string]opsv1alpha1.LastComponentConfiguration{}
	}
	for _, target := range targets {
		last := opsRes.OpsRequest.Status.LastConfiguration.Components[target.component]
		last.Parameters = snapshotParameterAssignments(target.parameter, target.reconfigure.Parameters)
		opsRes.OpsRequest.Status.LastConfiguration.Components[target.component] = last
	}
	return nil
}

func (r *reconfigureAction) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	opsDeepCopy := resource.OpsRequest.DeepCopy()
	phase, msg, err := r.aggregatePhase(reqCtx, cli, resource)
	if err != nil {
		return "", noRequeueAfter, err
	}
	if phase == opsv1alpha1.OpsRunningPhase {
		if resource.OpsRequest.Status.ReconfigureRollback != nil && reconfigureRollbackTimedOut(resource.OpsRequest) {
			setReconfigureManualCleanup(resource.OpsRequest.Status.ReconfigureRollback,
				"automatic rollback exceeded the OpsRequest timeout")
			phase = opsv1alpha1.OpsFailedPhase
			msg = reconfigureManualMessage
		} else {
			resultPhase, _, err := r.syncReconfigureForOps(reqCtx, cli, resource, opsDeepCopy, opsv1alpha1.OpsRunningPhase)
			return resultPhase, reconfigureRollbackRequeueAfter(resource.OpsRequest), err
		}
	}
	if phase == opsv1alpha1.OpsSucceedPhase {
		return r.syncReconfigureForOps(reqCtx, cli, resource, opsDeepCopy, opsv1alpha1.OpsSucceedPhase)
	}
	if resource.OpsRequest.Status.ReconfigureRollback != nil {
		// Persist the compensation outcome before OpsManager commits the terminal
		// Failed phase. This also prevents the global timeout handler from
		// replacing a bounded rollback result with an unrelated Aborted phase.
		if err := PatchOpsStatusWithOpsDeepCopy(reqCtx.Ctx, cli, resource, opsDeepCopy, opsv1alpha1.OpsRunningPhase); err != nil {
			return "", noRequeueAfter, err
		}
	}
	return opsv1alpha1.OpsFailedPhase, 0, intctrlutil.NewFatalError(fmt.Sprintf("reconfigure failed: %s", msg))
}

func (r *reconfigureAction) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource) (err error) {
	if len(resource.OpsRequest.Spec.Reconfigures) == 0 {
		return intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal, `invalid reconfigure request: %s`, resource.OpsRequest.GetName())
	}
	targets, err := r.listReconfigureTargets(reqCtx.Ctx, cli, resource)
	if err != nil {
		return err
	}
	if err := preflightReconfigureTargets(resource.OpsRequest, targets); err != nil {
		return err
	}
	for _, target := range targets {
		if err := r.applyReconfigureToParameters(reqCtx, cli, resource, target); err != nil {
			return err
		}
	}
	return nil
}

func (r *reconfigureAction) syncReconfigureForOps(reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource, opsDeepCopy *opsv1alpha1.OpsRequest, phase opsv1alpha1.OpsPhase) (opsv1alpha1.OpsPhase, time.Duration, error) {
	if err := PatchOpsStatusWithOpsDeepCopy(reqCtx.Ctx, cli, resource, opsDeepCopy, phase); err != nil {
		return "", noRequeueAfter, err
	}
	return phase, noRequeueAfter, nil
}

func (r *reconfigureAction) aggregatePhase(reqCtx intctrlutil.RequestCtx, cli client.Client, resource *OpsResource) (opsv1alpha1.OpsPhase, string, error) {
	targets, err := r.listReconfigureTargets(reqCtx.Ctx, cli, resource)
	if err != nil {
		return "", "", err
	}
	if resource.OpsRequest.Status.ReconfigureRollback != nil {
		return r.reconcileRollback(reqCtx, cli, resource, targets)
	}

	allFinished := true
	failedTargets := make([]reconfigureTarget, 0)
	failedParameters := make([]*parametersv1alpha1.ComponentParameter, 0)
	for _, target := range targets {
		if target.parameter.Generation != target.parameter.Status.ObservedGeneration {
			allFinished = false
			continue
		}
		switch target.parameter.Status.Phase {
		case parametersv1alpha1.CMergeFailedPhase, parametersv1alpha1.CFailedAndPausePhase:
			failedTargets = append(failedTargets, target)
			failedParameters = append(failedParameters, target.parameter)
			allFinished = false
		case parametersv1alpha1.CFinishedPhase:
		default:
			allFinished = false
		}
	}
	if len(failedTargets) == 0 {
		if allFinished {
			return opsv1alpha1.OpsSucceedPhase, "", nil
		}
		return opsv1alpha1.OpsRunningPhase, "", nil
	}
	if !rollbackSnapshotsAvailable(resource.OpsRequest, failedTargets) {
		return opsv1alpha1.OpsFailedPhase, failedTargets[0].parameter.Status.Message, nil
	}
	if !rollbackOwnershipCurrent(resource.OpsRequest, failedTargets) {
		resource.OpsRequest.Status.ReconfigureRollback = &opsv1alpha1.ReconfigureRollbackStatus{
			Phase:   opsv1alpha1.ReconfigureManualCleanupRequired,
			Message: reconfigureManualMessage,
		}
		return opsv1alpha1.OpsFailedPhase, reconfigureManualMessage, nil
	}
	code, retryable, deterministic := classifyDeterministicReconfigureFailure(failedParameters)
	if !deterministic {
		resource.OpsRequest.Status.ReconfigureRollback = &opsv1alpha1.ReconfigureRollbackStatus{
			Phase:   opsv1alpha1.ReconfigureManualCleanupRequired,
			Message: reconfigureManualMessage,
		}
		return opsv1alpha1.OpsFailedPhase, reconfigureManualMessage, nil
	}
	now := metav1.Now()
	resource.OpsRequest.Status.ReconfigureRollback = &opsv1alpha1.ReconfigureRollbackStatus{
		Phase:                opsv1alpha1.ReconfigureRollbackPending,
		StartTime:            &now,
		Code:                 code,
		Retryable:            retryable,
		ComponentGenerations: rollbackTargetGenerations(failedTargets),
		RestartRequired:      rollbackRestartRequired(failedTargets),
		Message:              "normalized reconfigure failure accepted for rollback",
	}
	if reconfigureRollbackTimedOut(resource.OpsRequest) {
		setReconfigureManualCleanup(resource.OpsRequest.Status.ReconfigureRollback,
			"OpsRequest timeout elapsed before automatic rollback could start")
		return opsv1alpha1.OpsFailedPhase, reconfigureManualMessage, nil
	}
	return opsv1alpha1.OpsRunningPhase, "", nil
}

func (r *reconfigureAction) listReconfigureTargets(ctx context.Context, cli client.Client, resource *OpsResource) ([]reconfigureTarget, error) {
	targets := make([]reconfigureTarget, 0)
	seen := map[string]struct{}{}
	for _, reconfigure := range resource.OpsRequest.Spec.Reconfigures {
		if len(reconfigure.Parameters) == 0 {
			return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal,
				"invalid reconfigure request for component %s: no parameters", reconfigure.ComponentName)
		}
		if err := validateReconfigureParameters(reconfigure.Parameters); err != nil {
			return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal,
				"invalid reconfigure request for component %s: %v", reconfigure.ComponentName, err)
		}
		compNames, err := r.resolveReconfigureComponents(ctx, cli, resource.Cluster, reconfigure.ComponentName)
		if err != nil {
			return nil, err
		}
		for _, compName := range compNames {
			if _, ok := seen[compName]; ok {
				return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal,
					"component %s is targeted by more than one reconfigure entry", compName)
			}
			seen[compName] = struct{}{}
			compParam, err := r.getRunningComponentParameter(ctx, cli,
				resource.Cluster.Namespace, resource.Cluster.Name, compName)
			if err != nil {
				return nil, err
			}
			if err := validateComponentParameterInputs(compParam); err != nil {
				return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal,
					"invalid ComponentParameter inputs for component %s: %v", compName, err)
			}
			targets = append(targets, reconfigureTarget{
				requestName: reconfigure.ComponentName,
				component:   compName,
				reconfigure: reconfigure,
				parameter:   compParam,
			})
		}
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].component < targets[j].component
	})
	return targets, nil
}

func validateReconfigureParameters(parameters []opsv1alpha1.ParameterPair) error {
	seen := make(map[string]struct{}, len(parameters))
	for _, parameter := range parameters {
		if parameter.Key == "" {
			return fmt.Errorf("parameter key must not be empty")
		}
		if _, ok := seen[parameter.Key]; ok {
			return fmt.Errorf("parameter key %q is duplicated", parameter.Key)
		}
		seen[parameter.Key] = struct{}{}
	}
	return nil
}

func validateComponentParameterInputs(compParam *parametersv1alpha1.ComponentParameter) error {
	if compParam.Spec.Desired == nil {
		return nil
	}
	for key := range compParam.Spec.Desired.Assignments {
		if key == "" {
			return fmt.Errorf("assignment key must not be empty")
		}
	}
	for i, update := range compParam.Spec.Desired.Updates {
		if update.Key == "" {
			return fmt.Errorf("update %d key must not be empty", i)
		}
		switch update.Type {
		case parametersv1alpha1.ParameterUpdateSet:
			if update.Value == nil {
				return fmt.Errorf("update %d for key %q uses Set without a value", i, update.Key)
			}
		case parametersv1alpha1.ParameterUpdateRemove:
		default:
			return fmt.Errorf("update %d for key %q has unsupported type %q", i, update.Key, update.Type)
		}
	}
	return nil
}

func snapshotParameterAssignments(compParam *parametersv1alpha1.ComponentParameter,
	parameters []opsv1alpha1.ParameterPair) []opsv1alpha1.LastParameterAssignment {
	keys := requestedParameterKeys(parameters)
	assignments := componentParameterAssignments(compParam)
	result := make([]opsv1alpha1.LastParameterAssignment, 0, len(keys))
	for _, key := range keys {
		value, present := assignments[key]
		result = append(result, opsv1alpha1.LastParameterAssignment{
			Key:     key,
			Present: present,
			Value:   copyStringPointer(value),
		})
	}
	return result
}

func requestedParameterKeys(parameters []opsv1alpha1.ParameterPair) []string {
	set := make(map[string]struct{}, len(parameters))
	for _, parameter := range parameters {
		set[parameter.Key] = struct{}{}
	}
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func requestedParameterAssignments(parameters []opsv1alpha1.ParameterPair) map[string]*string {
	result := make(map[string]*string, len(parameters))
	for _, parameter := range parameters {
		result[parameter.Key] = copyStringPointer(parameter.Value)
	}
	return result
}

func copyStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	return ptr.To(*value)
}

func componentParameterAssignments(compParam *parametersv1alpha1.ComponentParameter) map[string]*string {
	if compParam.Spec.Desired == nil {
		return nil
	}
	inputs := compParam.Spec.Desired
	if len(inputs.Assignments) == 0 && len(inputs.Updates) == 0 {
		return nil
	}
	result := make(map[string]*string, len(inputs.Assignments)+len(inputs.Updates))
	for key, value := range inputs.Assignments {
		result[key] = copyStringPointer(value)
	}
	for _, update := range inputs.Updates {
		switch update.Type {
		case parametersv1alpha1.ParameterUpdateSet:
			result[update.Key] = copyStringPointer(update.Value)
		case parametersv1alpha1.ParameterUpdateRemove:
			result[update.Key] = nil
		}
	}
	return result
}

func rollbackSnapshotsAvailable(opsRequest *opsv1alpha1.OpsRequest, targets []reconfigureTarget) bool {
	for _, target := range targets {
		last, ok := opsRequest.Status.LastConfiguration.Components[target.component]
		if !ok || !snapshotCoversParameters(last.Parameters, target.reconfigure.Parameters) {
			return false
		}
	}
	return true
}

func rollbackOwnershipCurrent(opsRequest *opsv1alpha1.OpsRequest, targets []reconfigureTarget) bool {
	for _, target := range targets {
		if target.parameter.Annotations[constant.OpsRequestUIDAnnotationKey] != string(opsRequest.UID) {
			return false
		}
	}
	return true
}

func preflightReconfigureTargets(opsRequest *opsv1alpha1.OpsRequest, targets []reconfigureTarget) error {
	hasSnapshot := false
	for _, target := range targets {
		if last, ok := opsRequest.Status.LastConfiguration.Components[target.component]; ok && len(last.Parameters) > 0 {
			hasSnapshot = true
			break
		}
	}
	if !hasSnapshot {
		// Reconfigure OpsRequests created before parameter snapshots were introduced
		// retain their legacy apply behavior. They cannot use automatic rollback.
		return nil
	}

	for _, target := range targets {
		last, ok := opsRequest.Status.LastConfiguration.Components[target.component]
		if !ok || !snapshotCoversParameters(last.Parameters, target.reconfigure.Parameters) {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal,
				"parameter snapshot for component %s is incomplete", target.component)
		}
		current := componentParameterAssignments(target.parameter)
		if assignmentsMatchSnapshot(current, last.Parameters) {
			continue
		}
		requested := requestedParameterAssignments(target.reconfigure.Parameters)
		if assignmentsMatchRequested(current, requested) &&
			target.parameter.Annotations[constant.OpsRequestUIDAnnotationKey] == string(opsRequest.UID) {
			continue
		}
		return intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal,
			"current parameter assignments for component %s no longer match its snapshot or this OpsRequest's H1",
			target.component)
	}
	return nil
}

func snapshotCoversParameters(snapshot []opsv1alpha1.LastParameterAssignment,
	parameters []opsv1alpha1.ParameterPair) bool {
	if len(snapshot) != len(requestedParameterKeys(parameters)) {
		return false
	}
	keys := make(map[string]struct{}, len(snapshot))
	for _, item := range snapshot {
		if _, ok := keys[item.Key]; ok {
			return false
		}
		keys[item.Key] = struct{}{}
	}
	for _, key := range requestedParameterKeys(parameters) {
		if _, ok := keys[key]; !ok {
			return false
		}
	}
	return true
}

func classifyDeterministicReconfigureFailure(failed []*parametersv1alpha1.ComponentParameter) (appsv1.ActionResultCode, *bool, bool) {
	for _, compParam := range failed {
		expectedRevision := strconv.FormatInt(compParam.Generation, 10)
		sawFailedItem := false
		for _, item := range compParam.Status.ConfigurationItemStatus {
			if !isTerminalParameterFailure(item.Phase) {
				continue
			}
			sawFailedItem = true
			if item.ReconcileDetail == nil ||
				item.UpdateRevision != expectedRevision ||
				item.ReconcileDetail.CurrentRevision != expectedRevision ||
				item.ReconcileDetail.Code != invalidParameterActionResultCode ||
				item.ReconcileDetail.Retryable == nil || *item.ReconcileDetail.Retryable {
				return "", nil, false
			}
		}
		if !sawFailedItem {
			return "", nil, false
		}
	}
	retryable := false
	return invalidParameterActionResultCode, &retryable, true
}

func isTerminalParameterFailure(phase parametersv1alpha1.ParameterPhase) bool {
	return phase == parametersv1alpha1.CMergeFailedPhase || phase == parametersv1alpha1.CFailedAndPausePhase
}

func rollbackRestartRequired(targets []reconfigureTarget) bool {
	// A failed exec result cannot prove that the script made no runtime change
	// before exiting, and another Pod may still be completing the old H1 action.
	// Conservatively restart every affected target after restoring H0.
	return len(targets) > 0
}

func rollbackTargetGenerations(targets []reconfigureTarget) map[string]int64 {
	result := make(map[string]int64, len(targets))
	for _, target := range targets {
		result[target.component] = target.parameter.Generation
	}
	return result
}

func selectRollbackTargets(status *opsv1alpha1.ReconfigureRollbackStatus,
	targets []reconfigureTarget) ([]reconfigureTarget, string) {
	if len(status.ComponentGenerations) == 0 {
		return nil, "automatic rollback target set is missing"
	}
	selected := make([]reconfigureTarget, 0, len(status.ComponentGenerations))
	for _, target := range targets {
		if _, ok := status.ComponentGenerations[target.component]; ok {
			selected = append(selected, target)
		}
	}
	if len(selected) != len(status.ComponentGenerations) {
		known := make(map[string]struct{}, len(selected))
		for _, target := range selected {
			known[target.component] = struct{}{}
		}
		missing := make([]string, 0)
		for component := range status.ComponentGenerations {
			if _, ok := known[component]; !ok {
				missing = append(missing, component)
			}
		}
		sort.Strings(missing)
		return nil, fmt.Sprintf("rollback target components no longer exist: %v", missing)
	}
	return selected, ""
}

func (r *reconfigureAction) reconcileRollback(reqCtx intctrlutil.RequestCtx, cli client.Client,
	resource *OpsResource, targets []reconfigureTarget) (opsv1alpha1.OpsPhase, string, error) {
	status := resource.OpsRequest.Status.ReconfigureRollback
	if status.Phase == opsv1alpha1.ReconfigureRolledBack {
		return opsv1alpha1.OpsFailedPhase, reconfigureRolledBackMessage, nil
	}
	if status.Phase == opsv1alpha1.ReconfigureManualCleanupRequired {
		return opsv1alpha1.OpsFailedPhase, reconfigureManualMessage, nil
	}
	if status.Phase != opsv1alpha1.ReconfigureRolledBack && status.Phase != opsv1alpha1.ReconfigureManualCleanupRequired &&
		(status.StartTime == nil || status.StartTime.IsZero()) {
		setReconfigureManualCleanup(status, "automatic rollback start time is missing")
		return opsv1alpha1.OpsFailedPhase, reconfigureManualMessage, nil
	}
	if status.Phase != opsv1alpha1.ReconfigureRolledBack && status.Phase != opsv1alpha1.ReconfigureManualCleanupRequired &&
		reconfigureRollbackTimedOut(resource.OpsRequest) {
		setReconfigureManualCleanup(status, "automatic rollback exceeded the OpsRequest timeout")
		return opsv1alpha1.OpsFailedPhase, reconfigureManualMessage, nil
	}
	rollbackTargets, manualReason := selectRollbackTargets(status, targets)
	if manualReason != "" {
		setReconfigureManualCleanup(status, manualReason)
		return opsv1alpha1.OpsFailedPhase, reconfigureManualMessage, nil
	}
	switch status.Phase {
	case opsv1alpha1.ReconfigureRollbackPending:
		generations, manualReason, err := r.restoreParameterSnapshots(reqCtx, cli, resource, rollbackTargets)
		if err != nil {
			return "", "", err
		}
		if manualReason != "" {
			setReconfigureManualCleanup(status, manualReason)
			return opsv1alpha1.OpsFailedPhase, reconfigureManualMessage, nil
		}
		status.ComponentGenerations = generations
		status.Phase = opsv1alpha1.ReconfigureRollingBack
		status.Message = "waiting for restored parameter assignments to converge"
		return opsv1alpha1.OpsRunningPhase, "", nil

	case opsv1alpha1.ReconfigureRollingBack:
		ready, cleared, err := prepareRollbackFailureGates(reqCtx.Ctx, cli, resource.Cluster, rollbackTargets, status.ComponentGenerations)
		if err != nil {
			return "", "", err
		}
		if !ready || cleared {
			return opsv1alpha1.OpsRunningPhase, "", nil
		}
		completed, manualReason := rollbackParametersConverged(resource.OpsRequest, rollbackTargets)
		if manualReason != "" {
			setReconfigureManualCleanup(status, manualReason)
			return opsv1alpha1.OpsFailedPhase, reconfigureManualMessage, nil
		}
		if !completed {
			return opsv1alpha1.OpsRunningPhase, "", nil
		}
		if status.RestartRequired {
			if manualReason := validateRollbackRestartScope(rollbackTargets, targets); manualReason != "" {
				setReconfigureManualCleanup(status, manualReason)
				return opsv1alpha1.OpsFailedPhase, reconfigureManualMessage, nil
			}
			now := metav1.Now()
			status.RestartAt = &now
			status.Phase = opsv1alpha1.ReconfigureRestartPending
			status.Message = "parameter assignments restored; controlled restart pending"
		} else {
			status.Phase = opsv1alpha1.ReconfigureRolledBack
			status.Message = reconfigureRolledBackMessage
			return opsv1alpha1.OpsFailedPhase, reconfigureRolledBackMessage, nil
		}
		return opsv1alpha1.OpsRunningPhase, "", nil

	case opsv1alpha1.ReconfigureRestartPending:
		generation, manualReason, err := r.restartRollbackTargets(reqCtx, cli, resource, rollbackTargets)
		if err != nil {
			return "", "", err
		}
		if manualReason != "" {
			setReconfigureManualCleanup(status, manualReason)
			return opsv1alpha1.OpsFailedPhase, reconfigureManualMessage, nil
		}
		status.ClusterGeneration = generation
		status.Phase = opsv1alpha1.ReconfigureRestarting
		status.Message = "waiting for controlled restart to complete"
		return opsv1alpha1.OpsRunningPhase, "", nil

	case opsv1alpha1.ReconfigureRestarting:
		completed, manualReason, err := restartRollbackConverged(reqCtx.Ctx, cli, resource, rollbackTargets)
		if err != nil {
			return "", "", err
		}
		if manualReason != "" {
			setReconfigureManualCleanup(status, manualReason)
			return opsv1alpha1.OpsFailedPhase, reconfigureManualMessage, nil
		}
		if !completed {
			return opsv1alpha1.OpsRunningPhase, "", nil
		}
		status.Phase = opsv1alpha1.ReconfigureRolledBack
		status.Message = reconfigureRolledBackMessage
		return opsv1alpha1.OpsFailedPhase, reconfigureRolledBackMessage, nil

	case opsv1alpha1.ReconfigureRolledBack:
		return opsv1alpha1.OpsFailedPhase, reconfigureRolledBackMessage, nil
	case opsv1alpha1.ReconfigureManualCleanupRequired:
		return opsv1alpha1.OpsFailedPhase, reconfigureManualMessage, nil
	default:
		setReconfigureManualCleanup(status, "unknown persisted rollback phase")
		return opsv1alpha1.OpsFailedPhase, reconfigureManualMessage, nil
	}
}

func validateRollbackRestartScope(rollbackTargets, allTargets []reconfigureTarget) string {
	totalByRequest := make(map[string]int, len(allTargets))
	for _, target := range allTargets {
		totalByRequest[target.requestName]++
	}
	selectedByRequest := make(map[string]int, len(rollbackTargets))
	for _, target := range rollbackTargets {
		selectedByRequest[target.requestName]++
	}
	for _, target := range rollbackTargets {
		if target.requestName != target.component && selectedByRequest[target.requestName] != totalByRequest[target.requestName] {
			return fmt.Sprintf("automatic rollback cannot restart only failed component %s from sharding %s",
				target.component, target.requestName)
		}
	}
	return ""
}

func reconfigureRollbackTimedOut(opsRequest *opsv1alpha1.OpsRequest) bool {
	deadline, ok := reconfigureRollbackDeadline(opsRequest)
	if !ok {
		return false
	}
	return !time.Now().Before(deadline)
}

func reconfigureRollbackRequeueAfter(opsRequest *opsv1alpha1.OpsRequest) time.Duration {
	deadline, ok := reconfigureRollbackDeadline(opsRequest)
	if !ok {
		return noRequeueAfter
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return time.Millisecond
	}
	return remaining
}

func reconfigureRollbackDeadline(opsRequest *opsv1alpha1.OpsRequest) (time.Time, bool) {
	status := opsRequest.Status.ReconfigureRollback
	if status == nil {
		return time.Time{}, false
	}
	var deadline time.Time
	if status.StartTime != nil && !status.StartTime.IsZero() {
		deadline = status.StartTime.Add(defaultReconfigureRollbackTimeout)
	}
	if timeoutSeconds := opsRequest.Spec.TimeoutSeconds; timeoutSeconds != nil && *timeoutSeconds > 0 &&
		!opsRequest.Status.StartTimestamp.IsZero() {
		opsDeadline := opsRequest.Status.StartTimestamp.Add(time.Duration(*timeoutSeconds) * time.Second)
		if deadline.IsZero() || opsDeadline.Before(deadline) {
			deadline = opsDeadline
		}
	}
	return deadline, !deadline.IsZero()
}

func setReconfigureManualCleanup(status *opsv1alpha1.ReconfigureRollbackStatus, reason string) {
	status.Phase = opsv1alpha1.ReconfigureManualCleanupRequired
	status.Message = fmt.Sprintf("%s: %s", reconfigureManualMessage, reason)
}

func (r *reconfigureAction) restoreParameterSnapshots(reqCtx intctrlutil.RequestCtx, cli client.Client,
	resource *OpsResource, targets []reconfigureTarget) (map[string]int64, string, error) {
	for _, target := range targets {
		if target.parameter.Annotations[constant.OpsRequestUIDAnnotationKey] != string(resource.OpsRequest.UID) {
			return nil, fmt.Sprintf("ComponentParameter %s is no longer owned by this OpsRequest", target.parameter.Name), nil
		}
		last, ok := resource.OpsRequest.Status.LastConfiguration.Components[target.component]
		if !ok || !snapshotCoversParameters(last.Parameters, target.reconfigure.Parameters) {
			return nil, fmt.Sprintf("parameter snapshot for component %s is incomplete", target.component), nil
		}
		assignments := componentParameterAssignments(target.parameter)
		requested := requestedParameterAssignments(target.reconfigure.Parameters)
		if !assignmentsMatchRequested(assignments, requested) && !assignmentsMatchSnapshot(assignments, last.Parameters) {
			return nil, fmt.Sprintf("current parameter assignments for component %s no longer match H1 or its rollback snapshot", target.component), nil
		}
	}

	generations := make(map[string]int64, len(targets))
	for _, target := range targets {
		last := resource.OpsRequest.Status.LastConfiguration.Components[target.component]
		if assignmentsMatchSnapshot(componentParameterAssignments(target.parameter), last.Parameters) {
			generations[target.component] = target.parameter.Generation
			continue
		}
		patch := client.MergeFromWithOptions(target.parameter.DeepCopy(), client.MergeFromWithOptimisticLock{})
		restoreParameterAssignments(target.parameter, last.Parameters)
		if err := cli.Patch(reqCtx.Ctx, target.parameter, patch); err != nil {
			return nil, "", err
		}
		generations[target.component] = target.parameter.Generation
	}
	return generations, "", nil
}

func assignmentsMatchRequested(current, requested map[string]*string) bool {
	for key, expected := range requested {
		value, present := current[key]
		if !present || !ptr.Equal(value, expected) {
			return false
		}
	}
	return true
}

func assignmentsMatchSnapshot(current map[string]*string, snapshot []opsv1alpha1.LastParameterAssignment) bool {
	for _, item := range snapshot {
		value, present := current[item.Key]
		if present != item.Present {
			return false
		}
		if present && !ptr.Equal(value, item.Value) {
			return false
		}
	}
	return true
}

func restoreParameterAssignments(compParam *parametersv1alpha1.ComponentParameter,
	snapshot []opsv1alpha1.LastParameterAssignment) {
	states := make(map[string]parameterAssignmentState, len(snapshot))
	for _, item := range snapshot {
		states[item.Key] = parameterAssignmentState{present: item.Present, value: copyStringPointer(item.Value)}
	}
	replaceParameterAssignments(compParam, states)
}

func replaceParameterAssignments(compParam *parametersv1alpha1.ComponentParameter,
	states map[string]parameterAssignmentState) {
	if compParam.Spec.Desired == nil {
		compParam.Spec.Desired = &parametersv1alpha1.ParameterInputs{}
	}
	inputs := compParam.Spec.Desired
	keys := make(map[string]struct{}, len(states))
	for key := range states {
		keys[key] = struct{}{}
		delete(inputs.Assignments, key)
	}
	updates := make([]parametersv1alpha1.ParameterUpdate, 0, len(inputs.Updates)+len(states))
	for _, update := range inputs.Updates {
		if _, replace := keys[update.Key]; !replace {
			updates = append(updates, update)
		}
	}

	sortedKeys := make([]string, 0, len(states))
	for key := range states {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)
	for _, key := range sortedKeys {
		state := states[key]
		if !state.present {
			continue
		}
		if state.value == nil {
			updates = append(updates, parametersv1alpha1.ParameterUpdate{
				Type: parametersv1alpha1.ParameterUpdateRemove,
				Key:  key,
			})
			continue
		}
		if inputs.Assignments == nil {
			inputs.Assignments = map[string]*string{}
		}
		inputs.Assignments[key] = copyStringPointer(state.value)
	}
	inputs.Updates = updates
}

func prepareRollbackFailureGates(ctx context.Context, cli client.Client, cluster *appsv1.Cluster,
	targets []reconfigureTarget, generations map[string]int64) (bool, bool, error) {
	configMaps := make([]*corev1.ConfigMap, 0)
	seen := map[string]struct{}{}
	for _, target := range targets {
		generation, ok := generations[target.component]
		if !ok {
			return false, false, fmt.Errorf("rollback generation for component %s is missing", target.component)
		}
		expectedRevision := strconv.FormatInt(generation, 10)
		for _, item := range target.parameter.Spec.ConfigItemDetails {
			name := parameterscore.GetComponentCfgName(cluster.Name, target.component, item.Name)
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			configMap := &corev1.ConfigMap{}
			key := client.ObjectKey{Namespace: cluster.Namespace, Name: name}
			if err := cli.Get(ctx, key, configMap); err != nil {
				if apierrors.IsNotFound(err) {
					return false, false, nil
				}
				return false, false, err
			}
			if configMap.Annotations[constant.ConfigurationRevision] != expectedRevision {
				return false, false, nil
			}
			configMaps = append(configMaps, configMap)
		}
	}

	cleared := false
	for _, configMap := range configMaps {
		if _, ok := configMap.Annotations[constant.DisableUpgradeInsConfigurationAnnotationKey]; !ok {
			continue
		}
		patch := client.MergeFromWithOptions(configMap.DeepCopy(), client.MergeFromWithOptimisticLock{})
		delete(configMap.Annotations, constant.DisableUpgradeInsConfigurationAnnotationKey)
		if err := cli.Patch(ctx, configMap, patch); err != nil {
			return false, cleared, err
		}
		cleared = true
	}
	return true, cleared, nil
}

func rollbackParametersConverged(opsRequest *opsv1alpha1.OpsRequest,
	targets []reconfigureTarget) (bool, string) {
	status := opsRequest.Status.ReconfigureRollback
	for _, target := range targets {
		if target.parameter.Annotations[constant.OpsRequestUIDAnnotationKey] != string(opsRequest.UID) {
			return false, fmt.Sprintf("ComponentParameter %s ownership changed during rollback", target.parameter.Name)
		}
		generation, ok := status.ComponentGenerations[target.component]
		if !ok || target.parameter.Generation != generation {
			return false, fmt.Sprintf("ComponentParameter %s generation changed during rollback", target.parameter.Name)
		}
		last, ok := opsRequest.Status.LastConfiguration.Components[target.component]
		if !ok || !assignmentsMatchSnapshot(componentParameterAssignments(target.parameter), last.Parameters) {
			return false, fmt.Sprintf("ComponentParameter %s no longer matches its rollback snapshot", target.parameter.Name)
		}
		if target.parameter.Status.ObservedGeneration != generation {
			return false, ""
		}
		switch target.parameter.Status.Phase {
		case parametersv1alpha1.CFinishedPhase:
		case parametersv1alpha1.CMergeFailedPhase, parametersv1alpha1.CFailedAndPausePhase:
			return false, fmt.Sprintf("restoring parameters for component %s failed", target.component)
		default:
			return false, ""
		}
	}
	return true, ""
}

func (r *reconfigureAction) restartRollbackTargets(reqCtx intctrlutil.RequestCtx, cli client.Client,
	resource *OpsResource, targets []reconfigureTarget) (int64, string, error) {
	status := resource.OpsRequest.Status.ReconfigureRollback
	if status.RestartAt == nil {
		return 0, "controlled restart timestamp is missing", nil
	}
	cluster := &appsv1.Cluster{}
	key := client.ObjectKeyFromObject(resource.Cluster)
	if err := cli.Get(reqCtx.Ctx, key, cluster); err != nil {
		return 0, "", err
	}
	if resource.Cluster.UID != "" && cluster.UID != resource.Cluster.UID {
		return 0, "target Cluster identity changed before controlled restart", nil
	}
	patch := client.MergeFromWithOptions(cluster.DeepCopy(), client.MergeFromWithOptimisticLock{})
	restartAt := status.RestartAt.Time.UTC().Format(time.RFC3339)
	changed := false
	seen := map[string]struct{}{}
	for _, target := range targets {
		if _, ok := seen[target.requestName]; ok {
			continue
		}
		seen[target.requestName] = struct{}{}
		if compSpec := cluster.Spec.GetComponentByName(target.requestName); compSpec != nil {
			if compSpec.Annotations == nil {
				compSpec.Annotations = map[string]string{}
			}
			if compSpec.Annotations[constant.RestartAnnotationKey] != restartAt {
				compSpec.Annotations[constant.RestartAnnotationKey] = restartAt
				changed = true
			}
			continue
		}
		shardingSpec := cluster.Spec.GetShardingByName(target.requestName)
		if shardingSpec == nil {
			return 0, fmt.Sprintf("restart target %s no longer exists in Cluster spec", target.requestName), nil
		}
		if shardingSpec.Template.Annotations == nil {
			shardingSpec.Template.Annotations = map[string]string{}
		}
		if shardingSpec.Template.Annotations[constant.RestartAnnotationKey] != restartAt {
			shardingSpec.Template.Annotations[constant.RestartAnnotationKey] = restartAt
			changed = true
		}
	}
	if changed {
		if err := cli.Patch(reqCtx.Ctx, cluster, patch); err != nil {
			return 0, "", err
		}
	}
	return cluster.Generation, "", nil
}

func restartRollbackConverged(ctx context.Context, cli client.Client, resource *OpsResource,
	targets []reconfigureTarget) (bool, string, error) {
	status := resource.OpsRequest.Status.ReconfigureRollback
	cluster := &appsv1.Cluster{}
	if err := cli.Get(ctx, client.ObjectKeyFromObject(resource.Cluster), cluster); err != nil {
		return false, "", err
	}
	if resource.Cluster.UID != "" && cluster.UID != resource.Cluster.UID {
		return false, "target Cluster identity changed during controlled restart", nil
	}
	if cluster.Generation != status.ClusterGeneration {
		return false, "target Cluster generation changed during controlled restart", nil
	}
	if cluster.Status.ObservedGeneration != status.ClusterGeneration {
		return false, "", nil
	}
	for _, target := range targets {
		componentStatus, ok := cluster.Status.Components[target.component]
		if !ok {
			return false, "", nil
		}
		if componentStatus.Phase == appsv1.FailedComponentPhase {
			return false, fmt.Sprintf("component %s failed during controlled restart", target.component), nil
		}
		if componentStatus.Phase != appsv1.RunningComponentPhase || !componentStatus.UpToDate {
			return false, "", nil
		}
	}
	return true, "", nil
}

func (r *reconfigureAction) applyReconfigureToParameters(reqCtx intctrlutil.RequestCtx, cli client.Client,
	resource *OpsResource, target reconfigureTarget) error {
	compParam := target.parameter
	patch := client.MergeFromWithOptions(compParam.DeepCopy(), client.MergeFromWithOptimisticLock{})
	if compParam.Annotations == nil {
		compParam.Annotations = map[string]string{}
	}
	compParam.Annotations[constant.OpsRequestUIDAnnotationKey] = string(resource.OpsRequest.UID)
	if compParam.Spec.Desired == nil {
		compParam.Spec.Desired = &parametersv1alpha1.ParameterInputs{}
	}
	if len(target.reconfigure.Parameters) != 0 {
		states := make(map[string]parameterAssignmentState, len(target.reconfigure.Parameters))
		for _, param := range target.reconfigure.Parameters {
			states[param.Key] = parameterAssignmentState{present: true, value: copyStringPointer(param.Value)}
		}
		replaceParameterAssignments(compParam, states)
	}
	return cli.Patch(reqCtx.Ctx, compParam, patch)
}

func (r *reconfigureAction) resolveReconfigureComponents(ctx context.Context, reader client.Reader, cluster *appsv1.Cluster, compName string) ([]string, error) {
	if compSpec := cluster.Spec.GetComponentByName(compName); compSpec != nil {
		return []string{compSpec.Name}, nil
	}
	shardingComp := cluster.Spec.GetShardingByName(compName)
	if shardingComp == nil {
		return nil, intctrlutil.NewErrorf(intctrlutil.ErrorTypeFatal, "component not found: %s", compName)
	}
	comps, err := sharding.ListShardingComponents(ctx, reader, cluster, compName)
	if err != nil {
		return nil, err
	}
	compNames := make([]string, 0, len(comps))
	for _, comp := range comps {
		shortName, err := component.ShortName(cluster.Name, comp.Name)
		if err != nil {
			return nil, err
		}
		compNames = append(compNames, shortName)
	}
	return compNames, nil
}

func (r *reconfigureAction) getRunningComponentParameter(ctx context.Context, cli client.Client, namespace, clusterName, compName string) (*parametersv1alpha1.ComponentParameter, error) {
	compParam := &parametersv1alpha1.ComponentParameter{}
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      parameterscore.GenerateComponentConfigurationName(clusterName, compName),
	}
	if err := cli.Get(ctx, key, compParam); err != nil {
		return nil, err
	}
	return compParam, nil
}
