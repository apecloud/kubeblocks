/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package v1alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// condition types
	ConditionTypeCancelled          = "Cancelled"
	ConditionTypeWaitForProgressing = "WaitForProgressing"
	ConditionTypeValidated          = "Validated"
	ConditionTypeSucceed            = "Succeed"
	ConditionTypeFailed             = "Failed"
	ConditionTypeAborted            = "Aborted"
	ConditionTypeRestarting         = "Restarting"
	ConditionTypeVerticalScaling    = "VerticalScaling"
	ConditionTypeHorizontalScaling  = "HorizontalScaling"
	ConditionTypeVolumeExpanding    = "VolumeExpanding"
	ConditionTypeReconfigure        = "Reconfigure"
	ConditionTypeSwitchover         = "Switchover"
	ConditionTypeStop               = "Stopping"
	ConditionTypeStart              = "Starting"
	ConditionTypeVersionUpgrading   = "VersionUpgrading"
	ConditionTypeExpose             = "Exposing"
	ConditionTypeDataScript         = "ExecuteDataScript"
	ConditionTypeBackup             = "Backup"
	ConditionTypeInstanceRebuilding = "InstancesRebuilding"
	ConditionTypeCustomOperation    = "CustomOperation"

	// condition and event reasons

	ReasonReconfigurePersisting    = "ReconfigurePersisting"
	ReasonReconfigurePersisted     = "ReconfigurePersisted"
	ReasonReconfigureFailed        = "ReconfigureFailed"
	ReasonReconfigureRestartFailed = "ReconfigureRestartFailed"
	ReasonReconfigureRestart       = "ReconfigureRestarted"
	ReasonReconfigureNoChanged     = "ReconfigureNoChanged"
	ReasonReconfigureSucceed       = "ReconfigureSucceed"
	ReasonReconfigureRunning       = "ReconfigureRunning"
	ReasonClusterPhaseMismatch     = "ClusterPhaseMismatch"
	ReasonOpsTypeNotSupported      = "OpsTypeNotSupported"
	ReasonValidateFailed           = "ValidateFailed"
	ReasonClusterNotFound          = "ClusterNotFound"
	ReasonOpsRequestFailed         = "OpsRequestFailed"
	ReasonOpsCanceling             = "Canceling"
	ReasonOpsCancelFailed          = "CancelFailed"
	ReasonOpsCancelSucceed         = "CancelSucceed"
	ReasonOpsCancelByController    = "CancelByController"
)

func (r *OpsRequest) SetStatusCondition(condition metav1.Condition) {
	meta.SetStatusCondition(&r.Status.Conditions, condition)
}

// NewWaitForProcessingCondition waits the controller to process the opsRequest.
func NewWaitForProcessingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeWaitForProgressing,
		Status:             metav1.ConditionTrue,
		Reason:             ConditionTypeWaitForProgressing,
		LastTransitionTime: metav1.Now(),
		Message: fmt.Sprintf("wait for the controller to process the OpsRequest: %s in Cluster: %s",
			ops.Name, ops.Spec.GetClusterName()),
	}
}

// NewCancelingCondition the controller is canceling the OpsRequest
func NewCancelingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeCancelled,
		Status:             metav1.ConditionFalse,
		Reason:             ReasonOpsCanceling,
		LastTransitionTime: metav1.Now(),
		Message: fmt.Sprintf(`Start to cancel the OpsRequest "%s" in Cluster: "%s"`,
			ops.Name, ops.Spec.GetClusterName()),
	}
}

// NewCancelFailedCondition creates a condition for canceling failed.
func NewCancelFailedCondition(ops *OpsRequest, err error) *metav1.Condition {
	msg := fmt.Sprintf(`Failed to cancel OpsRequest "%s"`, ops.Name)
	if err != nil {
		msg = err.Error()
	}
	return &metav1.Condition{
		Type:               ConditionTypeCancelled,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonOpsCancelFailed,
		LastTransitionTime: metav1.Now(),
		Message:            msg,
	}
}

// NewAbortedCondition creates a condition for aborted phase.
func NewAbortedCondition(ops *OpsRequest) metav1.Condition {
	return metav1.Condition{
		Type:               ConditionTypeAborted,
		Status:             metav1.ConditionTrue,
		Reason:             ConditionTypeAborted,
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf(`Aborted as a result of the latest opsRequest "%s"`, ops.Name),
	}
}

// NewCancelSucceedCondition creates a condition for canceling successfully.
func NewCancelSucceedCondition(opsName string) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeCancelled,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonOpsCancelSucceed,
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf(`Cancel OpsRequest "%s" successfully`, opsName),
	}
}

// NewValidatePassedCondition creates a condition for operation validation to pass.
func NewValidatePassedCondition(opsRequestName string) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeValidated,
		Status:             metav1.ConditionTrue,
		Reason:             "ValidateOpsRequestPassed",
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("OpsRequest: %s is validated", opsRequestName),
	}
}

// NewValidateFailedCondition creates a condition for operation validation failure.
func NewValidateFailedCondition(reason, message string) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeValidated,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		LastTransitionTime: metav1.Now(),
		Message:            message,
	}
}

// NewFailedCondition creates a condition that the OpsRequest processing failed
func NewFailedCondition(ops *OpsRequest, err error) *metav1.Condition {
	msg := fmt.Sprintf("Failed to process OpsRequest: %s in cluster: %s, more detailed informations in status.components", ops.Name, ops.Spec.GetClusterName())
	if err != nil {
		msg = err.Error()
	}
	return &metav1.Condition{
		Type:               ConditionTypeFailed,
		Status:             metav1.ConditionFalse,
		Reason:             ReasonOpsRequestFailed,
		LastTransitionTime: metav1.Now(),
		Message:            msg,
	}
}

// NewSucceedCondition creates a condition that the controller has successfully processed the OpsRequest
func NewSucceedCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeSucceed,
		Status:             metav1.ConditionTrue,
		Reason:             "OpsRequestProcessedSuccessfully",
		LastTransitionTime: metav1.Now(),
		Message: fmt.Sprintf("Successfully processed the OpsRequest: %s in Cluster: %s",
			ops.Name, ops.Spec.GetClusterName()),
	}
}

// NewRestartingCondition creates a condition that the operation starts to restart components
func NewRestartingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeRestarting,
		Status:             metav1.ConditionTrue,
		Reason:             "RestartStarted",
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("Start to restart database in Cluster: %s", ops.Spec.GetClusterName()),
	}
}

// NewInstancesRebuildingCondition creates a condition that the operation starts to rebuild the instances.
func NewInstancesRebuildingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeInstanceRebuilding,
		Status:             metav1.ConditionTrue,
		Reason:             "StartToRebuildInstances",
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("Start to rebuild the instances in Cluster: %s", ops.Spec.GetClusterName()),
	}
}

// NewSwitchoveringCondition creates a condition that the operation starts to switchover components
func NewSwitchoveringCondition(generation int64, message string) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeSwitchover,
		Status:             metav1.ConditionTrue,
		Reason:             "SwitchoverStarted",
		LastTransitionTime: metav1.Now(),
		Message:            message,
		ObservedGeneration: generation,
	}
}

// NewVerticalScalingCondition creates a condition that the OpsRequest starts to vertical scale cluster
func NewVerticalScalingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeVerticalScaling,
		Status:             metav1.ConditionTrue,
		Reason:             "VerticalScalingStarted",
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("Start to vertical scale resources in Cluster: %s", ops.Spec.GetClusterName()),
	}
}

// NewHorizontalScalingCondition creates a condition that the OpsRequest starts to horizontal scale cluster
func NewHorizontalScalingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeHorizontalScaling,
		Status:             metav1.ConditionTrue,
		Reason:             "HorizontalScalingStarted",
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("Start to horizontal scale replicas in Cluster: %s", ops.Spec.GetClusterName()),
	}
}

// NewVolumeExpandingCondition creates a condition that the OpsRequest starts to expand volume
func NewVolumeExpandingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeVolumeExpanding,
		Status:             metav1.ConditionTrue,
		Reason:             "VolumeExpansionStarted",
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("Start to expand the volumes in Cluster: %s", ops.Spec.GetClusterName()),
	}
}

func NewExposingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeExpose,
		Status:             metav1.ConditionTrue,
		Reason:             "ExposeStarted",
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("Start to expose the services in Cluster: %s", ops.Spec.GetClusterName()),
	}
}

// NewUpgradingCondition creates a condition that the OpsRequest starts to upgrade the cluster version
func NewUpgradingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeVersionUpgrading,
		Status:             metav1.ConditionTrue,
		Reason:             "VersionUpgradeStarted",
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("Start to upgrade the version in Cluster: %s", ops.Spec.GetClusterName()),
	}
}

// NewStopCondition creates a condition that the OpsRequest starts to stop the cluster.
func NewStopCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeStop,
		Status:             metav1.ConditionTrue,
		Reason:             "StopStarted",
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("Start to stop the Cluster: %s", ops.Spec.GetClusterName()),
	}
}

// NewStartCondition creates a condition that the OpsRequest starts the cluster.
func NewStartCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeStart,
		Status:             metav1.ConditionTrue,
		Reason:             "StartCluster",
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("Start the Cluster: %s", ops.Spec.GetClusterName()),
	}
}

// NewReconfigureCondition creates a condition that the OpsRequest updating component configuration
func NewReconfigureCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeReconfigure,
		Status:             metav1.ConditionTrue,
		Reason:             "ReconfigureStarted",
		LastTransitionTime: metav1.Now(),
		Message: fmt.Sprintf("Start to reconfigure in Cluster: %s, Component: %s",
			ops.Spec.GetClusterName(),
			getComponentName(ops.Spec)),
	}
}

func NewDataScriptCondition(ops *OpsRequest) *metav1.Condition {
	return newOpsCondition(ops, ConditionTypeDataScript, "DataScriptStarted", fmt.Sprintf("Start to execute data script in Cluster: %s", ops.Spec.GetClusterName()))
}

func newOpsCondition(_ *OpsRequest, condType, reason, message string) *metav1.Condition {
	return &metav1.Condition{
		Type:               condType,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		LastTransitionTime: metav1.Now(),
		Message:            message,
	}
}

// NewReconfigureRunningCondition creates a condition that the OpsRequest reconfigure workflow
func NewReconfigureRunningCondition(ops *OpsRequest, conditionType string, configSpecName string, info ...string) *metav1.Condition {
	status := metav1.ConditionTrue
	if conditionType == ReasonReconfigureFailed {
		status = metav1.ConditionFalse
	}
	message := fmt.Sprintf("Reconfiguring in Cluster: %s, Component: %s, ConfigSpec: %s",
		ops.Spec.GetClusterName(),
		getComponentName(ops.Spec),
		configSpecName)
	if len(info) > 0 {
		message = message + ", info: " + info[0]
	}
	return &metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             conditionType,
		LastTransitionTime: metav1.Now(),
		Message:            message,
	}
}

func getComponentName(request OpsRequestSpec) string {
	if request.Reconfigure != nil {
		return request.Reconfigure.ComponentName
	}
	return ""
}

// NewReconfigureFailedCondition creates a condition for the failed reconfigure.
func NewReconfigureFailedCondition(ops *OpsRequest, err error) *metav1.Condition {
	var msg string
	if err != nil {
		msg = err.Error()
	} else {
		msg = fmt.Sprintf("Failed to reconfigure: %s in cluster: %s", ops.Name, ops.Spec.GetClusterName())
	}
	return &metav1.Condition{
		Type:               ReasonReconfigureFailed,
		Status:             metav1.ConditionFalse,
		Reason:             "ReconfigureFailed",
		LastTransitionTime: metav1.Now(),
		Message:            msg,
	}
}

// NewBackupCondition creates a condition that the OpsRequest backup the cluster.
func NewBackupCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeBackup,
		Status:             metav1.ConditionTrue,
		Reason:             "BackupStarted",
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("Start to backup the Cluster: %s", ops.Spec.GetClusterName()),
	}
}

// NewRestoreCondition creates a condition that the OpsRequest restore the cluster.
func NewRestoreCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeBackup,
		Status:             metav1.ConditionTrue,
		Reason:             "RestoreStarted",
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("Start to restore the Cluster: %s", ops.Spec.GetClusterName()),
	}
}
