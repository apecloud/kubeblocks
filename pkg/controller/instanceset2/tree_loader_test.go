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

package instanceset2

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apps "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

func TestTreeLoaderReadsLifecycleRuntimeFromInstancePlacement(t *testing.T) {
	action := &apps.Action{Exec: &apps.ExecAction{Command: []string{"true"}}}
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "placed", Namespace: "default", UID: "its", Annotations: map[string]string{constant.KBAppMultiClusterPlacementKey: "worker-a,worker-b"}}, Spec: workloads.InstanceSetSpec{Replicas: ptr.To[int32](2), Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{ServiceAccountName: "worker", Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}, Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "target-data"}}}}}}, LifecycleActions: &workloads.LifecycleActions{DataDump: action, DataLoad: action, DataVolume: "data", Worker: &corev1.Container{Name: "worker", Image: "agent:v1"}}}, Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{{PodName: "placed-0", DataLoaded: ptr.To(false)}}}}
	inst := &workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: "placed-0", Namespace: "default", Labels: getMatchLabels(its.Name), Annotations: map[string]string{constant.KBAppMultiClusterPlacementKey: "worker-b"}}, Spec: workloads.InstanceSpec{InstanceSetName: its.Name, Template: its.Spec.Template}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: inst.Name, Namespace: inst.Namespace}, Spec: inst.Spec.Template.Spec, Status: corev1.PodStatus{PodIP: "10.1.0.2"}}
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "target-data", Namespace: "default"}}
	wrongPVC := pvc.DeepCopy()
	wrongPVC.Annotations = map[string]string{proto.DataLoadedAnnotationKey: "true"}
	instA := inst.DeepCopy()
	instA.Name = "placed-1"
	instA.Annotations[constant.KBAppMultiClusterPlacementKey] = "worker-a"
	podA := pod.DeepCopy()
	podA.Name = instA.Name
	podA.Status.PodIP = "10.1.0.1"
	control := fake.NewClientBuilder().WithScheme(model.GetScheme()).WithObjects(its, wrongPVC).Build()
	workerA := fake.NewClientBuilder().WithScheme(model.GetScheme()).WithObjects(instA, podA, wrongPVC).Build()
	workerB := fake.NewClientBuilder().WithScheme(model.GetScheme()).WithObjects(inst, pod, pvc).Build()
	cli := multicluster.NewClient(control, map[string]client.Client{"worker-a": workerA, "worker-b": workerB})
	tree, err := NewTreeLoader().Load(context.Background(), cli, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(its)}, record.NewFakeRecorder(10), logr.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.List(&corev1.PersistentVolumeClaim{})) != 0 {
		t.Fatal("parent tree stores placement-ambiguous PVC resources")
	}
	for _, tc := range []struct {
		pod       *corev1.Pod
		completed bool
	}{{pod, false}, {podA, true}} {
		observed, err := dataPVC(tree, its, tc.pod)
		if err != nil {
			t.Fatal(err)
		}
		if observed == nil || (observed.Annotations[proto.DataLoadedAnnotationKey] == "true") != tc.completed {
			t.Fatalf("wrong PVC result for %s: %#v", tc.pod.Name, observed)
		}
	}
	loaded, opts, err := tree.GetWithOption(pod)
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil || loaded.(*corev1.Pod).Status.PodIP != "10.1.0.2" || !opts.SkipToReconcile {
		t.Fatalf("runtime Pod missing or managed by parent: %#v %#v", loaded, opts)
	}
}
