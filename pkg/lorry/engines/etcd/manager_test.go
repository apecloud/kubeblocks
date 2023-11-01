/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package etcd

import (
	"context"
	"fmt"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
)

const (
	urlWithPort = "127.0.0.1:2379"
)

// Test case for Init() function
var _ = Describe("ETCD DBManager", func() {
	// Set up relevant viper config variables
	viper.Set("KB_SERVICE_USER", "testuser")
	viper.Set("KB_SERVICE_PASSWORD", "testpassword")
	Context("new db manager", func() {
		It("with rigth configurations", func() {
			properties := engines.Properties{
				"endpoint": urlWithPort,
			}
			dbManger, err := NewManager(properties)
			Expect(err).Should(Succeed())
			Expect(dbManger).ShouldNot(BeNil())
		})

		It("with wrong configurations", func() {
			properties := engines.Properties{}
			dbManger, err := NewManager(properties)
			Expect(err).Should(HaveOccurred())
			Expect(dbManger).Should(BeNil())
		})
	})

	Context("is db startup ready", func() {
		It("it is ready", func() {
			etcdServer, err := StartEtcdServer()
			Expect(err).Should(BeNil())
			defer etcdServer.Stop()
			testEndpoint := fmt.Sprintf("http://%s", etcdServer.ETCD.Clients[0].Addr().(*net.TCPAddr).String())
			manager := &Manager{
				etcd:     etcdServer.client,
				endpoint: testEndpoint,
			}
			Expect(manager.IsDBStartupReady()).Should(BeTrue())
		})

		It("it is not ready", func() {
			etcdServer, err := StartEtcdServer()
			Expect(err).Should(BeNil())
			etcdServer.Stop()
			testEndpoint := fmt.Sprintf("http://%s", etcdServer.ETCD.Clients[0].Addr().(*net.TCPAddr).String())
			properties := engines.Properties{
				"endpoint": testEndpoint,
			}
			manager, err := NewManager(properties)
			Expect(err).Should(BeNil())
			Expect(manager).ShouldNot(BeNil())
			Expect(manager.IsDBStartupReady()).Should(BeFalse())
		})
	})

	Context("get replica role", func() {
		It("get leader", func() {
			etcdServer, err := StartEtcdServer()
			Expect(err).Should(BeNil())
			defer etcdServer.Stop()
			testEndpoint := fmt.Sprintf("http://%s", etcdServer.ETCD.Clients[0].Addr().(*net.TCPAddr).String())
			manager := &Manager{
				etcd:     etcdServer.client,
				endpoint: testEndpoint,
			}
			role, err := manager.GetReplicaRole(context.Background(), nil)
			Expect(err).Should(BeNil())
			Expect(role).Should(Equal("Leader"))
		})
	})
})
