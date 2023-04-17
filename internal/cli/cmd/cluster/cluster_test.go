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
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
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
			err := o.Validate()
			Expect(err).Should(Succeed())

			o.ClusterDefRef = ""
			Expect(o.ClusterDefRef).Should(BeEmpty())
			err = o.Validate()
			Expect(err).ShouldNot(Succeed())
		})

		It("can validate whether the TerminationPolicy is null when create a new cluster ", func() {
			Expect(o.TerminationPolicy).ShouldNot(BeEmpty())
			err := o.Validate()
			Expect(err).Should(Succeed())

			o.TerminationPolicy = ""
			Expect(o.TerminationPolicy).Should(BeEmpty())
			err = o.Validate()
			Expect(err).ShouldNot(Succeed())
		})

		It("can validate whether the ClusterVersionRef is null and can't get latest version from client when create a new cluster ", func() {
			Expect(o.ClusterVersionRef).ShouldNot(BeEmpty())
			err := o.Validate()
			Expect(err).Should(Succeed())
			o.ClusterVersionRef = ""
			Expect(o.ClusterVersionRef).Should(BeEmpty())
			_, errGetVersion := cluster.GetLatestVersion(o.Dynamic, o.ClusterDefRef)
			err = o.Validate()
			if errGetVersion == nil {
				Expect(err).Should(Succeed())
			} else {
				Expect(err).ShouldNot(Succeed())
			}
		})

		It("can validate whether --set and --set-file both are specified when create a new cluster ", func() {
			Expect(o.SetFile).ShouldNot(BeEmpty())
			Expect(o.Values).Should(BeNil())
			err := o.Validate()
			Expect(err).Should(Succeed())
			o.Values = []string{"notEmpty"}
			Expect(o.Values).ShouldNot(BeNil())
			err = o.Validate()
			Expect(err).ShouldNot(Succeed())
		})

		It("can validate whether the name is not specified and fail to generate a random cluster name when create a new cluster ", func() {
			Expect(o.Name).ShouldNot(BeEmpty())
			err := o.Validate()
			Expect(err).Should(Succeed())
			o.Name = ""
			Expect(o.Name).Should(BeEmpty())
			_, errGenerateName := generateClusterName(o.Dynamic, o.ClusterDefRef)
			err = o.Validate()
			if errGenerateName == nil {
				Expect(err).Should(Succeed())
			} else {
				Expect(err).ShouldNot(Succeed())
			}
		})

		It("can validate whether the name is not longer than 16 characters when create a new cluster", func() {
			Expect(len(o.Name)).Should(BeNumerically("<=", 16))
			err := o.Validate()
			Expect(err).Should(Succeed())
			moreThan16 := 17
			bytes := make([]byte, 0)
			var clusterNameMoreThan16 string
			for i := 0; i < moreThan16; i++ {
				bytes = append(bytes, byte(i+'a'))
			}
			clusterNameMoreThan16 = string(bytes)
			Expect(len(clusterNameMoreThan16)).Should(BeNumerically(">", 16))
			o.Name = clusterNameMoreThan16
			err = o.Validate()
			Expect(err).ShouldNot(Succeed())
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
