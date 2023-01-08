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

package v1alpha1

import (
	"fmt"
	"time"

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
	ConditionTypeUpgrading         = "Upgrading"

	// condition and event reasons

	ReasonReconfigureMerged  = "ReconfigureMerged"
	ReasonReconfigureFailed  = "ReconfigureFailed"
	ReasonReconfigureSucceed = "ReconfigureSucceed"
	ReasonReconfigureRunning = "ReconfigureRunning"
	ReasonClusterPhaseMisMatch = "ClusterPhaseMisMatch"
	ReasonOpsTypeNotSupported  = "OpsTypeNotSupported"
	ReasonValidateError        = "ValidateError"
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
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message: fmt.Sprintf("Start to process the OpsRequest: %s in Cluster: %s",
			ops.Name, ops.Spec.ClusterRef),
	}
}

// NewValidatePassedCondition news a condition that the operation validate passed
func NewValidatePassedCondition(opsRequestName string) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeValidated,
		Status:             metav1.ConditionTrue,
		Reason:             "ValidateOpsRequestPassed",
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message:            fmt.Sprintf("OpsRequest: %s is validated", opsRequestName),
	}
}

// NewValidateFailedCondition news a condition that the operation validate passed
func NewValidateFailedCondition(reason, message string) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeValidated,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message:            message,
	}
}

// NewFailedCondition news a condition that the OpsRequest processing failed
func NewFailedCondition(ops *OpsRequest, err error) *metav1.Condition {
	msg := fmt.Sprintf("Failed to process OpsRequest: %s in cluster: %s", ops.Name, ops.Spec.ClusterRef)
	if err != nil {
		msg = err.Error()
	}
	return &metav1.Condition{
		Type:               ConditionTypeFailed,
		Status:             metav1.ConditionFalse,
		Reason:             ReasonOpsRequestFailed,
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message:            msg,
	}
}

// NewSucceedCondition news a condition that the controller has successfully processed the OpsRequest
func NewSucceedCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeSucceed,
		Status:             metav1.ConditionTrue,
		Reason:             "OpsRequestProcessedSuccessfully",
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message: fmt.Sprintf("Successfully processed the OpsRequest: %s in Cluster: %s",
			ops.Name, ops.Spec.ClusterRef),
	}
}

// NewRestartingCondition news a condition that the operation start to restart components
func NewRestartingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeRestarting,
		Status:             metav1.ConditionTrue,
		Reason:             "RestartingStarted",
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message:            fmt.Sprintf("Start to restart database in Cluster: %s", ops.Spec.ClusterRef),
	}
}

// NewVerticalScalingCondition news a condition that the OpsRequest start to vertical scaling cluster
func NewVerticalScalingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeVerticalScaling,
		Status:             metav1.ConditionTrue,
		Reason:             "VerticalScalingStarted",
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message:            fmt.Sprintf("Start to vertical scale in Cluster: %s", ops.Spec.ClusterRef),
	}
}

// NewHorizontalScalingCondition news a condition that the OpsRequest start horizontal scaling cluster
func NewHorizontalScalingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeHorizontalScaling,
		Status:             metav1.ConditionTrue,
		Reason:             "HorizontalScalingStarted",
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message:            fmt.Sprintf("Start to horizontal scale in Cluster: %s", ops.Spec.ClusterRef),
	}
}

// NewVolumeExpandingCondition news a condition that the OpsRequest start to expand volume
func NewVolumeExpandingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeVolumeExpanding,
		Status:             metav1.ConditionTrue,
		Reason:             "VolumeExpandingStarted",
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message:            fmt.Sprintf("Start to expand the volume in Cluster: %s", ops.Spec.ClusterRef),
	}
}

// NewUpgradingCondition news a condition that the OpsRequest start to upgrade cluster
func NewUpgradingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeUpgrading,
		Status:             metav1.ConditionTrue,
		Reason:             "UpgradingStarted",
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message:            fmt.Sprintf("Start to upgrade in Cluster: %s", ops.Spec.ClusterRef),
	}
}

// NewReconfigureCondition new a condition that the OpsRequest updating component configuration
func NewReconfigureCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeReconfigure,
		Status:             metav1.ConditionTrue,
		Reason:             "ReconfigureStarted",
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message:            fmt.Sprintf("Start to reconfigure in Cluster: %s", ops.Spec.ClusterRef),
	}
}

// NewReconfigureRunningCondition new a condition that the OpsRequest reconfigure workflow
func NewReconfigureRunningCondition(ops *OpsRequest, reason string) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeReconfigure,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message:            fmt.Sprintf("Reconfiguring in Cluster: %s", ops.Spec.ClusterRef),
	}
}
