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

package rollout

import (
	. "github.com/onsi/ginkgo/v2"

	"sigs.k8s.io/controller-runtime/pkg/client"

	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("Rollout Controller", func() {
	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		ml := client.HasLabels{testCtx.TestObjLabelKey}

		// non-namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.ComponentDefinitionSignature, true, ml)

		// namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.RolloutSignature, true, ml)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("inplace", func() {
		It("ok", func() {
		})

		It("partly", func() {
		})

		It("rollback", func() {
		})
	})

	Context("replace", func() {
		It("ok", func() {
		})

		It("partly", func() {
		})

		It("rollback", func() {
		})
	})

	Context("create", func() {
		It("auto promotion", func() {
		})

		It("partly", func() {
		})

		It("partly - done", func() {
		})

		It("promote condition", func() {
		})

		It("scale down", func() {
		})

		It("rollback", func() {
		})
	})
})
