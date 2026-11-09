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
	"strconv"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestHostPortsPreservePersistedAllocations(t *testing.T) {
	g := NewWithT(t)
	previous := defaultPortManager
	t.Cleanup(func() { defaultPortManager = previous })
	for key, value := range map[string]any{
		constant.CfgKeyCtrlrMgrNS:         "default",
		constant.CfgHostPortConfigMapName: "host-ports",
		constant.CfgHostPortIncludeRanges: "14000-14100",
		constant.CfgHostPortExcludeRanges: "",
	} {
		old := viper.Get(key)
		t.Cleanup(func() { viper.Set(key, old) })
		viper.Set(key, value)
	}
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(Succeed())
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "host-ports", Namespace: "default"},
		Data: map[string]string{
			"cluster-comp-database-db":                   "14001",
			"cluster-comp-config-manager-config-manager": "14002",
			"cluster-other-database-db":                  "14009",
		}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()
	g.Expect(InitDefaultHostPortManager(cli)).To(Succeed())
	manager := GetPortManager(nil)
	key := manager.PortKey("cluster", "comp", "database", "db")
	g.Expect(key).To(Equal("cluster-comp-database-db"))
	port, err := manager.AllocatePort(key)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(port).To(Equal(int32(14001)))

	network := &appsv1.ComponentNetwork{HostNetwork: true, HostPorts: []appsv1.HostPort{{Name: "db", Port: 24001}}}
	defined := GetPortManager(network)
	port, err = defined.AllocatePort(defined.PortKey("cluster", "comp", "database", "db"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(port).To(Equal(int32(24001)))
	_, err = defined.AllocatePort(defined.PortKey("cluster", "comp", "database", "missing"))
	g.Expect(err).To(MatchError(ContainSubstring("no available port")))

	agentPorts := map[string]int32{}
	for _, name := range []string{kbagent.DefaultHTTPPortName, kbagent.DefaultStreamingPortName} {
		agentKey := defined.PortKey("cluster", "comp", kbagent.ContainerName, name)
		allocated, allocErr := defined.AllocatePort(agentKey)
		g.Expect(allocErr).NotTo(HaveOccurred())
		g.Expect(allocated).To(BeNumerically(">=", 14000))
		g.Expect(allocated).To(BeNumerically("<=", 14100))
		agentPorts[agentKey] = allocated
	}
	g.Expect(cli.Get(context.Background(), client.ObjectKeyFromObject(cm), cm)).To(Succeed())
	g.Expect(cm.Data).To(HaveKeyWithValue(key, "14001"))
	g.Expect(cm.Data).NotTo(HaveKey("db"))
	for agentKey, allocated := range agentPorts {
		g.Expect(cm.Data).To(HaveKeyWithValue(agentKey, strconv.Itoa(int(allocated))))
	}

	// Re-initializing the real allocator must recover the old and fallback keys.
	g.Expect(InitDefaultHostPortManager(cli)).To(Succeed())
	manager = GetPortManager(nil)
	for recoveredKey, expected := range agentPorts {
		port, err = manager.AllocatePort(recoveredKey)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(port).To(Equal(expected))
	}
	port, err = manager.AllocatePort(key)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(port).To(Equal(int32(14001)))
	g.Expect(manager.ReleaseByPrefix("cluster-comp")).To(Succeed())
	g.Expect(cli.Get(context.Background(), client.ObjectKeyFromObject(cm), cm)).To(Succeed())
	g.Expect(cm.Data).To(Equal(map[string]string{"cluster-other-database-db": "14009"}))
}
