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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
)

var _ = Describe("provider util", func() {

	buildNodes := func(provider string) *corev1.NodeList {
		return &corev1.NodeList{
			Items: []corev1.Node{
				{
					Spec: corev1.NodeSpec{
						ProviderID: fmt.Sprintf("%s://blabla", provider),
					},
				},
			},
		}
	}
	It("GetK8sProvider", func() {
		cases := []struct {
			description    string
			version        string
			expectVersion  string
			expectProvider K8sProvider
			isCloud        bool
			nodes          *corev1.NodeList
		}{
			{
				"unknown provider without providerID and unique version identifier",
				"v1.25.0",
				"1.25.0",
				UnknownProvider,
				false,
				buildNodes(""),
			},
			{
				"EKS with unique version identifier",
				"v1.25.0-eks-123456",
				"1.25.0",
				EKSProvider,
				true,
				buildNodes(""),
			},
			{
				"EKS with providerID",
				"1.25.0",
				"1.25.0",
				EKSProvider,
				true,
				buildNodes("aws"),
			},
			{
				"GKE with unique version identifier",
				"v1.24.9-gke.3200",
				"1.24.9",
				GKEProvider,
				true,
				buildNodes(""),
			},
			{
				"GKE with providerID",
				"v1.24.9",
				"1.24.9",
				GKEProvider,
				true,
				buildNodes("gce"),
			},
			{
				"TKE with unique version identifier",
				"v1.24.4-tke.5",
				"1.24.4",
				TKEProvider,
				true,
				buildNodes(""),
			},
			{
				"TKE with providerID",
				"v1.24.9",
				"1.24.9",
				TKEProvider,
				true,
				buildNodes("qcloud"),
			},
			{
				"ACK with unique version identifier, as ACK don't have providerID",
				"v1.24.6-aliyun.1",
				"1.24.6",
				ACKProvider,
				true,
				buildNodes(""),
			},
			{
				"AKS with providerID, as AKS don't have unique version identifier",
				"v1.24.9",
				"1.24.9",
				AKSProvider,
				true,
				buildNodes("azure"),
			},
		}

		for _, c := range cases {
			By(c.description)
			Expect(GetK8sVersion(c.version)).Should(Equal(c.expectVersion))
			client := testing.FakeClientSet(c.nodes)
			p, err := GetK8sProvider(c.version, client)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(p).Should(Equal(c.expectProvider))
			Expect(p.IsCloud()).Should(Equal(c.isCloud))
		}
	})
})
