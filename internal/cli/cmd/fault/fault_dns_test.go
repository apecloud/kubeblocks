package fault

import (
	"github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
)

var _ = Describe("Fault Network DNS", func() {
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
	Context("test fault network dns", func() {

		It("fault network dns random", func() {
			inputs := [][]string{
				{"--mode=one", "--patterns=baidu.com", "--dry-run=client"},
				{"--mode=fixed", "--value=2", "--patterns=baidu.com", "--dry-run=client"},
				{"--mode=fixed-percent", "--value=50", "--patterns=baidu.com", "--dry-run=client"},
				{"--mode=random-max-percent", "--value=50", "--patterns=baidu.com", "--dry-run=client"},
				{"--ns-fault=kb-system", "--patterns=baidu.com", "--dry-run=client"},
				{"--node=minikube-m02", "--patterns=baidu.com", "--dry-run=client"},
				{"--label=app.kubernetes.io/component=mysql", "--patterns=baidu.com", "--dry-run=client"},
				{"--node-label=kubernetes.io/arch=arm64", "--patterns=baidu.com", "--dry-run=client"},
				{"--annotation=example-annotation=group-a", "--patterns=baidu.com", "--dry-run=client"},
			}
			o := NewDNSChaosOptions(tf, streams, string(v1alpha1.RandomAction))
			cmd := o.NewCobraCommand(Random, RandomShort)
			o.AddCommonFlag(cmd)

			for _, input := range inputs {
				Expect(cmd.Flags().Parse(input)).Should(Succeed())
				Expect(o.CreateOptions.Complete())
				Expect(o.Complete()).Should(Succeed())
				Expect(o.Validate()).Should(Succeed())
				Expect(o.Run()).Should(Succeed())
			}
		})

		It("fault network dns error", func() {
			inputs := [][]string{
				{"--mode=one", "--patterns=baidu.com", "--dry-run=client"},
				{"--mode=fixed", "--value=2", "--patterns=baidu.com", "--dry-run=client"},
				{"--mode=fixed-percent", "--value=50", "--patterns=baidu.com", "--dry-run=client"},
				{"--mode=random-max-percent", "--value=50", "--patterns=baidu.com", "--dry-run=client"},
				{"--ns-fault=kb-system", "--patterns=baidu.com", "--dry-run=client"},
				{"--node=minikube-m02", "--patterns=baidu.com", "--dry-run=client"},
				{"--label=app.kubernetes.io/component=mysql", "--patterns=baidu.com", "--dry-run=client"},
				{"--node-label=kubernetes.io/arch=arm64", "--patterns=baidu.com", "--dry-run=client"},
				{"--annotation=example-annotation=group-a", "--patterns=baidu.com", "--dry-run=client"},
			}
			o := NewDNSChaosOptions(tf, streams, string(v1alpha1.ErrorAction))
			cmd := o.NewCobraCommand(Error, ErrorShort)
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
