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

	"github.com/replicatedhq/troubleshoot/pkg/preflight"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

var _ = Describe("Preflight Test", func() {
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory
	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace("default")
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("Complete and Validate Test", func() {
		p := &preflightOptions{
			factory:        tf,
			IOStreams:      streams,
			PreflightFlags: preflight.NewPreflightFlags(),
		}
		Expect(p.complete(nil)).Should(Succeed())
		Expect(p.validate()).Should(HaveOccurred())
		Expect(p.complete([]string{"file1", "file2"})).Should(Succeed())
		Expect(len(p.yamlCheckFiles)).Should(Equal(2))
		Expect(p.validate()).Should(Succeed())
	})
})
