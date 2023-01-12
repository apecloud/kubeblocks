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
	"testing"
	"time"

	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
		consensusSetStatus.Leader.Pod != ConsensusSetStatusDefaultPodName {
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
