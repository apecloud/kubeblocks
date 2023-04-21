/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package patch

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("Patch", func() {
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace("default")
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("complete", func() {
		cmd := &cobra.Command{}
		o := NewOptions(tf, streams, types.ClusterGVR())
		o.AddFlags(cmd)
		Expect(o.complete(cmd)).Should(HaveOccurred())

		o.Names = []string{"c1"}
		Expect(o.complete(cmd)).Should(Succeed())
	})

	It("run", func() {
		cmd := &cobra.Command{}
		o := NewOptions(tf, streams, types.ClusterGVR())
		o.Names = []string{"c1"}
		o.AddFlags(cmd)

		o.Patch = "{terminationPolicy: Delete}"
		Expect(o.Run(cmd)).Should(HaveOccurred())
	})
})
