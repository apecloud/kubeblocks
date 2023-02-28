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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	cp "github.com/apecloud/kubeblocks/internal/cli/cloudprovider"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
)

var _ = Describe("playground", func() {
	var streams genericclioptions.IOStreams

	// use a fake URL to test
	types.KubeBlocksChartName = testing.KubeBlocksChartName
	types.KubeBlocksChartURL = testing.KubeBlocksChartURL

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
	})

	It("new playground command", func() {
		cmd := NewPlaygroundCmd(streams)
		Expect(cmd).ShouldNot(BeNil())
		Expect(cmd.HasSubCommands()).Should(BeTrue())
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
		}
		Expect(o.validate()).Should(Succeed())
		Expect(o.run()).To(MatchError(MatchRegexp("failed to set up k3d cluster")))
		Expect(o.installKubeBlocks()).Should(HaveOccurred())
		Expect(o.installCluster()).Should(HaveOccurred())
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

	It("destroy command", func() {
		cmd := newDestroyCmd(streams)
		Expect(cmd).ShouldNot(BeNil())

		o := &destroyOptions{
			IOStreams: streams,
		}
		Expect(o.destroyPlayground()).Should(HaveOccurred())
	})

	It("guide", func() {
		cmd := newGuideCmd()
		Expect(cmd).ShouldNot(BeNil())
		printGuide(false, "")
	})

	It("test", func() {
		fmt.Println(rand.String(10))
	})
})
