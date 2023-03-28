/*
Copyright ApeCloud, Inc.

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

package replicationset

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
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
		// must wait until resources deleted and no longer exist before the testcases start,
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
		// testapps.ClearResources(&testCtx, intctrlutil.StatefulSetSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	testHandleReplicationSetHASwitch := func() {
		var (
			DefaultReplicationPrimaryIndex        = int32(0)
			DefaultPrimaryIndexDiffWithStsOrdinal = int32(1)
		)
		clusterSwitchPolicy := &appsv1alpha1.ClusterSwitchPolicy{
			Type: appsv1alpha1.MaximumDataProtection,
		}
		By("Creating a cluster with replication workloadType.")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(testapps.DefaultRedisCompName, testapps.DefaultRedisCompType).
			SetReplicas(testapps.DefaultReplicationReplicas).
			SetPrimaryIndex(testapps.DefaultReplicationPrimaryIndex).
			SetSwitchPolicy(clusterSwitchPolicy).
			Create(&testCtx).GetObject()

		By("Creating a statefulSet of replication workloadType.")
		container := corev1.Container{
			Name:            "mock-redis-container",
			Image:           testapps.DefaultRedisImageName,
			ImagePullPolicy: corev1.PullIfNotPresent,
		}
		stsList := make([]*appsv1.StatefulSet, 0)
		for k, v := range map[string]string{
			string(Primary):   clusterObj.Name + "-" + testapps.DefaultRedisCompName + "-0",
			string(Secondary): clusterObj.Name + "-" + testapps.DefaultRedisCompName + "-1",
		} {
			sts := testapps.NewStatefulSetFactory(testCtx.DefaultNamespace, v, clusterObj.Name, testapps.DefaultRedisCompName).
				AddContainer(container).
				AddAppInstanceLabel(clusterObj.Name).
				AddAppComponentLabel(testapps.DefaultRedisCompName).
				AddAppManangedByLabel().
				AddRoleLabel(k).
				SetReplicas(1).
				Create(&testCtx).GetObject()
			isStsPrimary, err := checkObjRoleLabelIsPrimary(sts)
			if k == string(Primary) {
				Expect(err).To(Succeed())
				Expect(isStsPrimary).Should(BeTrue())
			} else {
				Expect(err).To(Succeed())
				Expect(isStsPrimary).ShouldNot(BeTrue())
			}
			stsList = append(stsList, sts)
		}

		By("Creating Pods of replication workloadType.")
		for _, sts := range stsList {
			_ = testapps.NewPodFactory(testCtx.DefaultNamespace, sts.Name+"-0").
				AddContainer(container).
				AddLabelsInMap(sts.Labels).
				Create(&testCtx).GetObject()
		}
		clusterComponentSpec := &clusterObj.Spec.ComponentSpecs[0]

		By("Test primaryIndex has not changed.")
		changed, _, err := CheckPrimaryIndexChanged(testCtx.Ctx, k8sClient, clusterObj, clusterComponentSpec.Name,
			clusterComponentSpec.GetPrimaryIndex())
		Expect(err).Should(Succeed())
		Expect(changed).Should(BeFalse())

		By("Test HandleReplicationSetHASwitch success when primaryIndex has not changed.")
		err = HandleReplicationSetHASwitch(testCtx.Ctx, k8sClient, clusterObj, clusterComponentSpec)
		Expect(err).Should(Succeed())

		By("Test update cluster component primaryIndex should be successful.")
		testapps.UpdateClusterCompSpecPrimaryIndex(&testCtx, clusterObj, clusterComponentSpec.Name, &DefaultPrimaryIndexDiffWithStsOrdinal)

		By("Test new Switch obj and init SwitchInstance should be successful.")
		clusterObj.Spec.ComponentSpecs[0].PrimaryIndex = &DefaultPrimaryIndexDiffWithStsOrdinal
		clusterComponentSpec.PrimaryIndex = &DefaultPrimaryIndexDiffWithStsOrdinal
		s := newSwitch(testCtx.Ctx, k8sClient, clusterObj, &clusterDefObj.Spec.ComponentDefs[0], clusterComponentSpec, nil, nil, nil, nil, nil)
		err = s.initSwitchInstance(DefaultReplicationPrimaryIndex, DefaultPrimaryIndexDiffWithStsOrdinal)
		Expect(err).Should(Succeed())

		By("Test HandleReplicationSetHASwitch failed when primaryIndex has changed because controller reconciles many times, and switch job has not finished.")
		err = HandleReplicationSetHASwitch(ctx, k8sClient, clusterObj, clusterComponentSpec)
		Expect(err).ShouldNot(Succeed())
		Expect(err.Error()).Should(ContainSubstring("switch command job"))

		By("Test clean switch job.")
		err = cleanSwitchCmdJobs(s)
		Expect(err).Should(Succeed())
	}

	// Scenarios

	Context("test replicationSet switch util", func() {
		BeforeEach(func() {

			By("Mock a replicationSpec with SwitchPolicy and SwitchCmdExecutorConfig.")
			replicationSpec := &appsv1alpha1.ReplicationSpec{
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
				AddComponent(testapps.ReplicationRedisComponent, testapps.DefaultRedisCompType).
				AddReplicationSpec(replicationSpec).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj with replication workloadType.")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponent(testapps.DefaultRedisCompType).AddContainerShort(testapps.DefaultRedisContainerName, testapps.DefaultRedisImageName).
				Create(&testCtx).GetObject()

		})

		It("Test HandleReplicationSetHASwitch with different conditions", func() {
			testHandleReplicationSetHASwitch()
		})
	})
})
