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

	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	configcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("ParameterExtension Controller", func() {

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("When updating cluster configs", func() {
		It("Should reconcile success", func() {
			_, _, clusterObj, _, _ := mockReconcileResource()

			clusterKey := client.ObjectKeyFromObject(clusterObj)
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				compSpec := cluster.Spec.GetComponentByName(defaultCompName)
				g.Expect(compSpec).ShouldNot(BeNil())
				g.Expect(compSpec.Configs).Should(HaveLen(1))
				g.Expect(compSpec.Configs[0].ConfigMap).ShouldNot(BeNil())
				g.Expect(compSpec.Configs[0].ConfigMap.Name).Should(BeEquivalentTo(configcore.GetComponentCfgName(clusterObj.Name, defaultCompName, configSpecName)))
				g.Expect(pointer.BoolDeref(compSpec.Configs[0].ExternalManaged, false)).Should(BeTrue())
			})).Should(Succeed())
		})

	})
})
