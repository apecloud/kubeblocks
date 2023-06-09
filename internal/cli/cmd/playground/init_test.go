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

package playground

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	cp "github.com/apecloud/kubeblocks/internal/cli/cloudprovider"
	clitesting "github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
)

var _ = Describe("playground", func() {
	const (
		testKubeConfigPath = "./testdata/kubeconfig"
	)

	var streams genericclioptions.IOStreams

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		Expect(os.Setenv(types.CliHomeEnv, "./testdata")).Should(Succeed())
	})

	It("init at local host", func() {
		cmd := newInitCmd(streams)
		Expect(cmd != nil).Should(BeTrue())

		o := &initOptions{
			clusterDef:     clitesting.ClusterDefName,
			clusterVersion: clitesting.ClusterVersionName,
			IOStreams:      streams,
			cloudProvider:  defaultCloudProvider,
			helmCfg:        helm.NewConfig("", testKubeConfigPath, "", false),
		}
		Expect(o.validate()).Should(Succeed())
		Expect(o.run()).Should(HaveOccurred())
		Expect(o.installKubeBlocks("test")).Should(HaveOccurred())
		Expect(o.createCluster()).Should(HaveOccurred())
	})

	It("init at remote cloud", func() {
		o := &initOptions{
			IOStreams:      streams,
			clusterDef:     clitesting.ClusterDefName,
			clusterVersion: clitesting.ClusterVersionName,
			cloudProvider:  cp.AWS,
		}
		Expect(o.validate()).Should(HaveOccurred())
	})
})
