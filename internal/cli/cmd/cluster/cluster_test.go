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

package cluster

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("Cluster", func() {
	const testComponentPath = "../../testing/testdata/component.yaml"
	const testClassDefsPath = "../../testing/testdata/class.yaml"

	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace("default")
		cd := testing.FakeClusterDef()
		tf.FakeDynamicClient = testing.FakeDynamicClient(cd, testing.FakeClusterVersion())
		tf.Client = &clientfake.RESTClient{}
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	Context("create", func() {
		It("without name", func() {
			o := &CreateOptions{
				ClusterDefRef:     testing.ClusterDefName,
				ClusterVersionRef: testing.ClusterVersionName,
				SetFile:           testComponentPath,
				UpdatableFlags: UpdatableFlags{
					TerminationPolicy: "Delete",
				},
				BaseOptions: create.BaseOptions{
					Dynamic: tf.FakeDynamicClient,
				},
			}
			o.IOStreams = streams
			Expect(o.Validate()).To(Succeed())
			Expect(o.Name).ShouldNot(BeEmpty())
		})

		It("new command", func() {
			cmd := NewCreateCmd(tf, streams)
			Expect(cmd).ShouldNot(BeNil())
			Expect(cmd.Flags().Set("cluster-definition", testing.ClusterDefName)).Should(Succeed())
			Expect(cmd.Flags().Set("cluster-version", testing.ClusterVersionName)).Should(Succeed())
			Expect(cmd.Flags().Set("set-file", testComponentPath)).Should(Succeed())
			Expect(cmd.Flags().Set("termination-policy", "Delete")).Should(Succeed())

			// must succeed otherwise exit 1 and make test fails
			cmd.Run(nil, []string{"test1"})
		})

		It("run", func() {
			clusterDef := testing.FakeClusterDef()
			tf.FakeDynamicClient = testing.FakeDynamicClient(clusterDef)
			data, err := os.ReadFile(testClassDefsPath)
			Expect(err).NotTo(HaveOccurred())
			clientSet := testing.FakeClientSet(testing.FakeComponentClassDef(clusterDef, data))
			o := &CreateOptions{
				BaseOptions:       create.BaseOptions{IOStreams: streams, Name: "test", Dynamic: tf.FakeDynamicClient, ClientSet: clientSet},
				SetFile:           "",
				ClusterDefRef:     testing.ClusterDefName,
				ClusterVersionRef: "cluster-version",
				UpdatableFlags: UpdatableFlags{
					PodAntiAffinity: "Preferred",
					TopologyKeys:    []string{"kubernetes.io/hostname"},
					NodeLabels:      map[string]string{"testLabelKey": "testLabelValue"},
					TolerationsRaw:  []string{"key=engineType,value=mongo,operator=Equal,effect=NoSchedule"},
					Tenancy:         string(appsv1alpha1.SharedNode),
				},
			}

			Expect(len(o.TolerationsRaw)).Should(Equal(1))
			Expect(o.Complete()).Should(Succeed())
			Expect(len(o.Tolerations)).Should(Equal(1))
			Expect(o.Validate()).Should(HaveOccurred())

			o.TerminationPolicy = "WipeOut"
			o.SetFile = "test.yaml"
			Expect(o.Complete()).ShouldNot(Succeed())

			o.SetFile = ""
			Expect(o.Complete()).Should(Succeed())
			Expect(o.Validate()).Should(Succeed())

			o.SetFile = testComponentPath
			Expect(o.Complete()).Should(Succeed())
			Expect(o.Validate()).Should(Succeed())

			inputs := create.Inputs{
				ResourceName:    types.ResourceClusters,
				CueTemplateName: CueTemplateName,
				Options:         o,
				Factory:         tf,
			}

			Expect(o.BaseOptions.Complete(inputs, []string{"test"})).Should(Succeed())
			Expect(o.Namespace).To(Equal("default"))
			Expect(o.Name).To(Equal("test"))

			Expect(o.Run(inputs)).Should(Succeed())
		})
	})

	Context("create validate", func() {
		var o *CreateOptions
		BeforeEach(func() {
			o = &CreateOptions{
				ClusterDefRef:     testing.ClusterDefName,
				ClusterVersionRef: testing.ClusterVersionName,
				SetFile:           testComponentPath,
				UpdatableFlags: UpdatableFlags{
					TerminationPolicy: "Delete",
				},
				BaseOptions: create.BaseOptions{
					Namespace: "default",
					Name:      "mycluster",
					Dynamic:   tf.FakeDynamicClient,
					IOStreams: streams,
				},
			}
		})

		It("can validate whether the ClusterDefRef is null when create a new cluster ", func() {
			Expect(o.ClusterDefRef).ShouldNot(BeEmpty())
			Expect(o.Validate()).Should(Succeed())
			o.ClusterDefRef = ""
			Expect(o.Validate()).Should(HaveOccurred())
		})

		It("can validate whether the TerminationPolicy is null when create a new cluster ", func() {
			Expect(o.TerminationPolicy).ShouldNot(BeEmpty())
			Expect(o.Validate()).Should(Succeed())
			o.TerminationPolicy = ""
			Expect(o.Validate()).Should(HaveOccurred())
		})

		It("can validate whether the ClusterVersionRef is null and can't get latest version from client when create a new cluster ", func() {
			Expect(o.ClusterVersionRef).ShouldNot(BeEmpty())
			Expect(o.Validate()).Should(Succeed())
			o.ClusterVersionRef = ""
			Expect(o.Validate()).Should(Succeed())
		})

		It("can validate whether --set and --set-file both are specified when create a new cluster ", func() {
			Expect(o.SetFile).ShouldNot(BeEmpty())
			Expect(o.Values).Should(BeNil())
			Expect(o.Validate()).Should(Succeed())
			o.Values = []string{"notEmpty"}
			Expect(o.Validate()).Should(HaveOccurred())
		})

		It("can validate whether the name is not specified and fail to generate a random cluster name when create a new cluster ", func() {
			Expect(o.Name).ShouldNot(BeEmpty())
			Expect(o.Validate()).Should(Succeed())
			o.Name = ""
			Expect(o.Validate()).Should(Succeed())
		})

		It("can validate whether the name is not longer than 16 characters when create a new cluster", func() {
			Expect(len(o.Name)).Should(BeNumerically("<=", 16))
			Expect(o.Validate()).Should(Succeed())
			moreThan16 := 17
			bytes := make([]byte, 0)
			var clusterNameMoreThan16 string
			for i := 0; i < moreThan16; i++ {
				bytes = append(bytes, byte(i%26+'a'))
			}
			clusterNameMoreThan16 = string(bytes)
			Expect(len(clusterNameMoreThan16)).Should(BeNumerically(">", 16))
			o.Name = clusterNameMoreThan16
			Expect(o.Validate()).Should(HaveOccurred())
		})
	})

	It("delete", func() {
		cmd := NewDeleteCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("cluster", func() {
		cmd := NewClusterCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		Expect(cmd.HasSubCommands()).To(BeTrue())
	})

	It("connect", func() {
		cmd := NewConnectCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("list-logs-type", func() {
		cmd := NewListLogsCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("logs", func() {
		cmd := NewLogsCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})
})
