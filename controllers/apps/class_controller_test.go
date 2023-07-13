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

package apps

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("", func() {

	var componentClassDefinition *v1alpha1.ComponentClassDefinition

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
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
		constraint := testapps.NewComponentResourceConstraintFactory(testapps.DefaultResourceConstraintName).
			AddConstraints(testapps.GeneralResourceConstraint).
			Create(&testCtx).GetObject()

		componentClassDefinition = testapps.NewComponentClassDefinitionFactory("custom", "apecloud-mysql", "mysql").
			AddClasses(constraint.Name, []v1alpha1.ComponentClass{testapps.Class1c1g}).
			Create(&testCtx).GetObject()

		key := client.ObjectKeyFromObject(componentClassDefinition)
		Eventually(testapps.CheckObj(&testCtx, key, func(g Gomega, pobj *v1alpha1.ComponentClassDefinition) {
			g.Expect(pobj.Status.Classes).ShouldNot(BeEmpty())
			g.Expect(pobj.Status.Classes[0].Name).Should(Equal(testapps.Class1c1gName))
		})).Should(Succeed())
	})
})
