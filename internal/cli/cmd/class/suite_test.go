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
	namespace                          = "test"
	testDefaultClassDefsPath           = "../../testing/testdata/class.yaml"
	testCustomClassDefsPath            = "../../testing/testdata/custom_class.yaml"
	testGeneralClassFamilyPath         = "../../testing/testdata/classfamily-general.yaml"
	testMemoryOptimizedClassFamilyPath = "../../testing/testdata/classfamily-memory-optimized.yaml"
)

var (
	classDef                   []byte
	generalFamilyDef           []byte
	memoryOptimizedFamilyDef   []byte
	generalClassFamily         appsv1alpha1.ClassFamily
	memoryOptimizedClassFamily appsv1alpha1.ClassFamily
)

var _ = BeforeSuite(func() {
	var err error

	classDef, err = os.ReadFile(testDefaultClassDefsPath)
	Expect(err).ShouldNot(HaveOccurred())

	generalFamilyDef, err = os.ReadFile(testGeneralClassFamilyPath)
	Expect(err).ShouldNot(HaveOccurred())
	err = yaml.Unmarshal(generalFamilyDef, &generalClassFamily)
	Expect(err).ShouldNot(HaveOccurred())

	memoryOptimizedFamilyDef, err = os.ReadFile(testMemoryOptimizedClassFamilyPath)
	Expect(err).ShouldNot(HaveOccurred())
	err = yaml.Unmarshal(memoryOptimizedFamilyDef, &memoryOptimizedClassFamily)
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
