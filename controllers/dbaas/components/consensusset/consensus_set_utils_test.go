/*
Copyright ApeCloud Inc.

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

package consensusset

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

func TestIsReady(t *testing.T) {
	set := testk8s.NewFakeStatefulSet("foo", 3)
	pod := testk8s.NewFakeStatefulSetPod(set, 1)
	pod.Status.Conditions = []v1.PodCondition{
		{
			Type:   v1.PodReady,
			Status: v1.ConditionTrue,
		},
	}
	pod.Labels = map[string]string{intctrlutil.ConsensusSetRoleLabelKey: "leader"}
	if !isReady(*pod) {
		t.Errorf("isReady returned false negative")
	}
	pod.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	if isReady(*pod) {
		t.Errorf("isReady returned false positive")
	}
	pod.Labels = nil
	if isReady(*pod) {
		t.Errorf("isReady returned false positive")
	}
	pod.Status.Conditions = nil
	if isReady(*pod) {
		t.Errorf("isReady returned false positive")
	}
	pod.Status.Conditions = []v1.PodCondition{}
	if isReady(*pod) {
		t.Errorf("isReady returned false positive")
	}
}

func TestInitClusterComponentStatusIfNeed(t *testing.T) {
	componentName := "foo"
	cluster := &dbaasv1alpha1.Cluster{
		Spec: dbaasv1alpha1.ClusterSpec{
			Components: []dbaasv1alpha1.ClusterComponent{
				{
					Name: componentName,
					Type: componentName,
				},
			},
		},
	}

	initClusterComponentStatusIfNeed(cluster, componentName)

	if cluster.Status.Components == nil {
		t.Errorf("cluster.Status.Components[*] not intialized properly")
	}
	if _, ok := cluster.Status.Components[componentName]; !ok {
		t.Errorf("cluster.Status.Components[componentName] not initialized properly")
	}
	consensusSetStatus := cluster.Status.Components[componentName].ConsensusSetStatus
	if consensusSetStatus == nil {
		t.Errorf("cluster.Status.Components[componentName].ConsensusSetStatus not initialized properly")
	} else if consensusSetStatus.Leader.Name != "" ||
		consensusSetStatus.Leader.AccessMode != dbaasv1alpha1.None ||
		consensusSetStatus.Leader.Pod != DefaultPodName {
		t.Errorf("cluster.Status.Components[componentName].ConsensusSetStatus.Leader not initialized properly")
	}
}

func TestGetPodRevision(t *testing.T) {
	set := testk8s.NewFakeStatefulSet("foo", 3)
	pod := testk8s.NewFakeStatefulSetPod(set, 1)
	if util.GetPodRevision(pod) != "" {
		t.Errorf("revision should be empty")
	}

	pod.Labels = make(map[string]string, 0)
	pod.Labels[apps.StatefulSetRevisionLabel] = "bar"

	if util.GetPodRevision(pod) != "bar" {
		t.Errorf("revision not matched")
	}
}

func TestSortPods(t *testing.T) {
	createMockPods := func(replicas int, stsName string) []v1.Pod {
		pods := make([]v1.Pod, replicas)
		for i := 0; i < replicas; i++ {
			pods[i] = v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      stsName + "-" + strconv.Itoa(i),
					Namespace: "default",
					Labels: map[string]string{
						intctrlutil.ConsensusSetRoleLabelKey: "learner",
					},
				},
			}
		}
		return pods
	}
	randSort := func(pods []v1.Pod) []v1.Pod {
		n := len(pods)
		newPod := make([]v1.Pod, n)
		copy(newPod, pods)
		for i := n; i > 0; i-- {
			randIndex := rand.Intn(i)
			newPod[n-1], newPod[randIndex] = newPod[randIndex], newPod[n-1]
		}
		return newPod
	}

	type args struct {
		pods            []v1.Pod
		rolePriorityMap map[string]int
	}
	tests := []struct {
		name    string
		args    args
		want    []v1.Pod
		wantErr bool
	}{{
		name: "test_normal",
		args: args{
			rolePriorityMap: map[string]int{
				"learner": 10,
			},
		},
		want:    createMockPods(8, "for-test"),
		wantErr: false,
	}, {
		name: "badcase",
		args: args{
			rolePriorityMap: map[string]int{
				"learner": 10,
			},
		},
		want:    createMockPods(12, "for-test"),
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.args.pods = randSort(tt.want)
			SortPods(tt.args.pods, tt.args.rolePriorityMap)
			if !tt.wantErr {
				assert.Equal(t, tt.args.pods, tt.want)
			}
		})
	}
}
