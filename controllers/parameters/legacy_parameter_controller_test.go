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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testparameters "github.com/apecloud/kubeblocks/pkg/testutil/parameters"
)

var _ = Describe("Deprecated Parameter Controller", func() {
	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	It("marks parameter as deprecated", func() {
		key := testapps.GetRandomizedKey(testCtx.DefaultNamespace, "parameter")
		parameter := testparameters.NewParameterFactory(key.Name, key.Namespace, "test-cluster", "mysql").
			AddParameters("max_connections", "100").
			Create(&testCtx).
			GetObject()

		Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(parameter), func(g Gomega, obj *parametersv1alpha1.Parameter) {
			g.Expect(obj.Status.ObservedGeneration).Should(Equal(obj.Generation))
			g.Expect(obj.Status.Phase).Should(Equal(parametersv1alpha1.CMergeFailedPhase))
			g.Expect(obj.Status.Message).Should(Equal(parameterDeprecatedMessage))
		})).Should(Succeed())
	})

	It("removes legacy finalizer for deleting parameter", func() {
		key := testapps.GetRandomizedKey(testCtx.DefaultNamespace, "parameter-delete")
		parameter := testparameters.NewParameterFactory(key.Name, key.Namespace, "test-cluster", "mysql").
			AddParameters("max_connections", "100").
			AddFinalizers([]string{constant.ConfigFinalizerName}).
			Create(&testCtx).
			GetObject()

		Expect(testCtx.Cli.Delete(testCtx.Ctx, parameter)).Should(Succeed())

		Eventually(func(g Gomega) {
			obj := &parametersv1alpha1.Parameter{}
			err := testCtx.Cli.Get(testCtx.Ctx, client.ObjectKeyFromObject(parameter), obj)
			g.Expect(apierrors.IsNotFound(err)).Should(BeTrue())
		}).Should(Succeed())
	})
})
