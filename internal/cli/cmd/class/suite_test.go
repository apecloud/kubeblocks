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
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

const (
	namespace                                 = "test"
	testDefaultClassDefsPath                  = "../../testing/testdata/class.yaml"
	testCustomClassDefsPath                   = "../../testing/testdata/custom_class.yaml"
	testGeneralResourceConstraintPath         = "../../testing/testdata/resource-constraint-general.yaml"
	testMemoryOptimizedResourceConstraintPath = "../../testing/testdata/resource-constraint-memory-optimized.yaml"
)

var (
	classDef                          appsv1alpha1.ComponentClassDefinition
	generalResourceConstraint         appsv1alpha1.ComponentResourceConstraint
	memoryOptimizedResourceConstraint appsv1alpha1.ComponentResourceConstraint
)

var _ = BeforeSuite(func() {
	var err error

	classDefBytes, err := os.ReadFile(testDefaultClassDefsPath)
	Expect(err).ShouldNot(HaveOccurred())
	err = yaml.Unmarshal(classDefBytes, &classDef)
	Expect(err).ShouldNot(HaveOccurred())

	generalResourceConstraintBytes, err := os.ReadFile(testGeneralResourceConstraintPath)
	Expect(err).ShouldNot(HaveOccurred())
	err = yaml.Unmarshal(generalResourceConstraintBytes, &generalResourceConstraint)
	Expect(err).ShouldNot(HaveOccurred())

	memoryOptimizedResourceConstraintBytes, err := os.ReadFile(testMemoryOptimizedResourceConstraintPath)
	Expect(err).ShouldNot(HaveOccurred())
	err = yaml.Unmarshal(memoryOptimizedResourceConstraintBytes, &memoryOptimizedResourceConstraint)
	Expect(err).ShouldNot(HaveOccurred())

	err = appsv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).ShouldNot(HaveOccurred())

	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).ShouldNot(HaveOccurred())

})

func TestClass(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Class Suite")
}
