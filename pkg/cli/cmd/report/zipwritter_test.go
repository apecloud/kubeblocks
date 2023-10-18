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

package report

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/printers"

	"github.com/apecloud/kubeblocks/pkg/cli/testing"
)

var _ = Describe("zipwritter", func() {
	var printer printers.ResourcePrinter
	const fileName = "test.zip"

	Context("zipwritter", func() {
		BeforeEach(func() {
			printer = &printers.JSONPrinter{}
		})
		AfterEach(func() {
			os.Remove(fileName)
		})

		It("should succeed to new zipwritter", func() {
			zipwritter := NewReportWritter()
			printer = &printers.JSONPrinter{}
			err := zipwritter.Init(fileName, printer.PrintObj)
			Expect(err).Should(Succeed())
			err = zipwritter.Init(fileName, printer.PrintObj)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("already exists"))
		})

		It("should succeed to close zipwritter", func() {
			zipwritter := NewReportWritter()
			printer = &printers.JSONPrinter{}
			err := zipwritter.Init(fileName, printer.PrintObj)
			Expect(err).Should(Succeed())
			err = zipwritter.Close()
			Expect(err).Should(Succeed())
		})

		It("should succeed to write kbversion", func() {
			zipwritter := NewReportWritter()
			printer = &printers.JSONPrinter{}
			err := zipwritter.Init(fileName, printer.PrintObj)
			Expect(err).Should(Succeed())

			client := testing.FakeClientSet(testing.FakeKBDeploy("0.5.23"))
			err = zipwritter.WriteKubeBlocksVersion("versions.txt", client)
			Expect(err).Should(Succeed())
			err = zipwritter.Close()
			Expect(err).Should(Succeed())
		})

		It("should succeed to write objects", func() {
			zipwritter := NewReportWritter()
			printer = &printers.JSONPrinter{}
			err := zipwritter.Init(fileName, printer.PrintObj)
			Expect(err).Should(Succeed())

			deploy := testing.FakeKBDeploy("0.5.23")
			unstructuredDeploy, err := runtime.DefaultUnstructuredConverter.ToUnstructured(deploy)
			Expect(err).Should(Succeed())
			unstructuredList := unstructured.UnstructuredList{}
			unstructuredList.Items = []unstructured.Unstructured{{Object: unstructuredDeploy}}

			err = zipwritter.WriteObjects("objects", []*unstructured.UnstructuredList{&unstructuredList}, "json")
			Expect(err).Should(Succeed())

			err = zipwritter.WriteSingleObject("single-object", deploy.Kind, deploy.Name, deploy, "json")
			Expect(err).Should(Succeed())

			err = zipwritter.Close()
			Expect(err).Should(Succeed())
		})

		It("should succeed to write events", func() {
			zipwritter := NewReportWritter()
			printer = &printers.JSONPrinter{}
			err := zipwritter.Init(fileName, printer.PrintObj)
			Expect(err).Should(Succeed())

			deploy := testing.FakeKBDeploy("0.5.23")
			event := testing.FakeEventForObject("test-events", deploy.Namespace, deploy.Name)
			events := map[string][]corev1.Event{"pod": {*event}}

			err = zipwritter.WriteEvents("events", events, "json")
			Expect(err).Should(Succeed())

			err = zipwritter.Close()
			Expect(err).Should(Succeed())
		})

		It("should succeed to write logs", func() {
			zipwritter := NewReportWritter()
			printer = &printers.JSONPrinter{}
			err := zipwritter.Init(fileName, printer.PrintObj)
			Expect(err).Should(Succeed())

			ctx := context.Background()
			pods := testing.FakePods(1, "test", "test-cluster")
			client := testing.FakeClientSet(&pods.Items[0])

			logOption := corev1.PodLogOptions{}
			err = zipwritter.WriteLogs("logs", ctx, client, pods, logOption, true)
			Expect(err).Should(Succeed())

			err = zipwritter.Close()
			Expect(err).Should(Succeed())
		})
	})
})
