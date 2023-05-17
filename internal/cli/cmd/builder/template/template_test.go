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

package template

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/test/testdata"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

var _ = Describe("template", func() {
	var (
		tf      *cmdtesting.TestFactory
		streams genericclioptions.IOStreams
	)

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = testing.NewTestFactory("default")
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	testComponentTemplate := func(helmPath string, helmOutput string) {
		cmd := NewComponentTemplateRenderCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())

		if helmPath != "" {
			_ = cmd.Flags().Set("helm", helmPath)
		}
		if helmOutput != "" {
			_ = cmd.Flags().Set("helm-output", helmOutput)
		}
		_ = cmd.Flags().Set("memory", "8Gi")
		_ = cmd.Flags().Set("cpu", "8")
		_ = cmd.Flags().Set("replicas", "3")
		_ = cmd.Flags().Set("all", "true")
		cmd.Run(cmd, []string{})
	}

	It("should succeed", func() {
		componentRootPath := testdata.SubTestDataPath("../../deploy")
		testComponents := []string{
			"apecloud-mysql",
			"postgresql",
			"redis",
			"clickhouse",
		}

		_, err := os.ReadDir(componentRootPath)
		Expect(err).Should(Succeed())
		for _, component := range testComponents {
			componentPath := filepath.Join(componentRootPath, component)
			_, err := os.ReadDir(componentPath)
			Expect(err).Should(Succeed())
			testComponentTemplate(componentPath, "")
		}
	})

	It("test config template render without depend on helm", func() {
		testComponentTemplate("", testdata.SubTestDataPath("helm_template_output"))
	})
})
