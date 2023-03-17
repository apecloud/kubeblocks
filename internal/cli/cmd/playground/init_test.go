/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package playground

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	cp "github.com/apecloud/kubeblocks/internal/cli/cloudprovider"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
)

var _ = Describe("playground", func() {
	var streams genericclioptions.IOStreams

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
	})

	It("init at local host", func() {
		cmd := newInitCmd(streams)
		Expect(cmd != nil).Should(BeTrue())

		o := &initOptions{
			clusterDef: "test-cd",
			IOStreams:  streams,
			baseOptions: baseOptions{
				cloudProvider: defaultCloudProvider,
			},
			helmCfg: helm.NewConfig("", util.ConfigPath("config"), "", false),
		}
		Expect(o.validate()).Should(Succeed())
		Expect(o.run()).Should(HaveOccurred())
		Expect(o.installKubeBlocks()).Should(HaveOccurred())
		Expect(o.createCluster()).Should(HaveOccurred())
	})

	It("init at remote cloud", func() {
		o := &initOptions{
			IOStreams: streams,
			baseOptions: baseOptions{
				cloudProvider: cp.AWS,
			},
			clusterDef: "test-cd",
		}
		Expect(o.validate()).Should(HaveOccurred())
	})

	It("guide", func() {
		cmd := newGuideCmd()
		Expect(cmd).ShouldNot(BeNil())
	})
})
