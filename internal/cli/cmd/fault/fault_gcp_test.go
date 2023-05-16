package fault

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
)

var _ = Describe("Fault Node GCP", func() {
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

	Context("test fault node gcp", func() {
		It("fault node gcp stop", func() {
			inputs := [][]string{
				{"--region=us-central1-c", "--project=apecloud-platform-engineering", "--instance=gke-hyqtest-default-pool-2fe51a08-45rl", "--secret-name=cloud-key-secret", "--duration=3m", "--dry-run=client"},
			}
			o := NewGCPOptions(tf, streams, string(v1alpha1.NodeStop))
			cmd := o.NewCobraCommand(Stop, StopShort)
			o.AddCommonFlag(cmd)

			for _, input := range inputs {
				Expect(cmd.Flags().Parse(input)).Should(Succeed())
				Expect(o.CreateOptions.Complete())
				Expect(o.Complete()).Should(Succeed())
				Expect(o.Validate()).Should(Succeed())
				Expect(o.Run()).Should(Succeed())
			}
		})

		It("fault node gcp detach-volume", func() {
			inputs := [][]string{
				{"--region=us-central1-c", "--project=apecloud-platform-engineering", "--instance=gke-hyqtest-default-pool-2fe51a08-45rl", "--secret-name=cloud-key-secret", "--device-name=xxx", "--duration=3m", "--dry-run=client"},
			}
			o := NewGCPOptions(tf, streams, string(v1alpha1.DiskLoss))
			cmd := o.NewCobraCommand(DetachVolume, DetachVolumeShort)
			o.AddCommonFlag(cmd)
			cmd.Flags().StringArrayVar(&o.DeviceNames, "device-name", nil, "The device name of the volumes.")

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
