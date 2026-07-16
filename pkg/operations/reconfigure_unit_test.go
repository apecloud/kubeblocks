/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package operations

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestSnapshotParameterAssignmentsPreservesPresenceAndNil(t *testing.T) {
	oldValue := "old"
	compParam := &parametersv1alpha1.ComponentParameter{
		Spec: parametersv1alpha1.ComponentParameterSpec{
			Desired: &parametersv1alpha1.ParameterInputs{
				Assignments: map[string]*string{
					"present-value": &oldValue,
				},
				Updates: []parametersv1alpha1.ParameterUpdate{{
					Type: parametersv1alpha1.ParameterUpdateRemove,
					Key:  "present-nil",
				}},
			},
		},
	}
	parameters := []opsv1alpha1.ParameterPair{
		{Key: "present-value", Value: ptr.To("new")},
		{Key: "absent", Value: ptr.To("new")},
		{Key: "present-nil", Value: ptr.To("new")},
	}

	got := snapshotParameterAssignments(compParam, parameters)

	require.Equal(t, []opsv1alpha1.LastParameterAssignment{
		{Key: "absent", Present: false},
		{Key: "present-nil", Present: true},
		{Key: "present-value", Present: true, Value: ptr.To("old")},
	}, got)
	oldValue = "mutated"
	require.Equal(t, "old", *got[2].Value, "the persisted snapshot must not alias the source assignment")
}

func TestValidateReconfigureParameters(t *testing.T) {
	tests := []struct {
		name       string
		parameters []opsv1alpha1.ParameterPair
		wantErr    string
	}{
		{
			name: "valid set and remove",
			parameters: []opsv1alpha1.ParameterPair{
				{Key: "set", Value: ptr.To("value")},
				{Key: "remove"},
			},
		},
		{
			name:       "empty key",
			parameters: []opsv1alpha1.ParameterPair{{Value: ptr.To("value")}},
			wantErr:    "must not be empty",
		},
		{
			name: "duplicate key",
			parameters: []opsv1alpha1.ParameterPair{
				{Key: "same", Value: ptr.To("first")},
				{Key: "same", Value: ptr.To("second")},
			},
			wantErr: "duplicated",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateReconfigureParameters(test.parameters)
			if test.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

func TestValidateComponentParameterInputs(t *testing.T) {
	valid := func() *parametersv1alpha1.ComponentParameter {
		return &parametersv1alpha1.ComponentParameter{Spec: parametersv1alpha1.ComponentParameterSpec{
			Desired: &parametersv1alpha1.ParameterInputs{
				Assignments: map[string]*string{"assignment": ptr.To("value")},
				Updates: []parametersv1alpha1.ParameterUpdate{
					{Type: parametersv1alpha1.ParameterUpdateSet, Key: "set", Value: ptr.To("value")},
					{Type: parametersv1alpha1.ParameterUpdateRemove, Key: "remove"},
				},
			},
		}}
	}
	require.NoError(t, validateComponentParameterInputs(valid()))

	tests := []struct {
		name    string
		mutate  func(*parametersv1alpha1.ComponentParameter)
		wantErr string
	}{
		{
			name: "empty assignment key",
			mutate: func(compParam *parametersv1alpha1.ComponentParameter) {
				compParam.Spec.Desired.Assignments[""] = ptr.To("value")
			},
			wantErr: "assignment key must not be empty",
		},
		{
			name: "empty update key",
			mutate: func(compParam *parametersv1alpha1.ComponentParameter) {
				compParam.Spec.Desired.Updates[0].Key = ""
			},
			wantErr: "key must not be empty",
		},
		{
			name: "set without value",
			mutate: func(compParam *parametersv1alpha1.ComponentParameter) {
				compParam.Spec.Desired.Updates[0].Value = nil
			},
			wantErr: "Set without a value",
		},
		{
			name: "unsupported update type",
			mutate: func(compParam *parametersv1alpha1.ComponentParameter) {
				compParam.Spec.Desired.Updates[0].Type = parametersv1alpha1.ParameterUpdateType("Unknown")
			},
			wantErr: "unsupported type",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			compParam := valid()
			test.mutate(compParam)
			require.ErrorContains(t, validateComponentParameterInputs(compParam), test.wantErr)
		})
	}
}

func TestClassifyDeterministicReconfigureFailure(t *testing.T) {
	normalizedFailure := func(code appsv1.ActionResultCode, retryable *bool) *parametersv1alpha1.ComponentParameter {
		return &parametersv1alpha1.ComponentParameter{
			ObjectMeta: metav1.ObjectMeta{Generation: 7},
			Status: parametersv1alpha1.ComponentParameterStatus{
				Phase: parametersv1alpha1.CMergeFailedPhase,
				ConfigurationItemStatus: []parametersv1alpha1.ConfigTemplateItemDetailStatus{{
					Name:           "mysql-config",
					Phase:          parametersv1alpha1.CMergeFailedPhase,
					UpdateRevision: "7",
					ReconcileDetail: &parametersv1alpha1.ReconcileDetail{
						CurrentRevision: "7",
						Code:            code,
						Retryable:       retryable,
					},
				}},
			},
		}
	}

	code, retryable, ok := classifyDeterministicReconfigureFailure([]*parametersv1alpha1.ComponentParameter{
		normalizedFailure(invalidParameterActionResultCode, ptr.To(false)),
	})
	require.True(t, ok)
	require.Equal(t, invalidParameterActionResultCode, code)
	require.Equal(t, ptr.To(false), retryable)

	for _, failed := range []*parametersv1alpha1.ComponentParameter{
		normalizedFailure("", nil),
		normalizedFailure(invalidParameterActionResultCode, ptr.To(true)),
		normalizedFailure("OtherFailure", ptr.To(false)),
		{Status: parametersv1alpha1.ComponentParameterStatus{Phase: parametersv1alpha1.CMergeFailedPhase}},
	} {
		_, _, ok = classifyDeterministicReconfigureFailure([]*parametersv1alpha1.ComponentParameter{failed})
		require.False(t, ok)
	}

	for name, mutate := range map[string]func(*parametersv1alpha1.ComponentParameter){
		"stale item revision": func(compParam *parametersv1alpha1.ComponentParameter) {
			compParam.Status.ConfigurationItemStatus[0].UpdateRevision = "6"
		},
		"stale result revision": func(compParam *parametersv1alpha1.ComponentParameter) {
			compParam.Status.ConfigurationItemStatus[0].ReconcileDetail.CurrentRevision = "6"
		},
	} {
		t.Run(name, func(t *testing.T) {
			failed := normalizedFailure(invalidParameterActionResultCode, ptr.To(false))
			mutate(failed)
			_, _, ok := classifyDeterministicReconfigureFailure([]*parametersv1alpha1.ComponentParameter{failed})
			require.False(t, ok)
		})
	}
}

func TestRollbackAssignmentGuardsAndRestore(t *testing.T) {
	current := map[string]*string{
		"remove": nil,
		"set":    ptr.To("new"),
	}
	requested := map[string]*string{
		"remove": nil,
		"set":    ptr.To("new"),
	}
	snapshot := []opsv1alpha1.LastParameterAssignment{
		{Key: "remove", Present: true, Value: ptr.To("old-remove")},
		{Key: "set", Present: false},
	}
	require.True(t, assignmentsMatchRequested(current, requested))
	require.False(t, assignmentsMatchSnapshot(current, snapshot))

	compParam := &parametersv1alpha1.ComponentParameter{
		Spec: parametersv1alpha1.ComponentParameterSpec{
			Desired: &parametersv1alpha1.ParameterInputs{Assignments: current},
		},
	}
	restoreParameterAssignments(compParam, snapshot)

	require.True(t, assignmentsMatchSnapshot(componentParameterAssignments(compParam), snapshot))
	require.Equal(t, "old-remove", *compParam.Spec.Desired.Assignments["remove"])
	require.NotContains(t, compParam.Spec.Desired.Assignments, "set")
	require.False(t, assignmentsMatchRequested(componentParameterAssignments(compParam), requested))
}

func TestReplaceParameterAssignmentsPersistsRemovalAndClearsOverrides(t *testing.T) {
	compParam := &parametersv1alpha1.ComponentParameter{Spec: parametersv1alpha1.ComponentParameterSpec{
		Desired: &parametersv1alpha1.ParameterInputs{
			Assignments: map[string]*string{
				"remove": ptr.To("old"),
				"set":    ptr.To("stale"),
				"keep":   ptr.To("unchanged"),
			},
			Updates: []parametersv1alpha1.ParameterUpdate{
				{Type: parametersv1alpha1.ParameterUpdateSet, Key: "remove", Value: ptr.To("override")},
				{Type: parametersv1alpha1.ParameterUpdateRemove, Key: "set"},
				{Type: parametersv1alpha1.ParameterUpdateSet, Key: "keep-update", Value: ptr.To("unchanged")},
			},
		},
	}}

	replaceParameterAssignments(compParam, map[string]parameterAssignmentState{
		"remove": {present: true},
		"set":    {present: true, value: ptr.To("new")},
	})

	require.NotContains(t, compParam.Spec.Desired.Assignments, "remove")
	require.Equal(t, ptr.To("new"), compParam.Spec.Desired.Assignments["set"])
	require.Equal(t, ptr.To("unchanged"), compParam.Spec.Desired.Assignments["keep"])
	require.Equal(t, []parametersv1alpha1.ParameterUpdate{
		{Type: parametersv1alpha1.ParameterUpdateSet, Key: "keep-update", Value: ptr.To("unchanged")},
		{Type: parametersv1alpha1.ParameterUpdateRemove, Key: "remove"},
	}, compParam.Spec.Desired.Updates)
	require.True(t, assignmentsMatchRequested(componentParameterAssignments(compParam), map[string]*string{
		"remove": nil,
		"set":    ptr.To("new"),
	}))
}

func TestRollbackRestartRequiredIsConservativeAfterAnyDispatch(t *testing.T) {
	target := func(expected, succeed int32) reconfigureTarget {
		return reconfigureTarget{parameter: &parametersv1alpha1.ComponentParameter{
			Status: parametersv1alpha1.ComponentParameterStatus{
				Phase: parametersv1alpha1.CMergeFailedPhase,
				ConfigurationItemStatus: []parametersv1alpha1.ConfigTemplateItemDetailStatus{{
					Phase: parametersv1alpha1.CMergeFailedPhase,
					ReconcileDetail: &parametersv1alpha1.ReconcileDetail{
						ExpectedCount: expected,
						SucceedCount:  succeed,
					},
				}},
			},
		}}
	}

	require.True(t, rollbackRestartRequired([]reconfigureTarget{target(1, 0)}),
		"a failed script can partially apply H1 before returning a zero-success result")
	require.True(t, rollbackRestartRequired([]reconfigureTarget{target(1, 1)}))
	require.True(t, rollbackRestartRequired([]reconfigureTarget{target(0, 0)}),
		"a zero expected count cannot prove that no instance applied H1")
	unknown := target(1, -1)
	require.True(t, rollbackRestartRequired([]reconfigureTarget{unknown}))
	nonterminal := target(1, 0)
	nonterminal.parameter.Status.ConfigurationItemStatus[0].Phase = parametersv1alpha1.CRunningPhase
	require.True(t, rollbackRestartRequired([]reconfigureTarget{nonterminal}),
		"a nonterminal item cannot prove that no instance applied H1")
	require.False(t, rollbackRestartRequired(nil))
}

func TestPreflightReconfigureTargets(t *testing.T) {
	h0 := ptr.To("100")
	h1 := ptr.To("200")
	newTarget := func(current *string, owner string) reconfigureTarget {
		return reconfigureTarget{
			component: "mysql",
			reconfigure: opsv1alpha1.Reconfigure{Parameters: []opsv1alpha1.ParameterPair{{
				Key: "max_connections", Value: h1,
			}}},
			parameter: &parametersv1alpha1.ComponentParameter{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
					constant.OpsRequestUIDAnnotationKey: owner,
				}},
				Spec: parametersv1alpha1.ComponentParameterSpec{Desired: &parametersv1alpha1.ParameterInputs{
					Assignments: map[string]*string{"max_connections": current},
				}},
			},
		}
	}
	opsRequest := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{UID: types.UID("ops-uid")}}
	opsRequest.Status.LastConfiguration.Components = map[string]opsv1alpha1.LastComponentConfiguration{
		"mysql": {Parameters: []opsv1alpha1.LastParameterAssignment{{
			Key: "max_connections", Present: true, Value: h0,
		}}},
	}

	require.NoError(t, preflightReconfigureTargets(opsRequest, []reconfigureTarget{newTarget(h0, "old-ops")}),
		"the initial H0 state is eligible even when an earlier OpsRequest left its ownership marker")
	require.NoError(t, preflightReconfigureTargets(opsRequest, []reconfigureTarget{newTarget(h1, "ops-uid")}),
		"an exact own-H1 state is an idempotent response-loss replay")
	require.Error(t, preflightReconfigureTargets(opsRequest, []reconfigureTarget{newTarget(h1, "foreign-ops")}),
		"an H1 state owned by another request must fail closed")
	require.Error(t, preflightReconfigureTargets(opsRequest, []reconfigureTarget{newTarget(ptr.To("300"), "ops-uid")}),
		"a value outside H0 and H1 must fail before any target is written")

	legacy := opsRequest.DeepCopy()
	legacy.Status.LastConfiguration.Components = map[string]opsv1alpha1.LastComponentConfiguration{
		"mysql": {Replicas: ptr.To(int32(3))},
	}
	require.NoError(t, preflightReconfigureTargets(legacy, []reconfigureTarget{newTarget(ptr.To("300"), "")}),
		"pre-snapshot in-flight OpsRequests retain their legacy apply behavior even when older fields are present")
}

func TestRollbackSnapshotAndOwnershipGatesAreIndependent(t *testing.T) {
	opsRequest := &opsv1alpha1.OpsRequest{}
	opsRequest.UID = types.UID("ops-uid")
	opsRequest.Status.LastConfiguration.Components = map[string]opsv1alpha1.LastComponentConfiguration{
		"mysql": {
			Parameters: []opsv1alpha1.LastParameterAssignment{{Key: "max_connections", Present: false}},
		},
	}
	targets := []reconfigureTarget{{
		component: "mysql",
		reconfigure: opsv1alpha1.Reconfigure{
			Parameters: []opsv1alpha1.ParameterPair{{Key: "max_connections", Value: ptr.To("200")}},
		},
		parameter: &parametersv1alpha1.ComponentParameter{},
	}}

	require.True(t, rollbackSnapshotsAvailable(opsRequest, targets))
	require.False(t, rollbackOwnershipCurrent(opsRequest, targets),
		"a new-format snapshot must not turn an overwritten ownership marker into a legacy request")

	targets[0].parameter.Annotations = map[string]string{
		constant.OpsRequestUIDAnnotationKey: string(opsRequest.UID),
	}
	require.True(t, rollbackOwnershipCurrent(opsRequest, targets))

	delete(opsRequest.Status.LastConfiguration.Components, "mysql")
	require.False(t, rollbackSnapshotsAvailable(opsRequest, targets),
		"only requests without the persisted snapshot may use the legacy failure path")
}

func TestReconfigureRollbackRequeueAfterOwnsTimeoutBoundary(t *testing.T) {
	timeoutSeconds := int32(10)
	now := metav1.Now()
	opsRequest := &opsv1alpha1.OpsRequest{
		Spec: opsv1alpha1.OpsRequestSpec{TimeoutSeconds: &timeoutSeconds},
		Status: opsv1alpha1.OpsRequestStatus{
			StartTimestamp:      metav1.Now(),
			ReconfigureRollback: &opsv1alpha1.ReconfigureRollbackStatus{StartTime: &now},
		},
	}

	requeueAfter := reconfigureRollbackRequeueAfter(opsRequest)
	require.Greater(t, requeueAfter, time.Duration(0))
	require.LessOrEqual(t, requeueAfter, 10*time.Second)

	opsRequest.Status.StartTimestamp = metav1.NewTime(time.Now().Add(-11 * time.Second))
	require.Equal(t, time.Millisecond, reconfigureRollbackRequeueAfter(opsRequest),
		"a crossed boundary must schedule the rollback-owned timeout transition")

	opsRequest.Spec.TimeoutSeconds = nil
	opsRequest.Status.ReconfigureRollback.StartTime = &now
	requeueAfter = reconfigureRollbackRequeueAfter(opsRequest)
	require.Greater(t, requeueAfter, time.Duration(0))
	require.LessOrEqual(t, requeueAfter, defaultReconfigureRollbackTimeout)
	require.False(t, reconfigureRollbackTimedOut(opsRequest))

	old := metav1.NewTime(time.Now().Add(-defaultReconfigureRollbackTimeout - time.Second))
	opsRequest.Status.ReconfigureRollback.StartTime = &old
	require.True(t, reconfigureRollbackTimedOut(opsRequest),
		"automatic rollback must remain bounded when OpsRequest timeoutSeconds is omitted")
	require.Equal(t, time.Millisecond, reconfigureRollbackRequeueAfter(opsRequest))

	opsRequest.Status.ReconfigureRollback = nil
	require.Equal(t, noRequeueAfter, reconfigureRollbackRequeueAfter(opsRequest))
}
