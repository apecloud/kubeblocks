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
	replicas := int32(1)
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  namespace,
			Name:       constant.GenerateClusterComponentName(clusterName, component),
			Labels:     constant.GetCompLabels(clusterName, component),
			Generation: 2,
		},
		Spec: workloads.InstanceSetSpec{
			MinReadySeconds: 15,
			Replicas:        &replicas,
		},
		Status: workloads.InstanceSetStatus{
			Replicas: 1,
			CurrentRevisions: map[string]string{
				instanceName: "rev-a",
			},
			UpdateRevisions: map[string]string{
				instanceName: "rev-b",
			},
			InstanceStatus: []workloads.InstanceStatus{{
				PodName:         instanceName,
				CurrentRevision: "rev-a",
				UpdateRevision:  "rev-b",
				Ready:           true,
				Available:       true,
			}},
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
	if !workload.Exists() {
		t.Fatal("expected an InstanceSet workload")
	}
	if workload.GetDesiredReplicas() != 1 {
		t.Fatalf("unexpected desired replicas: %d", workload.GetDesiredReplicas())
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
	if instance.GetNodeName() != "node-a" {
		t.Fatalf("unexpected node name: %s", instance.GetNodeName())
	}
	if len(instance.GetTolerations()) != 1 {
		t.Fatalf("expected tolerations")
	}
	if instance.GetAffinity() == nil {
		t.Fatalf("expected affinity")
	}
	if len(instance.GetTopologySpreadConstraints()) != 1 {
		t.Fatalf("expected topology spread constraints")
	}
	if len(instance.GetPodVolumes()) != 1 {
		t.Fatalf("expected pod volumes")
	}
	if len(instance.GetVolumeMounts("mysql")) != 1 {
		t.Fatalf("expected mysql volume mounts")
	}
	if instance.GetVolumeMounts("missing") != nil {
		t.Fatalf("expected nil missing container volume mounts")
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
	requestedStorage := volume.GetRequestedStorage()
	if requestedStorage.String() != "2Gi" {
		t.Fatalf("unexpected requested storage: %s", requestedStorage.String())
	}
	capacity := volume.GetCapacity()
	if capacity.String() != "2Gi" {
		t.Fatalf("unexpected capacity: %s", capacity.String())
	}
	if !volume.IsBound() {
		t.Fatalf("expected volume to be bound")
	}
	if volume.IsExpanding() {
		t.Fatalf("did not expect volume to be expanding")
	}
}

func TestOpsRuntimeWorkloadMissingAndUsesPublicInstanceStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := workloads.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	const (
		namespace   = "default"
		clusterName = "cluster"
		component   = "mysql"
	)

	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	rt := newOpsRuntime(context.Background(), cli, "")
	workload, err := rt.GetWorkload(namespace, clusterName, component)
	if err != nil {
		t.Fatalf("get missing workload: %v", err)
	}
	if workload.Exists() {
		t.Fatal("missing InstanceSet must remain missing")
	}

	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  namespace,
			Name:       constant.GenerateClusterComponentName(clusterName, component),
			Generation: 2,
		},
		Status: workloads.InstanceSetStatus{
			CurrentRevisions: map[string]string{"zstd": "not-base64"},
			UpdateRevisions:  map[string]string{"zstd": "not-base64"},
			InstanceStatus: []workloads.InstanceStatus{{
				PodName:         "cluster-mysql-0",
				CurrentRevision: "rev-a",
				UpdateRevision:  "rev-b",
				UpToDate:        true,
				Failed:          true,
			}, {
				PodName:      "cluster-mysql-offline",
				DesiredState: workloads.InstanceDesiredStateOffline,
				CurrentState: workloads.InstanceCurrentStateAbsent,
			}, {
				PodName:      "cluster-mysql-released",
				DesiredState: workloads.InstanceDesiredStateReleased,
				CurrentState: workloads.InstanceCurrentStatePresent,
			}},
		},
	}
	cli = fake.NewClientBuilder().WithScheme(scheme).WithObjects(its).Build()
	rt = newOpsRuntime(context.Background(), cli, "")
	workload, err = rt.GetWorkload(namespace, clusterName, component)
	if err != nil {
		t.Fatalf("get workload from public instance status: %v", err)
	}
	if workload.GetCurrentRevisionMap()["cluster-mysql-0"] != "rev-a" ||
		!workload.GetUpToDateInstanceNameSet().Has("cluster-mysql-0") ||
		!workload.GetFailedInstanceNameSet().Has("cluster-mysql-0") {
		t.Fatal("workload did not use explicit InstanceStatus fields")
	}
	if !workload.GetActiveInstanceNameSet().Has("cluster-mysql-0") ||
		workload.GetActiveInstanceNameSet().Has("cluster-mysql-offline") ||
		workload.GetActiveInstanceNameSet().Has("cluster-mysql-released") {
		t.Fatal("workload did not filter active rolling instances by effective desired state")
	}
	if !workload.GetPresentInstanceNameSet().Has("cluster-mysql-0") ||
		!workload.GetPresentInstanceNameSet().Has("cluster-mysql-released") ||
		workload.GetPresentInstanceNameSet().Has("cluster-mysql-offline") {
		t.Fatal("workload did not expose current presence from effective current state")
	}
}

func TestDefaultInstanceAndVolumeNilBranches(t *testing.T) {
	instance := &defaultInstance{name: "missing", componentName: "mysql"}
	if instance.GetComponentName() != "mysql" {
		t.Fatalf("unexpected component name: %s", instance.GetComponentName())
	}
	if instance.GetName() != "missing" {
		t.Fatalf("unexpected instance name: %s", instance.GetName())
	}
	if instance.IsDeleting() {
		t.Fatalf("nil pod should not be deleting")
	}
	if instance.GetRole() != "" {
		t.Fatalf("expected empty role")
	}
	if instance.IsAvailable(0, false) {
		t.Fatalf("nil pod should not be available")
	}
	if instance.IsFailedAndTimedOut() {
		t.Fatalf("nil pod should not be failed and timed out")
	}
	if instance.GetNodeName() != "" {
		t.Fatalf("expected empty node name")
	}
	if instance.GetTolerations() != nil {
		t.Fatalf("expected nil tolerations")
	}
	if instance.GetAffinity() != nil {
		t.Fatalf("expected nil affinity")
	}
	if instance.GetTopologySpreadConstraints() != nil {
		t.Fatalf("expected nil topology constraints")
	}
	if instance.GetPodVolumes() != nil {
		t.Fatalf("expected nil pod volumes")
	}
	if instance.GetVolumeMounts("") != nil {
		t.Fatalf("expected nil volume mounts")
	}

	now := metav1.Now()
	deleting := &defaultInstance{pod: &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "deleting",
			DeletionTimestamp: &now,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
			}},
		},
	}}
	if !deleting.IsDeleting() {
		t.Fatalf("expected deleting pod")
	}
	if deleting.IsAvailable(0, false) {
		t.Fatalf("deleting pod should not be available")
	}

	volume := &instanceVolume{}
	if volume.GetClaimName() != "" {
		t.Fatalf("expected empty claim name")
	}
	requestedStorage := volume.GetRequestedStorage()
	if !requestedStorage.IsZero() {
		t.Fatalf("expected zero requested storage")
	}
	capacity := volume.GetCapacity()
	if !capacity.IsZero() {
		t.Fatalf("expected zero capacity")
	}
	if volume.IsBound() {
		t.Fatalf("nil pvc should not be bound")
	}
	if volume.IsExpanding() {
		t.Fatalf("nil pvc should not be expanding")
	}

	expanding := &instanceVolume{pvc: &corev1.PersistentVolumeClaim{
		Status: corev1.PersistentVolumeClaimStatus{
			Conditions: []corev1.PersistentVolumeClaimCondition{{
				Type:   corev1.PersistentVolumeClaimResizing,
				Status: corev1.ConditionTrue,
			}},
		},
	}}
	if !expanding.IsExpanding() {
		t.Fatalf("expected resizing pvc to be expanding")
	}
}
