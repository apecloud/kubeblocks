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
	"encoding/json"
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		role := testapps.Follower
		if index == 0 {
			role = testapps.Leader
		}
		return role
	}

	It("reads the cross-release structured switchover dispatch fixture", func() {
		var dispatch opsv1alpha1.SwitchoverDispatchStatus
		Expect(json.Unmarshal([]byte(`{"version":"v1","opsRequestUID":"fixture-ops-uid","componentName":"fixture-component","instanceName":"fixture-instance","candidateName":"fixture-candidate","token":"fixture-token","state":"Resolved"}`), &dispatch)).Should(Succeed())
		Expect(dispatch).Should(Equal(opsv1alpha1.SwitchoverDispatchStatus{
			Version:       opsv1alpha1.SwitchoverDispatchProtocolVersionV1,
			OpsRequestUID: "fixture-ops-uid",
			ComponentName: "fixture-component",
			InstanceName:  "fixture-instance",
			CandidateName: "fixture-candidate",
			Token:         "fixture-token",
			State:         opsv1alpha1.SwitchoverDispatchResolved,
		}))
	})

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

		newDispatch := func(opsRequest *opsv1alpha1.OpsRequest, instanceName, candidateName string,
			state opsv1alpha1.SwitchoverDispatchState) *opsv1alpha1.SwitchoverDispatchStatus {
			return &opsv1alpha1.SwitchoverDispatchStatus{
				Version:       opsv1alpha1.SwitchoverDispatchProtocolVersionV1,
				OpsRequestUID: string(opsRequest.UID),
				ComponentName: defaultCompName,
				InstanceName:  instanceName,
				CandidateName: candidateName,
				Token:         "reader-token",
				State:         state,
			}
		}

		It("persists the versioned dispatch API independently of Message", func() {
			ops := testops.NewOpsRequestObj("ops-switchover-api-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace,
				clusterObj.Name, opsv1alpha1.SwitchoverType)
			instanceName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, defaultCompName, 1)
			ops.Spec.SwitchoverList = []opsv1alpha1.Switchover{{
				ComponentName: defaultCompName,
				InstanceName:  instanceName,
			}}
			created := testops.CreateOpsRequest(ctx, testCtx, ops)
			dispatch := newDispatch(created, instanceName, "", opsv1alpha1.SwitchoverDispatchResolved)
			created.Status.Progress = "0/1"
			created.Status.Components = map[string]opsv1alpha1.OpsRequestComponentStatus{
				defaultCompName: {
					ProgressDetails: []opsv1alpha1.ProgressStatusDetail{{
						ObjectKey:          getProgressObjectKey(KBSwitchoverKey, defaultCompName),
						Status:             opsv1alpha1.ProcessingProgressStatus,
						Message:            "display wording has no protocol meaning",
						SwitchoverDispatch: dispatch,
					}},
				},
			}
			Expect(k8sClient.Status().Update(ctx, created)).Should(Succeed())
			fresh := &opsv1alpha1.OpsRequest{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(created), fresh)).Should(Succeed())
			persisted := fresh.Status.Components[defaultCompName].ProgressDetails[0]
			Expect(persisted.SwitchoverDispatch).Should(Equal(dispatch))
			Expect(persisted.Message).Should(Equal("display wording has no protocol meaning"))
		})

		runPersistedProtocolReader := func(useComponentObjectName, useLegacyObjectKey, addLegacyObjectKey bool, candidateName string,
			dispatchBuilder func(*opsv1alpha1.OpsRequest, string, string) *opsv1alpha1.SwitchoverDispatchStatus,
			message string, progressStatuses ...opsv1alpha1.ProgressStatus) (*opsv1alpha1.ProgressStatusDetail, int32, int32, error) {
			ops := testops.NewOpsRequestObj("ops-switchover-reader-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace,
				clusterObj.Name, opsv1alpha1.SwitchoverType)
			instanceName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, defaultCompName, 1)
			ops.Spec.SwitchoverList = []opsv1alpha1.Switchover{{
				ComponentName: defaultCompName,
				InstanceName:  instanceName,
				CandidateName: candidateName,
			}}
			if useComponentObjectName {
				ops.Spec.SwitchoverList[0].ComponentName = ""
				ops.Spec.SwitchoverList[0].ComponentObjectName = compObj.Name
			}

			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)
			opsRes.OpsRequest.Status.Conditions = []metav1.Condition{{
				Type:   opsv1alpha1.ConditionTypeSwitchover,
				Status: metav1.ConditionTrue,
			}}
			progressComponentName := defaultCompName
			if useLegacyObjectKey {
				progressComponentName = compObj.Name
			}
			progressStatus := opsv1alpha1.ProcessingProgressStatus
			if len(progressStatuses) > 0 {
				progressStatus = progressStatuses[0]
			}
			var dispatch *opsv1alpha1.SwitchoverDispatchStatus
			if dispatchBuilder != nil {
				dispatch = dispatchBuilder(opsRes.OpsRequest, instanceName, candidateName)
			}
			opsRes.OpsRequest.Status.Components = map[string]opsv1alpha1.OpsRequestComponentStatus{
				progressComponentName: {
					Phase: appsv1.UpdatingComponentPhase,
					ProgressDetails: []opsv1alpha1.ProgressStatusDetail{{
						Group:              testapps.Follower,
						ObjectKey:          getProgressObjectKey(KBSwitchoverKey, progressComponentName),
						Status:             progressStatus,
						Message:            message,
						SwitchoverDispatch: dispatch,
					}},
				},
			}
			if addLegacyObjectKey {
				opsRes.OpsRequest.Status.Components[compObj.Name] = opsv1alpha1.OpsRequestComponentStatus{
					Phase: appsv1.UpdatingComponentPhase,
					ProgressDetails: []opsv1alpha1.ProgressStatusDetail{{
						Group:     testapps.Follower,
						ObjectKey: getProgressObjectKey(KBSwitchoverKey, compObj.Name),
						Status:    opsv1alpha1.ProcessingProgressStatus,
						Message:   "doing switchover",
					}},
				}
			}

			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).Times(0)
			})
			var completedCount, failedCount int32
			err := handleSwitchover(reqCtx, k8sClient, opsRes, &opsRes.OpsRequest.Spec.SwitchoverList[0],
				opsRes.OpsRequest, &completedCount, &failedCount)
			progressDetail := findStatusProgressDetail(opsRes.OpsRequest.Status.Components[progressComponentName].ProgressDetails,
				getProgressObjectKey(KBSwitchoverKey, progressComponentName))
			return progressDetail, completedCount, failedCount, err
		}

		It("fails closed without replaying a Claimed no-candidate dispatch", func() {
			progressDetail, completedCount, failedCount, err := runPersistedProtocolReader(false, false, false, "",
				func(opsRequest *opsv1alpha1.OpsRequest, instanceName, candidateName string) *opsv1alpha1.SwitchoverDispatchStatus {
					return newDispatch(opsRequest, instanceName, candidateName, opsv1alpha1.SwitchoverDispatchClaimed)
				}, "human-readable claim text")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(progressDetail).ShouldNot(BeNil())
			Expect(progressDetail.Status).Should(Equal(opsv1alpha1.FailedProgressStatus))
			Expect(progressDetail.Message).Should(ContainSubstring("no committed outcome"))
			Expect(completedCount).Should(Equal(int32(1)))
			Expect(failedCount).Should(Equal(int32(1)))
		})

		It("fails closed without replaying a Claimed candidate dispatch", func() {
			candidateName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, defaultCompName, 0)
			progressDetail, completedCount, failedCount, err := runPersistedProtocolReader(false, false, false, candidateName,
				func(opsRequest *opsv1alpha1.OpsRequest, instanceName, candidateName string) *opsv1alpha1.SwitchoverDispatchStatus {
					return newDispatch(opsRequest, instanceName, candidateName, opsv1alpha1.SwitchoverDispatchClaimed)
				}, "wording is not protocol state")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(progressDetail).ShouldNot(BeNil())
			Expect(progressDetail.Status).Should(Equal(opsv1alpha1.FailedProgressStatus))
			Expect(progressDetail.Message).Should(ContainSubstring("no committed outcome"))
			Expect(completedCount).Should(Equal(int32(1)))
			Expect(failedCount).Should(Equal(int32(1)))
		})

		It("continues candidate role observation for an exact Resolved dispatch", func() {
			candidateName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, defaultCompName, 0)
			progressDetail, completedCount, failedCount, err := runPersistedProtocolReader(false, false, false, candidateName,
				func(opsRequest *opsv1alpha1.OpsRequest, instanceName, candidateName string) *opsv1alpha1.SwitchoverDispatchStatus {
					return newDispatch(opsRequest, instanceName, candidateName, opsv1alpha1.SwitchoverDispatchResolved)
				}, "display-only resolved wording")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(progressDetail).ShouldNot(BeNil())
			Expect(progressDetail.Status).Should(Equal(opsv1alpha1.ProcessingProgressStatus))
			Expect(completedCount).Should(BeZero())
			Expect(failedCount).Should(BeZero())
		})

		It("accepts an exact Resolved dispatch without parsing Message", func() {
			progressDetail, completedCount, failedCount, err := runPersistedProtocolReader(false, false, false, "",
				func(opsRequest *opsv1alpha1.OpsRequest, instanceName, candidateName string) *opsv1alpha1.SwitchoverDispatchStatus {
					return newDispatch(opsRequest, instanceName, candidateName, opsv1alpha1.SwitchoverDispatchResolved)
				}, "this wording may change freely / SwitchoverDispatch/not-a-protocol")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(progressDetail).ShouldNot(BeNil())
			Expect(progressDetail.Status).Should(Equal(opsv1alpha1.SucceedProgressStatus))
			Expect(completedCount).Should(Equal(int32(1)))
			Expect(failedCount).Should(BeZero())
		})

		DescribeTable("fails closed on unsupported or foreign structured dispatch state without replay",
			func(candidateName string, progressStatus opsv1alpha1.ProgressStatus,
				mutate func(*opsv1alpha1.SwitchoverDispatchStatus), expectedMessage string) {
				progressDetail, completedCount, failedCount, err := runPersistedProtocolReader(false, false, false, candidateName,
					func(opsRequest *opsv1alpha1.OpsRequest, instanceName, candidateName string) *opsv1alpha1.SwitchoverDispatchStatus {
						dispatch := newDispatch(opsRequest, instanceName, candidateName, opsv1alpha1.SwitchoverDispatchResolved)
						mutate(dispatch)
						return dispatch
					}, "display text", progressStatus)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(progressDetail).ShouldNot(BeNil())
				Expect(progressDetail.Status).Should(Equal(opsv1alpha1.FailedProgressStatus))
				Expect(progressDetail.Message).Should(ContainSubstring(expectedMessage))
				Expect(completedCount).Should(Equal(int32(1)))
				Expect(failedCount).Should(Equal(int32(1)))
			},
			Entry("unsupported version", "", opsv1alpha1.ProcessingProgressStatus,
				func(dispatch *opsv1alpha1.SwitchoverDispatchStatus) { dispatch.Version = "v2" }, "unsupported protocol version"),
			Entry("foreign OpsRequest UID", "", opsv1alpha1.ProcessingProgressStatus,
				func(dispatch *opsv1alpha1.SwitchoverDispatchStatus) { dispatch.OpsRequestUID = "foreign-uid" }, "identity is incomplete"),
			Entry("foreign component", "", opsv1alpha1.ProcessingProgressStatus,
				func(dispatch *opsv1alpha1.SwitchoverDispatchStatus) { dispatch.ComponentName = "foreign-component" }, "identity is incomplete"),
			Entry("foreign instance", "", opsv1alpha1.ProcessingProgressStatus,
				func(dispatch *opsv1alpha1.SwitchoverDispatchStatus) { dispatch.InstanceName = "foreign-instance" }, "identity is incomplete"),
			Entry("foreign candidate", "foreign-candidate", opsv1alpha1.ProcessingProgressStatus,
				func(dispatch *opsv1alpha1.SwitchoverDispatchStatus) { dispatch.CandidateName = "different-candidate" }, "identity is incomplete"),
			Entry("empty token", "", opsv1alpha1.ProcessingProgressStatus,
				func(dispatch *opsv1alpha1.SwitchoverDispatchStatus) { dispatch.Token = "" }, "identity is incomplete"),
			Entry("Claimed", "", opsv1alpha1.ProcessingProgressStatus,
				func(dispatch *opsv1alpha1.SwitchoverDispatchStatus) {
					dispatch.State = opsv1alpha1.SwitchoverDispatchClaimed
				}, "no committed outcome"),
			Entry("OutcomeUnknown", "", opsv1alpha1.ProcessingProgressStatus,
				func(dispatch *opsv1alpha1.SwitchoverDispatchStatus) {
					dispatch.State = opsv1alpha1.SwitchoverDispatchOutcomeUnknown
				}, "no committed outcome"),
			Entry("unknown state", "", opsv1alpha1.ProcessingProgressStatus,
				func(dispatch *opsv1alpha1.SwitchoverDispatchStatus) { dispatch.State = "Future" }, "unknown state"),
			Entry("Resolved with Pending progress", "", opsv1alpha1.PendingProgressStatus,
				func(*opsv1alpha1.SwitchoverDispatchStatus) {}, "paired with Pending"),
		)

		It("keeps the legacy no-structured-state path readable", func() {
			progressDetail, completedCount, failedCount, err := runPersistedProtocolReader(
				false, false, false, "", nil, "doing switchover")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(progressDetail).ShouldNot(BeNil())
			Expect(progressDetail.Status).Should(Equal(opsv1alpha1.SucceedProgressStatus))
			Expect(completedCount).Should(Equal(int32(1)))
			Expect(failedCount).Should(BeZero())
		})

		It("reads ComponentObjectName dispatch state in the writer's logical component value domain", func() {
			progressDetail, completedCount, failedCount, err := runPersistedProtocolReader(true, false, false, "",
				func(opsRequest *opsv1alpha1.OpsRequest, instanceName, candidateName string) *opsv1alpha1.SwitchoverDispatchStatus {
					return newDispatch(opsRequest, instanceName, candidateName, opsv1alpha1.SwitchoverDispatchResolved)
				}, "resolved")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(progressDetail).ShouldNot(BeNil())
			Expect(progressDetail.Status).Should(Equal(opsv1alpha1.SucceedProgressStatus))
			Expect(completedCount).Should(Equal(int32(1)))
			Expect(failedCount).Should(BeZero())
		})

		It("continues a legacy ComponentObjectName progress stored under the object-name key", func() {
			progressDetail, completedCount, failedCount, err := runPersistedProtocolReader(
				true, true, false, "", nil, "doing switchover")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(progressDetail).ShouldNot(BeNil())
			Expect(progressDetail.Status).Should(Equal(opsv1alpha1.SucceedProgressStatus))
			Expect(completedCount).Should(Equal(int32(1)))
			Expect(failedCount).Should(BeZero())
		})

		It("fails closed on structured dispatch state stored under the legacy object-name key", func() {
			progressDetail, completedCount, failedCount, err := runPersistedProtocolReader(true, true, false, "",
				func(opsRequest *opsv1alpha1.OpsRequest, instanceName, candidateName string) *opsv1alpha1.SwitchoverDispatchStatus {
					return newDispatch(opsRequest, instanceName, candidateName, opsv1alpha1.SwitchoverDispatchResolved)
				}, "resolved")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(progressDetail).ShouldNot(BeNil())
			Expect(progressDetail.Status).Should(Equal(opsv1alpha1.FailedProgressStatus))
			Expect(progressDetail.Message).Should(ContainSubstring("legacy component-object progress key"))
			Expect(completedCount).Should(Equal(int32(1)))
			Expect(failedCount).Should(Equal(int32(1)))
		})

		It("fails closed before dispatch when a Pending legacy object-name key contains structured state", func() {
			progressDetail, completedCount, failedCount, err := runPersistedProtocolReader(true, true, false, "",
				func(opsRequest *opsv1alpha1.OpsRequest, instanceName, candidateName string) *opsv1alpha1.SwitchoverDispatchStatus {
					return newDispatch(opsRequest, instanceName, candidateName, opsv1alpha1.SwitchoverDispatchResolved)
				}, "resolved", opsv1alpha1.PendingProgressStatus)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(progressDetail).ShouldNot(BeNil())
			Expect(progressDetail.Status).Should(Equal(opsv1alpha1.FailedProgressStatus))
			Expect(progressDetail.Message).Should(ContainSubstring("legacy component-object progress key"))
			Expect(completedCount).Should(Equal(int32(1)))
			Expect(failedCount).Should(Equal(int32(1)))
		})

		It("prefers the logical progress key when legacy and logical keys both exist", func() {
			progressDetail, completedCount, failedCount, err := runPersistedProtocolReader(true, false, true, "",
				func(opsRequest *opsv1alpha1.OpsRequest, instanceName, candidateName string) *opsv1alpha1.SwitchoverDispatchStatus {
					return newDispatch(opsRequest, instanceName, candidateName, opsv1alpha1.SwitchoverDispatchClaimed)
				}, "claimed")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(progressDetail).ShouldNot(BeNil())
			Expect(progressDetail.Status).Should(Equal(opsv1alpha1.FailedProgressStatus))
			Expect(progressDetail.Message).Should(ContainSubstring("no committed outcome"))
			Expect(completedCount).Should(Equal(int32(1)))
			Expect(failedCount).Should(Equal(int32(1)))
			legacyDetail := opsRes.OpsRequest.Status.Components[compObj.Name].ProgressDetails[0]
			Expect(legacyDetail.Status).Should(Equal(opsv1alpha1.ProcessingProgressStatus))
			Expect(legacyDetail.Message).Should(Equal("doing switchover"))
		})

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
			progressComponentName := defaultCompName
			if useComponentObjectName {
				progressComponentName = compObj.Name
				Expect(opsRes.OpsRequest.Status.Components).ShouldNot(HaveKey(defaultCompName))
			}
			Expect(opsRes.OpsRequest.Status.Components).Should(HaveKey(progressComponentName))
			Expect(opsRes.OpsRequest.Status.Components[progressComponentName].ProgressDetails).Should(ContainElement(
				HaveField("ObjectKey", getProgressObjectKey(KBSwitchoverKey, progressComponentName))))

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
