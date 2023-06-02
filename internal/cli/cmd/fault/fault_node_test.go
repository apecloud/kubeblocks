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
				{"-c=aws", "--region=cn-northwest-1", "--dry-run=client"},
				{"-c=aws", "--region=cn-northwest-1", "--secret=test-secret", "--dry-run=client"},
				{"-c=gcp", "--region=us-central1-c", "--project=apecloud-platform-engineering", "--dry-run=client"},
			}
			o := NewNodeOptions(tf, streams)
			cmd := o.NewCobraCommand(Stop, StopShort)
			o.AddCommonFlag(cmd)

			o.Args = []string{"node1", "node2"}
			for _, input := range inputs {
				Expect(cmd.Flags().Parse(input)).Should(Succeed())
				Expect(o.Execute(Stop, o.Args, true)).Should(Succeed())
			}
		})

		It("fault node restart", func() {
			inputs := [][]string{
				{"-c=aws", "--region=cn-northwest-1", "--dry-run=client"},
				{"-c=aws", "--region=cn-northwest-1", "--secret=test-secret", "--dry-run=client"},
				{"-c=gcp", "--region=us-central1-c", "--project=apecloud-platform-engineering", "--dry-run=client"},
			}
			o := NewNodeOptions(tf, streams)
			cmd := o.NewCobraCommand(Restart, RestartShort)
			o.AddCommonFlag(cmd)

			o.Args = []string{"node1", "node2"}
			for _, input := range inputs {
				Expect(cmd.Flags().Parse(input)).Should(Succeed())
				Expect(o.Execute(Restart, o.Args, true)).Should(Succeed())
			}
		})

		It("fault node detach-volume", func() {
			inputs := [][]string{
				{"-c=aws", "--region=cn-northwest-1", "--volume-id=v1,v2", "--device-name=/d1,/d2", "--dry-run=client"},
				{"-c=aws", "--region=cn-northwest-1", "--volume-id=v1,v2", "--device-name=/d1,/d2", "--secret=test-secret", "--dry-run=client"},
				{"-c=gcp", "--region=us-central1-c", "--project=apecloud-platform-engineering", "--device-name=/d1,/d2", "--dry-run=client"},
			}
			o := NewNodeOptions(tf, streams)
			cmd := o.NewCobraCommand(DetachVolume, DetachVolumeShort)
			o.AddCommonFlag(cmd)
			cmd.Flags().StringSliceVar(&o.VolumeIDs, "volume-id", nil, "The volume id of the ec2.")
			cmd.Flags().StringSliceVar(&o.DeviceNames, "device-name", nil, "The device name of the volume.")

			o.Args = []string{"node1", "node2"}
			for _, input := range inputs {
				Expect(cmd.Flags().Parse(input)).Should(Succeed())
				Expect(o.Execute(DetachVolume, o.Args, true)).Should(Succeed())
				o.VolumeIDs = nil
				o.DeviceNames = nil
			}
		})
	})
})
