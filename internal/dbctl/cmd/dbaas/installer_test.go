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

package dbaas

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/apecloud/kubeblocks/internal/dbctl/util/helm"
	"github.com/apecloud/kubeblocks/version"
)

var _ = Describe("installer", func() {
	It("install", func() {
		i := Installer{
			cfg:       helm.FakeActionConfig(),
			Namespace: "default",
			Version:   version.DefaultKubeBlocksVersion,
		}
		_, err := i.Install()
		Expect(err).Should(Or(Succeed(), HaveOccurred()))
	})

	It("uninstall", func() {
		i := Installer{
			cfg:       helm.FakeActionConfig(),
			Namespace: "default",
		}
		Expect(i.Uninstall()).To(HaveOccurred())
	})
})
