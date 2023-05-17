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
				{"--dry-run=client"},
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
				{"--container=mysql", "--container=config-manager", "--dry-run=client"},
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
