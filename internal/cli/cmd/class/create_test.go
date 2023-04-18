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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/cli-runtime/pkg/genericclioptions"
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
		streams       genericclioptions.IOStreams
	)

	fillResources := func(o *CreateOptions, cpu string, memory string, storage []string) {
		o.CPU = cpu
		o.Memory = memory
		o.Storage = storage
		o.ClassName = fmt.Sprintf("custom-%s-%s", cpu, memory)
		o.Constraint = generalResourceConstraint.Name
	}

	BeforeEach(func() {
		streams, _, out, _ = genericclioptions.NewTestIOStreams()
		tf = testing.NewTestFactory(namespace)
		_ = appsv1alpha1.AddToScheme(scheme.Scheme)
		tf.FakeDynamicClient = testing.FakeDynamicClient(&classDef, &generalResourceConstraint, &memoryOptimizedResourceConstraint)

		createOptions = &CreateOptions{
			Factory:       tf,
			IOStreams:     streams,
			ClusterDefRef: "apecloud-mysql",
			ComponentType: "mysql",
		}
		Expect(createOptions.complete(tf)).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("should succeed to new command", func() {
		cmd := NewCreateCommand(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	Context("with resource arguments", func() {

		It("should fail if required arguments is missing", func() {
			fillResources(createOptions, "", "48Gi", nil)
			Expect(createOptions.validate([]string{"general-12c48g"})).Should(HaveOccurred())
			fillResources(createOptions, "12", "", nil)
			Expect(createOptions.validate([]string{"general-12c48g"})).Should(HaveOccurred())
			fillResources(createOptions, "12", "48g", nil)
			Expect(createOptions.validate([]string{})).Should(HaveOccurred())
		})

		It("should succeed with required arguments", func() {
			fillResources(createOptions, "2", "8Gi", []string{"name=data,size=10Gi", "name=log,size=1Gi"})
			Expect(createOptions.validate([]string{"general-2c8g"})).ShouldNot(HaveOccurred())
			Expect(createOptions.run()).ShouldNot(HaveOccurred())
			Expect(out.String()).Should(ContainSubstring(createOptions.ClassName))
		})

		It("should fail if constraint not exist", func() {
			createOptions.Constraint = "constraint-not-exist"
			fillResources(createOptions, "2", "8Gi", []string{"name=data,size=10Gi", "name=log,size=1Gi"})
			Expect(createOptions.run()).Should(HaveOccurred())
		})

		It("should fail if not conform to constraint", func() {
			By("memory not conform to constraint")
			fillResources(createOptions, "2", "9Gi", []string{"name=data,size=10Gi", "name=log,size=1Gi"})
			Expect(createOptions.run()).Should(HaveOccurred())

			By("cpu with invalid step")
			fillResources(createOptions, "0.6", "0.6Gi", []string{"name=data,size=10Gi", "name=log,size=1Gi"})
			Expect(createOptions.run()).Should(HaveOccurred())
		})

		It("should fail if class name is conflicted", func() {
			fillResources(createOptions, "1", "1Gi", []string{"name=data,size=10Gi", "name=log,size=1Gi"})
			createOptions.ClassName = "general-1c1g"
			Expect(createOptions.run()).Should(HaveOccurred())

			fillResources(createOptions, "0.5", "0.5Gi", []string{})
			Expect(createOptions.run()).ShouldNot(HaveOccurred())

			fillResources(createOptions, "0.5", "0.5Gi", []string{})
			Expect(createOptions.run()).Should(HaveOccurred())
		})
	})

	Context("with class definitions file", func() {
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
