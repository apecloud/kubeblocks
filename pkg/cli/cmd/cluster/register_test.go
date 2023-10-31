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

package cluster

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/cli-runtime/pkg/genericiooptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

var _ = Describe("cluster register", func() {
	var streams genericiooptions.IOStreams
	var tf *cmdtesting.TestFactory
	var tempLocalPath string
	BeforeEach(func() {
		streams, _, _, _ = genericiooptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace("default")
		tempLocalPath = filepath.Join(os.TempDir(), "fake.tgz")
		Expect(os.WriteFile(tempLocalPath, []byte("fake-data"), 0666)).Should(Succeed())
	})

	AfterEach(func() {
		os.Remove(tempLocalPath)
	})

	It("register command", func() {
		option := newRegisterOption(tf, streams)
		Expect(option).ShouldNot(BeNil())

		cmd := newRegisterCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("register command validate", func() {
		o := &registerOption{
			Factory:     tf,
			IOStreams:   streams,
			clusterType: "not-allow-name",
			source:      tempLocalPath,
		}
		Expect(o.validate()).Should(HaveOccurred())

		o.clusterType = "mysql"
		// builtin chart
		Expect(o.validate()).Should(HaveOccurred())

		o.clusterType = "oracle"
		Expect(o.validate()).Should(Succeed())

	})

	It("test validateSource", func() {
		o := &registerOption{
			Factory:     tf,
			IOStreams:   streams,
			clusterType: "newCLuster",
			source:      tempLocalPath,
		}
		Expect(o.validate()).Should(Succeed())
		o.source = "https://github.com/apecloud/helm-charts/releases/download/orioledb-cluster-0.6.0-beta.44/orioledb-cluster-0.6.0-beta.44.tgz"
		Expect(o.validate()).Should(Succeed())
		o.source = "This is a bad url or a local file path do not existed"
		Expect(o.validate()).Should(HaveOccurred())

	})

	It("test copy file", func() {
		Expect(copyFile(tempLocalPath, tempLocalPath)).Should(Succeed())
		Expect(copyFile("bad local path", tempLocalPath)).Should(HaveOccurred())
		Expect(copyFile(tempLocalPath, filepath.Join(os.TempDir(), "fake-other.tgz"))).Should(Succeed())
		file, _ := os.ReadFile(filepath.Join(os.TempDir(), "fake-other.tgz"))
		Expect(string(file)).Should(Equal("fake-data"))
		os.Remove(filepath.Join(os.TempDir(), "fake-other.tgz"))
	})
})
