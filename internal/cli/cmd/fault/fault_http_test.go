package fault

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
)

var _ = Describe("Fault Network HTPP", func() {
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

	Context("test fault network http", func() {
		It("fault network http abort", func() {
			inputs := [][]string{
				{"--dry-run=client"},
				{"--mode=one", "--dry-run=client"},
				{"--mode=fixed", "--value=2", "--dry-run=client"},
				{"--mode=fixed-percent", "--value=50", "--dry-run=client"},
				{"--mode=random-max-percent", "--value=50", "--dry-run=client"},
				{"--ns-fault=kb-system", "--dry-run=client"},
				{"--node=minikube-m02", "--dry-run=client"},
				{"--label=app.kubernetes.io/component=mysql", "--dry-run=client"},
				{"--node-label=kubernetes.io/arch=arm64", "--dry-run=client"},
				{"--annotation=example-annotation=group-a", "--dry-run=client"},
				{"--abort=true", "--dry-run=client"},
			}
			o := NewHTTPChaosOptions(tf, streams, "")
			cmd := o.NewCobraCommand(Abort, AbortShort)
			o.AddCommonFlag(cmd)
			cmd.Flags().BoolVar(&o.Abort, "abort", true, `Indicates whether to inject the fault that interrupts the connection.`)

			for _, input := range inputs {
				Expect(cmd.Flags().Parse(input)).Should(Succeed())
				Expect(o.CreateOptions.Complete())
				Expect(o.Complete()).Should(Succeed())
				Expect(o.Validate()).Should(Succeed())
				Expect(o.Run()).Should(Succeed())
			}
		})

		It("fault network http delay", func() {
			inputs := [][]string{
				{""},
				{"--delay=50s", "--dry-run=client"},
			}
			o := NewHTTPChaosOptions(tf, streams, "")
			cmd := o.NewCobraCommand(Delay, DelayShort)
			o.AddCommonFlag(cmd)
			cmd.Flags().StringVar(&o.Delay, "delay", "10s", `The time for delay.`)

			for _, input := range inputs {
				Expect(cmd.Flags().Parse(input)).Should(Succeed())
				Expect(o.CreateOptions.Complete())
				Expect(o.Complete()).Should(Succeed())
				Expect(o.Validate()).Should(Succeed())
				Expect(o.Run()).Should(Succeed())
			}
		})

		It("fault network http replace", func() {
			inputs := [][]string{
				{"--dry-run=client"},
				{"--replace-method=PUT", "--body=\"you are good luck\"", "--replace-path=/local/", "--duration=1m", "--dry-run=client"},
				{"--target=Response", "--replace-method=PUT", "--body=you", "--replace-path=/local/", "--duration=1m", "--dry-run=client"},
			}
			o := NewHTTPChaosOptions(tf, streams, "")
			cmd := o.NewCobraCommand(Replace, ReplaceShort)
			o.AddCommonFlag(cmd)
			cmd.Flags().StringVar(&o.InputReplaceBody, "body", "", `The content of the request body or response body to replace the failure.`)
			cmd.Flags().StringVar(&o.ReplacePath, "replace-path", "", `The URI path used to replace content.`)
			cmd.Flags().StringVar(&o.ReplaceMethod, "replace-method", "", `The replaced content of the HTTP request method.`)

			for _, input := range inputs {
				Expect(cmd.Flags().Parse(input)).Should(Succeed())
				Expect(o.CreateOptions.Complete())
				Expect(o.Complete()).Should(Succeed())
				Expect(o.Validate()).Should(Succeed())
				Expect(o.Run()).Should(Succeed())
			}
		})

		It("fault network http patch", func() {
			inputs := [][]string{
				{"--dry-run=client"},
				{"--body=\"you are good luck\"", "--type=JSON", "--duration=1m", "--dry-run=client"},
				{"--target=Response", "--body=\"you are good luck\"", "--type=JSON", "--duration=1m", "--dry-run=client"},
			}
			o := NewHTTPChaosOptions(tf, streams, "")
			cmd := o.NewCobraCommand(Patch, PatchShort)
			o.AddCommonFlag(cmd)
			cmd.Flags().StringVar(&o.PatchBodyValue, "body", "", `The fault of the request body or response body with patch faults.`)
			cmd.Flags().StringVar(&o.PatchBodyType, "type", "", `The type of patch faults of the request body or response body. Currently, it only supports JSON.`)

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
