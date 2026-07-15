/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"context"
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	kbagentproto "github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testops "github.com/apecloud/kubeblocks/pkg/testutil/operations"
)

var _ = Describe("", func() {
	var (
		compDefName = "test-compdef-"
		clusterName = "test-cluster-"
		compDefObj  *appsv1.ComponentDefinition
		compObj     *appsv1.Component
		clusterObj  *appsv1.Cluster
	)

	defaultRole := func(index int32) string {
		role := constant.Follower
		if index == 0 {
			role = constant.Leader
		}
		return role
	}

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), cluster definition
		testapps.ClearClusterResourcesWithRemoveFinalizerOption(&testCtx)

		// delete rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResources(&testCtx, generics.OpsRequestSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.ComponentSignature, inNS, ml)
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Test OpsRequest", func() {
		var reqCtx intctrlutil.RequestCtx
		var opsRes *OpsResource

		BeforeEach(func() {
			By("Create a componentDefinition obj.")
			compDefObj = testapps.NewComponentDefinitionFactory(compDefName).
				WithRandomName().
				SetDefaultSpec().
				SetLifecycleAction("Switchover", testapps.NewLifecycleAction("switchover")).
				Create(&testCtx).
				GetObject()

			By("Creating a cluster")
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
				WithRandomName().
				AddComponent(defaultCompName, compDefObj.GetName()).
				SetReplicas(2).
				Create(&testCtx).GetObject()

			By("creating a component")
			compObj = testapps.NewComponentFactory(testCtx.DefaultNamespace, clusterObj.Name+"-"+defaultCompName, compDefObj.Name).
				AddAppManagedByLabel().
				AddAppInstanceLabel(clusterObj.Name).
				AddAppComponentLabel(defaultCompName).
				AddAnnotations(constant.KBAppClusterUIDKey, string(clusterObj.UID)).
				Create(&testCtx).
				GetObject()

			By("Creating a instanceset")
			container := corev1.Container{
				Name:            "mock-container-name",
				Image:           testapps.ApeCloudMySQLImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
			}
			its := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				clusterObj.Name+"-"+defaultCompName, clusterObj.Name, defaultCompName).
				AddFinalizers([]string{constant.DBClusterFinalizerName}).
				AddContainer(container).
				AddAppInstanceLabel(clusterObj.Name).
				AddAppComponentLabel(defaultCompName).
				AddAppManagedByLabel().
				SetReplicas(2).
				Create(&testCtx).GetObject()

			By("Creating Pods of replication.")
			for i := int32(0); i < *its.Spec.Replicas; i++ {
				_ = testapps.NewPodFactory(testCtx.DefaultNamespace, fmt.Sprintf("%s-%d", its.Name, i)).
					AddContainer(container).
					AddLabelsInMap(its.Labels).
					AddRoleLabel(defaultRole(i)).
					Create(&testCtx).GetObject()
			}

			By("mock cluster is Running and the status operations")
			Expect(testapps.ChangeObjStatus(&testCtx, clusterObj, func() {
				clusterObj.Status.Phase = appsv1.RunningClusterPhase
				clusterObj.Status.Components = map[string]appsv1.ClusterComponentStatus{
					defaultCompName: {
						Phase: appsv1.RunningComponentPhase,
					},
				}
			})).Should(Succeed())

			reqCtx = intctrlutil.RequestCtx{
				Ctx:      testCtx.Ctx,
				Recorder: k8sManager.GetEventRecorderFor("opsrequest-controller"),
			}

			opsRes = &OpsResource{
				Cluster:  clusterObj,
				Recorder: k8sManager.GetEventRecorderFor("opsrequest-controller"),
			}
		})

		DescribeTable("reads a durable switchover dispatch claim without replaying an unconfirmed action",
			func(uid, claimID string, state opsv1alpha1.ActionDispatchClaimState, status opsv1alpha1.ProgressStatus, expectedError string) {
				opsRequest := &opsv1alpha1.OpsRequest{}
				opsRequest.Namespace = testCtx.DefaultNamespace
				opsRequest.Name = "reader-only"
				opsRequest.UID = types.UID(uid)
				progressDetail := &opsv1alpha1.ProgressStatusDetail{
					ObjectKey: getProgressObjectKey(KBSwitchoverKey, defaultCompName),
					Status:    status,
					DispatchClaim: &opsv1alpha1.ActionDispatchClaim{
						ID:    claimID,
						State: state,
					},
				}

				err := validateSwitchoverDispatchClaimForReader(opsRequest, defaultCompName, "instance-0", progressDetail)
				if expectedError == "" {
					Expect(err).ShouldNot(HaveOccurred())
					return
				}
				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).Should(ContainSubstring(expectedError))
			},
			Entry("continues observing an exact resolved outcome", "uid", "SwitchoverDispatch/uid/default/instance-0",
				opsv1alpha1.DispatchClaimStateResolved, opsv1alpha1.ProcessingProgressStatus, ""),
			Entry("accepts an exact resolved terminal outcome", "uid", "SwitchoverDispatch/uid/default/instance-0",
				opsv1alpha1.DispatchClaimStateResolved, opsv1alpha1.SucceedProgressStatus, ""),
			Entry("rejects a resolved claim in Pending", "uid", "SwitchoverDispatch/uid/default/instance-0",
				opsv1alpha1.DispatchClaimStateResolved, opsv1alpha1.PendingProgressStatus, "resolved switchover dispatch claim in Pending"),
			Entry("blocks an unconfirmed Claimed outcome", "uid", "SwitchoverDispatch/uid/default/instance-0",
				opsv1alpha1.DispatchClaimStateClaimed, opsv1alpha1.ProcessingProgressStatus, "no committed outcome"),
			Entry("blocks an OutcomeUnknown outcome", "uid", "SwitchoverDispatch/uid/default/instance-0",
				opsv1alpha1.DispatchClaimStateOutcomeUnknown, opsv1alpha1.ProcessingProgressStatus, "no committed outcome"),
			Entry("rejects a foreign state", "uid", "SwitchoverDispatch/uid/default/instance-0",
				opsv1alpha1.ActionDispatchClaimState("Foreign"), opsv1alpha1.ProcessingProgressStatus, "unknown state"),
			Entry("rejects a mismatched identity", "uid", "SwitchoverDispatch/wrong",
				opsv1alpha1.DispatchClaimStateResolved, opsv1alpha1.ProcessingProgressStatus, "does not match"),
			Entry("rejects a claim when the OpsRequest UID is empty", "", "SwitchoverDispatch/uid/comp/instance-0",
				opsv1alpha1.DispatchClaimStateResolved, opsv1alpha1.ProcessingProgressStatus, "UID is empty"),
		)

		It("keeps the legacy no-claim path readable", func() {
			err := validateSwitchoverDispatchClaimForReader(&opsv1alpha1.OpsRequest{}, defaultCompName, "instance-0",
				&opsv1alpha1.ProgressStatusDetail{Status: opsv1alpha1.PendingProgressStatus})
			Expect(err).ShouldNot(HaveOccurred())
		})

		DescribeTable("reads ComponentObjectName claims in the writer's logical component value domain",
			func(state opsv1alpha1.ActionDispatchClaimState, expectBlocked bool) {
				ops := testops.NewOpsRequestObj("ops-switchover-reader-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace,
					clusterObj.Name, opsv1alpha1.SwitchoverType)
				instanceName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, defaultCompName, 1)
				ops.Spec.SwitchoverList = []opsv1alpha1.Switchover{{
					ComponentObjectName: compObj.Name,
					InstanceName:        instanceName,
				}}
				opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)
				opsRes.OpsRequest.Status.Conditions = []metav1.Condition{{
					Type:   opsv1alpha1.ConditionTypeSwitchover,
					Status: metav1.ConditionTrue,
				}}
				objectKey := getProgressObjectKey(KBSwitchoverKey, defaultCompName)
				opsRes.OpsRequest.Status.Components = map[string]opsv1alpha1.OpsRequestComponentStatus{
					defaultCompName: {
						Phase: appsv1.UpdatingComponentPhase,
						ProgressDetails: []opsv1alpha1.ProgressStatusDetail{{
							Group:     defaultRole(1),
							ObjectKey: objectKey,
							Status:    opsv1alpha1.ProcessingProgressStatus,
							DispatchClaim: &opsv1alpha1.ActionDispatchClaim{
								ID: fmt.Sprintf("%s/%s/%s/%s", switchoverDispatchClaimKind,
									opsRes.OpsRequest.UID, defaultCompName, instanceName),
								State: state,
							},
						}},
					},
				}

				var completedCount, failedCount int32
				err := handleSwitchover(reqCtx, k8sClient, opsRes, &opsRes.OpsRequest.Spec.SwitchoverList[0],
					opsRes.OpsRequest, &completedCount, &failedCount)
				if expectBlocked {
					Expect(err).Should(HaveOccurred())
					Expect(err.Error()).Should(ContainSubstring("no committed outcome"))
					Expect(completedCount).Should(BeZero())
					Expect(failedCount).Should(BeZero())
					return
				}
				Expect(err).ShouldNot(HaveOccurred())
				Expect(completedCount).Should(Equal(int32(1)))
				Expect(failedCount).Should(BeZero())
			},
			Entry("continues an exact Resolved claim without replay", opsv1alpha1.DispatchClaimStateResolved, false),
			Entry("blocks an exact Claimed outcome without replay", opsv1alpha1.DispatchClaimStateClaimed, true),
			Entry("blocks an exact OutcomeUnknown outcome without replay", opsv1alpha1.DispatchClaimStateOutcomeUnknown, true),
		)

		It("Test switchover OpsRequest", func() {
			By("create switchover opsRequest")
			ops := testops.NewOpsRequestObj("ops-switchover-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace,
				clusterObj.Name, opsv1alpha1.SwitchoverType)
			instanceName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, defaultCompName, 1)
			ops.Spec.SwitchoverList = []opsv1alpha1.Switchover{
				{
					ComponentName: defaultCompName,
					InstanceName:  instanceName,
				},
			}
			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase

			By("mock switchover OpsRequest phase is Creating")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("do switchover action")
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(meta.FindStatusCondition(opsRes.OpsRequest.Status.Conditions, opsv1alpha1.ConditionTypeFailed)).Should(BeNil())

			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(ctx context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					GinkgoWriter.Printf("ActionRequest: %#v\n", req)
					Expect(req.Parameters["KB_SWITCHOVER_CURRENT_NAME"]).Should(Equal(instanceName))
					rsp := kbagentproto.ActionResponse{Message: "mock success"}
					return rsp, nil
				})
			})

			By("do reconcile switchover action")
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
		})

		testSwitchoverWithCandidate := func(useComponentObjectName bool) {
			By("create switchover opsRequest")
			ops := testops.NewOpsRequestObj("ops-switchover-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace,
				clusterObj.Name, opsv1alpha1.SwitchoverType)
			instanceName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, defaultCompName, 1)
			candidateName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, defaultCompName, 0)
			ops.Spec.SwitchoverList = []opsv1alpha1.Switchover{
				{
					ComponentName: defaultCompName,
					InstanceName:  instanceName,
					CandidateName: candidateName,
				},
			}
			if useComponentObjectName {
				ops.Spec.SwitchoverList[0].ComponentName = ""
				ops.Spec.SwitchoverList[0].ComponentObjectName = compObj.Name
			}
			fmt.Printf("ops: %#v\n", ops.Spec.SwitchoverList[0])
			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase

			By("mock switchover OpsRequest phase is Creating")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("do switchover action")
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(meta.FindStatusCondition(opsRes.OpsRequest.Status.Conditions, opsv1alpha1.ConditionTypeFailed)).Should(BeNil())

			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(ctx context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					GinkgoWriter.Printf("ActionRequest: %#v\n", req)
					Expect(req.Parameters["KB_SWITCHOVER_CURRENT_NAME"]).Should(Equal(instanceName))
					Expect(req.Parameters["KB_SWITCHOVER_CANDIDATE_NAME"]).Should(Equal(candidateName))
					rsp := kbagentproto.ActionResponse{Message: "mock success"}
					return rsp, nil
				})
			})

			By("do reconcile switchover action")
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
		}

		It("Test switchover OpsRequest with candidate", func() {
			testSwitchoverWithCandidate(false)
		})

		It("Test switchover OpsRequest with candidate and specified a component object name", func() {
			testSwitchoverWithCandidate(true)
		})
	})
})
