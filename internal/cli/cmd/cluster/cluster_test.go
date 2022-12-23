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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmddelete "k8s.io/kubectl/pkg/cmd/delete"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/builder"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/delete"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("Cluster", func() {
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace("default")
		tf.FakeDynamicClient = testing.FakeDynamicClient(testing.FakeClusterDef(), testing.FakeAppVersion())
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	Context("create", func() {
		It("without name", func() {
			o := &CreateOptions{}
			o.IOStreams = streams
			Expect(o.Validate()).To(MatchError("missing cluster name"))
		})

		It("new command", func() {
			cmd := NewCreateCmd(tf, streams)
			Expect(cmd).ShouldNot(BeNil())
			Expect(cmd.Flags().GetString("termination-policy")).Should(Equal(""))

			Expect(cmd.Flags().Set("cluster-definition", testing.ClusterDefName)).Should(Succeed())
			Expect(cmd.Flags().Set("cluster-version", testing.AppVersionName)).Should(Succeed())
			Expect(cmd.Flags().Set("components", "../../testing/testdata/component.yaml")).Should(Succeed())
			Expect(cmd.Flags().Set("termination-policy", "Delete")).Should(Succeed())

			// must succeed otherwise exit 1 and make test fails
			cmd.Run(nil, []string{"test1"})
		})

		It("run", func() {
			tf.FakeDynamicClient = testing.FakeDynamicClient(testing.FakeClusterDef())
			o := &CreateOptions{
				BaseOptions:        create.BaseOptions{IOStreams: streams, Name: "test", Client: tf.FakeDynamicClient},
				ComponentsFilePath: "",
				ClusterDefRef:      testing.ClusterDefName,
				AppVersionRef:      "app-version",
				UpdatableFlags: UpdatableFlags{
					PodAntiAffinity: "Preferred",
					TopologyKeys:    []string{"kubernetes.io/hostname"},
					NodeLabels:      map[string]string{"testLabelKey": "testLabelValue"},
				},
			}

			Expect(o.Validate()).Should(HaveOccurred())

			o.TerminationPolicy = "WipeOut"
			o.ComponentsFilePath = "test.yaml"
			Expect(o.Complete()).ShouldNot(Succeed())

			o.ComponentsFilePath = ""
			Expect(o.Complete()).Should(Succeed())
			Expect(o.Validate()).ShouldNot(Succeed())

			o.ComponentsFilePath = "../../testing/testdata/component.yaml"
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

	It("delete", func() {
		cmd := NewDeleteCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("cluster", func() {
		cmd := NewClusterCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
		Expect(cmd.HasSubCommands()).To(BeTrue())
	})

	It("operations", func() {
		o := &OperationsOptions{
			BaseOptions:            create.BaseOptions{IOStreams: streams},
			TTLSecondsAfterSucceed: 30,
		}
		By("validate o.name is null")
		Expect(o.Validate()).To(MatchError("missing cluster name"))

		By("validate upgrade when app-version is null")
		o.Name = "test"
		o.OpsType = OpsTypeUpgrade
		Expect(o.Validate()).To(MatchError("missing app-version"))
		o.AppVersionRef = "test-app-version"
		Expect(o.Validate()).Should(Succeed())

		By("validate volumeExpansion when components is null")
		o.OpsType = OpsTypeVolumeExpansion
		Expect(o.Validate()).To(MatchError("missing component-names"))

		By("validate volumeExpansion when vct-names is null")
		o.ComponentNames = []string{"replicasets"}
		Expect(o.Validate()).To(MatchError("missing vct-names"))
		By("validate volumeExpansion when storage is null")
		o.VCTNames = []string{"data"}
		Expect(o.Validate()).To(MatchError("missing storage"))
		o.Storage = "2Gi"
		Expect(o.Validate()).Should(Succeed())

		By("validate horizontalScaling when replicas less than -1 ")
		o.OpsType = OpsTypeHorizontalScaling
		o.Replicas = -2
		Expect(o.Validate()).To(MatchError("replicas required natural number"))

		o.Replicas = 1
		Expect(o.Validate()).Should(Succeed())
	})

	It("list and delete operations", func() {
		clusterName := "wesql"
		clusterLabel := fmt.Sprintf("%s=%s", types.InstanceLabelKey, clusterName)
		testLabel := "kubeblocks.io/test=test"

		By("test delete OpsRequest with cluster")
		deleteFlags := &delete.DeleteFlags{
			DeleteFlags: cmddelete.NewDeleteCommandFlags("containing the resource to delete."),
		}
		c := &builder.Command{Options: deleteFlags, Args: []string{clusterName}}
		Expect(completeForDeleteOps(c)).Should(Succeed())
		Expect(*deleteFlags.LabelSelector == clusterLabel).Should(BeTrue())

		By("test delete OpsRequest with cluster and custom label")
		deleteFlags.LabelSelector = &testLabel
		Expect(completeForDeleteOps(c)).Should(Succeed())
		Expect(*deleteFlags.LabelSelector == testLabel+","+clusterLabel).Should(BeTrue())

		By("test delete OpsRequest with name")
		deleteFlags.ClusterName = ""
		deleteFlags.ResourceNames = []string{"test1"}
		c.Args = []string{}
		Expect(completeForDeleteOps(c)).Should(Succeed())
		Expect(deleteFlags.ClusterName == "").Should(BeTrue())
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
