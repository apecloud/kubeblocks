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

package cluster

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("cluster label", func() {
	var (
		streams genericclioptions.IOStreams
		tf      *cmdtesting.TestFactory
	)

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace("default")
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("label command", func() {
		cmd := NewLabelCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	Context("complete", func() {
		var o *LabelOptions
		var cmd *cobra.Command
		var args []string
		BeforeEach(func() {
			cmd = NewLabelCmd(tf, streams)
			o = NewLabelOptions(tf, streams, types.ClusterDefGVR())
			args = []string{"c1", "env=dev"}
		})

		It("args is empty", func() {
			Expect(o.complete(cmd, nil)).Should(Succeed())
			Expect(o.validate()).Should(HaveOccurred())
		})

		It("cannot set --all and --selector at the same time", func() {
			o.all = true
			o.selector = "status=unhealthy"
			Expect(o.complete(cmd, args)).Should(Succeed())
			Expect(o.validate()).Should(HaveOccurred())
		})

		It("at least one label update is required", func() {
			Expect(o.complete(cmd, []string{"c1"})).Should(Succeed())
			Expect(o.validate()).Should(HaveOccurred())
		})

		It("can not both modify and remove label in the same command", func() {
			Expect(o.complete(cmd, []string{"c1", "env=dev", "env-"})).Should(HaveOccurred())
		})
	})
})
