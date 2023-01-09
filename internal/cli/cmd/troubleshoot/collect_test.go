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

package troubleshoot

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

var _ = Describe("Collect Test", func() {
	It("parseTimeFlags Test", func() {
		sinceStr := "5m"
		sinceTimeStr := "2023-01-09T15:18:46+08:00"
		Expect(parseTimeFlags(sinceStr, sinceTimeStr, []*troubleshootv1beta2.Collect{})).Should(HaveOccurred())
		Expect(parseTimeFlags("", sinceTimeStr, []*troubleshootv1beta2.Collect{})).Should(Succeed())
		Expect(parseTimeFlags(sinceStr, "", []*troubleshootv1beta2.Collect{})).Should(Succeed())
	})
})
