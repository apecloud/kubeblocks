/*
Copyright (C) 2022 ApeCloud Co., Ltd

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
