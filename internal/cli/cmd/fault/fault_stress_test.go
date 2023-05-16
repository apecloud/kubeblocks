package fault

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
)

var _ = Describe("Fault Stress", func() {
	var (
		tf      *cmdtesting.TestFactory
		streams genericclioptions.IOStreams
	)
	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(testing.Namespace)
		tf.Client = &clientfake.RESTClient{}
	})

	AfterEach(func() {
		tf.Cleanup()
	})
	Context("test fault stress", func() {

		It("fault stress", func() {
			inputs := [][]string{
				{"--mode=one", "--cpu-worker=2", "--cpu-load=50", "--dry-run=client"},
				{"--mode=fixed", "--value=2", "--cpu-worker=2", "--cpu-load=50", "--dry-run=client"},
				{"--mode=fixed-percent", "--value=50", "--cpu-worker=2", "--cpu-load=50", "--dry-run=client"},
				{"--mode=random-max-percent", "--value=50", "--cpu-worker=2", "--cpu-load=50", "--dry-run=client"},
				{"--ns-fault=kb-system", "--cpu-worker=2", "--cpu-load=50", "--dry-run=client"},
				{"--node=minikube-m02", "--cpu-worker=2", "--cpu-load=50", "--dry-run=client"},
				{"--label=app.kubernetes.io/component=mysql", "--cpu-worker=2", "--cpu-load=50", "--dry-run=client"},
				{"--node-label=kubernetes.io/arch=arm64", "--cpu-worker=2", "--cpu-load=50", "--dry-run=client"},
				{"--annotation=example-annotation=group-a", "--cpu-worker=2", "--cpu-load=50", "--dry-run=client"},
				{"--cpu-worker=2", "--cpu-load=50", "--memory-worker=1", "--memory-size=100Mi", "--dry-run=client"},
			}
			o := NewStressChaosOptions(tf, streams, "")
			cmd := o.NewCobraCommand(Stress, StressShort)
			o.AddCommonFlag(cmd)

			for _, input := range inputs {
				Expect(cmd.Flags().Parse(input)).Should(Succeed())
				Expect(o.CreateOptions.Complete())
				Expect(o.Complete()).Should(Succeed())
				Expect(o.Validate()).Should(Succeed())
				Expect(o.Run()).Should(Succeed())
			}
		})
	})
})
