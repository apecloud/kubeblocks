/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
