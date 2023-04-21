/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package testing

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("test fake", func() {
	It("cluster", func() {
		cluster := FakeCluster(ClusterName, Namespace)
		Expect(cluster).ShouldNot(BeNil())
		Expect(cluster.Name).Should(Equal(ClusterName))
	})

	It("cluster definition", func() {
		clusterDef := FakeClusterDef()
		Expect(clusterDef).ShouldNot(BeNil())
		Expect(clusterDef.Name).Should(Equal(ClusterDefName))
	})

	It("cluster definition", func() {
		clusterVersion := FakeClusterVersion()
		Expect(clusterVersion).ShouldNot(BeNil())
		Expect(clusterVersion.Name).Should(Equal(ClusterVersionName))
	})

	It("pods", func() {
		pods := FakePods(3, Namespace, ClusterName)
		Expect(pods).ShouldNot(BeNil())
		Expect(len(pods.Items)).Should(Equal(3))
	})

	It("secrets", func() {
		secrets := FakeSecrets(Namespace, ClusterName)
		Expect(secrets).ShouldNot(BeNil())
		Expect(len(secrets.Items)).Should(Equal(1))
	})

	It("services", func() {
		svcs := FakeServices()
		Expect(svcs).ShouldNot(BeNil())
		Expect(len(svcs.Items)).Should(Equal(4))
	})

	It("node", func() {
		node := FakeNode()
		Expect(node).ShouldNot(BeNil())
		Expect(node.Name).Should(Equal(NodeName))
	})

	It("fake client set", func() {
		client := FakeClientSet()
		Expect(client).ShouldNot(BeNil())
	})

	It("fake dynamic set", func() {
		dynamic := FakeDynamicClient()
		Expect(dynamic).ShouldNot(BeNil())
	})

	It("fake PVCs", func() {
		pvcs := FakePVCs()
		Expect(pvcs).ShouldNot(BeNil())
	})

	It("fake events", func() {
		events := FakeEvents()
		Expect(events).ShouldNot(BeNil())
	})
})
