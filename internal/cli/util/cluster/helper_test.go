/*
Copyright ApeCloud Inc.

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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/util/fake"
)

var _ = Describe("helper", func() {
	It("Get default pod name from cluster", func() {
		cluster := fake.Cluster("test", "test")
		dynamic := fake.NewDynamicClient(cluster)
		name, err := GetDefaultPodName(dynamic, "test", "test")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(len(name) > 0).Should(BeTrue())
	})

	It("Get type from pod", func() {
		mockPod := func(name string) *corev1.Pod {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "foo",
					Namespace:       "test",
					ResourceVersion: "10",
					Labels: map[string]string{
						"app.kubernetes.io/name": name,
					},
				},
			}
			return pod
		}

		pod := mockPod("state.mysql-apecloud-wesql")
		typeName, err := GetClusterTypeByPod(pod)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(typeName).Should(Equal("state.mysql"))

		pod = mockPod("")
		typeName, err = GetClusterTypeByPod(pod)
		Expect(err).Should(HaveOccurred())
		Expect(typeName).Should(Equal(""))
	})

	It("find component in cluster by type name", func() {
		cluster := fake.Cluster("test", "test")
		component := FindCompInCluster(cluster, "test")
		Expect(component).Should(BeNil())

		component = FindCompInCluster(cluster, fake.ComponentType)
		Expect(component).ShouldNot(BeNil())
	})

	It("get all clusters", func() {
		cluster := fake.Cluster("test", "test")
		dynamic := fake.NewDynamicClient(cluster)
		clusters := &dbaasv1alpha1.ClusterList{}

		By("get clusters from specified namespace")
		Expect(GetAllCluster(dynamic, "test", clusters)).ShouldNot(HaveOccurred())
		Expect(len(clusters.Items)).Should(Equal(1))

		By("get clusters from nonexistent namespace")
		Expect(GetAllCluster(dynamic, "nonexistent", clusters)).ShouldNot(HaveOccurred())
		Expect(len(clusters.Items)).Should(Equal(0))

		By("get clusters from all namespace")
		anotherCluster := fake.Cluster("test", "test1")
		dynamic = fake.NewDynamicClient(cluster, anotherCluster)
		Expect(GetAllCluster(dynamic, "", clusters)).ShouldNot(HaveOccurred())
		Expect(len(clusters.Items)).Should(Equal(2))
	})

	It("get cluster endpoints", func() {
		cluster := fake.Cluster("test", "test")
		svcs := fake.Services()
		internalEPs, externalEPs := GetClusterEndpoints(svcs, &cluster.Spec.Components[0])
		Expect(len(internalEPs)).Should(Equal(3))
		Expect(len(externalEPs)).Should(Equal(1))
	})

	It("fake cluster objects", func() {
		objs := FakeClusterObjs()
		Expect(objs).ShouldNot(BeNil())
	})
})
