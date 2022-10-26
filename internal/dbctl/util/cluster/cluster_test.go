/*
Copyright 2022 The KubeBlocks Authors

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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/dbctl/types"
)

var _ = Describe("util", func() {
	It("Get type from pod", func() {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "foo",
				Namespace:       "test",
				ResourceVersion: "10",
				Labels: map[string]string{
					"app.kubernetes.io/name": "state.mysql-apecloud-wesql",
				},
			},
		}
		typeName, err := GetClusterTypeByPod(pod)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(typeName).Should(Equal("state.mysql"))

		pod.Labels = map[string]string{}
		typeName, err = GetClusterTypeByPod(pod)
		Expect(err).Should(HaveOccurred())
		Expect(typeName).Should(Equal(""))
	})

	It("get default pod from cluster", func() {
		const (
			podName     = "test-custer-leader"
			clusterName = "test-cluster"
			namespace   = "test"
		)

		cluster := &dbaasv1alpha1.Cluster{
			TypeMeta: metav1.TypeMeta{
				Kind:       types.KindCluster,
				APIVersion: "dbaas.infracreate.com/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterName,
				Namespace: namespace,
			},
			Spec: dbaasv1alpha1.ClusterSpec{},
			Status: dbaasv1alpha1.ClusterStatus{
				Components: map[string]*dbaasv1alpha1.ClusterStatusComponent{
					"test-component": {
						Type: (string)(dbaasv1alpha1.Consensus),
						ConsensusSetStatus: &dbaasv1alpha1.ConsensusSetStatus{
							Leader: dbaasv1alpha1.ConsensusMemberStatus{
								Pod: podName,
							},
						},
					},
				},
			},
		}
		client := fake.NewSimpleDynamicClient(runtime.NewScheme(), cluster)
		pod, err := GetDefaultPodName(client, clusterName, namespace)
		Expect(pod).Should(Equal(podName))
		Expect(err).ShouldNot(HaveOccurred())
	})
})
