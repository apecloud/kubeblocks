/*
Copyright ApeCloud, Inc.

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
