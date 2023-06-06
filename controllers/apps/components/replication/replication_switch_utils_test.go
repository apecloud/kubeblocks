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

package replication

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	componentutil "github.com/apecloud/kubeblocks/controllers/apps/components/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

var _ = Describe("ReplicationSet Switch Util", func() {

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
		testapps.ClearResources(&testCtx, intctrlutil.StatefulSetSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	testHandleReplicationSetHASwitch := func() {
		var (
			defaultCandidateInstanceIndex = int32(0)
			newCandidateInstanceIndex     = int32(1)
		)
		clusterSwitchPolicy := &appsv1alpha1.ClusterSwitchPolicy{
			Type: appsv1alpha1.MaximumDataProtection,
		}
		candidateInstance := &appsv1alpha1.CandidateInstance{
			Index:    defaultCandidateInstanceIndex,
			Operator: appsv1alpha1.CandidateOpEqual,
		}
		By("Creating a cluster with replication workloadType.")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(testapps.DefaultRedisCompSpecName, testapps.DefaultRedisCompDefName).
			SetReplicas(testapps.DefaultReplicationReplicas).
			SetCandidateInstance(candidateInstance).
			SetSwitchPolicy(clusterSwitchPolicy).
			Create(&testCtx).GetObject()

		By("Creating a statefulSet of replication workloadType.")
		container := corev1.Container{
			Name:            "mock-redis-container",
			Image:           testapps.DefaultRedisImageName,
			ImagePullPolicy: corev1.PullIfNotPresent,
		}
		sts := testapps.NewStatefulSetFactory(testCtx.DefaultNamespace,
			clusterObj.Name+"-"+testapps.DefaultRedisCompSpecName,
			clusterObj.Name,
			testapps.DefaultRedisCompSpecName).
			AddContainer(container).
			AddAppInstanceLabel(clusterObj.Name).
			AddAppComponentLabel(testapps.DefaultRedisCompSpecName).
			AddAppManangedByLabel().
			SetReplicas(2).
			Create(&testCtx).GetObject()

		By("Creating Pods of replication workloadType.")

		for i := int32(0); i < *sts.Spec.Replicas; i++ {
			_ = testapps.NewPodFactory(testCtx.DefaultNamespace, fmt.Sprintf("%s-%d", sts.Name, i)).
				AddContainer(container).
				AddLabelsInMap(sts.Labels).
				AddRoleLabel(DefaultRole(i)).
				Create(&testCtx).GetObject()
		}
		clusterComponentSpec := &clusterObj.Spec.ComponentSpecs[0]

		By("Test candidateInstance has not changed.")
		changed, currentPrimaryInstanceName, err := componentutil.CheckCandidateInstanceChanged(testCtx.Ctx, k8sClient, clusterObj, clusterComponentSpec.Name)
		Expect(err).Should(Succeed())
		Expect(changed).Should(BeFalse())

		By("Test HandleReplicationSetHASwitch success when candidateInstance has not changed.")
		err = HandleReplicationSetHASwitch(testCtx.Ctx, k8sClient, clusterObj, clusterComponentSpec)
		Expect(err).Should(Succeed())

		By("Test update cluster component candidateInstance should be successful.")
		candidateInstance = &appsv1alpha1.CandidateInstance{
			Index:    newCandidateInstanceIndex,
			Operator: appsv1alpha1.CandidateOpEqual,
		}
		testapps.UpdateClusterCompSpecCandidateInstance(&testCtx, clusterObj, clusterComponentSpec.Name, candidateInstance)

		By("Test new Switch obj and init SwitchInstance should be successful.")
		clusterObj.Spec.ComponentSpecs[0].CandidateInstance = candidateInstance
		clusterComponentSpec.CandidateInstance = candidateInstance
		s := newSwitch(testCtx.Ctx, k8sClient, clusterObj, &clusterDefObj.Spec.ComponentDefs[0], clusterComponentSpec,
			nil, nil, nil, nil, nil)
		candidateInstanceName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, clusterComponentSpec.Name, candidateInstance.Index)
		err = s.initSwitchInstance(currentPrimaryInstanceName, candidateInstanceName)
		Expect(err).Should(Succeed())

		By("Test HandleReplicationSetHASwitch failed when candidateInstance has changed because controller reconciles many times, and switch job has not finished.")
		err = HandleReplicationSetHASwitch(ctx, k8sClient, clusterObj, clusterComponentSpec)
		Expect(err).ShouldNot(Succeed())
		Expect(err.Error()).Should(ContainSubstring("job check conditions status failed"))

		By("Test clean switch job.")
		err = cleanSwitchCmdJobs(s)
		Expect(err).Should(Succeed())
	}

	// Scenarios

	Context("test replicationSet switch util", func() {
		BeforeEach(func() {

			By("Mock a replicationSpec with SwitchPolicy and SwitchCmdExecutorConfig.")
			replicationSpec := &appsv1alpha1.ReplicationSetSpec{
				SwitchPolicies: []appsv1alpha1.SwitchPolicy{
					{
						Type: appsv1alpha1.MaximumAvailability,
						SwitchStatements: &appsv1alpha1.SwitchStatements{
							Promote: []string{"echo MaximumAvailability promote"},
							Demote:  []string{"echo MaximumAvailability demote"},
							Follow:  []string{"echo MaximumAvailability follow"},
						},
					},
					{
						Type: appsv1alpha1.MaximumDataProtection,
						SwitchStatements: &appsv1alpha1.SwitchStatements{
							Promote: []string{"echo MaximumDataProtection promote"},
							Demote:  []string{"echo MaximumDataProtection demote"},
							Follow:  []string{"echo MaximumDataProtection follow"},
						},
					},
					{
						Type: appsv1alpha1.Noop,
					},
				},
				SwitchCmdExecutorConfig: &appsv1alpha1.SwitchCmdExecutorConfig{
					CommandExecutorEnvItem: appsv1alpha1.CommandExecutorEnvItem{
						Image: testapps.DefaultRedisImageName,
					},
					SwitchSteps: []appsv1alpha1.SwitchStep{
						{
							Role: appsv1alpha1.NewPrimary,
							CommandExecutorItem: appsv1alpha1.CommandExecutorItem{
								Command: []string{"echo $(KB_SWITCH_ROLE_ENDPOINT) $(KB_SWITCH_PROMOTE_STATEMENT)"},
							},
						},
						{
							Role: appsv1alpha1.OldPrimary,
							CommandExecutorItem: appsv1alpha1.CommandExecutorItem{
								Command: []string{"echo $(KB_SWITCH_ROLE_ENDPOINT) $(KB_SWITCH_DEMOTE_STATEMENT)"},
							},
						},
						{
							Role: appsv1alpha1.Secondaries,
							CommandExecutorItem: appsv1alpha1.CommandExecutorItem{
								Command: []string{"echo $(KB_SWITCH_ROLE_ENDPOINT) $(KB_SWITCH_FOLLOW_STATEMENT)"},
							},
						},
					},
				},
			}
			By("Create a clusterDefinition obj with replication workloadType.")
			clusterDefObj = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.ReplicationRedisComponent, testapps.DefaultRedisCompDefName).
				AddReplicationSpec(replicationSpec).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj with replication workloadType.")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponentVersion(testapps.DefaultRedisCompDefName).AddContainerShort(testapps.DefaultRedisContainerName, testapps.DefaultRedisImageName).
				Create(&testCtx).GetObject()

		})

		It("Test HandleReplicationSetHASwitch with different conditions", func() {
			testHandleReplicationSetHASwitch()
		})
	})
})
