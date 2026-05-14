/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"github.com/sethvargo/go-password/password"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestNewAllCondition(t *testing.T) {
	opsRequest := createTestOpsRequest("mysql-test", "mysql-restart", RestartType)
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
	NewBackupCondition(opsRequest)

	opsRequest.Spec.Reconfigures = []Reconfigure{
		{

			ComponentOps: ComponentOps{
				ComponentName: "testReconfiguring",
			},
		},
	}
	NewReconfigureCondition(opsRequest)
	NewReconfigureRunningCondition(opsRequest, appsv1alpha1.ReasonReconfigureRunning, "for_test", "")
}

// TestReasonConstantsStringDrift pins the literal value of every Reason
// constant emitted by the NewXxxCondition constructors. Console, Slock task
// templates, fixtures, and the upcoming OPS-S1 reasoncatalog registry all
// match on the string value, not the Go identifier, so a silent rename
// would break consumers without a compile-time signal.
//
// Slice A intentionally only centralizes existing inline literals and does
// NOT introduce the 9 OPS-S1 anchor reasons. Anchors land in Slice B with
// their own catalog and tests.
func TestReasonConstantsStringDrift(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		// pre-existing constants — pinned for completeness
		{"ReasonClusterPhaseMismatch", ReasonClusterPhaseMismatch, "ClusterPhaseMismatch"},
		{"ReasonOpsTypeNotSupported", ReasonOpsTypeNotSupported, "OpsTypeNotSupported"},
		{"ReasonValidateFailed", ReasonValidateFailed, "ValidateFailed"},
		{"ReasonClusterNotFound", ReasonClusterNotFound, "ClusterNotFound"},
		{"ReasonOpsRequestFailed", ReasonOpsRequestFailed, "OpsRequestFailed"},
		{"ReasonOpsCanceling", ReasonOpsCanceling, "Canceling"},
		{"ReasonOpsCancelFailed", ReasonOpsCancelFailed, "CancelFailed"},
		{"ReasonOpsCancelSucceed", ReasonOpsCancelSucceed, "CancelSucceed"},
		{"ReasonOpsCancelByController", ReasonOpsCancelByController, "CancelByController"},

		// constants centralized in Slice A — values must equal the
		// previously inline string literal, no behavior change allowed.
		{"ReasonValidateOpsRequestPassed", ReasonValidateOpsRequestPassed, "ValidateOpsRequestPassed"},
		{"ReasonOpsRequestProcessedSuccessfully", ReasonOpsRequestProcessedSuccessfully, "OpsRequestProcessedSuccessfully"},
		{"ReasonRestartStarted", ReasonRestartStarted, "RestartStarted"},
		{"ReasonStartToRebuildInstances", ReasonStartToRebuildInstances, "StartToRebuildInstances"},
		{"ReasonSwitchoverStarted", ReasonSwitchoverStarted, "SwitchoverStarted"},
		{"ReasonVerticalScalingStarted", ReasonVerticalScalingStarted, "VerticalScalingStarted"},
		{"ReasonHorizontalScalingStarted", ReasonHorizontalScalingStarted, "HorizontalScalingStarted"},
		{"ReasonVolumeExpansionStarted", ReasonVolumeExpansionStarted, "VolumeExpansionStarted"},
		{"ReasonExposeStarted", ReasonExposeStarted, "ExposeStarted"},
		{"ReasonVersionUpgradeStarted", ReasonVersionUpgradeStarted, "VersionUpgradeStarted"},
		{"ReasonStopStarted", ReasonStopStarted, "StopStarted"},
		{"ReasonStartCluster", ReasonStartCluster, "StartCluster"},
		{"ReasonReconfigureStarted", ReasonReconfigureStarted, "ReconfigureStarted"},
		{"ReasonReconfigureFailed", ReasonReconfigureFailed, "ReconfigureFailed"},
		{"ReasonBackupStarted", ReasonBackupStarted, "BackupStarted"},
		{"ReasonRestoreStarted", ReasonRestoreStarted, "RestoreStarted"},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("Reason constant %s drifted: got %q, want %q (consumer match would break)", tc.name, tc.got, tc.want)
		}
	}
}

// TestNewConditionReasonsMatchConstants verifies that the constructors
// in opsrequest_conditions.go set Condition.Reason to the centralized
// constants we just defined. Defends against an accidental in-place
// edit that re-introduces an inline string and silently drifts from the
// constant.
func TestNewConditionReasonsMatchConstants(t *testing.T) {
	opsRequest := createTestOpsRequest("mysql-test", "mysql-restart", RestartType)
	opsRequest.Spec.Reconfigures = []Reconfigure{{ComponentOps: ComponentOps{ComponentName: "testReconfiguring"}}}

	cases := []struct {
		name string
		got  string
		want string
	}{
		{"NewValidatePassedCondition", NewValidatePassedCondition(opsRequest.Name).Reason, ReasonValidateOpsRequestPassed},
		{"NewSucceedCondition", NewSucceedCondition(opsRequest).Reason, ReasonOpsRequestProcessedSuccessfully},
		{"NewRestartingCondition", NewRestartingCondition(opsRequest).Reason, ReasonRestartStarted},
		{"NewInstancesRebuildingCondition", NewInstancesRebuildingCondition(opsRequest).Reason, ReasonStartToRebuildInstances},
		{"NewSwitchoveringCondition", NewSwitchoveringCondition(0, "test").Reason, ReasonSwitchoverStarted},
		{"NewVerticalScalingCondition", NewVerticalScalingCondition(opsRequest).Reason, ReasonVerticalScalingStarted},
		{"NewHorizontalScalingCondition", NewHorizontalScalingCondition(opsRequest).Reason, ReasonHorizontalScalingStarted},
		{"NewVolumeExpandingCondition", NewVolumeExpandingCondition(opsRequest).Reason, ReasonVolumeExpansionStarted},
		{"NewExposingCondition", NewExposingCondition(opsRequest).Reason, ReasonExposeStarted},
		{"NewUpgradingCondition", NewUpgradingCondition(opsRequest).Reason, ReasonVersionUpgradeStarted},
		{"NewStopCondition", NewStopCondition(opsRequest).Reason, ReasonStopStarted},
		{"NewStartCondition", NewStartCondition(opsRequest).Reason, ReasonStartCluster},
		{"NewReconfigureCondition", NewReconfigureCondition(opsRequest).Reason, ReasonReconfigureStarted},
		{"NewReconfigureFailedCondition", NewReconfigureFailedCondition(opsRequest, nil).Reason, ReasonReconfigureFailed},
		{"NewBackupCondition", NewBackupCondition(opsRequest).Reason, ReasonBackupStarted},
		{"NewRestoreCondition", NewRestoreCondition(opsRequest).Reason, ReasonRestoreStarted},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s emitted Reason %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

func TestSetStatusCondition(t *testing.T) {
	opsRequest := createTestOpsRequest("mysql-test", "mysql-restart", RestartType)
	progressingCondition := NewVerticalScalingCondition(opsRequest)
	opsRequest.SetStatusCondition(*progressingCondition)
	checkCondition := meta.FindStatusCondition(opsRequest.Status.Conditions, progressingCondition.Type)
	if checkCondition == nil {
		t.Errorf(`Condition: %s should exist in OpsRequest.status.conditions`, progressingCondition.Type)
	}
}

func createTestOpsRequest(clusterName, opsRequestName string, opsType OpsType) *OpsRequest {
	randomStr, _ := password.Generate(6, 0, 0, true, false)
	return &OpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opsRequestName + randomStr,
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/instance":    clusterName,
				constant.OpsRequestTypeLabelKey: string(opsType),
			},
		},
		Spec: OpsRequestSpec{
			ClusterName: clusterName,
			Type:        opsType,
		},
	}
}
