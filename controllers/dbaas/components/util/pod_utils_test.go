/*
Copyright ApeCloud Inc.
Copyright 2016 The Kubernetes Authors.

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

package util

import (
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

func TestPodIsReady(t *testing.T) {
	set := testk8s.NewFakeStatefulSet("foo", 3)
	pod := testk8s.NewFakeStatefulSetPod(set, 1)

	testk8s.DeletePodLabelKey(ctx, testCtx, pod.Name, intctrlutil.RoleLabelKey)
	if podIsReady := PodIsReady(*pod); podIsReady {
		t.Errorf("Extracted the wrong parent name expected %t found %t", false, podIsReady)
	}

	pod.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	if PodIsReady(*pod) {
		t.Errorf("isReady returned false positive")
	}

	pod.Labels = nil
	if PodIsReady(*pod) {
		t.Errorf("isReady returned false positive")
	}

	pod.Status.Conditions = nil
	if PodIsReady(*pod) {
		t.Errorf("isReady returned false positive")
	}

	pod.Status.Conditions = []v1.PodCondition{}
	if PodIsReady(*pod) {
		t.Errorf("isReady returned false positive")
	}
}
