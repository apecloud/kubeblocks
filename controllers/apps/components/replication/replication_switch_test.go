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
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
)

// MockSwitchActionHandler mocks the implementation of the SwitchActionHandler interface for testing.
type MockSwitchActionHandler struct{}

// buildExecSwitchCommandEnvs builds a series of envs for test switching actions.
func (handler *MockSwitchActionHandler) buildExecSwitchCommandEnvs(s *Switch) ([]corev1.EnvVar, error) {
	switchJobHandler := &SwitchActionWithJobHandler{}
	return switchJobHandler.buildExecSwitchCommandEnvs(s)
}

// execSwitchCommands mocks the result of executes the specific switching commands.
func (handler *MockSwitchActionHandler) execSwitchCommands(s *Switch, switchEnvs []corev1.EnvVar) error {
	return nil
}

var _ = Describe("ReplicationSet Switch", func() {

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
		testapps.ClearResources(&testCtx, intctrlutil.StatefulSetSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	testReplicationSetSwitch := func() {
		var (
			DefaultReplicationPrimaryIndex        = int32(0)
			DefaultPrimaryIndexDiffWithStsOrdinal = int32(1)
		)

		var (
			notHealthy    HealthDetectResult = false
			healthy       HealthDetectResult = true
			lagNotZero    LagDetectResult    = 9999
			lagZero       LagDetectResult    = 0
			rolePrimary                      = DetectRolePrimary
			roleSecondary                    = DetectRoleSecondary
		)
		clusterSwitchPolicy := &appsv1alpha1.ClusterSwitchPolicy{
			Type: appsv1alpha1.MaximumAvailability,
		}
		By("Creating a cluster with replication workloadType.")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(testapps.DefaultRedisCompName, testapps.DefaultRedisCompDefName).
			SetReplicas(testapps.DefaultReplicationReplicas).
			SetPrimaryIndex(DefaultPrimaryIndexDiffWithStsOrdinal).
			SetSwitchPolicy(clusterSwitchPolicy).
			Create(&testCtx).GetObject()

		By("Creating a statefulSet of replication workloadType.")
		container := corev1.Container{
			Name:            "mock-redis-container",
			Image:           testapps.DefaultRedisImageName,
			ImagePullPolicy: corev1.PullIfNotPresent,
		}

		replicationSetSts := testapps.NewStatefulSetFactory(testCtx.DefaultNamespace,
			clusterObj.Name+"-"+testapps.DefaultRedisCompName, clusterObj.Name, testapps.DefaultRedisCompName).
			AddContainer(container).
			AddAppInstanceLabel(clusterObj.Name).
			AddAppComponentLabel(testapps.DefaultRedisCompName).
			AddAppManangedByLabel().
			SetReplicas(4).
			Create(&testCtx).GetObject()

		By("Creating Pods of replication workloadType.")
		for i := int32(0); i < *replicationSetSts.Spec.Replicas; i++ {
			podBuilder := testapps.NewPodFactory(testCtx.DefaultNamespace,
				fmt.Sprintf("%s-%d", replicationSetSts.Name, i)).
				AddContainer(container).
				AddLabelsInMap(replicationSetSts.Labels)
			if i == 0 {
				podBuilder.AddRoleLabel(string(Primary))
			} else {
				podBuilder.AddRoleLabel(string(Secondary))
			}
			_ = podBuilder.Create(&testCtx).GetObject()
		}
		clusterComponentSpec := &clusterObj.Spec.ComponentSpecs[0]

		mockSwitchHandler := &MockSwitchActionHandler{}
		By("Test create new Switch obj should be successful.")
		s := newSwitch(testCtx.Ctx, k8sClient, clusterObj, &clusterDefObj.Spec.ComponentDefs[0], clusterComponentSpec, nil, nil, nil, nil, mockSwitchHandler)

		By("Test switch detection when switchInstance is nil should be false.")
		s.detection(false)
		Expect(s.SwitchStatus.SwitchPhaseStatus).Should(Equal(SwitchPhaseStatusFailed))

		By("Test switch detection should be successful.")
		err := s.initSwitchInstance(DefaultReplicationPrimaryIndex, DefaultPrimaryIndexDiffWithStsOrdinal)
		Expect(err).Should(Succeed())

		By("Test switch detection should be successful.")
		s.detection(false)
		Expect(s.SwitchStatus.SwitchPhaseStatus).Should(Equal(SwitchPhaseStatusSucceed))

		By("Test switch election with multi secondaries should be successful, and the candidate primary should be the priorityPod.")
		priorityPod := clusterObj.Name + "-" + testapps.DefaultRedisCompName + "-3"
		for _, sri := range s.SwitchInstance.SecondariesRole {
			if sri.Pod.Name != priorityPod {
				sri.LagDetectInfo = &lagNotZero
			}
		}
		sri := s.election()
		Expect(sri.Pod.Name).Should(Equal(priorityPod))

		By("Test switch decision when candidate primary is not healthy should be false.")
		s.SwitchInstance.CandidatePrimaryRole.HealthDetectInfo = &notHealthy
		decision := s.decision()
		Expect(decision).Should(BeFalse())
		Expect(s.SwitchStatus.SwitchPhaseStatus).Should(Equal(SwitchPhaseStatusFailed))
		s.SwitchInstance.CandidatePrimaryRole.HealthDetectInfo = &healthy

		By("Test switch decision when candidate primary role label is primary should be false.")
		s.SwitchInstance.CandidatePrimaryRole.RoleDetectInfo = &rolePrimary
		decision = s.decision()
		Expect(decision).Should(BeFalse())
		Expect(s.SwitchStatus.SwitchPhaseStatus).Should(Equal(SwitchPhaseStatusFailed))
		s.SwitchInstance.CandidatePrimaryRole.RoleDetectInfo = &roleSecondary

		By("Test switch decision when switchPolicy is MaximumAvailability and old primary, candidate primary are healthy and candidate primary data lag is 0 should be true.")
		decision = s.decision()
		Expect(decision).Should(BeTrue())

		By("Test switch decision when switchPolicy is MaximumAvailability and old primary is not healthy should be true.")
		s.SwitchInstance.OldPrimaryRole.HealthDetectInfo = &notHealthy
		decision = s.decision()
		Expect(decision).Should(BeTrue())
		Expect(s.SwitchStatus.SwitchPhaseStatus).Should(Equal(SwitchPhaseStatusSucceed))
		s.SwitchInstance.OldPrimaryRole.HealthDetectInfo = &healthy

		By("Test switch decision when switchPolicy is MaximumAvailability and old primary is healthy and candidate primary data lag is not 0 should be false.")
		s.SwitchInstance.CandidatePrimaryRole.LagDetectInfo = &lagNotZero
		decision = s.decision()
		Expect(decision).Should(BeFalse())
		Expect(s.SwitchStatus.SwitchPhaseStatus).Should(Equal(SwitchPhaseStatusFailed))
		s.SwitchInstance.CandidatePrimaryRole.LagDetectInfo = &lagZero

		By("Test switch decision when switchPolicy is MaximumDataProtection and candidate primary data lag is 0 should be true.")
		s.SwitchResource.CompSpec.SwitchPolicy.Type = appsv1alpha1.MaximumDataProtection
		decision = s.decision()
		Expect(decision).Should(BeTrue())
		Expect(s.SwitchStatus.SwitchPhaseStatus).Should(Equal(SwitchPhaseStatusSucceed))

		By("Test switch decision when switchPolicy is MaximumDataProtection and candidate primary data lag is not 0 should be false.")
		s.SwitchInstance.CandidatePrimaryRole.LagDetectInfo = &lagNotZero
		s.SwitchResource.CompSpec.SwitchPolicy.Type = appsv1alpha1.MaximumDataProtection
		decision = s.decision()
		Expect(decision).Should(BeFalse())
		Expect(s.SwitchStatus.SwitchPhaseStatus).Should(Equal(SwitchPhaseStatusFailed))
		s.SwitchInstance.CandidatePrimaryRole.LagDetectInfo = &lagZero

		By("Test switch decision  when switchPolicy is Noop should be false.")
		s.SwitchResource.CompSpec.SwitchPolicy.Type = appsv1alpha1.Noop
		decision = s.decision()
		Expect(decision).Should(BeFalse())
		Expect(s.SwitchStatus.SwitchPhaseStatus).Should(Equal(SwitchPhaseStatusSucceed))

		By("Test do switch action with MockSwitchActionHandler should be true.")
		s.SwitchResource.CompSpec.SwitchPolicy.Type = appsv1alpha1.MaximumAvailability
		err = s.doSwitch()
		Expect(err).Should(Succeed())

		By("Test switch update role label should be successful.")
		err = s.updateRoleLabel()
		Expect(err).Should(Succeed())
	}

	// Scenarios

	Context("test replicationSet switch util", func() {
		BeforeEach(func() {

			By("Mock a replicationSpec with SwitchPolicy and SwitchCmdExecutorConfig.")
			mockReplicationSpec := &appsv1alpha1.ReplicationSetSpec{
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
				AddReplicationSpec(mockReplicationSpec).
				Create(&testCtx).GetObject()

			By("Create a clusterVersion obj with replication workloadType.")
			clusterVersionObj = testapps.NewClusterVersionFactory(clusterVersionName, clusterDefObj.GetName()).
				AddComponentVersion(testapps.DefaultRedisCompDefName).AddContainerShort(testapps.DefaultRedisContainerName, testapps.DefaultRedisImageName).
				Create(&testCtx).GetObject()

		})

		It("Test ReplicationSet switch lifecycle including detection, election, decision, do switch action, etc.", func() {
			testReplicationSetSwitch()
		})
	})
})
