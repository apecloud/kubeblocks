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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/version"
)

var _ = Describe("helm util", func() {
	It("add and remove repo", func() {
		r := repo.Entry{
			Name: "test-repo",
			URL:  "https://test-kubebllcks.com/test-repo",
		}
		Expect(AddRepo(&r)).Should(HaveOccurred())
		Expect(RemoveRepo(&r)).Should(Succeed())
	})

	It("Action Config", func() {
		cfg := NewConfig("test", "config", "context", false)
		actionCfg, err := NewActionConfig(cfg)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(actionCfg).ShouldNot(BeNil())
	})

	Context("Install", func() {
		var o *InstallOpts
		var cfg *Config
		var actionCfg *action.Configuration

		BeforeEach(func() {
			o = &InstallOpts{
				Name:      testing.KubeBlocksChartName,
				Chart:     testing.KubeBlocksChartURL,
				Namespace: "default",
				Version:   version.DefaultKubeBlocksVersion,
			}
			cfg = NewFakeConfig("default")
			actionCfg, _ = NewActionConfig(cfg)
			Expect(actionCfg).ShouldNot(BeNil())
		})

		It("Install", func() {
			_, err := o.Install(cfg)
			Expect(err).Should(HaveOccurred())
			Expect(o.Uninstall(cfg)).Should(HaveOccurred()) // release not found
		})

		It("should ignore when chart is already deployed", func() {
			err := actionCfg.Releases.Create(&release.Release{
				Name:    o.Name,
				Version: 1,
				Info: &release.Info{
					Status: release.StatusDeployed,
				},
			})
			Expect(err).Should(BeNil())
			_, err = o.tryInstall(actionCfg)
			Expect(err).Should(BeNil())
			Expect(o.tryUninstall(actionCfg)).Should(BeNil()) // release exists
		})

		It("should fail when chart is failed installed", func() {
			err := actionCfg.Releases.Create(&release.Release{
				Name:    o.Name,
				Version: 1,
				Info: &release.Info{
					Status: release.StatusFailed,
				},
			})
			Expect(err).Should(BeNil())
			_, err = o.Install(cfg)
			Expect(err).Should(HaveOccurred())
		})
	})

	Context("Upgrade", func() {
		var o *InstallOpts
		var cfg *Config
		var actionCfg *action.Configuration

		BeforeEach(func() {
			o = &InstallOpts{
				Name:      types.KubeBlocksChartName,
				Chart:     "kubeblocks-test-chart",
				Namespace: "default",
				Version:   version.DefaultKubeBlocksVersion,
			}
			cfg = NewFakeConfig("default")
			actionCfg, _ = NewActionConfig(cfg)
			Expect(actionCfg).ShouldNot(BeNil())
		})

		It("should fail when release is not found", func() {
			Expect(releaseNotFound(o.Upgrade(cfg))).Should(BeTrue())
			Expect(o.Uninstall(cfg)).Should(HaveOccurred()) // release not found
		})

		It("should fail at fetching charts when release is already deployed", func() {
			err := actionCfg.Releases.Create(&release.Release{
				Name:    o.Name,
				Version: 1,
				Info: &release.Info{
					Status: release.StatusDeployed,
				},
				Chart: &chart.Chart{},
			})
			Expect(err).Should(BeNil())
			_, err = o.tryUpgrade(actionCfg)
			Expect(err).Should(HaveOccurred())                // failed at fetching charts
			Expect(o.tryUninstall(actionCfg)).Should(BeNil()) // release exists
		})

		It("should fail when chart is already deployed", func() {
			err := actionCfg.Releases.Create(&release.Release{
				Name:    o.Name,
				Version: 1,
				Info: &release.Info{
					Status: release.StatusFailed,
				},
				Chart: &chart.Chart{},
			})
			Expect(err).Should(BeNil())
			_, err = o.tryUpgrade(actionCfg)
			Expect(errors.Is(err, ErrReleaseNotDeployed)).Should(BeTrue())
			Expect(o.tryUninstall(actionCfg)).Should(BeNil()) // release exists
		})
	})

	It("get chart versions", func() {
		versions, _ := GetChartVersions(testing.KubeBlocksChartName)
		Expect(versions).Should(BeNil())
	})
})
