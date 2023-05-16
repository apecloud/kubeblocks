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

	"github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
)

var _ = Describe("Fault IO", func() {
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

	Context("test fault io", func() {

		It("fault io latency", func() {
			inputs := [][]string{
				{"--mode=one", "--delay=10s", "--dry-run=client"},
				{"--mode=fixed", "--value=2", "--delay=10s", "--dry-run=client"},
				{"--mode=fixed-percent", "--value=50", "--delay=10s", "--dry-run=client"},
				{"--mode=random-max-percent", "--value=50", "--delay=10s", "--dry-run=client"},
				{"--ns-fault=kb-system", "--delay=10s", "--dry-run=client"},
				{"--node=minikube-m02", "--delay=10s", "--dry-run=client"},
				{"--label=app.kubernetes.io/component=mysql", "--delay=10s", "--dry-run=client"},
				{"--node-label=kubernetes.io/arch=arm64", "--delay=10s", "--dry-run=client"},
				{"--annotation=example-annotation=group-a", "--delay=10s", "--dry-run=client"},
				{"--delay=10s", "--volume-path=/data", "--dry-run=client"},
			}
			o := NewIOChaosOptions(tf, streams, string(v1alpha1.IoLatency))
			cmd := o.NewCobraCommand(Latency, LatencyShort)
			o.AddCommonFlag(cmd)
			cmd.Flags().StringVar(&o.Delay, "delay", "", `Specific delay time.`)

			for _, input := range inputs {
				Expect(cmd.Flags().Parse(input)).Should(Succeed())
				Expect(o.CreateOptions.Complete())
				Expect(o.Complete()).Should(Succeed())
				Expect(o.Validate()).Should(Succeed())
				Expect(o.Run()).Should(Succeed())
			}
		})

		It("fault io error", func() {
			inputs := [][]string{
				{"--errno=22", "--volume-path=/data", "--dry-run=client"},
			}
			o := NewIOChaosOptions(tf, streams, string(v1alpha1.IoFaults))
			cmd := o.NewCobraCommand(Errno, ErrnoShort)
			o.AddCommonFlag(cmd)
			cmd.Flags().IntVar(&o.Errno, "errno", 0, `The returned error number.`)

			for _, input := range inputs {
				Expect(cmd.Flags().Parse(input)).Should(Succeed())
				Expect(o.CreateOptions.Complete())
				Expect(o.Complete()).Should(Succeed())
				Expect(o.Validate()).Should(Succeed())
				Expect(o.Run()).Should(Succeed())
			}
		})

		It("fault io attribute", func() {
			inputs := [][]string{
				{"--perm=72", "--size=72", "--blocks=72", "--nlink=72", "--ino=72",
					"--uid=72", "--gid=72", "--volume-path=/data", "--dry-run=client"},
			}
			o := NewIOChaosOptions(tf, streams, string(v1alpha1.IoAttrOverride))
			cmd := o.NewCobraCommand(Attribute, AttributeShort)
			o.AddCommonFlag(cmd)
			cmd.Flags().Uint64Var(&o.Ino, "ino", 0, `ino number.`)
			cmd.Flags().Uint64Var(&o.Size, "size", 0, `File size.`)
			cmd.Flags().Uint64Var(&o.Blocks, "blocks", 0, `The number of blocks the file occupies.`)
			cmd.Flags().Uint16Var(&o.Perm, "perm", 0, `Decimal representation of file permissions.`)
			cmd.Flags().Uint32Var(&o.Nlink, "nlink", 0, `The number of hard links.`)
			cmd.Flags().Uint32Var(&o.UID, "uid", 0, `Owner's user ID.`)
			cmd.Flags().Uint32Var(&o.GID, "gid", 0, `The owner's group ID.`)

			for _, input := range inputs {
				Expect(cmd.Flags().Parse(input)).Should(Succeed())
				Expect(o.CreateOptions.Complete())
				Expect(o.Complete()).Should(Succeed())
				Expect(o.Validate()).Should(Succeed())
				Expect(o.Run()).Should(Succeed())
			}
		})

		It("fault io mistake", func() {
			inputs := [][]string{
				{"--filling=zero", "--maxOccurrences=10", "--maxLength=1", "--volume-path=/data", "--dry-run=client"},
			}
			o := NewIOChaosOptions(tf, streams, string(v1alpha1.IoMistake))
			cmd := o.NewCobraCommand(Mistake, MistakeShort)
			o.AddCommonFlag(cmd)
			cmd.Flags().StringVar(&o.Filling, "filling", "", `The filling content of the error data can only be zero (filling with 0) or random (filling with random bytes).`)
			cmd.Flags().IntVar(&o.MaxOccurrences, "maxOccurrences", 1, `The maximum number of times an error can occur per operation.`)
			cmd.Flags().IntVar(&o.MaxLength, "maxLength", 1, `The maximum length (in bytes) of each error.`)

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
