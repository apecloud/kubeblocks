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
	ConditionTypeRestarting        = "Restarting"
	ConditionTypeVerticalScaling   = "VerticalScaling"
	ConditionTypeHorizontalScaling = "HorizontalScaling"
	ConditionTypeVolumeExpanding   = "VolumeExpanding"
	ConditionTypeUpgrading         = "Upgrading"

	// condition and event reasons

	ReasonClusterPhaseMisMatch         = "ClusterPhaseMisMatch"
	ReasonOpsTypeNotSupported          = "OpsTypeNotSupported"
	ReasonClusterExistOtherOperation   = "ClusterExistOtherOperation"
	ReasonClusterNotFound              = "ClusterNotFound"
	ReasonVolumeExpansionValidateError = "VolumeExpansionValidateError"
	ReasonStarting                     = "Starting"
	ReasonSuccessful                   = "Successful"
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
		Message: fmt.Sprintf("Controller has started to progress the OpsRequest: %s in Cluster: %s",
			ops.Name, ops.Spec.ClusterRef),
	}
}

// NewValidatePassedCondition new a condition that the operation validate passed
func NewValidatePassedCondition(opsRequestName string) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeValidated,
		Status:             metav1.ConditionTrue,
		Reason:             "ValidateOpsRequestPassed",
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message:            fmt.Sprintf("OpsRequest: %s is validated", opsRequestName),
	}
}

// NewValidateFailedCondition new a condition that the operation validate passed
func NewValidateFailedCondition(reason, message string) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeValidated,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message:            message,
	}
}

// NewSucceedCondition new a condition that the controller has successfully processed the OpsRequest
func NewSucceedCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeSucceed,
		Status:             metav1.ConditionTrue,
		Reason:             "OpsRequestProcessedSuccessfully",
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message: fmt.Sprintf("Controller has successfully processed the OpsRequest: %s in Cluster: %s",
			ops.Name, ops.Spec.ClusterRef),
	}
}

// NewRestartingCondition new a condition that the operation start to restart components
func NewRestartingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeRestarting,
		Status:             metav1.ConditionTrue,
		Reason:             "RestartingStarted",
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message:            fmt.Sprintf("start restarting database in Cluster: %s", ops.Spec.ClusterRef),
	}
}

// NewVerticalScalingCondition new a condition that the OpsRequest start to vertical scaling cluster
func NewVerticalScalingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeVerticalScaling,
		Status:             metav1.ConditionTrue,
		Reason:             "VerticalScalingStarted",
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message:            fmt.Sprintf("start vertical scaling in Cluster: %s", ops.Spec.ClusterRef),
	}
}

// NewHorizontalScalingCondition new a condition that the OpsRequest start horizontal scaling cluster
func NewHorizontalScalingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeHorizontalScaling,
		Status:             metav1.ConditionTrue,
		Reason:             "HorizontalScalingStarted",
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message:            fmt.Sprintf("start horizontal scaling in Cluster: %s", ops.Spec.ClusterRef),
	}
}

// NewVolumeExpandingCondition new a condition that the OpsRequest start to expand volume
func NewVolumeExpandingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeVolumeExpanding,
		Status:             metav1.ConditionTrue,
		Reason:             "VolumeExpandingStarted",
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message:            fmt.Sprintf("start expanding the volume in Cluster: %s", ops.Spec.ClusterRef),
	}
}

// NewUpgradingCondition new a condition that the OpsRequest start to upgrade cluster
func NewUpgradingCondition(ops *OpsRequest) *metav1.Condition {
	return &metav1.Condition{
		Type:               ConditionTypeUpgrading,
		Status:             metav1.ConditionTrue,
		Reason:             "UpgradingStarted",
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message:            fmt.Sprintf("start upgrading in Cluster: %s", ops.Spec.ClusterRef),
	}
}
