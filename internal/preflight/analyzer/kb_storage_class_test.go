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

package analyzer

import (
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	storagev1beta1 "k8s.io/api/storage/v1beta1"

	preflightv1beta2 "github.com/apecloud/kubeblocks/externalapis/preflight/v1beta2"
)

var (
	clusterResources = storagev1beta1.StorageClassList{
		Items: []storagev1beta1.StorageClass{
			{
				Provisioner: "ebs.csi.aws.com",
				Parameters:  map[string]string{"type": "gp3"},
			},
		},
	}
)

var _ = Describe("kb_storage_class_test", func() {
	var (
		analyzer AnalyzeStorageClassByKb
	)
	Context("analyze storage class test", func() {
		BeforeEach(func() {
			analyzer = AnalyzeStorageClassByKb{
				analyzer: &preflightv1beta2.KbStorageClassAnalyze{
					Provisioner:      "ebs.csi.aws.com",
					StorageClassType: "gp3",
					Outcomes: []*troubleshoot.Outcome{
						{
							Pass: &troubleshoot.SingleOutcome{

								Message: "analyze storage class success",
							},
							Fail: &troubleshoot.SingleOutcome{
								Message: "analyze storage class fail",
							},
						},
					}}}
		})
		It("Analyze test, and get file failed", func() {
			Eventually(func(g Gomega) {
				getCollectedFileContents := func(filename string) ([]byte, error) {
					return nil, errors.New("get file failed")
				}
				res, err := analyzer.Analyze(getCollectedFileContents, nil)
				g.Expect(err).To(HaveOccurred())
				g.Expect(res[0].IsFail).Should(BeTrue())
				g.Expect(res[0].IsPass).Should(BeFalse())
			}).Should(Succeed())
		})

		It("Analyze test, and return of get file is not clusterResource", func() {
			Eventually(func(g Gomega) {
				getCollectedFileContents := func(filename string) ([]byte, error) {
					return []byte("test"), nil
				}
				res, err := analyzer.Analyze(getCollectedFileContents, nil)
				g.Expect(err).To(HaveOccurred())
				g.Expect(res[0].IsFail).Should(BeTrue())
				g.Expect(res[0].IsPass).Should(BeFalse())
			}).Should(Succeed())
		})

		It("Analyze test, and analyzer result is expected that pass is true", func() {
			Eventually(func(g Gomega) {
				g.Expect(analyzer.IsExcluded()).Should(BeFalse())
				b, err := json.Marshal(clusterResources)
				g.Expect(err).NotTo(HaveOccurred())
				getCollectedFileContents := func(filename string) ([]byte, error) {
					return b, nil
				}
				res, err := analyzer.Analyze(getCollectedFileContents, nil)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res[0].IsPass).Should(BeTrue())
				g.Expect(res[0].IsFail).Should(BeFalse())
			}).Should(Succeed())
		})
		It("Analyze test, and analyzer result is expected that fail is true", func() {
			Eventually(func(g Gomega) {
				g.Expect(analyzer.IsExcluded()).Should(BeFalse())
				clusterResources.Items[0].Provisioner = "apecloud"
				b, err := json.Marshal(clusterResources)
				g.Expect(err).NotTo(HaveOccurred())
				getCollectedFileContents := func(filename string) ([]byte, error) {
					return b, nil
				}
				res, err := analyzer.Analyze(getCollectedFileContents, nil)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res[0].IsPass).Should(BeFalse())
				g.Expect(res[0].IsFail).Should(BeTrue())
			}).Should(Succeed())
		})
	})
})
