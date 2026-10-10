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

package instance

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apps "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func TestStoppedUpdateNeverConstructsLifecycleActions(t *testing.T) {
	inst := &workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: "demo-0", Namespace: "default", UID: "inst"}, Spec: workloads.InstanceSpec{Stop: ptr.To(true), Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:new"}}}}, LifecycleActions: &workloads.LifecycleActions{Switchover: &apps.Action{}}, Configs: []workloads.ConfigTemplate{{Name: "dynamic", ConfigHash: ptr.To("new"), Reconfigure: &apps.Action{}}}}}
	pod := readyPod("old")
	pod.Name = inst.Name
	pod.Namespace = inst.Namespace
	pod.UID = "pod"
	pod.Labels = getMatchLabels(inst.Name)
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := workloads.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	model.AddScheme(workloads.AddToScheme)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&workloads.Instance{}).WithObjects(inst, pod).Build()
	oldFactory := newLifecycleAction
	defer func() { newLifecycleAction = oldFactory }()
	calls := 0
	invoked := errors.New("action constructor invoked")
	newLifecycleAction = func(*workloads.Instance, []*corev1.Pod, *corev1.Pod) (lifecycle.Lifecycle, error) {
		calls++
		return nil, invoked
	}
	run := func() error {
		_, err := kubebuilderx.NewController(context.Background(), cli, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(inst)}, record.NewFakeRecorder(100), logr.Discard()).Prepare(NewTreeLoader()).Do(NewUpdateReconciler()).Commit()
		return err
	}
	if err := run(); err != nil {
		t.Fatal(err)
	}
	persisted := &corev1.Pod{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(pod), persisted); err != nil {
		t.Fatal(err)
	}
	if calls != 0 || !reflect.DeepEqual(persisted.Spec, pod.Spec) {
		t.Fatal("stopped chain changed the Pod or constructed an action")
	}
	current := &workloads.Instance{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(inst), current); err != nil {
		t.Fatal(err)
	}
	current.Spec.Stop = ptr.To(false)
	if err := cli.Update(context.Background(), current); err != nil {
		t.Fatal(err)
	}
	if err := run(); !errors.Is(err, invoked) || calls != 1 {
		t.Fatalf("running control did not exercise action path: %v %d", err, calls)
	}
}

func TestStopDoesNotChangePodRevision(t *testing.T) {
	inst := &workloads.Instance{}

	inst.Spec.Stop = nil
	runningRevision, err := BuildPodRevision(inst)
	if err != nil {
		t.Fatal(err)
	}
	inst.Spec.Stop = ptr.To(false)
	falseRevision, err := BuildPodRevision(inst)
	if err != nil {
		t.Fatal(err)
	}
	inst.Spec.Stop = ptr.To(true)
	stoppedRevision, err := BuildPodRevision(inst)
	if err != nil {
		t.Fatal(err)
	}
	if runningRevision != falseRevision || runningRevision != stoppedRevision {
		t.Fatal("Stop changed Pod revision")
	}
}
