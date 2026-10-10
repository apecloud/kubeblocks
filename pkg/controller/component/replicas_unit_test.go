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

package component

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestGetReplicaRestoreReplicasUsesInstanceTemplateVolumeClaims(t *testing.T) {
	scheme := newTestReplicaRestoreScheme(t)
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      "cluster-mysql",
			Labels: map[string]string{
				constant.AppInstanceLabelKey:    "cluster",
				constant.KBAppComponentLabelKey: "mysql",
			},
		},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To[int32](2),
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "data"},
			}},
			Instances: []workloads.InstanceTemplate{{
				Name:     "special",
				Replicas: ptr.To[int32](1),
				VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
					ObjectMeta: metav1.ObjectMeta{Name: "data"},
				}, {
					ObjectMeta: metav1.ObjectMeta{Name: "log"},
				}},
			}},
			ReplicaRestore: &appsv1.ClusterRestore{Source: appsv1.ClusterRestoreSource{
				APIGroup: "example.kubeblocks.io",
				Kind:     "Backup",
				Name:     "backup",
			}},
		},
	}
	defaultName := "cluster-mysql-0"
	specialName := "cluster-mysql-special-0"
	restorePVC := func(name, podName, volume string) *corev1.PersistentVolumeClaim {
		return &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
			Namespace: "test-ns",
			Name:      name,
			Labels: map[string]string{
				constant.AppInstanceLabelKey:             "cluster",
				constant.KBAppComponentLabelKey:          "mysql",
				constant.KBAppPodNameLabelKey:            podName,
				constant.VolumeClaimTemplateNameLabelKey: volume,
			},
			Annotations: map[string]string{constant.RestorePurposeAnnotationKey: constant.RestorePurposeReplica},
		}}
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		restorePVC("data-"+defaultName, defaultName, "data"),
		restorePVC("data-"+specialName, specialName, "data"),
	).Build()

	restored, pending, err := GetReplicaRestoreReplicas(context.Background(), cli, its, []string{defaultName, specialName})
	if err != nil {
		t.Fatalf("GetReplicaRestoreReplicas() error = %v", err)
	}
	if !restored.Has(defaultName) {
		t.Fatalf("expected %s to be restored", defaultName)
	}
	if restored.Has(specialName) {
		t.Fatalf("did not expect %s to be restored while its template-specific log PVC is missing", specialName)
	}
	if !pending.Has(specialName) {
		t.Fatalf("expected %s to be pending while its template-specific log PVC is missing", specialName)
	}
}

func newTestReplicaRestoreScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
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
	return scheme
}
