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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/delete"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var _ = Describe("Cluster", func() {
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace("default")
		tf.FakeDynamicClient = testing.FakeDynamicClient(testing.FakeClusterDef(), testing.FakeClusterVersion())
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
			Expect(cmd.Flags().Set("cluster-version", testing.ClusterVersionName)).Should(Succeed())
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
				ClusterVersionRef:  "cluster-version",
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

		By("validate upgrade when cluster-version is null")
		o.Name = "test"
		o.OpsType = dbaasv1alpha1.UpgradeType
		Expect(o.Validate()).To(MatchError("missing cluster-version"))
		o.ClusterVersionRef = "test-cluster-version"
		Expect(o.Validate()).Should(Succeed())

		By("validate volumeExpansion when components is null")
		o.OpsType = dbaasv1alpha1.VolumeExpansionType
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
		o.OpsType = dbaasv1alpha1.HorizontalScalingType
		o.Replicas = -2
		Expect(o.Validate()).To(MatchError("replicas required natural number"))

		o.Replicas = 1
		Expect(o.Validate()).Should(Succeed())
	})

	It("list and delete operations", func() {
		clusterName := "wesql"
		args := []string{clusterName}
		clusterLabel := util.BuildLabelSelectorByNames("", args)
		testLabel := "kubeblocks.io/test=test"

		By("test delete OpsRequest with cluster")
		o := delete.NewDeleteOptions(tf, streams, types.OpsGVR())
		Expect(completeForDeleteOps(o, args)).Should(Succeed())
		Expect(o.LabelSelector == clusterLabel).Should(BeTrue())

		By("test delete OpsRequest with cluster and custom label")
		o.LabelSelector = testLabel
		Expect(completeForDeleteOps(o, args)).Should(Succeed())
		Expect(o.LabelSelector == testLabel+","+clusterLabel).Should(BeTrue())

		By("test delete OpsRequest with name")
		o.Names = []string{"test1"}
		Expect(completeForDeleteOps(o, nil)).Should(Succeed())
		Expect(len(o.ConfirmedNames)).Should(Equal(1))
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
