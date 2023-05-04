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
				analyzer: &preflightv1beta2.KBStorageClassAnalyze{
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
