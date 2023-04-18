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

package apps

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("", func() {

	var componentClassDefinition *v1alpha1.ComponentClassDefinition

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete rest mocked objects
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		testapps.ClearResources(&testCtx, intctrlutil.ComponentResourceConstraintSignature, ml)
		testapps.ClearResources(&testCtx, intctrlutil.ComponentClassDefinitionSignature, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	It("Class should exist in status", func() {
		var (
			clsName = "test"
			class   = v1alpha1.ComponentClass{
				Name:   clsName,
				CPU:    resource.MustParse("1"),
				Memory: resource.MustParse("1Gi"),
			}
		)

		constraint := testapps.NewComponentResourceConstraintFactory(testapps.DefaultGeneralResourceConstraintName).
			AddConstraints(testapps.ResourceConstraintNormal).
			AddConstraints(testapps.ResourceConstraintSpecial).
			Create(&testCtx).GetObject()

		componentClassDefinition = testapps.NewComponentClassDefinitionFactory("custom", "apecloud-mysql", "mysql").
			AddClassGroup(constraint.Name).
			AddClasses([]v1alpha1.ComponentClass{class}).
			Create(&testCtx).GetObject()

		key := client.ObjectKeyFromObject(componentClassDefinition)
		Eventually(testapps.CheckObj(&testCtx, key, func(g Gomega, pobj *v1alpha1.ComponentClassDefinition) {
			g.Expect(pobj.Status.Classes).ShouldNot(BeEmpty())
			g.Expect(pobj.Status.Classes[0].Name).Should(Equal(clsName))
		})).Should(Succeed())
	})
})
