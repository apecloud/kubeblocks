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

package helm

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/version"
)

var _ = Describe("helm util", func() {
	It("repo", func() {
		r := repo.Entry{
			Name: "test-repo",
			URL:  "https://test-kubebllcks.com/test-repo",
		}
		Expect(AddRepo(&r)).Should(HaveOccurred())
		Expect(RemoveRepo(&r)).Should(Succeed())
	})

	It("Action Config", func() {
		cfg, err := NewActionConfig("test", "config", WithContext("context"))
		Expect(err).ShouldNot(HaveOccurred())
		Expect(cfg).ShouldNot(BeNil())
	})

	Context("Install", func() {
		var o *InstallOpts
		var cfg *action.Configuration

		BeforeEach(func() {
			o = &InstallOpts{
				Name:      testing.KubeBlocksChartName,
				Chart:     testing.KubeBlocksChartURL,
				Namespace: "default",
				Version:   version.DefaultKubeBlocksVersion,
			}
			cfg = FakeActionConfig()
			Expect(cfg).ShouldNot(BeNil())
		})

		It("Install", func() {
			_, err := o.Install(cfg)
			Expect(err).Should(HaveOccurred())
			Expect(o.Uninstall(cfg)).Should(HaveOccurred()) // release not found
		})

		It("should ignore when chart is already deployed", func() {
			err := cfg.Releases.Create(&release.Release{
				Name:    o.Name,
				Version: 1,
				Info: &release.Info{
					Status: release.StatusDeployed,
				},
			})
			Expect(err).Should(BeNil())
			_, err = o.Install(cfg)
			Expect(err).Should(BeNil())
			Expect(o.Uninstall(cfg)).Should(BeNil()) // release exists
		})

		It("should failed when chart is failed installed", func() {
			err := cfg.Releases.Create(&release.Release{
				Name:    o.Name,
				Version: 1,
				Info: &release.Info{
					Status: release.StatusFailed,
				},
			})
			Expect(err).Should(BeNil())
			_, err = o.Install(cfg)
			Expect(err.Error()).Should(ContainSubstring(ErrReleaseNotDeployed.Error()))
		})
	})

	Context("Upgrade", func() {
		var o *InstallOpts
		var cfg *action.Configuration

		BeforeEach(func() {
			o = &InstallOpts{
				Name:      types.KubeBlocksChartName,
				Chart:     "kubeblocks-test-chart",
				Namespace: "default",
				Version:   version.DefaultKubeBlocksVersion,
			}
			cfg = FakeActionConfig()
			Expect(cfg).ShouldNot(BeNil())
		})

		It("should fail when release is not found", func() {
			Expect(releaseNotFound(o.Upgrade(cfg))).Should(BeTrue())
			Expect(o.Uninstall(cfg)).Should(HaveOccurred()) // release not found
		})

		It("should failed at fetching charts when release is already deployed", func() {
			err := cfg.Releases.Create(&release.Release{
				Name:    o.Name,
				Version: 1,
				Info: &release.Info{
					Status: release.StatusDeployed,
				},
				Chart: &chart.Chart{},
			})
			Expect(err).Should(BeNil())
			Expect(o.Upgrade(cfg)).Should(HaveOccurred()) // failed at fetching charts
			Expect(o.Uninstall(cfg)).Should(BeNil())      // release exists
		})

		It("should fail when chart is already deployed", func() {
			err := cfg.Releases.Create(&release.Release{
				Name:    o.Name,
				Version: 1,
				Info: &release.Info{
					Status: release.StatusFailed,
				},
				Chart: &chart.Chart{},
			})
			Expect(err).Should(BeNil())
			Expect(errors.Is(o.Upgrade(cfg), ErrReleaseNotDeployed)).Should(BeTrue())
			Expect(o.Uninstall(cfg)).Should(BeNil()) // release exists
		})

	})

})
