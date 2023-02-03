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

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/list"
	clitesting "github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("Expose", func() {
	const (
		namespace = "test"
		pending   = "pending"
		running   = "running"
		failed    = "failed"
		succeed   = "succeed"
		all       = "all"
	)

	var (
		streams genericclioptions.IOStreams
		tf      *cmdtesting.TestFactory
	)

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = clitesting.NewTestFactory(namespace)
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	generateOpsObject := func(opsType dbaasv1alpha1.OpsType, phase dbaasv1alpha1.Phase) *dbaasv1alpha1.OpsRequest {
		return &dbaasv1alpha1.OpsRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "list-ops-" + clitesting.GetRandomStr(),
				Namespace: namespace,
			},
			Spec: dbaasv1alpha1.OpsRequestSpec{
				ClusterRef: "test-cluster",
				Type:       opsType,
			},
			Status: dbaasv1alpha1.OpsRequestStatus{
				Phase: phase,
			},
		}
	}

	initOpsRequests := func() {
		opsTypes := []dbaasv1alpha1.OpsType{
			dbaasv1alpha1.UpgradeType,
			dbaasv1alpha1.HorizontalScalingType,
			dbaasv1alpha1.HorizontalScalingType,
			dbaasv1alpha1.RestartType,
			dbaasv1alpha1.VerticalScalingType,
			dbaasv1alpha1.VerticalScalingType,
			dbaasv1alpha1.VerticalScalingType,
		}
		phases := []dbaasv1alpha1.Phase{
			dbaasv1alpha1.PendingPhase,
			dbaasv1alpha1.FailedPhase,
			dbaasv1alpha1.SucceedPhase,
			dbaasv1alpha1.SucceedPhase,
			dbaasv1alpha1.RunningPhase,
			dbaasv1alpha1.FailedPhase,
			dbaasv1alpha1.RunningPhase,
		}
		opsList := make([]runtime.Object, len(opsTypes))
		for i := range opsTypes {
			opsList[i] = generateOpsObject(opsTypes[i], phases[i])
		}
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

		By("init opsrequests for testing")
		initOpsRequests()

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
		o = initOpsOption([]string{all}, []string{string(dbaasv1alpha1.RestartType)})
		Expect(o.printOpsList()).Should(Succeed())
		// title + filter ops
		Expect(getStdoutLinesCount(o.Out)).Should(Equal(2))

		o = initOpsOption([]string{all}, []string{string(dbaasv1alpha1.RestartType), string(dbaasv1alpha1.VerticalScalingType)})
		Expect(o.printOpsList()).Should(Succeed())
		// title + filter ops
		Expect(getStdoutLinesCount(o.Out)).Should(Equal(5))
	})

})
