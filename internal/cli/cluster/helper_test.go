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

package cluster

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/constant"
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
		oldVersion := testing.FakeClusterVersion()
		oldVersion.Name = "test-old-version"
		oldVersion.SetCreationTimestamp(metav1.NewTime(time.Now().AddDate(0, 0, -1)))
		dynamic := testing.FakeDynamicClient(testing.FakeClusterVersion(), oldVersion)
		version, err := GetVersionByClusterDef(dynamic, testing.ClusterDefName)
		Expect(err).Should(Succeed())
		Expect(version).ShouldNot(BeNil())
		Expect(len(version.Items)).Should(Equal(2))
	})

	It("find latest version", func() {
		const clusterDefName = "test-cluster-def"
		genVersion := func(name string, t time.Time) appsv1alpha1.ClusterVersion {
			v := appsv1alpha1.ClusterVersion{}
			v.Name = name
			v.SetLabels(map[string]string{constant.ClusterDefLabelKey: clusterDefName})
			v.SetCreationTimestamp(metav1.NewTime(t))
			return v
		}

		versionList := &appsv1alpha1.ClusterVersionList{}
		versionList.Items = append(versionList.Items,
			genVersion("old-version", time.Now().AddDate(0, 0, -1)),
			genVersion("now-version", time.Now()))

		latestVer := findLatestVersion(versionList)
		Expect(latestVer).ShouldNot(BeNil())
		Expect(latestVer.Name).Should(Equal("now-version"))
	})

	It("get configmap by name", func() {
		cmName := "test-cm"
		dynamic := testing.FakeDynamicClient(testing.FakeConfigMap(cmName))
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

	It("get all storage classes and identify if there is a default storage class", func() {
		dynamic := testing.FakeDynamicClient(testing.FakeStorageClass("test", testing.ISDefautl))
		classes, ok, err := GetStorageClasses(dynamic)
		Expect(classes).To(HaveKey("test"))
		Expect(ok).Should(BeTrue())
		Expect(err).Should(Succeed())
		dynamic = testing.FakeDynamicClient(testing.FakeStorageClass("test2", testing.IsNotDefault))
		classes, ok, err = GetStorageClasses(dynamic)
		Expect(classes).ToNot(HaveKey("test"))
		Expect(ok).ShouldNot(BeTrue())
		Expect(err).Should(Succeed())
	})
})
