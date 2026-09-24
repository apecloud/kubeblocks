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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestSwitchoverRuntimeReadsPod(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apps scheme: %v", err)
	}

	const (
		namespace    = "default"
		clusterName  = "test-cluster"
		component    = "mysql"
		instanceName = "test-cluster-mysql-0"
	)
	enableInstanceAPI := true
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
			TopologySpreadConstraints: []corev1.TopologySpreadConstraint{{
				MaxSkew:           1,
				TopologyKey:       corev1.LabelHostname,
				WhenUnsatisfiable: corev1.ScheduleAnyway,
			}},
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
		WithObjects(pod).
		Build()
	rt := newSwitchoverRuntime(context.Background(), cli, cluster)
	if !rt.multiCluster {
		t.Fatal("expected multi-cluster runtime")
	}

	instance, err := rt.getInstance(namespace, clusterName, component, instanceName)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if instance.getRole() != "leader" {
		t.Fatalf("unexpected role: %s", instance.getRole())
	}
	if !instance.hasPod() {
		t.Fatal("expected Switchover pod")
	}
}

func TestSwitchoverInstanceNilPod(t *testing.T) {
	instance := &switchoverInstance{}
	if instance.hasPod() || instance.getRole() != "" {
		t.Fatal("nil pod must have no role or membership")
	}
}
