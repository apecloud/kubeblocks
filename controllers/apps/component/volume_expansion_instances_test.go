/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package component

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	compctrl "github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func TestExpandInstanceVolumes(t *testing.T) {
	for _, componentSize := range []string{"1Gi", "2Gi"} {
		t.Run(componentSize, func(t *testing.T) {
			scheme := runtime.NewScheme()
			if err := corev1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).Build()
			comp := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Name: "test-db", Namespace: "default"}}
			dag := graph.NewDAG()
			model.NewGraphClient(cli).Root(dag, comp, comp, nil)
			vct := corev1.PersistentVolumeClaimTemplate{ObjectMeta: metav1.ObjectMeta{Name: "data"}}
			vct.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(componentSize)}
			logs := *vct.DeepCopy()
			logs.Name = "logs"
			logs.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("3Gi")
			untouched := *vct.DeepCopy()
			untouched.Name = "untouched"
			untouched.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("1Gi")
			override := appsv1.ClusterComponentVolumeClaimTemplate{Name: "data"}
			override.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("6Gi")}
			extra := *override.DeepCopy()
			extra.Name = "extra"
			extra.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("4Gi")
			unchanged := *override.DeepCopy()
			unchanged.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("5Gi")
			ops := &componentWorkloadOps{
				transCtx: &componentTransformContext{Context: context.Background(), EventRecorder: record.NewFakeRecorder(10)},
				cli:      cli, component: comp, dag: dag,
				runningITS: &workloads.InstanceSet{ObjectMeta: comp.ObjectMeta, Spec: workloads.InstanceSetSpec{
					VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}}, {ObjectMeta: metav1.ObjectMeta{Name: "logs"}}, {ObjectMeta: metav1.ObjectMeta{Name: "untouched"}}},
					Instances:            []workloads.InstanceTemplate{{Name: "custom", VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "extra"}}}}},
				}},
				synthesizeComp: &compctrl.SynthesizedComponent{Namespace: "default", VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{vct, logs, untouched}, Instances: []appsv1.InstanceTemplate{
					{Name: "custom", VolumeClaimTemplates: []appsv1.ClusterComponentVolumeClaimTemplate{override, extra}},
					{Name: "unchanged", VolumeClaimTemplates: []appsv1.ClusterComponentVolumeClaimTemplate{unchanged}},
				}},
				runningItsPodNames: []string{"test-db-0", "test-db-inherit-3", "test-db-custom-0", "test-db-unchanged-0"},
			}
			initial := map[string]string{"data-test-db-0": "1Gi", "data-test-db-inherit-3": "1Gi", "data-test-db-custom-0": "5Gi", "extra-test-db-custom-0": "3Gi", "data-test-db-unchanged-0": "5Gi"}
			for _, pod := range ops.runningItsPodNames {
				initial["logs-"+pod] = "1Gi"
				initial["untouched-"+pod] = "1Gi"
			}
			for name, size := range initial {
				pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}
				pvc.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)}
				pvc.Status.Capacity = pvc.Spec.Resources.Requests.DeepCopy()
				if err := cli.Create(context.Background(), pvc); err != nil {
					t.Fatal(err)
				}
			}
			if err := ops.expandVolume(); err != nil {
				t.Fatal(err)
			}
			expected := map[string]string{"data-test-db-custom-0": "6Gi", "extra-test-db-custom-0": "4Gi"}
			if componentSize != "1Gi" {
				expected["data-test-db-0"] = componentSize
				expected["data-test-db-inherit-3"] = componentSize
			}
			for _, pod := range ops.runningItsPodNames {
				expected["logs-"+pod] = "3Gi"
			}
			for _, vertex := range dag.Vertices() {
				if pvc, ok := vertex.(*model.ObjectVertex).Obj.(*corev1.PersistentVolumeClaim); ok {
					want, ok := expected[pvc.Name]
					if !ok || pvc.Spec.Resources.Requests.Storage().Cmp(resource.MustParse(want)) != 0 {
						t.Fatalf("unexpected PVC update: %+v", pvc)
					}
					delete(expected, pvc.Name)
				}
			}
			if len(expected) != 0 {
				t.Fatalf("missing PVC updates: %v", expected)
			}
		})
	}
}
