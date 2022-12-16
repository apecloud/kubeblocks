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

package component

import (
	"fmt"
	"testing"

	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetParentNameAndOrdinal(t *testing.T) {
	set := newStatefulSet("foo", 3)
	pod := newStatefulSetPod(set, 1)
	if parent, ordinal := GetParentNameAndOrdinal(pod); parent != set.Name {
		t.Errorf("Extracted the wrong parent name expected %s found %s", set.Name, parent)
	} else if ordinal != 1 {
		t.Errorf("Extracted the wrong ordinal expected %d found %d", 1, ordinal)
	}
	pod.Name = "1-bar"
	if parent, ordinal := GetParentNameAndOrdinal(pod); parent != "" {
		t.Error("Expected empty string for non-member Pod parent")
	} else if ordinal != -1 {
		t.Error("Expected -1 for non member Pod ordinal")
	}
}

func TestIsMemberOf(t *testing.T) {
	set := newStatefulSet("foo", 3)
	set2 := newStatefulSet("bar", 3)
	set2.Name = "foo2"
	pod := newStatefulSetPod(set, 1)
	if !IsMemberOf(set, pod) {
		t.Error("isMemberOf returned false negative")
	}
	if IsMemberOf(set2, pod) {
		t.Error("isMemberOf returned false positive")
	}
}

func TestGetPodRevision(t *testing.T) {
	set := newStatefulSet("foo", 3)
	pod := newStatefulSetPod(set, 1)
	if GetPodRevision(pod) != "" {
		t.Errorf("revision should be empty")
	}

	pod.Labels = make(map[string]string, 0)
	pod.Labels[apps.StatefulSetRevisionLabel] = "bar"

	if GetPodRevision(pod) != "bar" {
		t.Errorf("revision not matched")
	}
}

func newStatefulSet(name string, replicas int) *apps.StatefulSet {
	template := v1.PodTemplateSpec{
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "nginx",
					Image: "nginx",
				},
			},
		},
	}

	template.Labels = map[string]string{"foo": "bar"}

	return &apps.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: v1.NamespaceDefault,
		},
		Spec: apps.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"foo": "bar"},
			},
			Replicas:    func() *int32 { i := int32(replicas); return &i }(),
			Template:    template,
			ServiceName: "governingsvc",
		},
	}
}

func newStatefulSetPod(set *apps.StatefulSet, ordinal int) *v1.Pod {
	pod := &v1.Pod{}
	pod.Name = fmt.Sprintf("%s-%d", set.Name, ordinal)

	return pod
}
