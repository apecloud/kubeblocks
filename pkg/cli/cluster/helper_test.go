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

package cluster

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/cli/testing"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

var _ = Describe("helper", func() {
	It("Get instance info from cluster", func() {
		cluster := testing.FakeCluster("test", "test")
		dynamic := testing.FakeDynamicClient(cluster)
		infos := GetSimpleInstanceInfos(dynamic, "test", "test")
		Expect(len(infos) == 1).Should(BeTrue())
	})

	It("find component in cluster by name", func() {
		cluster := testing.FakeCluster("test", "test")
		component := FindClusterComp(cluster, "test")
		Expect(component).Should(BeNil())

		component = FindClusterComp(cluster, testing.ComponentDefName)
		Expect(component).ShouldNot(BeNil())
	})

	It("get cluster endpoints", func() {
		cluster := testing.FakeCluster("test", "test")
		svcs := testing.FakeServices()
		internalEPs, externalEPs := GetComponentEndpoints(svcs, &cluster.Spec.ComponentSpecs[0])
		Expect(len(internalEPs)).Should(Equal(3))
		Expect(len(externalEPs)).Should(Equal(1))
	})

	It("fake cluster objects", func() {
		objs := FakeClusterObjs()
		Expect(objs).ShouldNot(BeNil())
	})

	It("get cluster cluster", func() {
		dynamic := testing.FakeDynamicClient(testing.FakeCluster("test", "test"))
		c, err := GetClusterByName(dynamic, "test", "test")
		Expect(err).Should(Succeed())
		Expect(c).ShouldNot(BeNil())
	})

	It("get cluster definition", func() {
		dynamic := testing.FakeDynamicClient(testing.FakeClusterDef())
		clusterDef, err := GetClusterDefByName(dynamic, testing.ClusterDefName)
		Expect(err).Should(Succeed())
		Expect(clusterDef).ShouldNot(BeNil())
	})

	It("get version by cluster def", func() {
		dynamic := testing.FakeDynamicClient(testing.FakeClusterVersion())
		version, err := GetVersionByClusterDef(dynamic, testing.ClusterDefName)
		Expect(err).Should(Succeed())
		Expect(version).ShouldNot(BeNil())
		Expect(version.Items).Should(HaveLen(1))
	})

	It("get default version", func() {
		const clusterDefName = "test-cluster-def"
		genVersion := func(name string) *appsv1alpha1.ClusterVersion {
			v := &appsv1alpha1.ClusterVersion{}
			v.Name = name
			v.SetLabels(map[string]string{constant.ClusterDefLabelKey: clusterDefName})
			return v
		}

		cv1 := genVersion("version1")
		cv2 := genVersion("version2")

		By("no default version, should throw error")
		dynamic := testing.FakeDynamicClient(testing.FakeClusterVersion(), cv1, cv2)
		defaultVer, err := GetDefaultVersion(dynamic, clusterDefName)
		Expect(err).Should(HaveOccurred())
		Expect(defaultVer).Should(BeEmpty())

		By("set default version, should return default version")
		cv1.Annotations = map[string]string{constant.DefaultClusterVersionAnnotationKey: "true"}
		dynamic = testing.FakeDynamicClient(testing.FakeClusterVersion(), cv1, cv2)
		defaultVer, err = GetDefaultVersion(dynamic, clusterDefName)
		Expect(err).Should(Succeed())
		Expect(defaultVer).Should(Equal(cv1.Name))
	})

	It("get configmap by name", func() {
		cmName := "test-cm"
		dynamic := testing.FakeDynamicClient(testing.FakeConfigMap(cmName, testing.Namespace, map[string]string{"fake": "fake"}))
		cm, err := GetConfigMapByName(dynamic, testing.Namespace, cmName)
		Expect(err).Should(Succeed())
		Expect(cm).ShouldNot(BeNil())

		cm, err = GetConfigMapByName(dynamic, testing.Namespace, cmName+"error")
		Expect(err).Should(HaveOccurred())
		Expect(cm).Should(BeNil())
	})

	It("get config constraint by name", func() {
		ccName := "test-cc"
		dynamic := testing.FakeDynamicClient(testing.FakeConfigConstraint(ccName))
		cm, err := GetConfigConstraintByName(dynamic, ccName)
		Expect(err).Should(Succeed())
		Expect(cm).ShouldNot(BeNil())
	})

	It("get all ServiceRefs from cluster-definition", func() {
		Expect(GetServiceRefs(testing.FakeClusterDef())).Should(Equal([]string{testing.ServiceRefName}))
	})

	It("get default ServiceRef from cluster-definition", func() {
		cd := testing.FakeClusterDef()
		ref, err := GetDefaultServiceRef(cd)
		Expect(ref).Should(Equal(testing.ServiceRefName))
		Expect(err).Should(Succeed())

		deepCopyCD := cd.DeepCopy()
		deepCopyCD.Spec.ComponentDefs[0].ServiceRefDeclarations = append(deepCopyCD.Spec.ComponentDefs[0].ServiceRefDeclarations, testing.FakeServiceRef("other-serviceRef"))
		_, err = GetDefaultServiceRef(deepCopyCD)
		Expect(err).Should(HaveOccurred())

		deepCopyCD = cd.DeepCopy()
		deepCopyCD.Spec.ComponentDefs[0].ServiceRefDeclarations = nil
		_, err = GetDefaultServiceRef(deepCopyCD)
		Expect(err).Should(HaveOccurred())
	})
})
