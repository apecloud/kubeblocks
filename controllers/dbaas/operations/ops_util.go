/*
Copyright ApeCloud Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package operations

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/util"
	opsutil "github.com/apecloud/kubeblocks/controllers/dbaas/operations/util"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

type handleStatusProgressWithComponent func(opsRes *OpsResource,
	pgRes progressResource,
	statusComponent *dbaasv1alpha1.OpsRequestStatusComponent) (expectProgressCount int32, succeedCount int32, err error)

// ReconcileActionWithComponentOps will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// if OpsRequest.spec.componentOps is not null, you can use it to OpsBehaviour.ReconcileAction.
// return the OpsRequest.status.phase
func ReconcileActionWithComponentOps(opsRes *OpsResource,
	opsMessageKey string,
	handleStatusProgress handleStatusProgressWithComponent,
) (dbaasv1alpha1.Phase, time.Duration, error) {
	var (
		opsRequest = opsRes.OpsRequest
		// check if all components of the OpsRequest are processed.
		isCompleted              = true
		isFailed                 bool
		opsRequestPhase          = dbaasv1alpha1.RunningPhase
		clusterDef               *dbaasv1alpha1.ClusterDefinition
		err                      error
		ok                       bool
		expectProgressCount      int32
		succeedProgressCount     int32
		checkAllClusterComponent bool
	)
	componentNameMap := opsRequest.GetComponentNameMap()
	// if no specified components, we should check the all components phase of cluster.
	if len(componentNameMap) == 0 {
		checkAllClusterComponent = true
	}
	if clusterDef, err = GetClusterDefByName(opsRes.Ctx, opsRes.Client,
		opsRes.Cluster.Spec.ClusterDefRef); err != nil {
		return opsRequestPhase, 0, nil
	}

	patch := client.MergeFrom(opsRequest.DeepCopy())
	oldOpsRequestStatus := opsRequest.Status.DeepCopy()
	if opsRequest.Status.Components == nil {
		opsRequest.Status.Components = map[string]dbaasv1alpha1.OpsRequestStatusComponent{}
	}
	for k, v := range opsRes.Cluster.Status.Components {
		if _, ok = componentNameMap[k]; !ok && !checkAllClusterComponent {
			continue
		}
		if !util.IsCompleted(v.Phase) {
			isCompleted = false
		}
		if util.IsFailedOrAbnormal(v.Phase) {
			isFailed = true
		}
		var statusComponent dbaasv1alpha1.OpsRequestStatusComponent
		if statusComponent, ok = opsRequest.Status.Components[k]; !ok {
			statusComponent = dbaasv1alpha1.OpsRequestStatusComponent{}
		}
		if statusComponent.Phase != v.Phase {
			statusComponent.Phase = v.Phase
		}
		clusterComponent := util.GetComponentByName(opsRes.Cluster, k)
		expectCount, succeedCount, err := handleStatusProgress(opsRes, progressResource{
			opsMessageKey:       opsMessageKey,
			clusterComponent:    clusterComponent,
			clusterComponentDef: util.GetComponentDefFromClusterDefinition(clusterDef, clusterComponent.Type),
		}, &statusComponent)
		if err != nil {
			return opsRequestPhase, 0, nil
		}
		expectProgressCount += expectCount
		succeedProgressCount += succeedCount
		opsRequest.Status.Components[k] = statusComponent
	}
	opsRequest.Status.Progress = fmt.Sprintf("%d/%d", succeedProgressCount, expectProgressCount)
	if !reflect.DeepEqual(opsRequest.Status, oldOpsRequestStatus) {
		if err = opsRes.Client.Status().Patch(opsRes.Ctx, opsRequest, patch); err != nil {
			return opsRequestPhase, 0, err
		}
	}
	// wait for all components to finish processing.
	if !isCompleted {
		return opsRequestPhase, 0, nil
	}
	if isFailed {
		opsRequestPhase = dbaasv1alpha1.FailedPhase
	} else {
		opsRequestPhase = dbaasv1alpha1.SucceedPhase
	}
	return opsRequestPhase, 0, nil
}

// GetClusterDefByName gets the ClusterDefinition object by the name.
func GetClusterDefByName(ctx context.Context, cli client.Client, clusterDefName string) (*dbaasv1alpha1.ClusterDefinition, error) {
	clusterDef := &dbaasv1alpha1.ClusterDefinition{}
	if err := cli.Get(ctx, client.ObjectKey{Name: clusterDefName}, clusterDef); err != nil {
		return nil, err
	}
	return clusterDef, nil
}

// opsRequestIsCompleted checks if OpsRequest is completed
func opsRequestIsCompleted(phase dbaasv1alpha1.Phase) bool {
	return slices.Index([]dbaasv1alpha1.Phase{dbaasv1alpha1.FailedPhase, dbaasv1alpha1.SucceedPhase}, phase) != -1
}

// PatchOpsStatus patches OpsRequest.status
func PatchOpsStatus(opsRes *OpsResource,
	phase dbaasv1alpha1.Phase,
	condition ...*metav1.Condition) error {

	opsRequest := opsRes.OpsRequest
	patch := client.MergeFrom(opsRequest.DeepCopy())
	for _, v := range condition {
		if v == nil {
			continue
		}
		opsRequest.SetStatusCondition(*v)
		// provide an event
		eventType := corev1.EventTypeNormal
		if phase == dbaasv1alpha1.FailedPhase {
			eventType = corev1.EventTypeWarning
		}
		opsRes.Recorder.Event(opsRequest, eventType, v.Reason, v.Message)
	}
	if opsRequestIsCompleted(phase) {
		opsRequest.Status.CompletionTimestamp = metav1.Time{Time: time.Now()}
		// when OpsRequest is completed, do it
		if err := deleteOpsRequestAnnotationInCluster(opsRes); err != nil {
			return err
		}
	}
	if phase == dbaasv1alpha1.RunningPhase && opsRequest.Status.Phase != phase {
		opsRequest.Status.StartTimestamp = metav1.Time{Time: time.Now()}
	}
	opsRequest.Status.Phase = phase
	return opsRes.Client.Status().Patch(opsRes.Ctx, opsRequest, patch)
}

// PatchClusterNotFound patches ClusterNotFound condition to the OpsRequest.status.conditions.
func PatchClusterNotFound(opsRes *OpsResource) error {
	message := fmt.Sprintf("spec.clusterRef %s is not found", opsRes.OpsRequest.Spec.ClusterRef)
	condition := dbaasv1alpha1.NewValidateFailedCondition(dbaasv1alpha1.ReasonClusterNotFound, message)
	return PatchOpsStatus(opsRes, dbaasv1alpha1.FailedPhase, condition)
}

// patchOpsHandlerNotSupported patches OpsNotSupported condition to the OpsRequest.status.conditions.
func patchOpsHandlerNotSupported(opsRes *OpsResource) error {
	message := fmt.Sprintf("spec.type %s is not supported by operator", opsRes.OpsRequest.Spec.Type)
	condition := dbaasv1alpha1.NewValidateFailedCondition(dbaasv1alpha1.ReasonOpsTypeNotSupported, message)
	return PatchOpsStatus(opsRes, dbaasv1alpha1.FailedPhase, condition)
}

// PatchValidateErrorCondition patches ValidateError condition to the OpsRequest.status.conditions.
func PatchValidateErrorCondition(opsRes *OpsResource, errMessage string) error {
	condition := dbaasv1alpha1.NewValidateFailedCondition(dbaasv1alpha1.ReasonValidateError, errMessage)
	return PatchOpsStatus(opsRes, dbaasv1alpha1.FailedPhase, condition)
}

// getOpsRequestNameFromAnnotation gets OpsRequest.name from cluster.annotations
func getOpsRequestNameFromAnnotation(cluster *dbaasv1alpha1.Cluster, toClusterPhase dbaasv1alpha1.Phase) string {
	opsRequestSlice, _ := opsutil.GetOpsRequestSliceFromCluster(cluster)
	opsRecorder := getOpsRecorderWithClusterPhase(opsRequestSlice, toClusterPhase)
	return opsRecorder.Name
}

// getOpsRecorderWithClusterPhase gets OpsRequest recorder from slice by target cluster phase
func getOpsRecorderWithClusterPhase(opsRequestSlice []dbaasv1alpha1.OpsRecorder,
	toClusterPhase dbaasv1alpha1.Phase) dbaasv1alpha1.OpsRecorder {
	for _, v := range opsRequestSlice {
		if v.ToClusterPhase == toClusterPhase {
			return v
		}
	}
	return dbaasv1alpha1.OpsRecorder{}
}

// GetOpsRecorderFromSlice gets OpsRequest recorder from slice by target cluster phase
func GetOpsRecorderFromSlice(opsRequestSlice []dbaasv1alpha1.OpsRecorder,
	opsRequestName string) (int, dbaasv1alpha1.OpsRecorder) {
	for i, v := range opsRequestSlice {
		if v.Name == opsRequestName {
			return i, v
		}
	}
	return 0, dbaasv1alpha1.OpsRecorder{}
}

// patchOpsRequestToRunning patches OpsRequest.status.phase to Running
func patchOpsRequestToRunning(opsRes *OpsResource, opsHandler OpsHandler) error {
	var condition *metav1.Condition
	validatePassCondition := dbaasv1alpha1.NewValidatePassedCondition(opsRes.OpsRequest.Name)
	condition = opsHandler.ActionStartedCondition(opsRes.OpsRequest)
	return PatchOpsStatus(opsRes, dbaasv1alpha1.RunningPhase, validatePassCondition, condition)
}

// patchClusterStatus updates Cluster.status to record cluster and components information
func patchClusterStatus(opsRes *OpsResource, opsBehaviour OpsBehaviour) error {
	toClusterState := opsBehaviour.ToClusterPhase
	if toClusterState == "" {
		return nil
	}
	patch := client.MergeFrom(opsRes.Cluster.DeepCopy())
	opsRes.Cluster.Status.Phase = toClusterState
	realChangeCompMap := opsBehaviour.OpsHandler.GetRealAffectedComponentMap(opsRes.OpsRequest)
	// update cluster.status.components phase
	if len(realChangeCompMap) != 0 {
		for k, v := range opsRes.Cluster.Status.Components {
			if _, ok := realChangeCompMap[k]; ok {
				v.Phase = toClusterState
				opsRes.Cluster.Status.Components[k] = v
			}
		}
	}
	if err := opsRes.Client.Status().Patch(opsRes.Ctx, opsRes.Cluster, patch); err != nil {
		return err
	}
	opsRes.Recorder.Eventf(opsRes.Cluster, corev1.EventTypeNormal, string(opsRes.OpsRequest.Spec.Type),
		"Start %s in Cluster: %s", opsRes.OpsRequest.Spec.Type, opsRes.Cluster.Name)
	return nil
}

// deleteOpsRequestAnnotationInCluster when OpsRequest.status.phase is Succeed or Failed
// we should delete the OpsRequest Annotation in cluster, unlock cluster
func deleteOpsRequestAnnotationInCluster(opsRes *OpsResource) error {
	var (
		opsRequestSlice []dbaasv1alpha1.OpsRecorder
		err             error
	)
	if opsRequestSlice, err = opsutil.GetOpsRequestSliceFromCluster(opsRes.Cluster); err != nil {
		return err
	}
	index, opsRecord := GetOpsRecorderFromSlice(opsRequestSlice, opsRes.OpsRequest.Name)
	if opsRecord.Name == "" {
		return nil
	}
	// delete the opsRequest information in Cluster.annotations
	opsRequestSlice = slices.Delete(opsRequestSlice, index, index+1)
	if err = patchClusterPhaseWhenExistsOtherOps(opsRes, opsRequestSlice); err != nil {
		return err
	}
	return opsutil.PatchClusterOpsAnnotations(opsRes.Ctx, opsRes.Client, opsRes.Cluster, opsRequestSlice)
}

// addOpsRequestAnnotationToCluster when OpsRequest.phase is Running, we should add the OpsRequest Annotation to Cluster.metadata.Annotations
func addOpsRequestAnnotationToCluster(opsRes *OpsResource, toClusterPhase dbaasv1alpha1.Phase) error {
	var (
		opsRequestSlice []dbaasv1alpha1.OpsRecorder
		err             error
	)
	if toClusterPhase == "" {
		return nil
	}
	if opsRequestSlice, err = opsutil.GetOpsRequestSliceFromCluster(opsRes.Cluster); err != nil {
		return err
	}
	// check the OpsRequest is existed
	if _, opsRecorder := GetOpsRecorderFromSlice(opsRequestSlice, opsRes.OpsRequest.Name); opsRecorder.Name != "" {
		return nil
	}
	if opsRequestSlice == nil {
		opsRequestSlice = make([]dbaasv1alpha1.OpsRecorder, 0)
	}
	opsRequestSlice = append(opsRequestSlice, dbaasv1alpha1.OpsRecorder{
		Name:           opsRes.OpsRequest.Name,
		ToClusterPhase: toClusterPhase,
	})
	return opsutil.PatchClusterOpsAnnotations(opsRes.Ctx, opsRes.Client, opsRes.Cluster, opsRequestSlice)
}

// patchClusterPhaseWhenExistsOtherOps
func patchClusterPhaseWhenExistsOtherOps(opsRes *OpsResource, opsRequestSlice []dbaasv1alpha1.OpsRecorder) error {
	// If there are other OpsRequests running, modify the cluster.status.phase with other opsRequest's ToClusterPhase
	if len(opsRequestSlice) == 0 {
		return nil
	}
	patch := client.MergeFrom(opsRes.Cluster.DeepCopy())
	opsRes.Cluster.Status.Phase = opsRequestSlice[0].ToClusterPhase
	if err := opsRes.Client.Status().Patch(opsRes.Ctx, opsRes.Cluster, patch); err != nil {
		return err
	}
	return nil
}

// isOpsRequestFailedPhase checks the OpsRequest phase is Failed
func isOpsRequestFailedPhase(opsRequestPhase dbaasv1alpha1.Phase) bool {
	return opsRequestPhase == dbaasv1alpha1.FailedPhase
}

// patchReconfigureStatus when Reconfigure is running, we should update status to OpsRequest.Status.ConfigurationStatus.
func patchReconfigureStatus(opsRes *OpsResource,
	tplName string,
	execStatus *cfgcore.PolicyExecStatus,
	phase dbaasv1alpha1.Phase,
	create func(key string) dbaasv1alpha1.ConfigurationStatus) error {
	var (
		opsRequest = opsRes.OpsRequest
		status     = &opsRequest.Status
	)

	findAndInitStatus := func(status *dbaasv1alpha1.OpsRequestStatus, key string,
		create func(key string) dbaasv1alpha1.ConfigurationStatus) *dbaasv1alpha1.ConfigurationStatus {
		if status.ReconfiguringStatus == nil {
			status.ReconfiguringStatus = &dbaasv1alpha1.ReconfiguringStatus{
				ConfigurationStatus: make([]dbaasv1alpha1.ConfigurationStatus, 0),
			}
		}
		cfgStatus := status.ReconfiguringStatus.ConfigurationStatus
		for i := range cfgStatus {
			configStatus := &cfgStatus[i]
			if configStatus.Name == key {
				return configStatus
			}
		}
		cfgStatus = append(cfgStatus, create(key))
		status.ReconfiguringStatus.ConfigurationStatus = cfgStatus
		return &cfgStatus[len(cfgStatus)-1]
	}
	updateReconfigureStatus := func(tplStatus *dbaasv1alpha1.ConfigurationStatus, execStatus *cfgcore.PolicyExecStatus,
		phase dbaasv1alpha1.Phase) {
		tplStatus.LastAppliedStatus = execStatus.ExecStatus
		tplStatus.UpdatePolicy = dbaasv1alpha1.UpgradePolicy(execStatus.PolicyName)
		tplStatus.SucceedCount = execStatus.SucceedCount
		tplStatus.ExpectedCount = execStatus.ExpectedCount
		if tplStatus.SucceedCount != cfgcore.Unconfirmed && tplStatus.ExpectedCount != cfgcore.Unconfirmed {
			status.Progress = calReconfiguringProgress(status.ReconfiguringStatus.ConfigurationStatus)
		}
		switch phase {
		case dbaasv1alpha1.SucceedPhase:
			tplStatus.Status = dbaasv1alpha1.ReasonReconfigureSucceed
		case dbaasv1alpha1.FailedPhase:
			tplStatus.Status = dbaasv1alpha1.ReasonReconfigureFailed
		default:
			tplStatus.Status = dbaasv1alpha1.ReasonReconfigureRunning
		}
	}

	patch := client.MergeFrom(opsRequest.DeepCopy())
	tplStatus := findAndInitStatus(status, tplName, create)
	if execStatus != nil {
		updateReconfigureStatus(tplStatus, execStatus, phase)
	}
	if err := opsRes.Client.Status().Patch(opsRes.Ctx, opsRequest, patch); err != nil {
		return err
	}
	return nil
}

// calReconfiguringProgress calculate the progress of the reconfiguring operations.
func calReconfiguringProgress(status []dbaasv1alpha1.ConfigurationStatus) string {
	slowest := dbaasv1alpha1.ConfigurationStatus{
		SucceedCount:  math.MaxInt32,
		ExpectedCount: -1,
	}

	for _, st := range status {
		if st.SucceedCount < slowest.SucceedCount {
			slowest = st
		}
	}
	return fmt.Sprintf("%d/%d", slowest.SucceedCount, slowest.ExpectedCount)
}
