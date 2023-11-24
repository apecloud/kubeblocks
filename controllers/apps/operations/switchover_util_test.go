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

package operations

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("Switchover Util", func() {

	var (
		clusterName        = "test-cluster-repl"
		clusterDefName     = "test-cluster-def-repl"
		clusterVersionName = "test-cluster-version-repl"
	)

	var (
		clusterDefObj     *appsv1alpha1.ClusterDefinition
		clusterVersionObj *appsv1alpha1.ClusterVersion
		clusterObj        *appsv1alpha1.Cluster
	)

	defaultRole := func(index int32) string {
		role := constant.Secondary
		if index == 0 {
			role = constant.Primary
		}
		return role
	}

	cleanAll := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")
		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testapps.ClearClusterResources(&testCtx)

		// clear rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced resources
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.StatefulSetSignature, true, inNS, ml)
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	testNeedDoSwitchover := func() {
		By("Creating a cluster with replication workloadType.")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(testapps.DefaultRedisCompSpecName, testapps.DefaultRedisCompDefName).
			SetReplicas(testapps.DefaultReplicationReplicas).
			Create(&testCtx).GetObject()

		By("Creating a statefulSet of replication workloadType.")
		container := corev1.Container{
			Name:            "mock-redis-container",
			Image:           testapps.DefaultRedisImageName,
			ImagePullPolicy: corev1.PullIfNotPresent,
		}
		sts := testapps.NewStatefulSetFactory(testCtx.DefaultNamespace,
			clusterObj.Name+"-"+testapps.DefaultRedisCompSpecName, clusterObj.Name, testapps.DefaultRedisCompSpecName).
			AddFinalizers([]string{constant.DBClusterFinalizerName}).
			AddContainer(container).
			AddAppInstanceLabel(clusterObj.Name).
			AddAppComponentLabel(testapps.DefaultRedisCompSpecName).
			AddAppManagedByLabel().
			SetReplicas(2).
			Create(&testCtx).GetObject()

		By("Creating Pods of replication workloadType.")
		for i := int32(0); i < *sts.Spec.Replicas; i++ {
			_ = testapps.NewPodFactory(testCtx.DefaultNamespace, fmt.Sprintf("%s-%d", sts.Name, i)).
				AddContainer(container).
				AddLabelsInMap(sts.Labels).
				AddRoleLabel(defaultRole(i)).
				Create(&testCtx).GetObject()
		}

		opsSwitchover := &appsv1alpha1.Switchover{
			ComponentOps: appsv1alpha1.ComponentOps{ComponentName: testapps.DefaultRedisCompSpecName},
			InstanceName: fmt.Sprintf("%s-%s-%d", clusterObj.Name, testapps.DefaultRedisCompSpecName, 0),
		}

		reqCtx := intctrlutil.RequestCtx{
			Ctx: testCtx.Ctx,
		}
		compSpec := clusterObj.Spec.GetComponentByName(opsSwitchover.ComponentName)
		synthesizedComp, err := component.BuildSynthesizedComponentWrapper(reqCtx, k8sClient, clusterObj, compSpec)
		Expect(err).Should(Succeed())
		Expect(synthesizedComp).ShouldNot(BeNil())

		By("Test opsSwitchover.Instance is already primary, and do not need to do switchover.")
		needSwitchover, err := needDoSwitchover(testCtx.Ctx, k8sClient, clusterObj, synthesizedComp, opsSwitchover)
		Expect(err).Should(Succeed())
		Expect(needSwitchover).Should(BeFalse())

		By("Test opsSwitchover.Instance is not primary, and need to do switchover.")
		opsSwitchover.InstanceName = fmt.Sprintf("%s-%s-%d", clusterObj.Name, testapps.DefaultRedisCompSpecName, 1)
		needSwitchover, err = needDoSwitchover(testCtx.Ctx, k8sClient, clusterObj, synthesizedComp, opsSwitchover)
		Expect(err).Should(Succeed())
		Expect(needSwitchover).Should(BeTrue())

		By("Test opsSwitchover.Instance is *, and need to do switchover.")
		opsSwitchover.InstanceName = "*"
		needSwitchover, err = needDoSwitchover(testCtx.Ctx, k8sClient, clusterObj, synthesizedComp, opsSwitchover)
		Expect(err).Should(Succeed())
		Expect(needSwitchover).Should(BeTrue())
	}

	testDoSwitchover := func() {
		By("Creating a cluster with replication workloadType.")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(testapps.DefaultRedisCompSpecName, testapps.DefaultRedisCompDefName).
			SetReplicas(testapps.DefaultReplicationReplicas).
			Create(&testCtx).GetObject()

		By("Creating a statefulSet of replication workloadType.")
		container := corev1.Container{
			Name:            "mock-redis-container",
			Image:           testapps.DefaultRedisImageName,
			ImagePullPolicy: corev1.PullIfNotPresent,
		}
		sts := testapps.NewStatefulSetFactory(testCtx.DefaultNamespace,
			clusterObj.Name+"-"+testapps.DefaultRedisCompSpecName, clusterObj.Name, testapps.DefaultRedisCompSpecName).
			AddFinalizers([]string{constant.DBClusterFinalizerName}).
			AddContainer(container).
			AddAppInstanceLabel(clusterObj.Name).
			AddAppComponentLabel(testapps.DefaultRedisCompSpecName).
			AddAppManagedByLabel().
			SetReplicas(2).
			Create(&testCtx).GetObject()

		By("Creating Pods of replication workloadType.")
		for i := int32(0); i < *sts.Spec.Replicas; i++ {
			_ = testapps.NewPodFactory(testCtx.DefaultNamespace, fmt.Sprintf("%s-%d", sts.Name, i)).
				AddContainer(container).
				AddLabelsInMap(sts.Labels).
				AddRoleLabel(defaultRole(i)).
				Create(&testCtx).GetObject()
		}
		opsSwitchover := &appsv1alpha1.Switchover{
			ComponentOps: appsv1alpha1.ComponentOps{ComponentName: testapps.DefaultRedisCompSpecName},
			InstanceName: fmt.Sprintf("%s-%s-%d", clusterObj.Name, testapps.DefaultRedisCompSpecName, 1),
		}
		reqCtx := intctrlutil.RequestCtx{
			Ctx:      testCtx.Ctx,
			Recorder: k8sManager.GetEventRecorderFor("opsrequest-controller"),
		}
		By("Test create a job to do switchover")
		compSpec := clusterObj.Spec.GetComponentByName(opsSwitchover.ComponentName)
		synthesizedComp, err := component.BuildSynthesizedComponentWrapper(reqCtx, k8sClient, clusterObj, compSpec)
		Expect(err).Should(Succeed())
		Expect(synthesizedComp).ShouldNot(BeNil())
		err = createSwitchoverJob(reqCtx, k8sClient, clusterObj, synthesizedComp, opsSwitchover)
		Expect(err).Should(Succeed())
	}

	// Scenarios
	Context("test switchover util", func() {
		BeforeEach(func() {
			By("Create a clusterDefinition obj with replication workloadType.")
			commandExecutorEnvItem := &appsv1alpha1.CommandExecutorEnvItem{
				Image: testapps.DefaultRedisImageName,
			}
			commandExecutorItem := &appsv1alpha1.CommandExecutorItem{
				Command: []string{"echo", "hello"},
				Args:    []string{},
			}
			scriptSpecSelectors := []appsv1alpha1.ScriptSpecSelector{
				{
					Name: "test-mock-cm",
				},
				{
					Name: "test-mock-cm-2",
				},
			}
			switchoverSpec := &appsv1alpha1.SwitchoverSpec{
				WithCandidate: &appsv1alpha1.SwitchoverAction{
					CmdExecutorConfig: &appsv1alpha1.CmdExecutorConfig{
						CommandExecutorEnvItem: *commandExecutorEnvItem,
						CommandExecutorItem:    *commandExecutorItem,
					},
					ScriptSpecSelectors: scriptSpecSelectors,
				},
				WithoutCandidate: &appsv1alpha1.SwitchoverAction{
					CmdExecutorConfig: &appsv1alpha1.CmdExecutorConfig{
						CommandExecutorEnvItem: *commandExecutorEnvItem,
						CommandExecutorItem:    *commandExecutorItem,
					},
					ScriptSpecSelectors: scriptSpecSelectors,
				},
			}
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.ReplicationRedisComponent, testapps.DefaultRedisCompDefName).
				AddSwitchoverSpec(switchoverSpec).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj with replication workloadType.")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponentVersion(testapps.DefaultRedisCompDefName).AddContainerShort(testapps.DefaultRedisContainerName, testapps.DefaultRedisImageName).
				Create(&testCtx).GetObject()

		})

		It("Test needDoSwitchover with different conditions", func() {
			testNeedDoSwitchover()
		})

		It("Test doSwitchover when opsRequest triggers", func() {
			testDoSwitchover()
		})
	})
})
