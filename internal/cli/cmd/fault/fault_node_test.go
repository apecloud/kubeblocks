/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package fault

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
)

var _ = Describe("Fault Node", func() {
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

	Context("test fault node", func() {
		It("fault node stop", func() {
			inputs := [][]string{
				{"-c=aws", "--secret-name=cloud-key-secret", "--region=cn-northwest-1", "--instance=i-0a4986881adf30039", "--duration=3m", "--dry-run=client"},
				{"-c=gcp", "--region=us-central1-c", "--project=apecloud-platform-engineering", "--instance=gke-hyqtest-default-pool-2fe51a08-45rl", "--secret-name=cloud-key-secret", "--dry-run=client"},
			}
			o := NewNodeOptions(tf, streams)
			cmd := o.NewCobraCommand(Kill, KillShort)
			o.AddCommonFlag(cmd, tf)

			for _, input := range inputs {
				Expect(cmd.Flags().Parse(input)).Should(Succeed())
				Expect(o.CreateOptions.Complete())
				Expect(o.Complete(Stop)).Should(Succeed())
				Expect(o.Validate()).Should(Succeed())
				Expect(o.Run()).Should(Succeed())
			}
		})

		It("fault node restart", func() {
			inputs := [][]string{
				{"-c=aws", "--secret-name=cloud-key-secret", "--region=cn-northwest-1", "--instance=i-0a4986881adf30039", "--duration=3m", "--dry-run=client"},
				{"-c=gcp", "--region=us-central1-c", "--project=apecloud-platform-engineering", "--instance=gke-hyqtest-default-pool-2fe51a08-45rl", "--secret-name=cloud-key-secret", "--dry-run=client"},
			}
			o := NewNodeOptions(tf, streams)
			cmd := o.NewCobraCommand(Restart, RestartShort)
			o.AddCommonFlag(cmd, tf)

			for _, input := range inputs {
				Expect(cmd.Flags().Parse(input)).Should(Succeed())
				Expect(o.CreateOptions.Complete())
				Expect(o.Complete(Restart)).Should(Succeed())
				Expect(o.Validate()).Should(Succeed())
				Expect(o.Run()).Should(Succeed())
			}
		})

		It("fault node detach-volume", func() {
			inputs := [][]string{
				{"-c=aws", "--secret-name=cloud-key-secret", "--region=cn-northwest-1", "--instance=i-0df0732607d54dd8e", "--duration=1m", "--volume-id=vol-072f0940c28664f74", "--device-name=/dev/xvdab", "--dry-run=client"},
				{"-c=gcp", "--region=us-central1-c", "--project=apecloud-platform-engineering", "--instance=gke-hyqtest-default-pool-2fe51a08-d9nd", "--secret-name=cloud-key-secret", "--device-name=/dev/sdb", "--dry-run=client"},
			}
			o := NewNodeOptions(tf, streams)
			cmd := o.NewCobraCommand(DetachVolume, DetachVolumeShort)
			o.AddCommonFlag(cmd, tf)
			cmd.Flags().StringVar(&o.VolumeID, "volume-id", "", "The volume id of the ec2.")
			cmd.Flags().StringVar(&o.DeviceName, "device-name", "", "The device name of the volume.")

			for _, input := range inputs {
				Expect(cmd.Flags().Parse(input)).Should(Succeed())
				Expect(o.CreateOptions.Complete())
				Expect(o.Complete(DetachVolume)).Should(Succeed())
				Expect(o.Validate()).Should(Succeed())
				Expect(o.Run()).Should(Succeed())
				o.VolumeID = ""
			}
		})
	})
})
