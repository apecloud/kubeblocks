/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

func mockExposeOps() *OpsRequest {
	ops := &OpsRequest{}
	ops.Spec.Type = ExposeType
	ops.Spec.ExposeList = []Expose{
		{
			ComponentName: componentName,
		},
	}
	return ops
}

func TestToExposeListToMap(t *testing.T) {
	ops := mockExposeOps()
	exposeMap := ops.Spec.ToExposeListToMap()
	if len(exposeMap) != len(ops.Spec.ExposeList) {
		t.Error(`Expected expose map length equals list length`)
	}
	if _, ok := exposeMap[componentName]; !ok {
		t.Error(`Expected component name map exists the key of "mysql"`)
	}
}

func TestSetStatusAndMessage(t *testing.T) {
	p := ProgressStatusDetail{}
	message := "handle successfully"
	p.SetStatusAndMessage(SucceedProgressStatus, message)
	if p.Status != SucceedProgressStatus && p.Message != message {
		t.Error("set progressDetail status and message failed")
	}
}
