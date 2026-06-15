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
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestGetPodListByInstanceSet(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := workloads.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "match", Namespace: "ns", Labels: map[string]string{"app": "mysql"}}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "ns", Labels: map[string]string{"app": "pg"}}},
	).Build()
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "mysql", Namespace: "ns"},
		Spec: workloads.InstanceSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "mysql"}},
		},
	}

	pods, err := GetPodListByInstanceSet(context.Background(), cli, its)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pods) != 1 || pods[0].Name != "match" {
		t.Fatalf("expected only matching pod, got %#v", pods)
	}
}

func TestWorkloadFQDNAndRuntimeMetrics(t *testing.T) {
	defer func() {
		viper.Set(constant.KubernetesClusterDomainEnv, "")
		viper.Set(FeatureGateEnableRuntimeMetrics, false)
	}()

	viper.Set(constant.KubernetesClusterDomainEnv, "cluster.local")
	if got := PodFQDN("default", "mysql", "mysql-0"); got != "mysql-0.mysql-headless.default.svc.cluster.local" {
		t.Fatalf("unexpected pod fqdn %q", got)
	}
	if got := ServiceFQDN("default", "mysql"); got != "mysql.default.svc.cluster.local" {
		t.Fatalf("unexpected service fqdn %q", got)
	}

	viper.Set(FeatureGateEnableRuntimeMetrics, true)
	if !EnabledRuntimeMetrics() {
		t.Fatalf("expected runtime metrics to be enabled")
	}
}
