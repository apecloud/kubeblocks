/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package v1alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// condition types

	ConditionTypeProgressing       = "Progressing"
	ConditionTypeValidated         = "Validated"
	ConditionTypeSucceed           = "Succeed"
	ConditionTypeFailed            = "Failed"
	ConditionTypeRestarting        = "Restarting"
	ConditionTypeVerticalScaling   = "VerticalScaling"
	ConditionTypeHorizontalScaling = "HorizontalScaling"
	ConditionTypeVolumeExpanding   = "VolumeExpanding"
	ConditionTypeReconfigure       = "Reconfigure"
	ConditionTypeStop              = "Stopping"
	ConditionTypeStart             = "Starting"
	ConditionTypeVersionUpgrading  = "VersionUpgrading"
	ConditionTypeExpose            = "Exposing"

	// condition and event reasons

	ReasonReconfigureMerging   = "ReconfigureMerging"
	ReasonReconfigureMerged    = "ReconfigureMerged"
	ReasonReconfigureFailed    = "ReconfigureFailed"
	ReasonReconfigureNoChanged = "ReconfigureNoChanged"
	ReasonReconfigureSucceed   = "ReconfigureSucceed"
	ReasonReconfigureRunning   = "ReconfigureRunning"
	ReasonClusterPhaseMismatch = "ClusterPhaseMismatch"
	ReasonOpsTypeNotSupported  = "OpsTypeNotSupported"
	ReasonValidateFailed       = "ValidateFailed"
	ReasonClusterNotFound      = "ClusterNotFound"
	ReasonOpsRequestFailed     = "OpsRequestFailed"
)

func (r *OpsRequest) SetStatusCondition(condition metav1.Condition) {
	meta.SetStatusCondition(&r.Status.Conditions, condition)
}

// NewProgressingCondition the controller is progressing the OpsRequest
func NewProgressingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeProgressing,
		Status:             metav1.ConditionTrue,
		Reason:             "OpsRequestProgressingStarted",
		LastTransitionTime: metav1.Now(),
		Message: fmt.Sprintf("Start to process the OpsRequest: %s in Cluster: %s",
			ops.Name, ops.Spec.ClusterRef),
	}
}

// NewValidatePassedCondition creates a condition that the operation validation.
func NewValidatePassedCondition(opsRequestName string) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeValidated,
		Status:             metav1.ConditionTrue,
		Reason:             "ValidateOpsRequestPassed",
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("OpsRequest: %s is validated", opsRequestName),
	}
}

// NewValidateFailedCondition creates a condition that the operation validation.
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
	msg := fmt.Sprintf("Failed to process OpsRequest: %s in cluster: %s", ops.Name, ops.Spec.ClusterRef)
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
			ops.Name, ops.Spec.ClusterRef),
	}
}

// NewRestartingCondition creates a condition that the operation starts to restart components
func NewRestartingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeRestarting,
		Status:             metav1.ConditionTrue,
		Reason:             "RestartStarted",
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("Start to restart database in Cluster: %s", ops.Spec.ClusterRef),
	}
}

// NewVerticalScalingCondition creates a condition that the OpsRequest starts to vertical scale cluster
func NewVerticalScalingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeVerticalScaling,
		Status:             metav1.ConditionTrue,
		Reason:             "VerticalScalingStarted",
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("Start to vertical scale resources in Cluster: %s", ops.Spec.ClusterRef),
	}
}

// NewHorizontalScalingCondition creates a condition that the OpsRequest starts to horizontal scale cluster
func NewHorizontalScalingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeHorizontalScaling,
		Status:             metav1.ConditionTrue,
		Reason:             "HorizontalScalingStarted",
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("Start to horizontal scale replicas in Cluster: %s", ops.Spec.ClusterRef),
	}
}

// NewVolumeExpandingCondition creates a condition that the OpsRequest starts to expand volume
func NewVolumeExpandingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeVolumeExpanding,
		Status:             metav1.ConditionTrue,
		Reason:             "VolumeExpansionStarted",
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("Start to expand the volumes in Cluster: %s", ops.Spec.ClusterRef),
	}
}

func NewExposingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeExpose,
		Status:             metav1.ConditionTrue,
		Reason:             "ExposeStarted",
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("Start to expose the services in Cluster: %s", ops.Spec.ClusterRef),
	}
}

// NewUpgradingCondition creates a condition that the OpsRequest starts to upgrade the cluster version
func NewUpgradingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeVersionUpgrading,
		Status:             metav1.ConditionTrue,
		Reason:             "VersionUpgradeStarted",
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("Start to upgrade the version in Cluster: %s", ops.Spec.ClusterRef),
	}
}

// NewStopCondition creates a condition that the OpsRequest starts to stop the cluster.
func NewStopCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeStop,
		Status:             metav1.ConditionTrue,
		Reason:             "StopStarted",
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("Start to stop the Cluster: %s", ops.Spec.ClusterRef),
	}
}

// NewStartCondition creates a condition that the OpsRequest starts the cluster.
func NewStartCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeStart,
		Status:             metav1.ConditionTrue,
		Reason:             "StartCluster",
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("Start the Cluster: %s", ops.Spec.ClusterRef),
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
			ops.Spec.ClusterRef,
			ops.Spec.Reconfigure.ComponentName),
	}
}

// NewReconfigureRunningCondition creates a condition that the OpsRequest reconfigure workflow
func NewReconfigureRunningCondition(ops *OpsRequest, conditionType string, configSpecName string, info ...string) *metav1.Condition {
	status := metav1.ConditionTrue
	if conditionType == ReasonReconfigureFailed {
		status = metav1.ConditionFalse
	}
	message := fmt.Sprintf("Reconfiguring in Cluster: %s, Component: %s, ConfigSpec: %s",
		ops.Spec.ClusterRef,
		ops.Spec.Reconfigure.ComponentName,
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

// NewReconfigureFailedCondition creates a condition for the failed reconfigure.
func NewReconfigureFailedCondition(ops *OpsRequest, err error) *metav1.Condition {
	var msg string
	if err != nil {
		msg = err.Error()
	} else {
		msg = fmt.Sprintf("Failed to reconfigure: %s in cluster: %s", ops.Name, ops.Spec.ClusterRef)
	}
	return &metav1.Condition{
		Type:               ReasonReconfigureFailed,
		Status:             metav1.ConditionFalse,
		Reason:             "ReconfigureFailed",
		LastTransitionTime: metav1.Now(),
		Message:            msg,
	}
}
