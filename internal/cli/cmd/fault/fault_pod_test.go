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

var _ = Describe("Fault POD", func() {
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

	Context("test fault pod", func() {

		It("fault pod kill", func() {
			inputs := [][]string{
				{"--mode=one", "--dry-run=client"},
				{"--mode=fixed", "--value=2", "--dry-run=client"},
				{"--mode=fixed-percent", "--value=50", "--dry-run=client"},
				{"--mode=random-max-percent", "--value=50", "--dry-run=client"},
				{"--grace-period=5", "--dry-run=client"},
				{"--ns-fault=kb-system", "--dry-run=client"},
				{"--node=minikube-m02", "--dry-run=client"},
				{"--label=app.kubernetes.io/component=mysql", "--dry-run=client"},
				{"--node-label=kubernetes.io/arch=arm64", "--dry-run=client"},
				{"--annotation=example-annotation=group-a", "--dry-run=client"},
			}
			o := NewPodChaosOptions(tf, streams, string(v1alpha1.PodKillAction))
			cmd := o.NewCobraCommand(Kill, KillShort)
			o.AddCommonFlag(cmd)
			cmd.Flags().Int64VarP(&o.GracePeriod, "grace-period", "g", 0, "Grace period represents the duration in seconds before the pod should be killed")
			for _, input := range inputs {
				Expect(cmd.Flags().Parse(input)).Should(Succeed())
				Expect(o.CreateOptions.Complete())
				Expect(o.Complete()).Should(Succeed())
				Expect(o.Validate()).Should(Succeed())
				Expect(o.Run()).Should(Succeed())
			}
		})

		It("fault pod kill-container", func() {
			inputs := [][]string{
				{"--container=mysql", "--container=nginx", "--container=config-manager"},
			}
			o := NewPodChaosOptions(tf, streams, string(v1alpha1.ContainerKillAction))
			cmd := o.NewCobraCommand(KillContainer, KillContainerShort)
			o.AddCommonFlag(cmd)
			cmd.Flags().StringArrayVarP(&o.ContainerNames, "container", "c", nil, "the name of the container you want to kill, such as mysql, prometheus.")

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
