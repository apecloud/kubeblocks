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

package alert

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

var _ = Describe("alter", func() {
	f := cmdtesting.NewTestFactory()
	s, _, _, _ := genericclioptions.NewTestIOStreams()
	defer f.Cleanup()

	It("create new alert cmd", func() {
		cmd := NewAlertCmd(f, s)
		Expect(cmd).NotTo(BeNil())
		Expect(cmd.HasSubCommands()).Should(BeTrue())
	})
})
