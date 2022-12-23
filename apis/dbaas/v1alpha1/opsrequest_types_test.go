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
	"testing"
)

var componentName = "mysql"

func mockRestartOps() *OpsRequest {
	ops := &OpsRequest{}
	ops.Spec.RestartList = []ComponentOps{
		{
			ComponentName: componentName,
		},
	}
	return ops
}

func TestGetRestartComponentNameMap(t *testing.T) {
	ops := mockRestartOps()
	componentNameMap := ops.GetRestartComponentNameMap()
	checkComponentMap(t, componentNameMap, len(ops.Spec.RestartList), componentName)
}

func mockVerticalScalingOps() *OpsRequest {
	ops := &OpsRequest{}
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
	componentNameMap := ops.GetVerticalScalingComponentNameMap()
	checkComponentMap(t, componentNameMap, len(ops.Spec.VerticalScalingList), componentName)
}

func mockHorizontalScalingOps() *OpsRequest {
	ops := &OpsRequest{}
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
	componentNameMap := ops.GetHorizontalScalingComponentNameMap()
	checkComponentMap(t, componentNameMap, len(ops.Spec.HorizontalScalingList), componentName)
}

func mockVolumeExpansionOps() *OpsRequest {
	ops := &OpsRequest{}
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
	componentNameMap := ops.GetVolumeExpansionComponentNameMap()
	checkComponentMap(t, componentNameMap, len(ops.Spec.VolumeExpansionList), componentName)
}

func checkComponentMap(t *testing.T, componentNameMap map[string]struct{}, expectLen int, expectName string) {
	if len(componentNameMap) != expectLen {
		t.Error(`Expected component name map length equals list length"`)
	}
	if _, ok := componentNameMap[expectName]; !ok {
		t.Error(`Expected component name map exists the key of mysql"`)
	}
}

func TestCovertVerticalScalingListToMap(t *testing.T) {
	ops := mockVerticalScalingOps()
	verticalScalingMap := ops.CovertVerticalScalingListToMap()
	if len(verticalScalingMap) != len(ops.Spec.VerticalScalingList) {
		t.Error(`Expected vertical scaling map length equals list length"`)
	}
	if _, ok := verticalScalingMap[componentName]; !ok {
		t.Error(`Expected component name map exists the key of mysql"`)
	}
}

func TestCovertVolumeExpansionListToMap(t *testing.T) {
	ops := mockVolumeExpansionOps()
	volumeExpansionMap := ops.CovertVolumeExpansionListToMap()
	if len(volumeExpansionMap) != len(ops.Spec.VolumeExpansionList) {
		t.Error(`Expected volume expansion map length equals list length"`)
	}
	if _, ok := volumeExpansionMap[componentName]; !ok {
		t.Error(`Expected component name map exists the key of mysql"`)
	}
}

func TestCovertHorizontalScalingListToMap(t *testing.T) {
	ops := mockHorizontalScalingOps()
	horizontalScalingMap := ops.CovertHorizontalScalingListToMap()
	if len(horizontalScalingMap) != len(ops.Spec.HorizontalScalingList) {
		t.Error(`Expected horizontal scaling map length equals list length"`)
	}
	if _, ok := horizontalScalingMap[componentName]; !ok {
		t.Error(`Expected component name map exists the key of mysql"`)
	}
}
