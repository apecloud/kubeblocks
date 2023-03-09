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

package collector

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/client-go/rest"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
)

var _ = Describe("host_region_test", func() {
	var (
		timeOut       = 10 * time.Second
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
		}, timeOut).Should(Succeed())
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
		}, timeOut).Should(Succeed())
	})
})
