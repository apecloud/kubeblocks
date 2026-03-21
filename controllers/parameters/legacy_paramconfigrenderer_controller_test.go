/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package parameters

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("Legacy ParamConfigRenderer Controller", func() {
	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	It("should register and remove the legacy finalizer on delete", func() {
		pcr := &parametersv1alpha1.ParamConfigRenderer{}
		pcr.Name = pdcrName + "-finalizer"
		pcr.Spec.ComponentDef = compDefName
		Expect(testCtx.CreateObj(testCtx.Ctx, pcr)).Should(Succeed())

		key := client.ObjectKeyFromObject(pcr)
		Eventually(testapps.CheckObj(&testCtx, key, func(g Gomega, fetched *parametersv1alpha1.ParamConfigRenderer) {
			g.Expect(fetched.Finalizers).Should(ContainElement(constant.ConfigFinalizerName))
		})).Should(Succeed())

		Expect(testCtx.Cli.Delete(testCtx.Ctx, pcr)).Should(Succeed())
		Eventually(testapps.CheckObjExists(&testCtx, key, &parametersv1alpha1.ParamConfigRenderer{}, false)).Should(Succeed())
	})
})
