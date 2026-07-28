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

package apps

import (
	"testing"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestValidateLifecycleActionsRejectsUnsupportedNonBlockingActions(t *testing.T) {
	tests := []struct {
		name   string
		config func(*appsv1.ComponentLifecycleActions)
	}{
		{
			name: "role probe",
			config: func(actions *appsv1.ComponentLifecycleActions) {
				actions.RoleProbe = &appsv1.Probe{Action: appsv1.Action{NonBlocking: true}}
			},
		},
		{
			name: "available probe",
			config: func(actions *appsv1.ComponentLifecycleActions) {
				actions.AvailableProbe = &appsv1.Probe{Action: appsv1.Action{NonBlocking: true}}
			},
		},
		{
			name: "data dump",
			config: func(actions *appsv1.ComponentLifecycleActions) {
				actions.DataDump = &appsv1.Action{NonBlocking: true}
			},
		},
		{
			name: "data load",
			config: func(actions *appsv1.ComponentLifecycleActions) {
				actions.DataLoad = &appsv1.Action{NonBlocking: true}
			},
		},
	}

	reconciler := &ComponentDefinitionReconciler{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actions := &appsv1.ComponentLifecycleActions{}
			test.config(actions)
			cmpd := &appsv1.ComponentDefinition{}
			cmpd.Spec.LifecycleActions = actions
			if err := reconciler.validateLifecycleActions(nil, intctrlutil.RequestCtx{}, cmpd); err == nil {
				t.Fatal("expected non-blocking lifecycle Action validation error")
			}
		})
	}
}

func TestValidateLifecycleActionsAllowsOrdinaryNonBlockingActions(t *testing.T) {
	reconciler := &ComponentDefinitionReconciler{}
	cmpd := &appsv1.ComponentDefinition{}
	cmpd.Spec.LifecycleActions = &appsv1.ComponentLifecycleActions{
		PostProvision: &appsv1.Action{NonBlocking: true},
		Switchover:    &appsv1.Action{NonBlocking: true},
	}
	if err := reconciler.validateLifecycleActions(nil, intctrlutil.RequestCtx{}, cmpd); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
