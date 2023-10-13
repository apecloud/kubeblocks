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

package class

import (
	"bytes"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"k8s.io/client-go/kubernetes/scheme"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
)

var _ = Describe("create", func() {
	var (
		createOptions *CreateOptions
		out           *bytes.Buffer
		tf            *cmdtesting.TestFactory
		streams       genericiooptions.IOStreams
	)

	fillResources := func(o *CreateOptions, cpu string, memory string) {
		o.CPU = cpu
		o.Memory = memory
		o.ClassName = fmt.Sprintf("custom-%s-%s", cpu, memory)
	}

	BeforeEach(func() {
		streams, _, out, _ = genericiooptions.NewTestIOStreams()
		tf = testing.NewTestFactory(namespace)
		_ = appsv1alpha1.AddToScheme(scheme.Scheme)
		createOptions = &CreateOptions{
			IOStreams:     streams,
			ClusterDefRef: "apecloud-mysql",
			ComponentType: "mysql",
		}
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("should succeed to new command", func() {
		cmd := NewCreateCommand(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	Context("with resource arguments", func() {

		Context("without constraints", func() {
			BeforeEach(func() {
				tf.FakeDynamicClient = testing.FakeDynamicClient(&classDef, &generalResourceConstraintWithoutSelector)
				createOptions.Factory = tf
				Expect(createOptions.complete(tf)).ShouldNot(HaveOccurred())
			})

			It("should succeed if component don't have any constraints", func() {
				fillResources(createOptions, "1", "100Gi")
				Expect(createOptions.run()).ShouldNot(HaveOccurred())
			})

		})

		Context("with constraints", func() {
			BeforeEach(func() {
				tf.FakeDynamicClient = testing.FakeDynamicClient(&classDef, &generalResourceConstraint, &memoryOptimizedResourceConstraint)
				createOptions.Factory = tf
				Expect(createOptions.complete(tf)).ShouldNot(HaveOccurred())
			})

			It("should fail if required arguments is missing", func() {
				fillResources(createOptions, "", "48Gi")
				Expect(createOptions.validate([]string{"general-12c48g"})).Should(HaveOccurred())
				fillResources(createOptions, "12", "")
				Expect(createOptions.validate([]string{"general-12c48g"})).Should(HaveOccurred())
				fillResources(createOptions, "12", "48g")
				Expect(createOptions.validate([]string{})).Should(HaveOccurred())
			})

			It("should succeed with required arguments", func() {
				fillResources(createOptions, "96", "384Gi")
				Expect(createOptions.validate([]string{"general-96c384g"})).ShouldNot(HaveOccurred())
				Expect(createOptions.run()).ShouldNot(HaveOccurred())
				Expect(out.String()).Should(ContainSubstring(createOptions.ClassName))
			})

			It("should fail if not conformed to constraint", func() {
				By("memory not conformed to constraint")
				fillResources(createOptions, "2", "9Gi")
				Expect(createOptions.run()).Should(HaveOccurred())

				By("CPU with invalid step")
				fillResources(createOptions, "0.6", "0.6Gi")
				Expect(createOptions.run()).Should(HaveOccurred())
			})

			It("should fail if class name is conflicted", func() {
				// class may be conflict only within the same object, so we set the objectName to be consistent with the name of the object classDef
				createOptions.objectName = "kb.classes.default.apecloud-mysql.mysql"

				fillResources(createOptions, "1", "1Gi")
				createOptions.ClassName = "general-1c1g"
				Expect(createOptions.run()).Should(HaveOccurred())

				fillResources(createOptions, "0.5", "0.5Gi")
				Expect(createOptions.run()).ShouldNot(HaveOccurred())

				fillResources(createOptions, "0.5", "0.5Gi")
				Expect(createOptions.run()).Should(HaveOccurred())
			})
		})
	})

	Context("with class definitions file", func() {
		BeforeEach(func() {
			tf.FakeDynamicClient = testing.FakeDynamicClient(&classDef, &generalResourceConstraint, &memoryOptimizedResourceConstraint)
			createOptions.Factory = tf
			Expect(createOptions.complete(tf)).ShouldNot(HaveOccurred())
		})

		It("should succeed", func() {
			createOptions.File = testCustomClassDefsPath
			Expect(createOptions.run()).ShouldNot(HaveOccurred())
			Expect(out.String()).Should(ContainSubstring("custom-1c1g"))
			Expect(out.String()).Should(ContainSubstring("custom-4c16g"))
			// memory optimized classes
			Expect(out.String()).Should(ContainSubstring("custom-2c16g"))
			Expect(out.String()).Should(ContainSubstring("custom-4c64g"))
		})

	})

})
