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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestInternalHostPortReallocation(t *testing.T) {
	for _, failure := range []string{"conflict", "exhaustion"} {
		t.Run(failure, func(t *testing.T) {
			previousManager := defaultPortManager
			t.Cleanup(func() { defaultPortManager = previousManager })
			for key, value := range map[string]string{
				constant.CfgKeyCtrlrMgrNS:         "default",
				constant.CfgHostPortConfigMapName: "test-host-ports",
				constant.CfgHostPortIncludeRanges: "15000-15003",
				constant.CfgHostPortExcludeRanges: "",
			} {
				previous := viper.Get(key)
				viper.Set(key, value)
				t.Cleanup(func() { viper.Set(key, previous) })
			}
			scheme := runtime.NewScheme()
			if err := corev1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).Build()
			if err := InitDefaultHostPortManager(cli); err != nil {
				t.Fatal(err)
			}
			key := defaultPortManager.PortKey("cluster", "comp", "kbagent", "streaming")
			if port, err := defaultPortManager.AllocatePort(key); err != nil || port != 15000 {
				t.Fatalf("initial allocation = %d, %v", port, err)
			}
			network := &appsv1.ComponentNetwork{HostNetwork: true, HostPorts: []appsv1.HostPort{{Name: "http", Port: 15000}}}
			cmKey := client.ObjectKey{Namespace: "default", Name: "test-host-ports"}
			if failure == "conflict" {
				cm := &corev1.ConfigMap{}
				if err := cli.Get(context.Background(), cmKey, cm); err != nil {
					t.Fatal(err)
				}
				cm.Data["other"] = "15002"
				if err := cli.Update(context.Background(), cm); err != nil {
					t.Fatal(err)
				}
			} else {
				network.HostPorts = append(network.HostPorts, appsv1.HostPort{Name: "mysql", Port: 15001},
					appsv1.HostPort{Name: "metrics", Port: 15002}, appsv1.HostPort{Name: "admin", Port: 15003})
			}
			_, err := GetPortManager(network).AllocatePort(key)
			if err == nil || (failure == "conflict" && !apierrors.IsConflict(err)) {
				t.Fatalf("expected %s failure, got %v", failure, err)
			}
			cm := &corev1.ConfigMap{}
			if err := cli.Get(context.Background(), cmKey, cm); err != nil {
				t.Fatal(err)
			}
			if cm.Data[key] != "15000" {
				t.Fatalf("failed replacement changed historical record to %q", cm.Data[key])
			}
			if failure == "exhaustion" {
				network.HostPorts = network.HostPorts[:1]
			}
			if port, err := GetPortManager(network).AllocatePort(key); err != nil || port != 15001 {
				t.Fatalf("retry allocation = %d, %v", port, err)
			}
			if err := defaultPortManager.UsePort("reuse-old", 15000); err != nil {
				t.Fatalf("old reservation was not released: %v", err)
			}
			if err := InitDefaultHostPortManager(cli); err != nil {
				t.Fatal(err)
			}
			if port, err := GetPortManager(network).AllocatePort(key); err != nil || port != 15001 {
				t.Fatalf("allocation after restart = %d, %v", port, err)
			}
		})
	}
}
