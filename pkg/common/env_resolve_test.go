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

package common

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestGetFieldRef(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "pod-0",
			Namespace:   "default",
			UID:         types.UID("uid-1"),
			Labels:      map[string]string{"app": "mysql", "role": "leader"},
			Annotations: map[string]string{"config": "enabled"},
		},
		Spec: corev1.PodSpec{
			NodeName:           "node-1",
			ServiceAccountName: "kb-sa",
		},
		Status: corev1.PodStatus{
			HostIP: "192.168.0.1",
			PodIP:  "10.0.0.1",
			PodIPs: []corev1.PodIP{{IP: "10.0.0.1"}, {IP: "fd00::1"}},
		},
	}

	cases := []struct {
		name      string
		fieldPath string
		want      string
		wantErr   bool
	}{
		{name: "annotation subscript", fieldPath: "metadata.annotations['config']", want: "enabled"},
		{name: "label subscript", fieldPath: "metadata.labels['role']", want: "leader"},
		{name: "annotations map is sorted", fieldPath: "metadata.annotations", want: `config="enabled"`},
		{name: "labels map is sorted", fieldPath: "metadata.labels", want: "app=\"mysql\"\nrole=\"leader\""},
		{name: "name", fieldPath: "metadata.name", want: "pod-0"},
		{name: "namespace", fieldPath: "metadata.namespace", want: "default"},
		{name: "uid", fieldPath: "metadata.uid", want: "uid-1"},
		{name: "node name", fieldPath: "spec.nodeName", want: "node-1"},
		{name: "service account", fieldPath: "spec.serviceAccountName", want: "kb-sa"},
		{name: "host ip", fieldPath: "status.hostIP", want: "192.168.0.1"},
		{name: "pod ip", fieldPath: "status.podIP", want: "10.0.0.1"},
		{name: "pod ips", fieldPath: "status.podIPs", want: "10.0.0.1,fd00::1"},
		{name: "unsupported field", fieldPath: "spec.unsupported", wantErr: true},
		{name: "unsupported subscript", fieldPath: "spec.containers['main']", wantErr: true},
		{name: "invalid label key", fieldPath: "metadata.labels['bad/key/again']", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := GetFieldRef(pod, &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: tc.fieldPath},
			})
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestSplitMaybeSubscriptedPath(t *testing.T) {
	cases := []struct {
		input     string
		wantPath  string
		wantKey   string
		wantFound bool
	}{
		{input: "metadata.annotations['myKey']", wantPath: "metadata.annotations", wantKey: "myKey", wantFound: true},
		{input: "metadata.annotations['a[b]c']", wantPath: "metadata.annotations", wantKey: "a[b]c", wantFound: true},
		{input: "metadata.labels", wantPath: "metadata.labels"},
		{input: "['missingPath']", wantPath: "['missingPath']"},
		{input: "metadata.labels['unterminated", wantPath: "metadata.labels['unterminated"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			path, key, found := splitMaybeSubscriptedPath(tc.input)
			if path != tc.wantPath || key != tc.wantKey || found != tc.wantFound {
				t.Fatalf("expected (%q, %q, %v), got (%q, %q, %v)",
					tc.wantPath, tc.wantKey, tc.wantFound, path, key, found)
			}
		})
	}
}
