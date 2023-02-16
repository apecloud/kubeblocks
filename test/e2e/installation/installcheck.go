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

package envcheck

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli/values"

	. "github.com/apecloud/kubeblocks/test/e2e"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
)

const releaseName = "kubeblocks"
const releaseNS = "kubeblocks-e2e-test"

var chart = helm.InstallOpts{
	Name:      releaseName,
	Chart:     "../../deploy/helm",
	Wait:      true,
	Namespace: releaseNS,
	ValueOpts: &values.Options{
		Values: []string{
			"image.tag=latest",
			"image.pullPolicy=Always",
			"wesql.enabled=false",
		},
	},
	Login:           true,
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

		It("Install KubeBlocks via Helm", func() {
			cfg := getHelmActionCfg()
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
	cfg := getHelmActionCfg()
	res, err := chart.GetInstalled(cfg)
	if res == nil {
		return
	}
	Expect(err).NotTo(HaveOccurred())

	Expect(chart.Uninstall(cfg)).NotTo(HaveOccurred())
	uninstallHelmRelease()
}

func getHelmActionCfg() *action.Configuration {
	cfg, err := helm.NewActionConfig(releaseNS, "")
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())
	return cfg
}

func uninstallHelmRelease() {
	ctx := context.Background()
	ns := &corev1.Namespace{}
	Expect(K8sClient.Get(ctx, client.ObjectKey{
		Name: releaseNS,
	}, ns)).NotTo(HaveOccurred())
	Expect(K8sClient.Delete(ctx, ns)).NotTo(HaveOccurred())
}
