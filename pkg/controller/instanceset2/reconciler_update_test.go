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

package instanceset2

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	instctrl "github.com/apecloud/kubeblocks/pkg/controller/instance"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/revisionmap"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestScaleOutPreservesExistingInstanceInitialization(t *testing.T) {
	for _, source := range []string{"", "backup-a"} {
		t.Run("existing source="+source, func(t *testing.T) {
			its := revisionTestInstanceSet()
			its.Annotations = map[string]string{constant.KBAppClusterUIDKey: "cluster-uid"}
			its.Labels = map[string]string{constant.KBAppComponentLabelKey: "mysql"}
			its.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
			its.Spec.Template.Spec.Containers = []corev1.Container{{Name: "db", Image: "mysql:8"}}
			its.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}}}
			if source != "" {
				its.Spec.ReplicaRestore = &kbappsv1.ClusterReplicaRestore{Source: kbappsv1.ClusterRestoreSource{
					APIGroup: "dataprotection.kubeblocks.io", Kind: "Backup", Name: source,
				}}
			}
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			if _, err := NewAlignmentReconciler().Reconcile(tree); err != nil {
				t.Fatal(err)
			}
			original := tree.List(&workloads.Instance{})[0].(*workloads.Instance).DeepCopy()
			replicas := int32(2)
			its.Spec.Replicas = &replicas
			its.Spec.ReplicaRestore = &kbappsv1.ClusterReplicaRestore{Source: kbappsv1.ClusterRestoreSource{
				APIGroup: "dataprotection.kubeblocks.io", Kind: "Backup", Name: "backup-b",
			}}
			if _, err := NewAlignmentReconciler().Reconcile(tree); err != nil {
				t.Fatal(err)
			}
			for _, obj := range tree.List(&workloads.Instance{}) {
				inst := obj.(*workloads.Instance)
				inst.Status.ObservedGeneration = inst.Generation
				inst.Status.UpToDate = true
				inst.Status.Conditions = []metav1.Condition{
					{Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue},
					{Type: string(workloads.InstanceAvailable), Status: metav1.ConditionTrue},
				}
			}
			if _, err := NewRevisionUpdateReconciler().Reconcile(tree); err != nil {
				t.Fatal(err)
			}
			if _, err := NewUpdateReconciler().Reconcile(tree); err != nil {
				t.Fatal(err)
			}
			instances := tree.List(&workloads.Instance{})
			if len(instances) != 2 {
				t.Fatalf("created %d Instances, want 2", len(instances))
			}
			for _, obj := range instances {
				inst := obj.(*workloads.Instance)
				if inst.Name == original.Name {
					if !equality.Semantic.DeepEqual(inst.Spec, original.Spec) || getInstanceRevision(inst) != getInstanceRevision(original) {
						t.Fatalf("scale-out changed an existing Instance: %#v", inst)
					}
				} else if !equality.Semantic.DeepEqual(inst.Spec.ReplicaRestore, its.Spec.ReplicaRestore) {
					t.Fatalf("new Instance did not receive its Backup source: %#v", inst.Spec.ReplicaRestore)
				}
				instanceTree := kubebuilderx.NewObjectTree()
				instanceTree.SetRoot(inst)
				if _, err := instctrl.NewAlignmentReconciler().Reconcile(instanceTree); err != nil {
					t.Fatal(err)
				}
				pvc := instanceTree.List(&corev1.PersistentVolumeClaim{})[0].(*corev1.PersistentVolumeClaim)
				if inst.Annotations[constant.KBAppClusterUIDKey] != "cluster-uid" {
					t.Fatal("Instance lost the owner Cluster UID")
				}
				expectedSource := "backup-b"
				if inst.Name == original.Name {
					expectedSource = source
				}
				if expectedSource == "" {
					if pvc.Spec.DataSourceRef != nil || pvc.Annotations[constant.RestorePurposeAnnotationKey] != "" {
						t.Fatalf("ordinary Instance received a restore PVC: %#v", pvc)
					}
				} else if pvc.Spec.DataSourceRef == nil || pvc.Spec.DataSourceRef.Name != expectedSource ||
					pvc.Annotations[constant.RestoreSourceNameAnnotationKey] != expectedSource {
					t.Fatalf("PVC did not use its Instance's Backup source %q: %#v", expectedSource, pvc)
				}
			}
		})
	}
}

func TestParseReplicasNMaxUnavailable(t *testing.T) {
	tests := []struct {
		name                   string
		totalReplicas          int
		replicas               intstr.IntOrString
		maxUnavailable         intstr.IntOrString
		expectedReplicas       int
		expectedMaxUnavailable int
	}{
		{
			name:                   "round replicas up and maxUnavailable down",
			totalReplicas:          3,
			replicas:               intstr.FromString("34%"),
			maxUnavailable:         intstr.FromString("34%"),
			expectedReplicas:       2,
			expectedMaxUnavailable: 1,
		},
		{
			name:                   "keep maxUnavailable at least one",
			totalReplicas:          3,
			replicas:               intstr.FromString("10%"),
			maxUnavailable:         intstr.FromString("10%"),
			expectedReplicas:       1,
			expectedMaxUnavailable: 1,
		},
		{
			name:                   "keep exact percentage results",
			totalReplicas:          4,
			replicas:               intstr.FromString("50%"),
			maxUnavailable:         intstr.FromString("50%"),
			expectedReplicas:       2,
			expectedMaxUnavailable: 2,
		},
		{
			name:                   "keep absolute values",
			totalReplicas:          3,
			replicas:               intstr.FromInt32(2),
			maxUnavailable:         intstr.FromInt32(1),
			expectedReplicas:       2,
			expectedMaxUnavailable: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := &workloads.InstanceUpdateStrategy{
				RollingUpdate: &workloads.RollingUpdate{
					Replicas:       &tt.replicas,
					MaxUnavailable: &tt.maxUnavailable,
				},
			}

			replicas, maxUnavailable, err := parseReplicasNMaxUnavailable(strategy, tt.totalReplicas)
			if err != nil {
				t.Fatalf("parse rolling update quotas: %v", err)
			}
			if replicas != tt.expectedReplicas || maxUnavailable != tt.expectedMaxUnavailable {
				t.Fatalf("quotas = (%d, %d), want (%d, %d)",
					replicas, maxUnavailable, tt.expectedReplicas, tt.expectedMaxUnavailable)
			}
		})
	}
}

func TestUpdateReconcilerKeepsConvergingInstanceInRollingWindow(t *testing.T) {
	tests := []struct {
		name                 string
		convergingName       string
		pendingName          string
		maxUnavailable       int32
		roles                []workloads.ReplicaRole
		memberUpdateStrategy workloads.MemberUpdateStrategy
		convergingRole       string
		pendingRole          string
	}{
		{name: "converging Instance sorts first", convergingName: "demo-1", pendingName: "demo-0", maxUnavailable: 1},
		{name: "pending Instance sorts first", convergingName: "demo-0", pendingName: "demo-1", maxUnavailable: 1},
		{
			name:           "existing participant occupies Serial member plan",
			convergingName: "demo-0", pendingName: "demo-1", maxUnavailable: 2,
			roles: []workloads.ReplicaRole{
				{Name: "follower", UpdatePriority: 1},
				{Name: "leader", UpdatePriority: 2},
			},
			memberUpdateStrategy: workloads.SerialUpdateStrategy,
			convergingRole:       "follower",
			pendingRole:          "leader",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			its := its2InstanceStatusSet(2)
			its.Generation = 2
			its.Spec.Template.Spec.Containers[0].Image = "mysql:new"
			its.Spec.Roles = tt.roles
			maxUnavailable := intstr.FromInt32(tt.maxUnavailable)
			its.Spec.InstanceUpdateStrategy = &workloads.InstanceUpdateStrategy{
				RollingUpdate: &workloads.RollingUpdate{MaxUnavailable: &maxUnavailable},
			}
			if tt.memberUpdateStrategy != "" {
				memberUpdateStrategy := tt.memberUpdateStrategy
				its.Spec.MemberUpdateStrategy = &memberUpdateStrategy
			}
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)

			desired, names, err := buildDesiredInstancesByName(tree, its)
			if err != nil {
				t.Fatal(err)
			}
			targetRevisions := make(map[string]string, len(names))
			for _, name := range names {
				targetRevisions[name] = getInstanceRevision(desired[name])
			}
			its.Status.UpdateRevisions, err = revisionmap.Encode(targetRevisions)
			if err != nil {
				t.Fatal(err)
			}

			readyAndAvailable := []metav1.Condition{
				{Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue},
				{Type: string(workloads.InstanceAvailable), Status: metav1.ConditionTrue},
			}
			converging := desired[tt.convergingName].DeepCopy()
			converging.Generation = 2
			converging.Status = workloads.InstanceStatus2{
				ObservedGeneration: 2,
				CurrentState:       workloads.InstanceCurrentStatePresent,
				UpToDate:           false,
				Role:               tt.convergingRole,
				Conditions:         readyAndAvailable,
			}
			pending := desired[tt.pendingName].DeepCopy()
			pending.Spec.Template.Spec.Containers[0].Image = "mysql:old"
			stampInstanceRevision(pending)
			pending.Generation = 1
			pending.Status = workloads.InstanceStatus2{
				ObservedGeneration: 1,
				CurrentState:       workloads.InstanceCurrentStatePresent,
				UpToDate:           true,
				Role:               tt.pendingRole,
				Conditions:         readyAndAvailable,
			}
			if err := tree.Add(converging, pending); err != nil {
				t.Fatal(err)
			}

			if _, err := NewUpdateReconciler().Reconcile(tree); err != nil {
				t.Fatal(err)
			}
			object, err := tree.Get(&workloads.Instance{ObjectMeta: metav1.ObjectMeta{Namespace: its.Namespace, Name: pending.Name}})
			if err != nil {
				t.Fatal(err)
			}
			got := object.(*workloads.Instance)
			if image := got.Spec.Template.Spec.Containers[0].Image; image != "mysql:old" {
				t.Fatalf("maxUnavailable=%d admitted a second unconverged Instance: image=%q", tt.maxUnavailable, image)
			}
			if !intctrlutil.IsInstanceReady(converging) || !intctrlutil.IsInstanceAvailable(converging) {
				t.Fatal("fixture must remain runtime healthy while desired-state convergence is pending")
			}
		})
	}
}
