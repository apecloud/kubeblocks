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
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestNamespacePredicateFilter(t *testing.T) {
	key := strings.ReplaceAll(constant.ManagedNamespacesFlag, "-", "_")
	previous := viper.Get(key)
	t.Cleanup(func() {
		viper.Set(key, previous)
		managedNamespaces = nil
		managedNamespacesOnce = sync.Once{}
	})

	for _, tc := range []struct {
		name        string
		namespaces  string
		wantOutside bool
	}{
		{name: "all namespaces", wantOutside: true},
		{name: "configured namespaces", namespaces: "allowed,another", wantOutside: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			managedNamespaces = nil
			managedNamespacesOnce = sync.Once{}
			viper.Set(key, tc.namespaces)
			objects := []struct {
				object client.Object
				want   bool
			}{
				{&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "allowed"}}, true},
				{&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "another"}}, true},
				{&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "outside"}}, tc.wantOutside},
				{&corev1.Node{}, true},
			}
			check := func() {
				for _, obj := range objects {
					if got := namespacePredicateFilter(obj.object); got != obj.want {
						t.Errorf("namespace %q: got %t, want %t", obj.object.GetNamespace(), got, obj.want)
					}
				}
			}

			var wg sync.WaitGroup
			start := make(chan struct{})
			for range 32 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					check()
				}()
			}
			close(start)
			wg.Wait()

			viper.Set(key, "outside")
			check()
		})
	}
}
