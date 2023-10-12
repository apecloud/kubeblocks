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

package kubeblocks

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
)

var _ = Describe("kubeblocks compare", func() {
	var (
		cmd     *cobra.Command
		streams genericiooptions.IOStreams
		tf      *cmdtesting.TestFactory
	)
	const kbVersion = "0.5.0"

	BeforeEach(func() {
		streams, _, _, _ = genericiooptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace(namespace)
		tf.Client = &clientfake.RESTClient{}
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("check compare", func() {
		cmd = newCompareCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	Context("run compare", func() {
		o := &InstallOptions{
			Options: Options{
				IOStreams: streams,
				HelmCfg:   helm.NewFakeConfig(namespace),
				Client:    testing.FakeClientSet(testing.FakeKBDeploy(kbVersion)),
				Dynamic:   testing.FakeDynamicClient(),
			},
		}

		It("validate compare version", func() {
			Expect(o.compare([]string{kbVersion}, true)).Should(HaveOccurred())
			Expect(o.compare([]string{}, true)).Should(HaveOccurred())
			Expect(o.compare([]string{"0.5.0", "0.5.1"}, true)).Should(HaveOccurred())
		})
	})
})
