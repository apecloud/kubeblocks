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

var _ = Describe("Fault Node AWS", func() {
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

	Context("test fault node aws", func() {
		It("fault node aws stop", func() {
			inputs := [][]string{
				{"--secret-name=cloud-key-secret", "--region=cn-northwest-1", "--instance=i-0a4986881adf30039", "--duration=3m", "--dry-run=client"},
			}
			o := NewAWSOptions(tf, streams, string(v1alpha1.Ec2Stop))
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

		It("fault node aws detach-volume", func() {
			inputs := [][]string{
				{"--secret-name=cloud-key-secret", "--region=cn-northwest-1", "--instance=i-0a4986881adf30039", "--volume-id=vol-072f0940c28664f74", "--device-name=/dev/xvdab", "--duration=3m", "--dry-run=client"},
			}
			o := NewAWSOptions(tf, streams, string(v1alpha1.DetachVolume))
			cmd := o.NewCobraCommand(DetachVolume, DetachVolumeShort)
			o.AddCommonFlag(cmd)
			cmd.Flags().StringVar(&o.DeviceName, "device-name", "", "The device name of the volume.")
			cmd.Flags().StringVar(&o.VolumeID, "volume-id", "", "The volume id of the ec2.")

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
