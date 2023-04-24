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
)

var componentName = "mysql"

func mockRestartOps() *OpsRequest {
	ops := &OpsRequest{}
	ops.Spec.Type = RestartType
	ops.Spec.RestartList = []ComponentOps{
		{
			ComponentName: componentName,
		},
	}
	return ops
}

func TestGetRestartComponentNameMap(t *testing.T) {
	ops := mockRestartOps()
	componentNameMap := ops.Spec.GetRestartComponentNameSet()
	checkComponentMap(t, componentNameMap, len(ops.Spec.RestartList), componentName)
	componentNameMap1 := ops.GetComponentNameSet()
	checkComponentMap(t, componentNameMap1, len(ops.Spec.RestartList), componentName)
}

func mockVerticalScalingOps() *OpsRequest {
	ops := &OpsRequest{}
	ops.Spec.Type = VerticalScalingType
	ops.Spec.VerticalScalingList = []VerticalScaling{
		{
			ComponentOps: ComponentOps{
				ComponentName: componentName,
			},
		},
	}
	return ops
}

func TestVerticalScalingComponentNameMap(t *testing.T) {
	ops := mockVerticalScalingOps()
	componentNameMap := ops.Spec.GetVerticalScalingComponentNameSet()
	checkComponentMap(t, componentNameMap, len(ops.Spec.VerticalScalingList), componentName)
	componentNameMap1 := ops.GetComponentNameSet()
	checkComponentMap(t, componentNameMap1, len(ops.Spec.VerticalScalingList), componentName)
}

func mockHorizontalScalingOps() *OpsRequest {
	ops := &OpsRequest{}
	ops.Spec.Type = HorizontalScalingType
	ops.Spec.HorizontalScalingList = []HorizontalScaling{
		{
			ComponentOps: ComponentOps{
				ComponentName: componentName,
			},
		},
	}
	return ops
}

func TestHorizontalScalingComponentNameMap(t *testing.T) {
	ops := mockHorizontalScalingOps()
	componentNameMap := ops.Spec.GetHorizontalScalingComponentNameSet()
	checkComponentMap(t, componentNameMap, len(ops.Spec.HorizontalScalingList), componentName)
	componentNameMap1 := ops.GetComponentNameSet()
	checkComponentMap(t, componentNameMap1, len(ops.Spec.HorizontalScalingList), componentName)
}

func mockVolumeExpansionOps() *OpsRequest {
	ops := &OpsRequest{}
	ops.Spec.Type = VolumeExpansionType
	ops.Spec.VolumeExpansionList = []VolumeExpansion{
		{
			ComponentOps: ComponentOps{
				ComponentName: componentName,
			},
		},
	}
	return ops
}

func TestVolumeExpansionComponentNameMap(t *testing.T) {
	ops := mockVolumeExpansionOps()
	componentNameMap := ops.Spec.GetVolumeExpansionComponentNameSet()
	checkComponentMap(t, componentNameMap, len(ops.Spec.VolumeExpansionList), componentName)
	componentNameMap1 := ops.GetComponentNameSet()
	checkComponentMap(t, componentNameMap1, len(ops.Spec.VolumeExpansionList), componentName)
}

func checkComponentMap(t *testing.T, componentNameMap map[string]struct{}, expectLen int, expectName string) {
	if len(componentNameMap) != expectLen {
		t.Error(`Expected component name map length equals list length`)
	}
	if _, ok := componentNameMap[expectName]; !ok {
		t.Error(`Expected component name map exists the key of "mysql"`)
	}
}

func TestToVerticalScalingListToMap(t *testing.T) {
	ops := mockVerticalScalingOps()
	verticalScalingMap := ops.Spec.ToVerticalScalingListToMap()
	if len(verticalScalingMap) != len(ops.Spec.VerticalScalingList) {
		t.Error(`Expected vertical scaling map length equals list length`)
	}
	if _, ok := verticalScalingMap[componentName]; !ok {
		t.Error(`Expected component name map exists the key of "mysql"`)
	}
}

func TestConvertVolumeExpansionListToMap(t *testing.T) {
	ops := mockVolumeExpansionOps()
	volumeExpansionMap := ops.Spec.ToVolumeExpansionListToMap()
	if len(volumeExpansionMap) != len(ops.Spec.VolumeExpansionList) {
		t.Error(`Expected volume expansion map length equals list length`)
	}
	if _, ok := volumeExpansionMap[componentName]; !ok {
		t.Error(`Expected component name map exists the key of "mysql"`)
	}
}

func TestToHorizontalScalingListToMap(t *testing.T) {
	ops := mockHorizontalScalingOps()
	horizontalScalingMap := ops.Spec.ToHorizontalScalingListToMap()
	if len(horizontalScalingMap) != len(ops.Spec.HorizontalScalingList) {
		t.Error(`Expected horizontal scaling map length equals list length`)
	}
	if _, ok := horizontalScalingMap[componentName]; !ok {
		t.Error(`Expected component name map exists the key of "mysql"`)
	}
}

func TestGetUpgradeComponentNameMap(t *testing.T) {
	ops := &OpsRequest{}
	ops.Spec.Type = UpgradeType
	componentNameMap := ops.GetUpgradeComponentNameSet()
	if componentNameMap != nil {
		t.Error(`Expected component name map of upgrade ops is nil`)
	}
	ops.Spec.Upgrade = &Upgrade{
		ClusterVersionRef: "test-version",
	}
	ops.Status.Components = map[string]OpsRequestComponentStatus{
		componentName: {},
	}

	componentNameMap = ops.GetUpgradeComponentNameSet()
	checkComponentMap(t, componentNameMap, len(ops.Status.Components), componentName)
	componentNameMap1 := ops.GetComponentNameSet()
	checkComponentMap(t, componentNameMap1, len(ops.Status.Components), componentName)
}

func TestSetStatusAndMessage(t *testing.T) {
	p := ProgressStatusDetail{}
	message := "handle successfully"
	p.SetStatusAndMessage(SucceedProgressStatus, message)
	if p.Status != SucceedProgressStatus && p.Message != message {
		t.Error("set progressDetail status and message failed")
	}
}
