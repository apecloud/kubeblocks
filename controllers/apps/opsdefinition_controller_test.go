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

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("OpsDefinition Controller", func() {

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		ml := client.HasLabels{testCtx.TestObjLabelKey}

		// resources should be released in following order
		testapps.ClearResources(&testCtx, intctrlutil.OpsDefinitionSignature, ml)
	}

	BeforeEach(func() {
		cleanEnv()

	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("Test OpsDefinition", func() {
		It("Test OpsDefinition", func() {
			opsDef := testapps.CreateCustomizedObj(&testCtx, "resources/mysql-opsdefinition-sql.yaml",
				&appsv1alpha1.OpsDefinition{}, testCtx.UseDefaultNamespace())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(opsDef), func(g Gomega, opsD *appsv1alpha1.OpsDefinition) {
				g.Expect(opsD.Status.Phase).Should(Equal(appsv1alpha1.AvailablePhase))
			}))
		})
	})

})
