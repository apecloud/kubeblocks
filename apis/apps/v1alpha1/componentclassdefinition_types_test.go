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

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestToResourceRequirements(t *testing.T) {
	var (
		cpu    = resource.MustParse("1")
		memory = resource.MustParse("1Gi")
	)
	cls := ComponentClass{
		CPU:    cpu,
		Memory: memory,
	}
	rr := cls.ToResourceRequirements()
	assert.True(t, rr.Requests.Cpu().Equal(cpu))
	assert.True(t, rr.Requests.Memory().Equal(memory))
	assert.True(t, rr.Limits.Cpu().Equal(cpu))
	assert.True(t, rr.Limits.Memory().Equal(memory))
}

func TestCmp(t *testing.T) {
	var (
		cls1 = ComponentClass{
			CPU:    resource.MustParse("1"),
			Memory: resource.MustParse("1Gi"),
		}
		cls2 = ComponentClass{
			CPU:    resource.MustParse("2"),
			Memory: resource.MustParse("1Gi"),
		}
		cls3 = ComponentClass{
			CPU:    resource.MustParse("2"),
			Memory: resource.MustParse("2Gi"),
		}
	)
	assert.True(t, cls1.Cmp(&cls2) < 0)
	assert.True(t, cls2.Cmp(&cls3) < 0)
}
