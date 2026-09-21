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

package operations

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestVolumeExpansionInstances(t *testing.T) {
	for _, multipleVolumes := range []bool{false, true} {
		for _, sharding := range []bool{false, true} {
			for _, mode := range []string{"component", "instance", "both"} {
				t.Run(fmt.Sprintf("sharding=%v/multipleVolumes=%v/%s", sharding, multipleVolumes, mode), func(t *testing.T) {
					scheme := runtime.NewScheme()
					for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, opsv1alpha1.AddToScheme} {
						if err := add(scheme); err != nil {
							t.Fatal(err)
						}
					}
					vct := appsv1.ClusterComponentVolumeClaimTemplate{Name: "data"}
					vct.Spec.StorageClassName = ptr.To("expandable")
					vct.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
					vct.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}
					override := *vct.DeepCopy()
					override.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("5Gi")
					spec := appsv1.ClusterComponentSpec{
						Name: "db", Replicas: 4, OfflineInstances: []string{"test-db-inherit-2", "test-db-a-inherit-2", "test-db-b-inherit-2"}, VolumeClaimTemplates: []appsv1.ClusterComponentVolumeClaimTemplate{vct},
						Instances: []appsv1.InstanceTemplate{
							{Name: "inherit", Ordinals: appsv1.Ordinals{Discrete: []int32{2, 3}}}, // nil replicas defaults to one.
							{Name: "custom", VolumeClaimTemplates: []appsv1.ClusterComponentVolumeClaimTemplate{override}},
							{Name: "idle", Replicas: ptr.To(int32(0))},
						},
					}
					if multipleVolumes {
						logs := *vct.DeepCopy()
						logs.Name = "logs"
						untouched := *vct.DeepCopy()
						untouched.Name = "untouched"
						spec.VolumeClaimTemplates = append(spec.VolumeClaimTemplates, logs, untouched)
						extra := *vct.DeepCopy()
						extra.Name = "extra"
						spec.Instances[1].VolumeClaimTemplates = append(spec.Instances[1].VolumeClaimTemplates, extra)
					}
					cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}}
					components := []string{"db"}
					if sharding {
						cluster.Spec.Shardings = []appsv1.ClusterSharding{{Name: "db", Template: spec}}
						components = []string{"db-a", "db-b"}
					} else {
						cluster.Spec.ComponentSpecs = []appsv1.ClusterComponentSpec{spec}
					}
					expansion := opsv1alpha1.VolumeExpansion{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"}}
					if mode != "instance" {
						expansion.VolumeClaimTemplates = []opsv1alpha1.OpsRequestVolumeClaimTemplate{{Name: "data", Storage: resource.MustParse("2Gi")}}
					}
					if mode != "component" {
						expansion.Instances = []opsv1alpha1.InstanceVolumeClaimTemplate{
							{Name: "inherit", VolumeClaimTemplates: []opsv1alpha1.OpsRequestVolumeClaimTemplate{{Name: "data", Storage: resource.MustParse("3Gi")}}},
							{Name: "custom", VolumeClaimTemplates: []opsv1alpha1.OpsRequestVolumeClaimTemplate{{Name: "data", Storage: resource.MustParse("6Gi")}}},
							{Name: "idle", VolumeClaimTemplates: []opsv1alpha1.OpsRequestVolumeClaimTemplate{{Name: "data", Storage: resource.MustParse("4Gi")}}},
						}
					}
					if multipleVolumes {
						if mode != "instance" {
							expansion.VolumeClaimTemplates = append(expansion.VolumeClaimTemplates, opsv1alpha1.OpsRequestVolumeClaimTemplate{Name: "logs", Storage: resource.MustParse("3Gi")})
						}
						if mode != "component" {
							expansion.Instances[0].VolumeClaimTemplates = append(expansion.Instances[0].VolumeClaimTemplates, opsv1alpha1.OpsRequestVolumeClaimTemplate{Name: "logs", Storage: resource.MustParse("4Gi")})
							expansion.Instances[1].VolumeClaimTemplates = append(expansion.Instances[1].VolumeClaimTemplates, opsv1alpha1.OpsRequestVolumeClaimTemplate{Name: "extra", Storage: resource.MustParse("8Gi")})
						}
					}
					ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "expand", Namespace: "default"}, Spec: opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{VolumeExpansionList: []opsv1alpha1.VolumeExpansion{expansion}}}}
					ops.Status.StartTimestamp = metav1.Now()
					objects := []client.Object{cluster, ops}
					for _, comp := range components {
						if sharding {
							objects = append(objects, &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Name: "test-" + comp, Namespace: "default", Labels: map[string]string{constant.AppInstanceLabelKey: "test", constant.KBAppShardingNameLabelKey: "db", constant.KBAppComponentLabelKey: comp}}})
						}
						for suffix, size := range map[string]string{"0": "2Gi", "1": "2Gi", "inherit-2": "1Gi", "inherit-3": "2Gi", "custom-0": "5Gi"} {
							if mode != "component" && suffix == "inherit-3" {
								size = "3Gi"
							}
							if mode != "component" && suffix == "custom-0" {
								size = "6Gi"
							}
							pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data-test-" + comp + "-" + suffix, Namespace: "default", Labels: map[string]string{constant.AppInstanceLabelKey: "test", constant.KBAppComponentLabelKey: comp, constant.VolumeClaimTemplateNameLabelKey: "data"}}}
							pvc.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)}
							pvc.Status.Capacity = pvc.Spec.Resources.Requests.DeepCopy()
							pvc.Status.Phase = corev1.ClaimBound
							objects = append(objects, pvc)
							if multipleVolumes {
								for _, name := range []string{"logs", "untouched", "extra"} {
									if name == "extra" && suffix != "custom-0" {
										continue
									}
									size := "1Gi"
									if name == "logs" && mode != "instance" {
										size = "3Gi"
									}
									if name == "logs" && suffix == "inherit-3" && mode != "component" {
										size = "4Gi"
									}
									if name == "extra" && mode != "component" {
										size = "8Gi"
									}
									other := pvc.DeepCopy()
									other.Name = name + "-test-" + comp + "-" + suffix
									other.Labels[constant.VolumeClaimTemplateNameLabelKey] = name
									other.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(size)
									other.Status.Capacity = other.Spec.Resources.Requests.DeepCopy()
									objects = append(objects, other)
								}
							}
						}
					}
					cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(ops).WithObjects(objects...).Build()
					res := &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(100)}
					req := intctrlutil.RequestCtx{Ctx: context.Background()}
					handler := volumeExpansionOpsHandler{}
					if err := handler.SaveLastConfiguration(req, cli, res); err != nil {
						t.Fatal(err)
					}
					if err := handler.Action(req, cli, res); err != nil {
						t.Fatal(err)
					}
					got := spec
					if sharding {
						got = cluster.Spec.Shardings[0].Template
					} else {
						got = cluster.Spec.ComponentSpecs[0]
					}
					want := "2Gi"
					if mode == "instance" {
						want = "1Gi"
					}
					if got.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().Cmp(resource.MustParse(want)) != 0 {
						t.Fatal("incorrect component storage")
					}
					if mode != "component" {
						for i, want := range []string{"3Gi", "6Gi", "4Gi"} {
							volume := got.Instances[i].VolumeClaimTemplates[0]
							if volume.Spec.Resources.Requests.Storage().Cmp(resource.MustParse(want)) != 0 || *volume.Spec.StorageClassName != "expandable" || len(volume.Spec.AccessModes) != 1 {
								t.Fatalf("incorrect template volume: %+v", volume)
							}
						}
						last := ops.Status.LastConfiguration.Components["db"].Instances
						if len(last) != 3 || len(last[0].VolumeClaimTemplates) != 0 || last[1].VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().Cmp(resource.MustParse("5Gi")) != 0 {
							t.Fatalf("incorrect previous templates: %+v", last)
						}
					} else if len(got.Instances[0].VolumeClaimTemplates) != 0 || got.Instances[1].VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().Cmp(resource.MustParse("5Gi")) != 0 {
						t.Fatal("component expansion modified template overrides")
					}
					count := map[string]int{"component": 3, "instance": 2, "both": 4}[mode] * len(components)
					if multipleVolumes {
						count = map[string]int{"component": 7, "instance": 4, "both": 9}[mode] * len(components)
						assertVolume := func(volumes []appsv1.ClusterComponentVolumeClaimTemplate, name, want string) {
							t.Helper()
							for _, volume := range volumes {
								if volume.Name == name {
									if volume.Spec.Resources.Requests.Storage().Cmp(resource.MustParse(want)) != 0 {
										t.Fatalf("%s: expected %s, got %s", name, want, volume.Spec.Resources.Requests.Storage())
									}
									return
								}
							}
							t.Fatalf("missing volume %s", name)
						}
						logsSize, extraSize := "3Gi", "1Gi"
						if mode == "instance" {
							logsSize = "1Gi"
						}
						if mode != "component" {
							extraSize = "8Gi"
							assertVolume(got.Instances[0].VolumeClaimTemplates, "logs", "4Gi")
						}
						assertVolume(got.VolumeClaimTemplates, "logs", logsSize)
						assertVolume(got.VolumeClaimTemplates, "untouched", "1Gi")
						assertVolume(got.Instances[1].VolumeClaimTemplates, "extra", extraSize)
						for _, volume := range got.Instances[1].VolumeClaimTemplates {
							if volume.Name == "logs" {
								t.Fatal("partial override must keep inheriting logs")
							}
						}
					}
					// A PVC still resizing must keep the operation running.
					pending := &corev1.PersistentVolumeClaim{}
					key := client.ObjectKey{Namespace: "default", Name: "data-test-" + components[0] + "-inherit-3"}
					if multipleVolumes {
						key.Name = "logs-test-" + components[0] + "-inherit-3"
					}
					if err := cli.Get(req.Ctx, key, pending); err != nil {
						t.Fatal(err)
					}
					capacity := pending.Status.Capacity.DeepCopy()
					pending.Status.Capacity[corev1.ResourceStorage] = resource.MustParse("1Gi")
					if err := cli.Status().Update(req.Ctx, pending); err != nil {
						t.Fatal(err)
					}
					phase, _, err := handler.ReconcileAction(req, cli, res)
					if err != nil || phase != opsv1alpha1.OpsRunningPhase || ops.Status.Progress != fmt.Sprintf("%d/%d", count-1, count) {
						t.Fatalf("pending: phase=%s progress=%s err=%v", phase, ops.Status.Progress, err)
					}
					pending.Status.Capacity = capacity
					if err := cli.Status().Update(req.Ctx, pending); err != nil {
						t.Fatal(err)
					}
					phase, _, err = handler.ReconcileAction(req, cli, res)
					if err != nil || phase != opsv1alpha1.OpsSucceedPhase || ops.Status.Progress != fmt.Sprintf("%d/%d", count, count) {
						t.Fatalf("phase=%s progress=%s err=%v", phase, ops.Status.Progress, err)
					}
				})
			}
		}
	}
}
