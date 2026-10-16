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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	apps "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	instctrl "github.com/apecloud/kubeblocks/pkg/controller/instance"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	workloadlifecycle "github.com/apecloud/kubeblocks/pkg/controller/workloads/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

func TestCompletedDataRefreshPreservesUnrelatedTaskAndRevisions(t *testing.T) {
	action := &apps.Action{Exec: &apps.ExecAction{Command: []string{"true"}}}
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: "default"}, Spec: workloads.InstanceSetSpec{LifecycleActions: &workloads.LifecycleActions{DataDump: action, DataLoad: action, DataVolume: "data", Worker: &corev1.Container{Name: "worker", Image: "agent:v1"}}}, Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{{PodName: "worker-0", DataLoaded: ptr.To(true)}}}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "worker-0", Namespace: "default"}, Spec: corev1.PodSpec{ServiceAccountName: "worker", Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}, Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "target-data"}}}}}}
	if err := workloadlifecycle.ConfigurePod(its, pod, nil, true, true); err != nil {
		t.Fatal(err)
	}
	result := &proto.DataLoadResult{Namespace: pod.Namespace, PVCName: "target-data"}
	tasks, err := kbagent.BuildEnv4Worker([]proto.Task{{UID: "generated", NewReplica: &proto.NewReplicaTask{Remote: "source", DataLoadResult: result}}, {UID: "unrelated", NewReplica: &proto.NewReplicaTask{DataLoadResult: &proto.DataLoadResult{Namespace: pod.Namespace, PVCName: "other-data"}}}})
	if err != nil {
		t.Fatal(err)
	}
	pod.Spec.InitContainers[0].Env = append(pod.Spec.InitContainers[0].Env, *tasks, corev1.EnvVar{Name: "USER_CONFIG", Value: "kept"})
	inst := &workloads.Instance{ObjectMeta: pod.ObjectMeta, Spec: workloads.InstanceSpec{Template: corev1.PodTemplateSpec{Spec: pod.Spec}, LifecycleActions: its.Spec.LifecycleActions.DeepCopy()}}
	before := buildInstanceRevision(inst)
	beforePod, err := instctrl.BuildPodRevision(inst)
	if err != nil {
		t.Fatal(err)
	}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	if err := tree.Add(&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: result.PVCName, Namespace: result.Namespace, Annotations: map[string]string{proto.DataLoadedAnnotationKey: "true"}}}); err != nil {
		t.Fatal(err)
	}
	if err := refreshInstanceData(tree, its, inst); err != nil {
		t.Fatal(err)
	}
	afterPod, err := instctrl.BuildPodRevision(inst)
	if err != nil {
		t.Fatal(err)
	}
	if before != buildInstanceRevision(inst) || beforePod != afterPod {
		t.Fatal("cleanup of generated task changed a persistent revision")
	}
	generated, err := workloadlifecycle.GeneratedTask(&inst.Spec.Template.Spec)
	if err != nil {
		t.Fatal(err)
	}
	if generated != "" {
		t.Fatal("completed generated task retained")
	}
	for _, env := range inst.Spec.Template.Spec.InitContainers[0].Env {
		if env.Name == tasks.Name && !strings.Contains(env.Value, "unrelated") {
			t.Fatal("unrelated task removed")
		}
	}
}
