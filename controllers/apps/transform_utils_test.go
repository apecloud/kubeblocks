/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package apps

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
)

func TestReflect(t *testing.T) {
	var list client.ObjectList
	sts := appsv1.StatefulSet{}
	sts.SetName("hello")
	list = &appsv1.StatefulSetList{Items: []appsv1.StatefulSet{sts}}
	v := reflect.ValueOf(list).Elem().FieldByName("Items")
	if v.Kind() != reflect.Slice {
		t.Error("not slice")
	}
	c := v.Len()
	objects := make([]client.Object, c)
	for i := 0; i < c; i++ {
		var st = v.Index(i).Addr().Interface()
		objects[i] = st.(client.Object)
	}
	for _, e := range objects {
		fmt.Println(e)
	}

	var o client.Object = &sts
	ptr := reflect.ValueOf(o)
	v = ptr.Elem().FieldByName("Spec")
	fmt.Println(v)
}

func TestIsVolumeClaimTemplatesEqual(t *testing.T) {
	buildVCT := func(size string) []appsv1alpha1.ClusterComponentVolumeClaimTemplate {
		return []appsv1alpha1.ClusterComponentVolumeClaimTemplate{
			{
				Name: "data",
				Spec: appsv1alpha1.PersistentVolumeClaimSpec{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse(size),
						},
					},
				},
			},
		}
	}

	assert.True(t, isVolumeClaimTemplatesEqual(buildVCT("1Gi"), buildVCT("1024Mi")))
}

func TestIsResourceRequirementsEqual(t *testing.T) {
	buildRR := func(cpu, memory string) corev1.ResourceRequirements {
		return corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(cpu),
				corev1.ResourceMemory: resource.MustParse(memory),
			},
		}
	}
	a := buildRR("1", "1Gi")
	b := buildRR("1000m", "1024Mi")
	assert.True(t, isResourceRequirementsEqual(a, b))
}

func TestIsOwnedByInstanceSet(t *testing.T) {
	its := &workloads.InstanceSet{}
	assert.False(t, isOwnedByInstanceSet(its))

	its.OwnerReferences = []metav1.OwnerReference{
		{
			Kind:       workloads.Kind,
			Controller: pointer.Bool(true),
		},
	}
	assert.True(t, isOwnedByInstanceSet(its))

	its.OwnerReferences = []metav1.OwnerReference{
		{
			Kind:       reflect.TypeOf(appsv1alpha1.Cluster{}).Name(),
			Controller: pointer.Bool(true),
		},
	}
	assert.False(t, isOwnedByInstanceSet(its))
}
