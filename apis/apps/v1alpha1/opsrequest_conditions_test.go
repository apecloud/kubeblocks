/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	"testing"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
)

func TestNewAllCondition(t *testing.T) {
	opsRequest := createTestOpsRequest("mysql-test", "mysql-restart", RestartType)
	NewProgressingCondition(opsRequest)
	NewVolumeExpandingCondition(opsRequest)
	NewRestartingCondition(opsRequest)
	NewHorizontalScalingCondition(opsRequest)
	NewValidatePassedCondition(opsRequest.Name)
	NewSucceedCondition(opsRequest)
	NewVerticalScalingCondition(opsRequest)
	NewUpgradingCondition(opsRequest)
	NewValidateFailedCondition(ReasonClusterPhaseMismatch, "fail")
	NewFailedCondition(opsRequest, nil)
	NewFailedCondition(opsRequest, errors.New("opsRequest run failed"))
	NewCancelingCondition(opsRequest)
	NewCancelFailedCondition(opsRequest, nil)
	NewCancelFailedCondition(opsRequest, errors.New("cancel opsRequest failed"))
	NewCancelSucceedCondition(opsRequest.Name)
	NewExposingCondition(opsRequest)
	NewStopCondition(opsRequest)
	NewStopCondition(opsRequest)
	NewReconfigureFailedCondition(opsRequest, nil)
	NewReconfigureFailedCondition(opsRequest, errors.New("reconfigure opsRequest failed"))

	opsRequest.Spec.Reconfigure = &Reconfigure{
		ComponentOps: ComponentOps{
			ComponentName: "testReconfiguring",
		},
	}
	NewReconfigureCondition(opsRequest)
	NewReconfigureRunningCondition(opsRequest, ReasonReconfigureRunning, "for_test", "")
}

func TestSetStatusCondition(t *testing.T) {
	opsRequest := createTestOpsRequest("mysql-test", "mysql-restart", RestartType)
	progressingCondition := NewProgressingCondition(opsRequest)
	opsRequest.SetStatusCondition(*progressingCondition)
	checkCondition := meta.FindStatusCondition(opsRequest.Status.Conditions, progressingCondition.Type)
	if checkCondition == nil {
		t.Errorf(`Condition: %s should exist in OpsRequest.status.conditions`, progressingCondition.Type)
	}
}
