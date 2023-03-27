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
	"io"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/list"
	clitesting "github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("Expose", func() {
	const (
		namespace         = "test"
		pending           = "pending"
		running           = "running"
		failed            = "failed"
		succeed           = "succeed"
		all               = "all"
		statelessCompName = "stateless"
		statefulCompName  = "stateful"
	)

	var (
		streams genericclioptions.IOStreams
		tf      *cmdtesting.TestFactory
		opsName string
	)

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = clitesting.NewTestFactory(namespace)
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	generateOpsObject := func(opsType appsv1alpha1.OpsType, phase appsv1alpha1.OpsPhase) *appsv1alpha1.OpsRequest {
		ops := &appsv1alpha1.OpsRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "list-ops-" + clitesting.GetRandomStr(),
				Namespace: namespace,
			},
			Spec: appsv1alpha1.OpsRequestSpec{
				ClusterRef: "test-cluster",
				Type:       opsType,
			},
			Status: appsv1alpha1.OpsRequestStatus{
				Phase: phase,
			},
		}
		ops.Status.Components = map[string]appsv1alpha1.OpsRequestComponentStatus{
			statelessCompName: {},
			statefulCompName:  {},
		}
		return ops
	}

	initOpsRequests := func() {
		opsTypes := []appsv1alpha1.OpsType{
			appsv1alpha1.UpgradeType,
			appsv1alpha1.HorizontalScalingType,
			appsv1alpha1.HorizontalScalingType,
			appsv1alpha1.RestartType,
			appsv1alpha1.VerticalScalingType,
			appsv1alpha1.VerticalScalingType,
			appsv1alpha1.VerticalScalingType,
		}
		phases := []appsv1alpha1.OpsPhase{
			appsv1alpha1.OpsPendingPhase,
			appsv1alpha1.OpsFailedPhase,
			appsv1alpha1.OpsSucceedPhase,
			appsv1alpha1.OpsSucceedPhase,
			appsv1alpha1.OpsRunningPhase,
			appsv1alpha1.OpsFailedPhase,
			appsv1alpha1.OpsRunningPhase,
		}
		opsList := make([]runtime.Object, len(opsTypes))
		for i := range opsTypes {
			opsList[i] = generateOpsObject(opsTypes[i], phases[i])
		}
		opsName = opsList[0].(*appsv1alpha1.OpsRequest).Name
		tf.FakeDynamicClient = clitesting.FakeDynamicClient(opsList...)
	}

	getStdoutLinesCount := func(out io.Writer) int {
		b := out.(*bytes.Buffer).String()
		b = strings.Trim(b, "\n")
		return len(strings.Split(b, "\n"))
	}

	initOpsOption := func(status []string, opsTypes []string) *opsListOptions {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		return &opsListOptions{
			ListOptions: list.NewListOptions(tf, streams, types.OpsGVR()),
			status:      status,
			opsType:     opsTypes,
		}
	}

	It("list ops", func() {
		By("new list ops command")
		cmd := NewListOpsCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())

		By("init opsRequests for testing")
		initOpsRequests()

		By("test run cmd")
		cmd.Run(cmd, nil)

		By("test status flag with default values")
		o := initOpsOption([]string{pending, running, failed}, nil)
		Expect(o.printOpsList()).Should(Succeed())
		// title + filter ops
		Expect(getStdoutLinesCount(o.Out)).Should(Equal(6))

		By("test status flag with `all` keyword")
		o = initOpsOption([]string{all}, nil)
		Expect(o.printOpsList()).Should(Succeed())
		// title + filter ops
		Expect(getStdoutLinesCount(o.Out)).Should(Equal(8))

		By("test status flag with custom inputs")
		o = initOpsOption([]string{succeed}, nil)
		Expect(o.printOpsList()).Should(Succeed())
		// title + filter ops
		Expect(getStdoutLinesCount(o.Out)).Should(Equal(3))

		o = initOpsOption([]string{failed}, nil)
		Expect(o.printOpsList()).Should(Succeed())
		// title + filter ops
		Expect(getStdoutLinesCount(o.Out)).Should(Equal(3))

		By("test type flag")
		o = initOpsOption([]string{all}, []string{string(appsv1alpha1.RestartType)})
		Expect(o.printOpsList()).Should(Succeed())
		// title + filter ops
		Expect(getStdoutLinesCount(o.Out)).Should(Equal(2))

		o = initOpsOption([]string{all}, []string{string(appsv1alpha1.RestartType), string(appsv1alpha1.VerticalScalingType)})
		Expect(o.printOpsList()).Should(Succeed())
		// title + filter ops
		Expect(getStdoutLinesCount(o.Out)).Should(Equal(5))

		By("test component for upgrade ops")
		o = initOpsOption([]string{all}, []string{string(appsv1alpha1.UpgradeType)})
		Expect(o.printOpsList()).Should(Succeed())
		Expect(o.Out).Should(ContainSubstring(statefulCompName + "," + statelessCompName))

		By("list-ops with specified name")
		o = initOpsOption(nil, nil)
		o.opsRequestName = opsName
		Expect(o.printOpsList()).Should(Succeed())
		Expect(getStdoutLinesCount(o.Out)).Should(Equal(2))

		By("list-ops with not exist ops")
		o = initOpsOption(nil, nil)
		o.opsRequestName = "not-exist-ops"
		done := clitesting.Capture()
		Expect(o.printOpsList()).Should(Succeed())
		capturedOutput, _ := done()
		Expect(clitesting.ContainExpectStrings(capturedOutput, "No opsRequests found")).Should(BeTrue())

		By("list-ops with not exist ops")
		o = initOpsOption([]string{pending}, []string{string(appsv1alpha1.RestartType)})
		done = clitesting.Capture()
		Expect(o.printOpsList()).Should(Succeed())
		capturedOutput, _ = done()
		Expect(clitesting.ContainExpectStrings(capturedOutput, "kbcli cluster list-ops --status all")).Should(BeTrue())
	})

})
