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
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestVolumeExpansionInstanceActionMaterializesInheritedVCT(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := opsv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	storageClass := "fast"
	vct := appsv1.PersistentVolumeClaimTemplate{
		Name:        "data",
		Annotations: map[string]string{"example.io/keep": "true"},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClass,
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("2Gi"),
			}},
		},
	}
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
		Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{
			Name: "db", Replicas: 1, VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{vct},
			Instances: []appsv1.InstanceTemplate{{Name: "large", Replicas: ptr.To(int32(1))}},
		}}},
	}
	ops := &opsv1alpha1.OpsRequest{Spec: opsv1alpha1.OpsRequestSpec{
		SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{VolumeExpansionList: []opsv1alpha1.VolumeExpansion{{
			ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"},
			Instances:    []opsv1alpha1.InstanceVolumeClaimTemplate{{Name: "large", VolumeClaimTemplates: []opsv1alpha1.OpsRequestVolumeClaimTemplate{{Name: "data", Storage: resource.MustParse("5Gi")}}}},
		}}},
	}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, ops).Build()
	res := &OpsResource{Cluster: cluster, OpsRequest: ops}
	if err := (volumeExpansionOpsHandler{}).Action(intctrlutil.RequestCtx{Ctx: context.Background()}, cli, res); err != nil {
		t.Fatal(err)
	}
	got := cluster.Spec.ComponentSpecs[0].Instances[0].VolumeClaimTemplates
	if len(got) != 1 || got[0].Spec.Resources.Requests.Storage().Cmp(resource.MustParse("5Gi")) != 0 {
		t.Fatalf("instance VCT = %#v", got)
	}
	if got[0].Spec.StorageClassName == nil || *got[0].Spec.StorageClassName != storageClass || got[0].Annotations["example.io/keep"] != "true" {
		t.Fatalf("inherited VCT fields were not preserved: %#v", got[0])
	}
}
