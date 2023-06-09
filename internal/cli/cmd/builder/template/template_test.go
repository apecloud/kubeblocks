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

package template

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
	"github.com/apecloud/kubeblocks/test/testdata"
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
		helmOutputRoot, err := os.MkdirTemp(os.TempDir(), "test")
		Expect(err).Should(Succeed())
		defer os.RemoveAll(helmOutputRoot)

		_, err = os.ReadDir(componentRootPath)
		Expect(err).Should(Succeed())
		for _, component := range testComponents {
			componentPath := filepath.Join(componentRootPath, component)
			_, err := os.ReadDir(componentPath)
			Expect(err).Should(Succeed())
			helmOutput := filepath.Join(helmOutputRoot, component)
			Expect(helmTemplate(componentPath, helmOutput)).Should(Succeed())
			testComponentTemplate(componentPath, helmOutput)
		}
	})

	It("test config template render without depend on helm", func() {
		testComponentTemplate(testdata.SubTestDataPath("../../deploy/apecloud-mysql"), "")
		testComponentTemplate(testdata.SubTestDataPath("../../deploy/postgresql"), "")
	})
})
