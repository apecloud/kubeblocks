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
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/delete"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var _ = Describe("operations", func() {
	const (
		clusterName  = "cluster-ops"
		clusterName1 = "cluster-ops1"
	)
	var (
		streams genericclioptions.IOStreams
		tf      *cmdtesting.TestFactory
		in      *bytes.Buffer
	)

	BeforeEach(func() {
		streams, in, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testing.Namespace)
		clusterWithTwoComps := testing.FakeCluster(clusterName, testing.Namespace)
		clusterWithOneComp := clusterWithTwoComps.DeepCopy()
		clusterWithOneComp.Name = clusterName1
		clusterWithOneComp.Spec.ComponentSpecs = []appsv1alpha1.ClusterComponentSpec{
			clusterWithOneComp.Spec.ComponentSpecs[0],
		}
		tf.FakeDynamicClient = testing.FakeDynamicClient(testing.FakeClusterDef(),
			testing.FakeClusterVersion(), clusterWithTwoComps, clusterWithOneComp)
		tf.Client = &clientfake.RESTClient{}
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	initCommonOperationOps := func(opsType appsv1alpha1.OpsType, clusterName string, hasComponentNamesFlag bool) *OperationsOptions {
		o := newBaseOperationsOptions(streams, opsType, hasComponentNamesFlag)
		o.Dynamic = tf.FakeDynamicClient
		o.Name = clusterName
		o.Namespace = testing.Namespace
		return o
	}

	It("Upgrade Ops", func() {
		o := newBaseOperationsOptions(streams, appsv1alpha1.UpgradeType, false)
		o.Dynamic = tf.FakeDynamicClient

		By("validate o.name is null")
		Expect(o.Validate()).To(MatchError(missingClusterArgErrMassage))

		By("validate upgrade when cluster-version is null")
		o.Namespace = testing.Namespace
		o.Name = clusterName
		o.OpsType = appsv1alpha1.UpgradeType
		Expect(o.Validate()).To(MatchError("missing cluster-version"))

		By("expect to validate success")
		o.ClusterVersionRef = "test-cluster-version"
		in.Write([]byte(o.Name + "\n"))
		Expect(o.Validate()).Should(Succeed())
	})

	It("VolumeExpand Ops", func() {
		o := initCommonOperationOps(appsv1alpha1.VolumeExpansionType, clusterName, true)
		By("validate volumeExpansion when components is null")
		Expect(o.Validate()).To(MatchError(`missing components, please specify the "--components" flag for multi-components cluster`))

		By("validate volumeExpansion when vct-names is null")
		o.ComponentNames = []string{"replicasets"}
		Expect(o.Validate()).To(MatchError("missing volume-claim-templates"))

		By("validate volumeExpansion when storage is null")
		o.VCTNames = []string{"data"}
		Expect(o.Validate()).To(MatchError("missing storage"))
		o.Storage = "2Gi"
		in.Write([]byte(o.Name + "\n"))
		Expect(o.Validate()).Should(Succeed())
	})

	It("Hscale Ops", func() {
		o := initCommonOperationOps(appsv1alpha1.HorizontalScalingType, clusterName1, true)
		By("test CompleteComponentsFlag function")
		o.ComponentNames = nil
		By("expect to auto complete components when cluster has only one component")
		Expect(o.CompleteComponentsFlag()).Should(Succeed())
		Expect(o.ComponentNames[0]).Should(Equal(testing.ComponentName))

		By("expect to Validate success")
		o.Replicas = 1
		in.Write([]byte(o.Name + "\n"))
		Expect(o.Validate()).Should(Succeed())

		By("expect for componentNames is nil when cluster has only two component")
		o.Name = clusterName
		o.ComponentNames = nil
		Expect(o.CompleteComponentsFlag()).Should(Succeed())
		Expect(o.ComponentNames).Should(BeEmpty())
	})

	It("Restart ops", func() {
		o := initCommonOperationOps(appsv1alpha1.RestartType, clusterName, true)
		By("expect for not found error")
		inputs := buildOperationsInputs(tf, o)
		Expect(o.Complete(inputs, []string{clusterName + "2"}))
		Expect(o.CompleteRestartOps().Error()).Should(ContainSubstring("not found"))

		By("expect for complete success")
		o.Name = clusterName
		Expect(o.CompleteRestartOps()).Should(Succeed())

		By("test Restart command")
		restartCmd := NewRestartCmd(tf, streams)
		_, _ = in.Write([]byte(clusterName + "\n"))
		done := testing.Capture()
		restartCmd.Run(restartCmd, []string{clusterName})
		capturedOutput, _ := done()
		Expect(testing.ContainExpectStrings(capturedOutput, "kbcli cluster describe-ops")).Should(BeTrue())
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

})
