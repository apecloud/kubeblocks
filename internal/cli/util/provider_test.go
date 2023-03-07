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

package util

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("provider util", func() {
	It("GetK8sProvider", func() {
		cases := []struct {
			version  string
			expected string
			provider K8sProvider
			isCloud  bool
		}{
			{
				"v1.25.0",
				"1.25.0",
				UnknownProvider,
				false,
			},
			{
				"v1.25.0-eks-123456",
				"1.25.0",
				EKSProvider,
				true,
			},
			{
				"1.25.0",
				"1.25.0",
				UnknownProvider,
				false,
			},
			{
				"",
				"",
				UnknownProvider,
				false,
			},
			{
				"v1.24.9-gke.3200",
				"1.24.9",
				GKEProvider,
				true,
			},
			{
				"v1.24.9-gke",
				"1.24.9",
				GKEProvider,
				true,
			},
		}

		for _, c := range cases {
			Expect(GetK8sVersion(c.version)).Should(Equal(c.expected))
			p := GetK8sProvider(c.version)
			Expect(p).Should(Equal(c.provider))
			Expect(p.IsCloud()).Should(Equal(c.isCloud))
		}
	})
})
