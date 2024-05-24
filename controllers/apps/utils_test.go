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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("utils", func() {
	var (
		randomStr      = testCtx.GetRandomStr()
		clusterDefName = "test-clusterdef-" + randomStr
		clusterVerName = "test-clusterver-" + randomStr
		clusterName    = "test-cluster-" + randomStr
	)

	const (
		consensusCompName = "consensus"
		statelessCompName = "stateless"
	)

	cleanAll := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")
		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResourcesWithRemoveFinalizerOption(&testCtx)

		// clear rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced resources
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.InstanceSetSignature, true, inNS, ml)
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	Context("utils", func() {
		It("getClusterByObject", func() {
			By(" init cluster, instanceSet, pods")
			testapps.InitClusterWithHybridComps(&testCtx, clusterDefName,
				clusterVerName, clusterName, statelessCompName, "stateful", consensusCompName)
			its := testapps.MockInstanceSetComponent(&testCtx, clusterName, consensusCompName)
			_ = testapps.MockInstanceSetPods(&testCtx, its, clusterName, consensusCompName)

			newCluster, _ := getClusterByObject(ctx, k8sClient, its)
			Expect(newCluster != nil).Should(BeTrue())
		})
	})
})
