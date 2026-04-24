/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestOpsRuntimeBuildsInstanceAPIView(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apps scheme: %v", err)
	}
	if err := workloads.AddToScheme(scheme); err != nil {
		t.Fatalf("add workloads scheme: %v", err)
	}

	const (
		namespace    = "default"
		clusterName  = "test-cluster"
		component    = "mysql"
		instanceName = "test-cluster-mysql-0"
	)
	enableInstanceAPI := true
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      constant.GenerateClusterComponentName(clusterName, component),
			Labels:    constant.GetCompLabels(clusterName, component),
		},
		Spec: workloads.InstanceSetSpec{
			MinReadySeconds: 15,
		},
		Status: workloads.InstanceSetStatus{
			CurrentRevisions: map[string]string{
				instanceName: "rev-a",
			},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:         namespace,
			Name:              instanceName,
			Labels:            map[string]string{constant.AppInstanceLabelKey: clusterName, constant.KBAppComponentLabelKey: component, constant.RoleLabelKey: "leader"},
			CreationTimestamp: metav1.Now(),
		},
		Spec: corev1.PodSpec{
			NodeName: "node-a",
			Containers: []corev1.Container{{
				Name:  "mysql",
				Image: "mysql:8.0.36",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("100m"),
					},
				},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "data",
					MountPath: "/var/lib/mysql",
				}},
			}},
			Volumes: []corev1.Volume{{
				Name: "data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "data-" + instanceName,
					},
				},
			}},
			Tolerations: []corev1.Toleration{{Key: "dedicated", Value: "db"}},
			Affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			}},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:  "mysql",
				Image: "mysql@sha256:abc",
				Ready: true,
			}},
		},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "data-" + instanceName,
			Labels: map[string]string{
				constant.AppInstanceLabelKey:             clusterName,
				constant.KBAppComponentLabelKey:          component,
				constant.KBAppPodNameLabelKey:            instanceName,
				constant.VolumeClaimTemplateNameLabelKey: "data",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("2Gi"),
				},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("2Gi"),
			},
		},
	}
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      clusterName,
			Annotations: map[string]string{
				constant.KBAppMultiClusterPlacementKey: "data-ctx-a",
			},
		},
		Spec: appsv1.ClusterSpec{
			ComponentSpecs: []appsv1.ClusterComponentSpec{{
				Name:              component,
				EnableInstanceAPI: &enableInstanceAPI,
			}},
		},
	}

	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(its, pod, pvc).
		Build()
	opsRes := &OpsResource{Cluster: cluster}
	runtimes, err := buildOpsRuntimes(context.Background(), cli, opsRes)
	if err != nil {
		t.Fatalf("build runtime: %v", err)
	}
	opsRes.Runtimes = runtimes
	rt, err := opsRes.GetRuntime(component)
	if err != nil {
		t.Fatalf("get runtime: %v", err)
	}
	opsRT, ok := rt.(*opsRuntime)
	if !ok {
		t.Fatalf("expected ops runtime, got %T", rt)
	}
	if !opsRT.multiCluster {
		t.Fatal("expected multi-cluster runtime")
	}

	workload, err := rt.GetWorkload(namespace, clusterName, component)
	if err != nil {
		t.Fatalf("get workload: %v", err)
	}
	if workload.GetMinReadySeconds() != 15 {
		t.Fatalf("unexpected minReadySeconds: %d", workload.GetMinReadySeconds())
	}
	if got := workload.GetCurrentRevisionMap()[instanceName]; got != "rev-a" {
		t.Fatalf("unexpected current revision: %s", got)
	}

	instance, err := rt.GetInstance(namespace, clusterName, component, instanceName)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if instance.GetRole() != "leader" {
		t.Fatalf("unexpected role: %s", instance.GetRole())
	}
	if instance.GetImage("mysql") != "mysql:8.0.36" {
		t.Fatalf("unexpected image: %s", instance.GetImage("mysql"))
	}
	if instance.GetStatusImage("mysql") != "mysql@sha256:abc" {
		t.Fatalf("unexpected status image: %s", instance.GetStatusImage("mysql"))
	}
	if !instance.IsAvailable(15, true) {
		t.Fatalf("expected instance to be available")
	}
	volume, ok := instance.GetVolume("data")
	if !ok {
		t.Fatalf("expected instance volume, got type=%T", instance)
	}
	if volume.GetClaimName() != "data-"+instanceName {
		t.Fatalf("unexpected pvc name: %s", volume.GetClaimName())
	}
}
