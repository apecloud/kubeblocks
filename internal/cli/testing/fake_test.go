/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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

	It("fake storageClass", func() {
		StorageClassDefault := FakeStorageClass(StorageClassName, ISDefautl)
		Expect(StorageClassDefault).ShouldNot(BeNil())
		StorageClassNotDefault := FakeStorageClass(StorageClassName, ISDefautl)
		Expect(StorageClassNotDefault).ShouldNot(BeNil())
	})
})
