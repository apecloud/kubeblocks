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

package configuration

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestConfigManagerHostPorts(t *testing.T) {
	for key, value := range map[string]any{
		constant.CfgKeyCtrlrMgrNS:         "default",
		constant.CfgHostPortConfigMapName: "config-manager-host-ports",
		constant.CfgHostPortIncludeRanges: "15000-15100",
		constant.CfgHostPortExcludeRanges: "",
	} {
		old := viper.Get(key)
		t.Cleanup(func() { viper.Set(key, old) })
		viper.Set(key, value)
	}
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprintf("explicit=%v", explicit), func(t *testing.T) {
			g := NewWithT(t)
			scheme := runtime.NewScheme()
			g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
			cli := fake.NewClientBuilder().WithScheme(scheme).Build()
			g.Expect(intctrlutil.InitDefaultHostPortManager(cli)).To(Succeed())
			comp := &component.SynthesizedComponent{ClusterName: "cluster", Name: "comp",
				Network: &appsv1.ComponentNetwork{HostNetwork: true, HostPorts: []appsv1.HostPort{
					{Name: "mysql", Port: 3306}, {Name: "http", Port: 15000}, {Name: "streaming", Port: 15002},
				}},
			}
			expected := int32(15001)
			if explicit {
				expected = 25004
				comp.Network.HostPorts = append(comp.Network.HostPorts, appsv1.HostPort{Name: constant.ConfigManagerPortName, Port: expected})
			}
			port, err := allocConfigManagerHostPort(comp)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(port).To(Equal(expected))
			cm := &corev1.ConfigMap{}
			g.Expect(cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "config-manager-host-ports"}, cm)).To(Succeed())
			if explicit {
				g.Expect(cm.Data).To(BeEmpty())
			} else {
				g.Expect(cm.Data).To(HaveKeyWithValue("cluster-comp-config-manager-config-manager", "15001"))
				g.Expect(intctrlutil.InitDefaultHostPortManager(cli)).To(Succeed())
				recovered, getErr := allocConfigManagerHostPort(comp)
				g.Expect(getErr).NotTo(HaveOccurred())
				g.Expect(recovered).To(Equal(port))

				comp.Network.HostPorts[1].Port = recovered
				replacement, allocErr := allocConfigManagerHostPort(comp)
				g.Expect(allocErr).NotTo(HaveOccurred())
				g.Expect(replacement).To(Equal(int32(15000)))
				g.Expect(cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "config-manager-host-ports"}, cm)).To(Succeed())
				g.Expect(cm.Data).To(HaveKeyWithValue("cluster-comp-config-manager-config-manager", "15000"))
				g.Expect(intctrlutil.InitDefaultHostPortManager(cli)).To(Succeed())
				recovered, getErr = allocConfigManagerHostPort(comp)
				g.Expect(getErr).NotTo(HaveOccurred())
				g.Expect(recovered).To(Equal(replacement))
			}
		})
	}
}
