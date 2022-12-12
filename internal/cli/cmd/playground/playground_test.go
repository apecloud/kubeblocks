/*
Copyright ApeCloud Inc.

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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cloudprovider"
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
			clusterDef:    "test-cd",
			CloudProvider: defaultCloudProvider,
			IOStreams:     streams,
		}
		Expect(o.validate()).Should(Succeed())
		Expect(o.run()).To(MatchError(MatchRegexp("failed to set up k3d cluster")))

		Expect(o.installKubeBlocks()).Should(HaveOccurred())
		Expect(o.installCluster()).Should(HaveOccurred())
	})

	It("init at remote cloud", func() {
		o := &initOptions{
			IOStreams:     streams,
			CloudProvider: cloudprovider.AWS,
		}
		Expect(o.run()).Should(HaveOccurred())
	})

	It("destroy command", func() {
		cmd := newDestroyCmd(streams)
		Expect(cmd).ShouldNot(BeNil())

		o := &destroyOptions{
			IOStreams: streams,
		}
		Expect(o.destroyPlayground()).Should(HaveOccurred())
		_, _ = removePlaygroundDir()
	})

	It("guide", func() {
		cmd := newGuideCmd()
		Expect(cmd).ShouldNot(BeNil())
		Expect(runGuide()).Should(HaveOccurred())
	})

	It("find latest version", func() {
		const clusterDefName = "test-cluster-def"
		genVersion := func(name string, t time.Time) dbaasv1alpha1.AppVersion {
			v := dbaasv1alpha1.AppVersion{}
			v.Name = name
			v.SetLabels(map[string]string{types.ClusterDefLabelKey: clusterDefName})
			v.SetCreationTimestamp(metav1.NewTime(t))
			return v
		}

		versionList := &dbaasv1alpha1.AppVersionList{}
		versionList.Items = append(versionList.Items,
			genVersion("old-version", time.Now().AddDate(0, 0, -1)),
			genVersion("now-version", time.Now()))

		latestVer := findLatestVersion(versionList)
		Expect(latestVer).ShouldNot(BeNil())
		Expect(latestVer.Name).Should(Equal("now-version"))
	})
})
