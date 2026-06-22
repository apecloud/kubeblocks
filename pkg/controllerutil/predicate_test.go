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

package controllerutil

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestAPIVersionSupported(t *testing.T) {
	defer viper.Set(constant.APIVersionSupported, "")

	if !IsAPIVersionSupported(appsv1.GroupVersion.String()) {
		t.Fatalf("expected built-in apps api version to be supported")
	}
	if IsAPIVersionSupported("example.io/v1") {
		t.Fatalf("unexpected support for arbitrary api version")
	}

	viper.Set(constant.APIVersionSupported, `^example\.io/v[0-9]+$`)
	if !IsAPIVersionSupported("example.io/v2") {
		t.Fatalf("expected configured api version regex to match")
	}
}

func TestObjectAPIVersionSupported(t *testing.T) {
	cluster := &appsv1.Cluster{}
	if !ObjectAPIVersionSupported(cluster) {
		t.Fatalf("cluster without api version annotation should be accepted for api version resolution")
	}

	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Annotations: map[string]string{constant.CRDAPIVersionAnnotationKey: appsv1.GroupVersion.String()},
	}}
	if !ObjectAPIVersionSupported(pod) {
		t.Fatalf("object with supported api version annotation should be accepted")
	}

	pod.Annotations[constant.CRDAPIVersionAnnotationKey] = "example.io/v1"
	if ObjectAPIVersionSupported(pod) {
		t.Fatalf("object with unsupported api version annotation should be rejected")
	}
}

func TestNamespacePredicateFilter(t *testing.T) {
	namespacesKey := strings.ReplaceAll(constant.ManagedNamespacesFlag, "-", "_")
	defer func() {
		managedNamespaces = nil
		viper.Set(namespacesKey, "")
	}()

	managedNamespaces = nil
	viper.Set(namespacesKey, "ns-a,ns-b")
	if !namespacePredicateFilter(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns-a"}}) {
		t.Fatalf("expected configured namespace to pass")
	}
	if namespacePredicateFilter(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns-c"}}) {
		t.Fatalf("expected non-configured namespace to be filtered")
	}
	if !namespacePredicateFilter(&corev1.Node{}) {
		t.Fatalf("cluster-scoped objects should pass namespace filter")
	}

	managedNamespaces = &sets.Set[string]{}
	if !namespacePredicateFilter(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns-c"}}) {
		t.Fatalf("empty managed namespace set should allow all namespaces")
	}
}
