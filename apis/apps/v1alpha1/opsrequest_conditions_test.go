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
