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
				{"--dry-run=client", "--patterns=kubeblocks.io"},
				{"--mode=one", "--patterns=kubeblocks.io", "--dry-run=client"},
				{"--mode=fixed", "--value=2", "--patterns=kubeblocks.io", "--dry-run=client"},
				{"--mode=fixed-percent", "--value=50", "--patterns=kubeblocks.io", "--dry-run=client"},
				{"--mode=random-max-percent", "--value=50", "--patterns=kubeblocks.io", "--dry-run=client"},
				{"--ns-fault=kb-system", "--patterns=kubeblocks.io", "--dry-run=client"},
				{"--node=minikube-m02", "--patterns=kubeblocks.io", "--dry-run=client"},
				{"--label=app.kubernetes.io/component=mysql", "--patterns=kubeblocks.io", "--dry-run=client"},
				{"--node-label=kubernetes.io/arch=arm64", "--patterns=kubeblocks.io", "--dry-run=client"},
				{"--annotation=example-annotation=group-a", "--patterns=kubeblocks.io", "--dry-run=client"},
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
				{"--patterns=kubeblocks.io", "--dry-run=client"},
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
