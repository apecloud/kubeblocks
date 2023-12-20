package apps

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

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/storage/names"

	"github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

const (
	namespace = "foo"
	name      = "bar"
)

var _ = Describe("transformer component workload", func() {

	var (
		podList                []*corev1.Pod
		pod0, pod1, pod2, pod3 *corev1.Pod
	)
	BeforeEach(func() {
		simpleNameGenerator := names.SimpleNameGenerator
		resetPods := func() {
			pod0 = builder.NewPodBuilder(name, simpleNameGenerator.GenerateName(name+"-")).
				GetObject()
			pod1 = builder.NewPodBuilder(namespace, simpleNameGenerator.GenerateName(name+"-")).
				GetObject()
			pod2 = builder.NewPodBuilder(namespace, simpleNameGenerator.GenerateName(name+"-")).
				GetObject()
			pod3 = builder.NewPodBuilder(namespace, simpleNameGenerator.GenerateName(name+"-")).
				GetObject()
		}
		resetPods()
		podList = []*corev1.Pod{pod0, pod1, pod2, pod3}
	})

	Context("Test DeletePodFromInstances", func() {
		It("Test DeletePodFromInstances", func() {
			instances := []string{pod0.Name}
			nodeAssignment := []v1alpha1.NodeAssignment{
				{
					Name: pod0.Name,
				},
				{
					Name: pod1.Name,
				},
				{
					Name: pod2.Name,
				},
				{
					Name: pod3.Name,
				},
			}
			expectedNodeAssignment := []v1alpha1.NodeAssignment{
				{
					Name: pod1.Name,
				},
				{
					Name: pod2.Name,
				},
				{
					Name: pod3.Name,
				},
			}

			newNodeAssignment, err := DeletePodFromInstances(podList, instances, 1, nodeAssignment)
			Expect(err).Should(BeNil())
			Expect(len(newNodeAssignment)).Should(Equal(3))
			for i := 0; i < len(newNodeAssignment); i++ {
				canFind := false
				for j := 0; j < len(expectedNodeAssignment); j++ {
					if newNodeAssignment[i].Name == expectedNodeAssignment[j].Name {
						canFind = true
					}
				}
				Expect(canFind).Should(Equal(true))
			}
		})
		It(
			"Test DeletePodFromInstances with no specified instances",
			func() {
				var instances []string
				nodeAssignment := []v1alpha1.NodeAssignment{
					{
						Name: pod0.Name,
					},
					{
						Name: pod1.Name,
					},
					{
						Name: pod2.Name,
					},
					{
						Name: pod3.Name,
					},
				}
				newNodeAssignment, err := DeletePodFromInstances(podList, instances, 1, nodeAssignment)
				Expect(err).Should(BeNil())
				Expect(len(newNodeAssignment)).Should(Equal(3))
			},
		)
		It("Test DeletePodFromInstances with specified one instances and delete two replicas", func() {
			instances := []string{pod0.Name}
			nodeAssignment := []v1alpha1.NodeAssignment{
				{
					Name: pod0.Name,
				},
				{
					Name: pod1.Name,
				},
				{
					Name: pod2.Name,
				},
				{
					Name: pod3.Name,
				},
			}
			newNodeAssignment, err := DeletePodFromInstances(podList, instances, 2, nodeAssignment)
			Expect(err).Should(BeNil())
			Expect(len(newNodeAssignment)).Should(Equal(2))
		})
	})

	Context("Test AllocateNodesForPod", func() {

		It("Test AllocateNodesForPod specified nodeList", func() {
			nodeList := []types.NodeName{"node1", "node2", "node3"}
			nodeAssignment := AllocateNodesForPod(nodeList, 5, "redis", "proxy")
			node1Num := 0
			node2Num := 0
			node3Num := 0
			for _, node := range nodeAssignment {
				if node.NodeSpec.NodeName == "node1" {
					node1Num++
				} else if node.NodeSpec.NodeName == "node2" {
					node2Num++
				} else if node.NodeSpec.NodeName == "node3" {
					node3Num++
				}
			}
			Expect(node1Num).Should(Equal(2))
			Expect(node2Num).Should(Equal(2))
			Expect(node3Num).Should(Equal(1))
		})
		It("Test AllocateNodesForPod no specified nodeList", func() {
			nodeAssignment := AllocateNodesForPod(nil, 5, "redis", "proxy")
			Expect(len(nodeAssignment)).Should(Equal(5))
		})
	})
})
