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
	"reflect"
	"strings"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// switchover constants
const (
	KBSwitchoverKey                        = "Switchover"
	switchoverDispatchClaimMessagePrefix   = "switchover dispatch claimed before lifecycle call: SwitchoverDispatch/"
	switchoverDispatchClaimMessageFmt      = switchoverDispatchClaimMessagePrefix + "%s/%s/%s/%s/%s"
	switchoverDispatchOutcomeMessagePrefix = "switchover dispatch outcome persisted: SwitchoverDispatch/"
	switchoverDispatchOutcomeMessageFmt    = switchoverDispatchOutcomeMessagePrefix + "%s/%s/%s/%s/%s; %s"
)

var errSwitchoverDispatchClaimLost = errors.New("switchover dispatch claim was lost")

type switchoverDispatchClaim struct {
	opsRequestUID string
	componentName string
	instanceName  string
	candidateName string
	token         string
}

func newSwitchoverDispatchClaim(opsRequest *opsv1alpha1.OpsRequest, compName string,
	switchover opsv1alpha1.Switchover) switchoverDispatchClaim {
	return switchoverDispatchClaim{
		opsRequestUID: string(opsRequest.UID),
		componentName: compName,
		instanceName:  switchover.InstanceName,
		candidateName: switchover.CandidateName,
		token:         string(uuid.NewUUID()),
	}
}

func parseSwitchoverDispatchClaim(message string) (switchoverDispatchClaim, bool) {
	if !strings.HasPrefix(message, switchoverDispatchClaimMessagePrefix) {
		return switchoverDispatchClaim{}, false
	}
	fields := strings.Split(strings.TrimPrefix(message, switchoverDispatchClaimMessagePrefix), "/")
	if len(fields) != 5 || fields[0] == "" || fields[1] == "" || fields[2] == "" || fields[4] == "" {
		return switchoverDispatchClaim{}, false
	}
	return switchoverDispatchClaim{
		opsRequestUID: fields[0],
		componentName: fields[1],
		instanceName:  fields[2],
		candidateName: fields[3],
		token:         fields[4],
	}, true
}

func (claim switchoverDispatchClaim) message() string {
	return fmt.Sprintf(switchoverDispatchClaimMessageFmt, claim.opsRequestUID, claim.componentName,
		claim.instanceName, claim.candidateName, claim.token)
}

func (claim switchoverDispatchClaim) outcomeMessage(message string) string {
	return fmt.Sprintf(switchoverDispatchOutcomeMessageFmt, claim.opsRequestUID, claim.componentName,
		claim.instanceName, claim.candidateName, claim.token, message)
}

func parseSwitchoverDispatchOutcomeMessage(message string) (switchoverDispatchClaim, string, bool) {
	if !strings.HasPrefix(message, switchoverDispatchOutcomeMessagePrefix) {
		return switchoverDispatchClaim{}, "", false
	}
	parts := strings.SplitN(strings.TrimPrefix(message, switchoverDispatchOutcomeMessagePrefix), "; ", 2)
	if len(parts) != 2 || parts[1] == "" {
		return switchoverDispatchClaim{}, "", false
	}
	fields := strings.Split(parts[0], "/")
	if len(fields) != 5 || fields[0] == "" || fields[1] == "" || fields[2] == "" || fields[4] == "" {
		return switchoverDispatchClaim{}, "", false
	}
	return switchoverDispatchClaim{
		opsRequestUID: fields[0],
		componentName: fields[1],
		instanceName:  fields[2],
		candidateName: fields[3],
		token:         fields[4],
	}, parts[1], true
}

func (claim switchoverDispatchClaim) matchesIdentity(opsRequest *opsv1alpha1.OpsRequest, compName string,
	switchover opsv1alpha1.Switchover) bool {
	return claim.opsRequestUID == string(opsRequest.UID) &&
		claim.componentName == compName &&
		claim.instanceName == switchover.InstanceName &&
		claim.candidateName == switchover.CandidateName
}

func (claim switchoverDispatchClaim) equal(other switchoverDispatchClaim) bool {
	return claim.opsRequestUID == other.opsRequestUID &&
		claim.componentName == other.componentName &&
		claim.instanceName == other.instanceName &&
		claim.candidateName == other.candidateName &&
		claim.token == other.token
}

type switchoverOpsHandler struct{}

var _ OpsHandler = switchoverOpsHandler{}

func init() {
	switchoverBehaviour := OpsBehaviour{
		FromClusterPhases: appsv1.GetClusterUpRunningPhases(),
		ToClusterPhase:    appsv1.UpdatingClusterPhase,
		QueueByCluster:    true,
		OpsHandler:        switchoverOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(opsv1alpha1.SwitchoverType, switchoverBehaviour)
}

// ActionStartedCondition the started condition when handle the switchover request.
func (r switchoverOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return opsv1alpha1.NewSwitchoveringCondition(opsRes.Cluster.Generation, ""), nil
}

func (r switchoverOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return switchoverPreCheck(reqCtx, cli, opsRes, opsRes.OpsRequest.Spec.SwitchoverList)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for switchover opsRequest.
func (r switchoverOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	var (
		opsRequestPhase = opsv1alpha1.OpsRunningPhase
	)

	expectCount, actualCount, failedCount, err := handleSwitchovers(reqCtx, cli, opsRes)
	if err != nil {
		return "", 0, err
	}

	if expectCount == actualCount {
		opsRequestPhase = opsv1alpha1.OpsSucceedPhase
		if failedCount > 0 {
			opsRequestPhase = opsv1alpha1.OpsFailedPhase
		}
	}

	return opsRequestPhase, 5 * time.Second, nil
}

// SaveLastConfiguration this operation only restart the pods of the component, no changes for Cluster.spec.
// empty implementation here.
func (r switchoverOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

// switchoverPreCheck checks whether the component need switchover.
func switchoverPreCheck(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource, switchoverList []opsv1alpha1.Switchover) error {
	opsRequest := opsRes.OpsRequest
	if opsRequest.Status.Components == nil {
		opsRequest.Status.Components = make(map[string]opsv1alpha1.OpsRequestComponentStatus)
	}

	for _, switchover := range switchoverList {
		compName, err := getSwitchoverClusterComponentName(reqCtx.Ctx, cli, opsRes.Cluster, switchover)
		if err != nil {
			return err
		}
		synthesizedComp, err := buildSynthesizedComp(reqCtx.Ctx, cli, opsRes, switchover)
		if err != nil {
			return err
		}
		if synthesizedComp.LifecycleActions.ComponentLifecycleActions == nil || synthesizedComp.LifecycleActions.Switchover == nil {
			return intctrlutil.NewFatalError(fmt.Sprintf(`the component "%s" does not define switchover lifecycle action`, compName))
		}
		if len(synthesizedComp.Roles) == 0 {
			return intctrlutil.NewFatalError(fmt.Sprintf(`the component "%s" does not have any role`, compName))
		}

		runtime, err := opsRes.GetRuntime(compName)
		if err != nil {
			return err
		}
		instance, err := getSwitchoverPodBackedInstance(runtime, synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name, switchover.InstanceName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return intctrlutil.NewFatalError(fmt.Sprintf(`instance "%s" not found`, switchover.InstanceName))
			}
			return err
		}
		roleName := instance.GetRole()
		if roleName == "" {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeNeedWaiting, "waiting for instance %s role label", switchover.InstanceName)
		}

		if switchover.CandidateName != "" {
			_, err := getSwitchoverPodBackedInstance(runtime, synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name, switchover.CandidateName)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return intctrlutil.NewFatalError(fmt.Sprintf(`candidate instance "%s" not found`, switchover.CandidateName))
				}
				return err
			}
		}

		opsRequest.Status.Components[compName] = opsv1alpha1.OpsRequestComponentStatus{
			Phase: appsv1.UpdatingComponentPhase,
			ProgressDetails: []opsv1alpha1.ProgressStatusDetail{
				{
					Group:     roleName,
					ObjectKey: getProgressObjectKey(KBSwitchoverKey, compName),
					Status:    opsv1alpha1.PendingProgressStatus,
					Message:   fmt.Sprintf("start to switchover for component %s", compName),
				},
			},
		}
	}
	return nil
}

// handleSwitchovers handles the component progressDetails during switchover.
// Returns:
// - expectCount: the expected count of switchover operations
// - completedCount: the number of completed switchover operations
// - failedCount: the number of failed switchover operations
// - error: any error that occurred during the handling
func handleSwitchovers(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (int32, int32, int32, error) {
	expectCount := int32(len(opsRes.OpsRequest.Spec.SwitchoverList))
	var completedCount, failedCount int32

	opsRequest := opsRes.OpsRequest
	for _, switchover := range opsRequest.Spec.SwitchoverList {
		if err := handleSwitchover(reqCtx, cli, opsRes, &switchover, opsRequest, &completedCount, &failedCount); err != nil {
			return expectCount, completedCount, failedCount, err
		}
	}

	progress := fmt.Sprintf("%d/%d", completedCount, expectCount)
	if opsRequest.Status.Progress != progress {
		patch := client.MergeFromWithOptions(opsRequest.DeepCopy(), client.MergeFromWithOptimisticLock{})
		opsRequest.Status.Progress = progress
		if err := cli.Status().Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
			return expectCount, completedCount, failedCount, err
		}
	}

	return expectCount, completedCount, failedCount, nil
}

func handleSwitchover(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource, switchover *opsv1alpha1.Switchover, opsRequest *opsv1alpha1.OpsRequest, completedCount, failedCount *int32) error {
	switchoverCondition := meta.FindStatusCondition(opsRequest.Status.Conditions, opsv1alpha1.ConditionTypeSwitchover)
	if switchoverCondition == nil {
		return errors.New("switchover condition is nil")
	}

	synthesizedComp, err := buildSynthesizedComp(reqCtx.Ctx, cli, opsRes, *switchover)
	if err != nil {
		return err
	}

	compDef, err := component.GetCompDefByName(reqCtx.Ctx, cli, synthesizedComp.CompDefName)
	if err != nil {
		return err
	}
	synthesizedComp.TemplateVars, _, err = component.ResolveTemplateNEnvVars(reqCtx.Ctx, cli, synthesizedComp, compDef.Spec.Vars)
	if err != nil {
		return err
	}

	compName, err := getSwitchoverClusterComponentName(reqCtx.Ctx, cli, opsRes.Cluster, *switchover)
	if err != nil {
		return err
	}
	objectKey := getProgressObjectKey(KBSwitchoverKey, compName)
	progressDetail := findStatusProgressDetail(opsRequest.Status.Components[compName].ProgressDetails, objectKey)
	if progressDetail == nil {
		return fmt.Errorf("progress detail not found for component %s", compName)
	}
	runtime, err := opsRes.GetRuntime(compName)
	if err != nil {
		return err
	}

	switch progressDetail.Status {
	case opsv1alpha1.PendingProgressStatus:
		if opsRes.APIReader == nil {
			return errors.New("APIReader is required to confirm a switchover dispatch claim")
		}
		if opsRequest.UID == "" {
			return errors.New("OpsRequest UID is required to create a switchover dispatch claim")
		}
		claim := newSwitchoverDispatchClaim(opsRequest, compName, *switchover)
		claimMessage := claim.message()
		patchErr := patchSwitchoverProgressStatus(reqCtx.Ctx, cli, opsRequest, compName, objectKey,
			func(detail *opsv1alpha1.ProgressStatusDetail) {
				detail.Status = opsv1alpha1.ProcessingProgressStatus
				detail.Message = claimMessage
				detail.StartTime = metav1.Now()
			})
		ownsClaim, confirmErr := confirmSwitchoverDispatchClaim(reqCtx.Ctx, opsRes, opsRequest, compName, objectKey, claim)
		if confirmErr != nil {
			if patchErr != nil {
				return errors.Wrapf(patchErr, "failed to confirm switchover dispatch claim: %v", confirmErr)
			}
			return confirmErr
		}
		if !ownsClaim {
			if patchErr != nil {
				return patchErr
			}
			return fmt.Errorf("switchover dispatch claim belongs to another writer for component %s", compName)
		}

		actionErr := runtime.Switchover(reqCtx.Ctx, synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name, switchover.InstanceName, switchover.CandidateName)
		outcomeStatus := opsv1alpha1.ProcessingProgressStatus
		outcomeMessage := "doing switchover"
		if actionErr != nil {
			outcomeStatus = opsv1alpha1.FailedProgressStatus
			outcomeMessage = fmt.Sprintf("component %s %s", compName, actionErr.Error())
		} else if !progressDetail.StartTime.IsZero() && time.Now().After(progressDetail.StartTime.Add(5*time.Minute)) {
			// StartTime is committed before the lifecycle call. A successful response that arrives after
			// this deadline remains a fail-closed timeout instead of being treated as a fresh success.
			outcomeStatus = opsv1alpha1.FailedProgressStatus
			outcomeMessage = "switchover timeout after 5 minutes"
		}
		progressDetail, err = persistKnownSwitchoverDispatchOutcome(reqCtx, cli, opsRes.APIReader, opsRequest,
			compName, objectKey, claim, outcomeStatus, outcomeMessage)
		if err != nil {
			return err
		}
		if isCompletedProgressStatus(progressDetail.Status) {
			*completedCount++
			if progressDetail.Status == opsv1alpha1.FailedProgressStatus {
				*failedCount++
			}
		}
		return nil
	case opsv1alpha1.ProcessingProgressStatus:
		oldOpsRequestStatus := opsRequest.Status.DeepCopy()
		patch := client.MergeFromWithOptions(opsRequest.DeepCopy(), client.MergeFromWithOptimisticLock{})
		claim, hasClaim := parseSwitchoverDispatchClaim(progressDetail.Message)
		outcomeClaim, _, hasOutcome := parseSwitchoverDispatchOutcomeMessage(progressDetail.Message)
		claimProtocolMismatch := strings.HasPrefix(progressDetail.Message, switchoverDispatchClaimMessagePrefix) &&
			(!hasClaim || !claim.matchesIdentity(opsRequest, compName, *switchover))
		outcomeProtocolMismatch := strings.HasPrefix(progressDetail.Message, switchoverDispatchOutcomeMessagePrefix) &&
			(!hasOutcome || !outcomeClaim.matchesIdentity(opsRequest, compName, *switchover))
		protocolIdentityMismatch := claimProtocolMismatch || outcomeProtocolMismatch
		switch {
		case protocolIdentityMismatch:
			progressDetail.Message = fmt.Sprintf("component %s switchover dispatch protocol identity changed or is malformed; outcome is unknown and lifecycle action will not be retried", compName)
			progressDetail.Status = opsv1alpha1.FailedProgressStatus
		case hasClaim && switchover.CandidateName == "":
			progressDetail.Message = fmt.Sprintf("component %s switchover dispatch outcome is unknown; lifecycle action will not be retried", compName)
			progressDetail.Status = opsv1alpha1.FailedProgressStatus
		default:
			targetRole := progressDetail.Group
			if switchover.CandidateName != "" {
				candidateInstance, err := getSwitchoverPodBackedInstance(runtime, synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name, switchover.CandidateName)
				switch {
				case err != nil && !apierrors.IsNotFound(err):
					return err
				case err != nil:
					progressDetail.Message = fmt.Sprintf(`component %s candidate instance "%s" not found`, compName, switchover.CandidateName)
					progressDetail.Status = opsv1alpha1.FailedProgressStatus
				case targetRole == candidateInstance.GetRole():
					progressDetail.Message = "do switchover succeed"
					progressDetail.Status = opsv1alpha1.SucceedProgressStatus
				default:
					progressDetail.Message = fmt.Sprintf("component %s is waiting for candidate pod %s role change, current role %q, expected role %q",
						compName, switchover.CandidateName, candidateInstance.GetRole(), targetRole)
				}
			} else {
				progressDetail.Message = "do switchover succeed"
				progressDetail.Status = opsv1alpha1.SucceedProgressStatus
			}
		}
		handleProgressDetail(reqCtx, opsRequest, progressDetail, compName, completedCount, failedCount)
		if !reflect.DeepEqual(*oldOpsRequestStatus, opsRequest.Status) {
			if err := cli.Status().Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
				return err
			}
		}
		return nil
	}
	handleProgressDetail(reqCtx, opsRequest, progressDetail, compName, completedCount, failedCount)
	return nil
}

func patchSwitchoverProgressStatus(ctx context.Context, cli client.Client, opsRequest *opsv1alpha1.OpsRequest,
	compName, objectKey string, mutate func(*opsv1alpha1.ProgressStatusDetail)) error {
	patch := client.MergeFromWithOptions(opsRequest.DeepCopy(), client.MergeFromWithOptimisticLock{})
	progressDetail := findStatusProgressDetail(opsRequest.Status.Components[compName].ProgressDetails, objectKey)
	if progressDetail == nil {
		return fmt.Errorf("progress detail not found for component %s", compName)
	}
	mutate(progressDetail)
	return cli.Status().Patch(ctx, opsRequest, patch)
}

func confirmSwitchoverDispatchClaim(ctx context.Context, opsRes *OpsResource,
	opsRequest *opsv1alpha1.OpsRequest, compName, objectKey string, expected switchoverDispatchClaim) (bool, error) {
	reader := opsRes.APIReader
	if reader == nil {
		return false, errors.New("APIReader is required to confirm a switchover dispatch claim")
	}
	var confirmed *opsv1alpha1.OpsRequest
	var live switchoverDispatchClaim
	err := retry.OnError(retry.DefaultBackoff, shouldRetrySwitchoverStatusError, func() error {
		fresh := &opsv1alpha1.OpsRequest{}
		if err := reader.Get(ctx, client.ObjectKeyFromObject(opsRequest), fresh); err != nil {
			return err
		}
		progressDetail := findStatusProgressDetail(fresh.Status.Components[compName].ProgressDetails, objectKey)
		if progressDetail == nil || progressDetail.Status != opsv1alpha1.ProcessingProgressStatus {
			return errors.Wrapf(errSwitchoverDispatchClaimLost, "claim was not committed for component %s", compName)
		}
		parsed, ok := parseSwitchoverDispatchClaim(progressDetail.Message)
		if !ok || parsed.opsRequestUID != expected.opsRequestUID || parsed.componentName != expected.componentName ||
			parsed.instanceName != expected.instanceName || parsed.candidateName != expected.candidateName {
			return errors.Wrapf(errSwitchoverDispatchClaimLost, "claim identity does not match component %s", compName)
		}
		confirmed = fresh
		live = parsed
		return nil
	})
	if err != nil {
		return false, err
	}
	confirmed.DeepCopyInto(opsRequest)
	return live.token == expected.token, nil
}

func shouldRetrySwitchoverStatusError(err error) bool {
	if errors.Is(err, errSwitchoverDispatchClaimLost) ||
		errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	return !apierrors.IsNotFound(err) && !apierrors.IsForbidden(err) &&
		!apierrors.IsUnauthorized(err) && !apierrors.IsInvalid(err) &&
		!apierrors.IsRequestEntityTooLargeError(err) && !apierrors.IsBadRequest(err) &&
		!apierrors.IsMethodNotSupported(err) && !apierrors.IsUnsupportedMediaType(err) &&
		!apierrors.IsNotAcceptable(err)
}

func retryKnownSwitchoverOutcomeUntilContextDone(ctx context.Context, fn func() error) error {
	delay := 10 * time.Millisecond
	const maxDelay = time.Second
	for {
		err := fn()
		if err == nil || !shouldRetrySwitchoverStatusError(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return errors.Wrapf(ctx.Err(), "stopped persisting the known switchover outcome after retryable error: %v", err)
		case <-time.After(delay):
		}
		if delay < maxDelay {
			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
			}
		}
	}
}

func persistKnownSwitchoverDispatchOutcome(
	reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	reader client.Reader,
	opsRequest *opsv1alpha1.OpsRequest,
	compName,
	objectKey string,
	expected switchoverDispatchClaim,
	status opsv1alpha1.ProgressStatus,
	message string) (*opsv1alpha1.ProgressStatusDetail, error) {
	var persisted *opsv1alpha1.OpsRequest
	persistedMessage := expected.outcomeMessage(message)
	err := retryKnownSwitchoverOutcomeUntilContextDone(reqCtx.Ctx, func() error {
		fresh := &opsv1alpha1.OpsRequest{}
		if err := reader.Get(reqCtx.Ctx, client.ObjectKeyFromObject(opsRequest), fresh); err != nil {
			return err
		}
		componentStatus, ok := fresh.Status.Components[compName]
		if !ok {
			return errors.Wrapf(errSwitchoverDispatchClaimLost, "component %s status is missing", compName)
		}
		progressDetail := findStatusProgressDetail(componentStatus.ProgressDetails, objectKey)
		if progressDetail == nil {
			return errors.Wrapf(errSwitchoverDispatchClaimLost, "component %s progress detail is missing", compName)
		}
		persistedClaim, persistedOutcome, hasPersistedOutcome := parseSwitchoverDispatchOutcomeMessage(progressDetail.Message)
		if progressDetail.Status == status && hasPersistedOutcome && persistedClaim.equal(expected) && persistedOutcome == message {
			persisted = fresh
			return nil
		}
		live, ok := parseSwitchoverDispatchClaim(progressDetail.Message)
		if !ok || progressDetail.Status != opsv1alpha1.ProcessingProgressStatus || !live.equal(expected) {
			return errors.Wrapf(errSwitchoverDispatchClaimLost, "component %s", compName)
		}

		old := fresh.DeepCopy()
		progressDetail.Status = status
		progressDetail.Message = persistedMessage
		updateProgressDetailTime(progressDetail)
		componentStatus.Phase = appsv1.UpdatingComponentPhase
		componentStatus.ProgressDetails = append([]opsv1alpha1.ProgressStatusDetail(nil), componentStatus.ProgressDetails...)
		fresh.Status.Components[compName] = componentStatus
		patch := client.MergeFromWithOptions(old, client.MergeFromWithOptimisticLock{})
		if err := cli.Status().Patch(reqCtx.Ctx, fresh, patch); err != nil {
			return err
		}
		persisted = fresh
		return nil
	})
	if err != nil {
		return nil, err
	}
	if persisted == nil {
		return nil, errors.New("switchover dispatch outcome was not persisted")
	}
	persisted.DeepCopyInto(opsRequest)
	progressDetail := findStatusProgressDetail(opsRequest.Status.Components[compName].ProgressDetails, objectKey)
	if progressDetail == nil {
		return nil, fmt.Errorf("progress detail not found for component %s after persisting switchover outcome", compName)
	}
	sendProgressDetailEvent(reqCtx.Recorder, opsRequest, *progressDetail)
	return progressDetail, nil
}

func getSwitchoverPodBackedInstance(runtime OpsRuntime, namespace, clusterName, compName, instanceName string) (Instance, error) {
	instance, err := runtime.GetInstance(namespace, clusterName, compName, instanceName)
	if err != nil {
		return nil, err
	}
	if !instance.HasPod() {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "pods"}, instanceName)
	}
	return instance, nil
}

// setComponentSwitchoverProgressDetails sets component switchover progress details.
func setComponentSwitchoverProgressDetails(recorder record.EventRecorder,
	opsRequest *opsv1alpha1.OpsRequest,
	phase appsv1.ComponentPhase,
	processDetail opsv1alpha1.ProgressStatusDetail,
	componentName string) {
	componentProcessDetails := opsRequest.Status.Components[componentName].ProgressDetails
	setComponentStatusProgressDetail(recorder, opsRequest, &componentProcessDetails, processDetail)
	opsRequest.Status.Components[componentName] = opsv1alpha1.OpsRequestComponentStatus{
		Phase:           phase,
		ProgressDetails: componentProcessDetails,
	}
}

func getClusterCompSpec(cluster *appsv1.Cluster, clusterCompName string) (*appsv1.ClusterComponentSpec, error) {
	compSpec := cluster.Spec.GetComponentByName(clusterCompName)
	if compSpec != nil {
		return compSpec, nil
	}
	shardingSpec := cluster.Spec.GetShardingByName(clusterCompName)
	if shardingSpec != nil {
		return &shardingSpec.Template, nil
	}
	return nil, fmt.Errorf(`component "%s" not found`, clusterCompName)
}

func getCompSpecBySwitchover(ctx context.Context, cli client.Client, cluster *appsv1.Cluster, switchover opsv1alpha1.Switchover) (*appsv1.ClusterComponentSpec, error) {
	if len(switchover.ComponentName) > 0 {
		return getClusterCompSpec(cluster, switchover.ComponentName)
	}
	compObj := &appsv1.Component{}
	if err := cli.Get(ctx, client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      switchover.ComponentObjectName,
	}, compObj); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf(`component object "%s" not found`, switchover.ComponentObjectName)
		}
		return nil, err
	}
	clusterCompName := compObj.Labels[constant.KBAppShardingNameLabelKey]
	if len(clusterCompName) == 0 {
		clusterCompName = compObj.Labels[constant.KBAppComponentLabelKey]
	}
	return getClusterCompSpec(cluster, clusterCompName)
}

func getSwitchoverClusterComponentName(ctx context.Context, cli client.Client, cluster *appsv1.Cluster, switchover opsv1alpha1.Switchover) (string, error) {
	if len(switchover.ComponentName) > 0 {
		if cluster.Spec.GetComponentByName(switchover.ComponentName) != nil ||
			cluster.Spec.GetShardingByName(switchover.ComponentName) != nil {
			return switchover.ComponentName, nil
		}
		return "", fmt.Errorf(`component "%s" not found`, switchover.ComponentName)
	}

	compObj := &appsv1.Component{}
	if err := cli.Get(ctx, client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      switchover.ComponentObjectName,
	}, compObj); err != nil {
		if apierrors.IsNotFound(err) {
			return "", fmt.Errorf(`component object "%s" not found`, switchover.ComponentObjectName)
		}
		return "", err
	}
	if shardingName := compObj.Labels[constant.KBAppShardingNameLabelKey]; len(shardingName) > 0 {
		return shardingName, nil
	}
	if compName := compObj.Labels[constant.KBAppComponentLabelKey]; len(compName) > 0 {
		return compName, nil
	}
	return "", fmt.Errorf(`component object "%s" has no component label`, switchover.ComponentObjectName)
}

func buildSynthesizedComp(ctx context.Context, cli client.Client, opsRes *OpsResource, switchover opsv1alpha1.Switchover) (*component.SynthesizedComponent, error) {
	clusterCompSpec, err := getCompSpecBySwitchover(ctx, cli, opsRes.Cluster, switchover)
	if err != nil {
		return nil, err
	}
	componentObjectName := constant.GenerateClusterComponentName(opsRes.Cluster.Name, clusterCompSpec.Name)
	if len(switchover.ComponentObjectName) > 0 {
		componentObjectName = switchover.ComponentObjectName
	} else if opsRes.Cluster.Spec.GetShardingByName(switchover.ComponentName) != nil {
		componentObjectName, err = getShardingComponentObjectNameByInstance(ctx, cli, opsRes.Cluster, switchover.ComponentName, switchover.InstanceName)
		if err != nil {
			return nil, err
		}
	}
	compObj, compDefObj, err := component.GetCompNCompDefByName(ctx, cli, opsRes.Cluster.Namespace, componentObjectName)
	if err != nil {
		return nil, err
	}
	return component.BuildSynthesizedComponent(ctx, cli, compDefObj, compObj)
}

func getShardingComponentObjectNameByInstance(ctx context.Context, cli client.Client, cluster *appsv1.Cluster, shardingName, instanceName string) (string, error) {
	pod := &corev1.Pod{}
	if err := cli.Get(ctx, client.ObjectKey{Namespace: cluster.Namespace, Name: instanceName}, pod); err != nil {
		if apierrors.IsNotFound(err) {
			return "", intctrlutil.NewFatalError(fmt.Sprintf(`instance "%s" not found`, instanceName))
		}
		return "", err
	}
	if pod.Labels[constant.AppInstanceLabelKey] != cluster.Name {
		return "", intctrlutil.NewFatalError(fmt.Sprintf(`instance "%s" does not belong to cluster "%s"`, instanceName, cluster.Name))
	}
	if pod.Labels[constant.KBAppShardingNameLabelKey] != shardingName {
		return "", intctrlutil.NewFatalError(fmt.Sprintf(`instance "%s" does not belong to sharding "%s"`, instanceName, shardingName))
	}
	componentName := pod.Labels[constant.KBAppComponentLabelKey]
	if componentName == "" {
		return "", intctrlutil.NewFatalError(fmt.Sprintf(`instance "%s" does not have component label`, instanceName))
	}
	return constant.GenerateClusterComponentName(cluster.Name, componentName), nil
}

func handleProgressDetail(
	reqCtx intctrlutil.RequestCtx,
	opsRequest *opsv1alpha1.OpsRequest,
	progressDetail *opsv1alpha1.ProgressStatusDetail,
	compName string,
	completedCount,
	failedCount *int32) {
	// set timeout to 5 minutes for handling the switchover
	if progressDetail.Status == opsv1alpha1.ProcessingProgressStatus &&
		!progressDetail.StartTime.IsZero() && time.Now().After(progressDetail.StartTime.Add(5*time.Minute)) {
		progressDetail.Status = opsv1alpha1.FailedProgressStatus
		progressDetail.Message = "switchover timeout after 5 minutes"
	}
	if isCompletedProgressStatus(progressDetail.Status) {
		*completedCount++
		if progressDetail.Status == opsv1alpha1.FailedProgressStatus {
			*failedCount++
		}
	}
	setComponentSwitchoverProgressDetails(reqCtx.Recorder, opsRequest, appsv1.UpdatingComponentPhase, *progressDetail, compName)
}
