/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package consensus

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/util"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controllerutil"
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
	pod.Labels = map[string]string{constant.RoleLabelKey: "leader"}
	if !controllerutil.PodIsReadyWithLabel(*pod) {
		t.Errorf("isReady returned false negative")
	}
}

func TestInitClusterComponentStatusIfNeed(t *testing.T) {
	componentName := "foo"
	cluster := &appsv1alpha1.Cluster{
		Spec: appsv1alpha1.ClusterSpec{
			ComponentSpecs: []appsv1alpha1.ClusterComponentSpec{
				{
					Name:            componentName,
					ComponentDefRef: componentName,
				},
			},
		},
	}
	component := &appsv1alpha1.ClusterComponentDefinition{
		WorkloadType: appsv1alpha1.Consensus,
	}
	if err := util.InitClusterComponentStatusIfNeed(cluster, componentName, *component); err != nil {
		t.Errorf("caught error %v", err)
	}

	if len(cluster.Status.Components) == 0 {
		t.Errorf("cluster.Status.ComponentDefs[*] not initialized properly")
	}
	if _, ok := cluster.Status.Components[componentName]; !ok {
		t.Errorf("cluster.Status.ComponentDefs[componentName] not initialized properly")
	}
	consensusSetStatus := cluster.Status.Components[componentName].ConsensusSetStatus
	if consensusSetStatus == nil {
		t.Errorf("cluster.Status.ComponentDefs[componentName].ConsensusSetStatus not initialized properly")
	} else if consensusSetStatus.Leader.Name != "" ||
		consensusSetStatus.Leader.AccessMode != appsv1alpha1.None ||
		consensusSetStatus.Leader.Pod != util.ComponentStatusDefaultPodName {
		t.Errorf("cluster.Status.ComponentDefs[componentName].ConsensusSetStatus.Leader not initialized properly")
	}
}

func TestGetPodRevision(t *testing.T) {
	set := testk8s.NewFakeStatefulSet("foo", 3)
	pod := testk8s.NewFakeStatefulSetPod(set, 1)
	if controllerutil.GetPodRevision(pod) != "" {
		t.Errorf("revision should be empty")
	}

	pod.Labels = make(map[string]string, 0)
	pod.Labels[apps.StatefulSetRevisionLabel] = "bar"

	if controllerutil.GetPodRevision(pod) != "bar" {
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
						constant.RoleLabelKey: "learner",
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
			util.SortPods(tt.args.pods, tt.args.rolePriorityMap, constant.RoleLabelKey)
			if !tt.wantErr {
				assert.Equal(t, tt.args.pods, tt.want)
			}
		})
	}
}

func TestComposeRoleEnv(t *testing.T) {
	componentDef := &appsv1alpha1.ClusterComponentDefinition{
		WorkloadType: appsv1alpha1.Consensus,
		ConsensusSpec: &appsv1alpha1.ConsensusSetSpec{
			Leader: appsv1alpha1.ConsensusMember{
				Name:       "leader",
				AccessMode: appsv1alpha1.ReadWrite,
			},
			Followers: []appsv1alpha1.ConsensusMember{
				{
					Name:       "follower",
					AccessMode: appsv1alpha1.Readonly,
				},
			},
		},
	}

	set := testk8s.NewFakeStatefulSet("foo", 3)
	pods := make([]v1.Pod, 0)
	for i := 0; i < 5; i++ {
		pod := testk8s.NewFakeStatefulSetPod(set, i)
		pod.Status.Conditions = []v1.PodCondition{
			{
				Type:   v1.PodReady,
				Status: v1.ConditionTrue,
			},
		}
		pod.Labels = map[string]string{constant.RoleLabelKey: "follower"}
		pods = append(pods, *pod)
	}
	pods[0].Labels = map[string]string{constant.RoleLabelKey: "leader"}
	leader, followers := composeRoleEnv(componentDef, pods)
	assert.Equal(t, "foo-0", leader)
	assert.Equal(t, "foo-1,foo-2,foo-3,foo-4", followers)

	dt := time.Now()
	pods[3].DeletionTimestamp = &metav1.Time{Time: dt}
	pods[4].DeletionTimestamp = &metav1.Time{Time: dt}
	leader, followers = composeRoleEnv(componentDef, pods)
	assert.Equal(t, "foo-0", leader)
	assert.Equal(t, "foo-1,foo-2", followers)
}
