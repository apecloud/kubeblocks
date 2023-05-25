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

package collector

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/client-go/rest"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
)

var _ = Describe("host_region_test", func() {
	var (
		clusterRegion CollectClusterRegion
		testName      = "testName"
		config        *rest.Config
	)
	BeforeEach(func() {
		clusterRegion = CollectClusterRegion{
			HostCollector: &preflightv1beta2.ClusterRegion{
				HostCollectorMeta: troubleshoot.HostCollectorMeta{
					CollectorName: testName,
				},
				ProviderName: "eks",
			},
			BundlePath: "",
		}
		config = &rest.Config{Host: "https://xxx.yl4.cn-northwest-1.eks.amazonaws.com.cn"}
	})
	It("CollectClusterRegion test, get config fail and expect error", func() {
		Eventually(func(g Gomega) {
			g.Expect(clusterRegion.Title()).Should(Equal(testName))
			g.Expect(clusterRegion.IsExcluded()).Should(BeFalse())
			data, err := doCollect(func() (*rest.Config, error) {
				return config, errors.New("fail")
			}, "eks")
			g.Expect(err).Should(HaveOccurred())
			g.Expect(data).Should(BeNil())
		}).Should(Succeed())
	})

	It("CollectClusterRegion test, and expect success", func() {
		Eventually(func(g Gomega) {
			g.Expect(clusterRegion.Title()).Should(Equal(testName))
			g.Expect(clusterRegion.IsExcluded()).Should(BeFalse())
			data, err := doCollect(func() (*rest.Config, error) {
				return config, nil
			}, "eks")
			g.Expect(err).ShouldNot(HaveOccurred())
			g.Expect(string(data)).Should(Equal(`{"regionName":"cn-northwest-1"}`))
		}).Should(Succeed())
	})
})
