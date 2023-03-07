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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	intctrlutil "github.com/apecloud/kubeblocks/internal/constant"
)

var _ = Describe("helper", func() {
	It("Get instance info from cluster", func() {
		cluster := testing.FakeCluster("test", "test")
		dynamic := testing.FakeDynamicClient(cluster)
		infos := GetSimpleInstanceInfos(dynamic, "test", "test")
		Expect(len(infos) == 1).Should(BeTrue())
	})

	It("Get type from pod", func() {
		mockPod := func(name string) *corev1.Pod {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "foo",
					Namespace:       "test",
					ResourceVersion: "10",
					Labels: map[string]string{
						intctrlutil.AppNameLabelKey: name,
					},
				},
			}
			return pod
		}

		pod := mockPod("mysql-apecloud-mysql")
		compDefName, err := GetClusterTypeByPod(pod)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(compDefName).Should(Equal("mysql"))

		pod = mockPod("")
		compDefName, err = GetClusterTypeByPod(pod)
		Expect(err).Should(HaveOccurred())
		Expect(compDefName).Should(Equal(""))
	})

	It("find component in cluster by name", func() {
		cluster := testing.FakeCluster("test", "test")
		component := FindClusterComp(cluster, "test")
		Expect(component).Should(BeNil())

		component = FindClusterComp(cluster, testing.ComponentDefName)
		Expect(component).ShouldNot(BeNil())
	})

	It("get all clusters", func() {
		cluster := testing.FakeCluster("test", "test")
		dynamic := testing.FakeDynamicClient(cluster)
		clusters := &appsv1alpha1.ClusterList{}

		By("get clusters from specified namespace")
		Expect(GetAllCluster(dynamic, "test", clusters)).ShouldNot(HaveOccurred())
		Expect(len(clusters.Items)).Should(Equal(1))

		By("get clusters from nonexistent namespace")
		Expect(GetAllCluster(dynamic, "nonexistent", clusters)).ShouldNot(HaveOccurred())
		Expect(len(clusters.Items)).Should(Equal(0))

		By("get clusters from all namespace")
		anotherCluster := testing.FakeCluster("test", "test1")
		dynamic = testing.FakeDynamicClient(cluster, anotherCluster)
		Expect(GetAllCluster(dynamic, "", clusters)).ShouldNot(HaveOccurred())
		Expect(len(clusters.Items)).Should(Equal(2))
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
			v.SetLabels(map[string]string{intctrlutil.ClusterDefLabelKey: clusterDefName})
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
})
