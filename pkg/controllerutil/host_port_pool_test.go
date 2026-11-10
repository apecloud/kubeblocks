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
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func newHostPortTestClient(t *testing.T, includes, excludes string, data map[string]string) client.Client {
	t.Helper()
	previousManager := defaultPortManager
	t.Cleanup(func() { defaultPortManager = previousManager })
	for key, value := range map[string]string{
		constant.CfgKeyCtrlrMgrNS:         "default",
		constant.CfgHostPortConfigMapName: "test-host-ports",
		constant.CfgHostPortIncludeRanges: includes,
		constant.CfgHostPortExcludeRanges: excludes,
	} {
		previous := viper.Get(key)
		viper.Set(key, value)
		t.Cleanup(func() { viper.Set(key, previous) })
	}
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "test-host-ports"},
		Data:       data,
	}).Build()
}

func TestHostPortPoolRangeChange(t *testing.T) {
	data := make(map[string]string)
	for port := 33000; port <= 35225; port++ {
		data[fmt.Sprintf("old-%d", port)] = fmt.Sprint(port)
	}
	cli := newHostPortTestClient(t, "1025-65536", "6443,10250,10257,10259,2379-2380,30000-32767", data)
	if err := InitDefaultHostPortManager(cli); err != nil {
		t.Fatal(err)
	}
	if port, err := defaultPortManager.AllocatePort("old-33000"); err != nil || port != 33000 {
		t.Fatalf("historical allocation = %d, %v", port, err)
	}
	if port, err := defaultPortManager.AllocatePort("old-range-new"); err != nil || port != 1025 {
		t.Fatalf("old range allocation = %d, %v", port, err)
	}
	viper.Set(constant.CfgHostPortIncludeRanges, "55000-59999")
	for restart := 0; restart < 2; restart++ {
		if err := InitDefaultHostPortManager(cli); err != nil {
			t.Fatal(err)
		}
		if port, err := defaultPortManager.AllocatePort("old-33000"); err != nil || port != 33000 {
			t.Fatalf("historical allocation after range change = %d, %v", port, err)
		}
		if port, err := defaultPortManager.AllocatePort("new-range"); err != nil || port != 55000 {
			t.Fatalf("new range allocation = %d, %v", port, err)
		}
		if err := defaultPortManager.UsePort("excluded", 6443); err == nil {
			t.Fatal("out-of-range exclusion was lost")
		}
	}
	cm := &corev1.ConfigMap{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "test-host-ports"}, cm); err != nil {
		t.Fatal(err)
	}
	for key, value := range data {
		if cm.Data[key] != value {
			t.Fatalf("historical record %s changed to %q", key, cm.Data[key])
		}
	}
}

func TestHostPortPoolExhaustionAndRelease(t *testing.T) {
	cli := newHostPortTestClient(t, "15000-15002", "15001,3306", map[string]string{"historical": "33000"})
	if err := InitDefaultHostPortManager(cli); err != nil {
		t.Fatal(err)
	}
	for i, expected := range []int32{15000, 15002} {
		if port, err := defaultPortManager.AllocatePort(fmt.Sprintf("new-%d", i)); err != nil || port != expected {
			t.Fatalf("allocation %d = %d, %v; want %d", i, port, err, expected)
		}
	}
	for restart := 0; restart < 2; restart++ {
		if _, err := defaultPortManager.AllocatePort("full"); err == nil {
			t.Fatal("expected exhaustion")
		}
		if err := InitDefaultHostPortManager(cli); err != nil {
			t.Fatal(err)
		}
	}
	if err := defaultPortManager.ReleaseByPrefix("new-0"); err != nil {
		t.Fatal(err)
	}
	if port, err := defaultPortManager.AllocatePort("replacement"); err != nil || port != 15000 {
		t.Fatalf("released allocation = %d, %v", port, err)
	}
}
