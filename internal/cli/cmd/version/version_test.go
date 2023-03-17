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

package version

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

var _ = Describe("version", func() {
	It("version", func() {
		tf := cmdtesting.NewTestFactory()
		tf.Client = &fake.RESTClient{}
		By("testing version command")
		cmd := NewVersionCmd(tf)
		Expect(cmd).ShouldNot(BeNil())

		By("testing run")
		o := &versionOptions{}
		o.Run(tf)
	})
})
