/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/testutil"
)

// GetComponentObservedGeneration gets the testing component's ObservedGeneration in status for verification.
func GetComponentObservedGeneration(testCtx *testutil.TestContext, compKey types.NamespacedName) func(gomega.Gomega) int64 {
	return func(g gomega.Gomega) int64 {
		comp := &appsv1.Component{}
		g.Expect(testCtx.Cli.Get(testCtx.Ctx, compKey, comp)).Should(gomega.Succeed())
		return comp.Status.ObservedGeneration
	}
}

// GetComponentPhase gets the testing component's phase in status for verification.
func GetComponentPhase(testCtx *testutil.TestContext, compKey types.NamespacedName) func(gomega.Gomega) appsv1.ClusterComponentPhase {
	return func(g gomega.Gomega) appsv1.ClusterComponentPhase {
		comp := &appsv1.Component{}
		g.Expect(testCtx.Cli.Get(testCtx.Ctx, compKey, comp)).Should(gomega.Succeed())
		return comp.Status.Phase
	}
}

// ComponentReconciled checks if the testing component has been reconciled.
func ComponentReconciled(testCtx *testutil.TestContext, compKey types.NamespacedName) func(gomega.Gomega) bool {
	return func(g gomega.Gomega) bool {
		comp := &appsv1.Component{}
		g.Expect(testCtx.Cli.Get(testCtx.Ctx, compKey, comp)).Should(gomega.Succeed())
		return comp.Status.ObservedGeneration > 0 && comp.Status.ObservedGeneration == comp.Generation
	}
}
