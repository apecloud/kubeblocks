/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
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

func TestValidateVolumeExpansionInstances(t *testing.T) {
	for _, sharding := range []bool{false, true} {
		for _, tc := range []struct {
			name, instance, volume, size string
			logsSize                     string
			logsAllow                    bool
			allow, valid, component      bool
		}{
			{name: "inherited volume", instance: "inherit", volume: "data", size: "3Gi", allow: true, valid: true},
			{name: "overridden volume", instance: "custom", volume: "data", size: "6Gi", allow: true, valid: true},
			{name: "unknown instance", instance: "missing", volume: "data", size: "3Gi", allow: true},
			{name: "unknown volume", instance: "inherit", volume: "missing", size: "3Gi", allow: true},
			{name: "shrink", instance: "custom", volume: "data", size: "3Gi", allow: true},
			{name: "unsupported storage class", instance: "inherit", volume: "data", size: "3Gi"},
			{name: "component excludes overrides", volume: "data", size: "3Gi", allow: true, valid: true, component: true},
			{name: "empty request", allow: true},
			{name: "multiple volumes", instance: "custom", volume: "data", size: "6Gi", logsSize: "6Gi", logsAllow: true, allow: true, valid: true},
			{name: "second volume shrink", instance: "custom", volume: "data", size: "6Gi", logsSize: "3Gi", logsAllow: true, allow: true},
			{name: "second volume unsupported class", instance: "custom", volume: "data", size: "6Gi", logsSize: "6Gi", allow: true},
		} {
			t.Run(tc.name+map[bool]string{false: "/component", true: "/sharding"}[sharding], func(t *testing.T) {
				scheme := runtime.NewScheme()
				if err := corev1.AddToScheme(scheme); err != nil {
					t.Fatal(err)
				}
				if err := storagev1.AddToScheme(scheme); err != nil {
					t.Fatal(err)
				}
				vct := appsv1.ClusterComponentVolumeClaimTemplate{Name: "data"}
				vct.Spec.StorageClassName = ptr.To("sc")
				vct.Spec.Resources.Requests = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}
				override := *vct.DeepCopy()
				override.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("5Gi")
				spec := appsv1.ClusterComponentSpec{Name: "db", Replicas: 3, VolumeClaimTemplates: []appsv1.ClusterComponentVolumeClaimTemplate{vct}, Instances: []appsv1.InstanceTemplate{{Name: "inherit"}, {Name: "custom", VolumeClaimTemplates: []appsv1.ClusterComponentVolumeClaimTemplate{override}}}}
				logs := *override.DeepCopy()
				logs.Name = "logs"
				logs.Spec.StorageClassName = ptr.To("logs-sc")
				spec.Instances[1].VolumeClaimTemplates = append(spec.Instances[1].VolumeClaimTemplates, logs)
				cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}}
				if sharding {
					cluster.Spec.Shardings = []appsv1.ClusterSharding{{Name: "db", Template: spec}}
				} else {
					cluster.Spec.ComponentSpecs = []appsv1.ClusterComponentSpec{spec}
				}
				expansion := VolumeExpansion{ComponentOps: ComponentOps{ComponentName: "db"}}
				if tc.volume != "" {
					volumes := []OpsRequestVolumeClaimTemplate{{Name: tc.volume, Storage: resource.MustParse(tc.size)}}
					if tc.logsSize != "" {
						volumes = append(volumes, OpsRequestVolumeClaimTemplate{Name: "logs", Storage: resource.MustParse(tc.logsSize)})
					}
					if tc.component {
						expansion.VolumeClaimTemplates = volumes
					} else {
						expansion.Instances = []InstanceVolumeClaimTemplate{{Name: tc.instance, VolumeClaimTemplates: volumes}}
					}
				}
				ops := &OpsRequest{ObjectMeta: metav1.ObjectMeta{Namespace: "default"}, Spec: OpsRequestSpec{ClusterName: "test", SpecificOpsRequest: SpecificOpsRequest{VolumeExpansionList: []VolumeExpansion{expansion}}}}
				cli := fake.NewClientBuilder().WithScheme(scheme).Build()
				sc := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "sc"}, AllowVolumeExpansion: ptr.To(tc.allow)}
				if err := cli.Create(context.Background(), sc); err != nil {
					t.Fatal(err)
				}
				for instance, size := range map[string]string{"": "1Gi", "inherit": "1Gi", "custom": "5Gi"} {
					labels := map[string]string{constant.AppInstanceLabelKey: "test", constant.KBAppComponentLabelKey: "db", constant.KBAppComponentInstanceTemplateLabelKey: instance, constant.VolumeClaimTemplateNameLabelKey: "data"}
					if sharding {
						labels[constant.KBAppComponentLabelKey] = "db-a"
						labels[constant.KBAppShardingNameLabelKey] = "db"
					}
					pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data-" + instance + "-0", Namespace: "default", Labels: labels}}
					pvc.Spec.StorageClassName = ptr.To("sc")
					pvc.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)}
					if err := cli.Create(context.Background(), pvc); err != nil {
						t.Fatal(err)
					}
				}
				if tc.logsSize != "" {
					logSC := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "logs-sc"}, AllowVolumeExpansion: ptr.To(tc.logsAllow)}
					if err := cli.Create(context.Background(), logSC); err != nil {
						t.Fatal(err)
					}
					labels := map[string]string{constant.AppInstanceLabelKey: "test", constant.KBAppComponentLabelKey: "db", constant.KBAppComponentInstanceTemplateLabelKey: "custom", constant.VolumeClaimTemplateNameLabelKey: "logs"}
					if sharding {
						labels[constant.KBAppComponentLabelKey] = "db-a"
						labels[constant.KBAppShardingNameLabelKey] = "db"
					}
					pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "logs-custom-0", Namespace: "default", Labels: labels}}
					pvc.Spec.StorageClassName = ptr.To("logs-sc")
					pvc.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("5Gi")}
					if err := cli.Create(context.Background(), pvc); err != nil {
						t.Fatal(err)
					}
				}
				err := ops.validateVolumeExpansion(context.Background(), cli, cluster)
				if (err == nil) != tc.valid {
					t.Fatalf("valid=%v, error=%v", tc.valid, err)
				}
			})
		}
	}
}
