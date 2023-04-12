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

package class

import (
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var _ = Describe("template", func() {
	var (
		out     *bytes.Buffer
		streams genericclioptions.IOStreams
	)

	BeforeEach(func() {
		streams, _, out, _ = genericclioptions.NewTestIOStreams()
	})

	It("command should succeed", func() {
		cmd := NewTemplateCmd(streams)
		Expect(cmd).ShouldNot(BeNil())

		cmd.Run(cmd, []string{})
		Expect(out.String()).ShouldNot(BeEmpty())
	})
})
