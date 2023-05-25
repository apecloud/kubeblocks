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

package installation

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/repo"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
	. "github.com/apecloud/kubeblocks/test/e2e"
)

const releaseName = "kubeblocks"
const releaseNS = "kubeblocks-e2e-test"

var chart = helm.InstallOpts{
	Name:      releaseName,
	Chart:     "kubeblocks/kubeblocks",
	Wait:      true,
	Namespace: releaseNS,
	ValueOpts: &values.Options{
		Values: []string{
			"image.pullPolicy=Always",
			"wesql.enabled=false",
		},
	},
	TryTimes:        2,
	CreateNamespace: true,
}

func InstallationTest() {

	BeforeEach(func() {

	})

	AfterEach(func() {
	})

	Context("KubeBlocks installation", func() {
		AfterEach(func() {
		})

		It("add repo", func() {
			err := helm.AddRepo(&repo.Entry{Name: types.KubeBlocksRepoName, URL: util.GetHelmChartRepoURL()})
			Expect(err).NotTo(HaveOccurred())
		})

		It("Install KubeBlocks via Helm", func() {
			cfg := getHelmConfig()
			chart.Version = Version
			_, err := chart.Install(cfg)
			Expect(err).NotTo(HaveOccurred())
			// Expect(notes).NotTo(BeEmpty())
		})
	})
}

func UninstallationTest() {

	BeforeEach(func() {
	})

	AfterEach(func() {
	})

	Context("KubeBlocks uninstallation", func() {
		AfterEach(func() {
		})

		It("Uninstall KubeBlocks via Helm", func() {
			uninstallHelmRelease()
		})
	})
}

func CheckedUninstallHelmRelease() {
	cfg := getHelmConfig()
	actionCfg := getHelmActionCfg(cfg)
	res, err := chart.GetInstalled(actionCfg)
	if res == nil {
		return
	}
	Expect(err).NotTo(HaveOccurred())

	Expect(chart.Uninstall(cfg)).NotTo(HaveOccurred())
	uninstallHelmRelease()
}

func getHelmConfig() *helm.Config {
	return helm.NewConfig(releaseNS, "", "", false)
}

func getHelmActionCfg(cfg *helm.Config) *action.Configuration {
	actionCfg, err := helm.NewActionConfig(cfg)
	Expect(err).NotTo(HaveOccurred())
	Expect(actionCfg).NotTo(BeNil())
	return actionCfg
}

func uninstallHelmRelease() {
	ctx := context.Background()
	ns := &corev1.Namespace{}
	Expect(K8sClient.Get(ctx, client.ObjectKey{
		Name: releaseNS,
	}, ns)).NotTo(HaveOccurred())
	Expect(K8sClient.Delete(ctx, ns)).NotTo(HaveOccurred())
}
