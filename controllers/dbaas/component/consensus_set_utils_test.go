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

package component

import (
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

func TestIsReady(t *testing.T) {
	set := newStatefulSet("foo", 3)
	pod := newStatefulSetPod(set, 1)
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
