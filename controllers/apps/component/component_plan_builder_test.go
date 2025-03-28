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

package component

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("component plan builder test", func() {
	const (
		compDefName = "test-compdef"
		compName    = "test-comp"
	)

	// Cleanups
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete components (and all dependent sub-resources), and component definitions & versions
		testapps.ClearComponentResourcesWithRemoveFinalizerOption(&testCtx)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("test init", func() {
		It("should init successfully", func() {
			compObj := testapps.NewComponentFactory(testCtx.DefaultNamespace, compName, compDefName).WithRandomName().GetObject()
			Expect(testCtx.Cli.Create(testCtx.Ctx, compObj)).Should(Succeed())
			compKey := client.ObjectKeyFromObject(compObj)
			Eventually(testapps.CheckObjExists(&testCtx, compKey, &appsv1.Component{}, true)).Should(Succeed())

			req := ctrl.Request{
				NamespacedName: compKey,
			}
			reqCtx := intctrlutil.RequestCtx{
				Ctx: testCtx.Ctx,
				Req: req,
				Log: log.FromContext(ctx).WithValues("component", req.NamespacedName),
			}
			planBuilder := newComponentPlanBuilder(reqCtx, testCtx.Cli)
			Expect(planBuilder.Init()).Should(Succeed())
		})
	})
})
