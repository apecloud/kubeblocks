/*
Copyright ApeCloud, Inc.
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

	apps "k8s.io/api/apps/v1"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

func TestGetParentNameAndOrdinal(t *testing.T) {
	set := testk8s.NewFakeStatefulSet("foo", 3)
	pod := testk8s.NewFakeStatefulSetPod(set, 1)
	if parent, ordinal := intctrlutil.GetParentNameAndOrdinal(pod); parent != set.Name {
		t.Errorf("Extracted the wrong parent name expected %s found %s", set.Name, parent)
	} else if ordinal != 1 {
		t.Errorf("Extracted the wrong ordinal expected %d found %d", 1, ordinal)
	}
	pod.Name = "1-bar"
	if parent, ordinal := intctrlutil.GetParentNameAndOrdinal(pod); parent != "" {
		t.Error("Expected empty string for non-member Pod parent")
	} else if ordinal != -1 {
		t.Error("Expected -1 for non member Pod ordinal")
	}
}

func TestIsMemberOf(t *testing.T) {
	set := testk8s.NewFakeStatefulSet("foo", 3)
	set2 := testk8s.NewFakeStatefulSet("bar", 3)
	set2.Name = "foo2"
	pod := testk8s.NewFakeStatefulSetPod(set, 1)
	if !IsMemberOf(set, pod) {
		t.Error("isMemberOf returned false negative")
	}
	if IsMemberOf(set2, pod) {
		t.Error("isMemberOf returned false positive")
	}
}

func TestGetPodRevision(t *testing.T) {
	set := testk8s.NewFakeStatefulSet("foo", 3)
	pod := testk8s.NewFakeStatefulSetPod(set, 1)
	if GetPodRevision(pod) != "" {
		t.Errorf("revision should be empty")
	}

	pod.Labels = make(map[string]string, 0)
	pod.Labels[apps.StatefulSetRevisionLabel] = "bar"

	if GetPodRevision(pod) != "bar" {
		t.Errorf("revision not matched")
	}
}

func TestStatefulSetPodsAreReady(t *testing.T) {
	sts := testk8s.NewFakeStatefulSet("test", 3)
	testk8s.MockStatefulSetReady(sts)
	ready := StatefulSetPodsAreReady(sts, *sts.Spec.Replicas)
	if !ready {
		t.Errorf("StatefulSet pods should be ready")
	}
	covertSts := CovertToStatefulSet(sts)
	if covertSts == nil {
		t.Errorf("Covert to statefulSet should be succeed")
	}
	covertSts = CovertToStatefulSet(&apps.Deployment{})
	if covertSts != nil {
		t.Errorf("Covert to statefulSet should be failed")
	}
	covertSts = CovertToStatefulSet(nil)
	if covertSts != nil {
		t.Errorf("Covert to statefulSet should be failed")
	}
}

func TestSStatefulSetOfComponentIsReady(t *testing.T) {
	sts := testk8s.NewFakeStatefulSet("test", 3)
	testk8s.MockStatefulSetReady(sts)
	ready := StatefulSetOfComponentIsReady(sts, true, nil)
	if !ready {
		t.Errorf("StatefulSet should be ready")
	}
	ready = StatefulSetOfComponentIsReady(sts, false, nil)
	if ready {
		t.Errorf("StatefulSet should not be ready")
	}
}
