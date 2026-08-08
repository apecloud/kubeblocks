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
	"net/http"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
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

type interceptStatusClient struct {
	client.Client
	patch func(context.Context, client.Object, client.Patch, ...client.SubResourcePatchOption) error
}

type interceptReader struct {
	client.Reader
	get func(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error
}

func (r *interceptReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if r.get != nil {
		return r.get(ctx, key, obj, opts...)
	}
	return r.Reader.Get(ctx, key, obj, opts...)
}

func (c *interceptStatusClient) Status() client.SubResourceWriter {
	return &interceptStatusWriter{
		SubResourceWriter: c.Client.Status(),
		patch:             c.patch,
	}
}

type interceptStatusWriter struct {
	client.SubResourceWriter
	patch func(context.Context, client.Object, client.Patch, ...client.SubResourcePatchOption) error
}

func (w *interceptStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	if w.patch != nil {
		return w.patch(ctx, obj, patch, opts...)
	}
	return w.SubResourceWriter.Patch(ctx, obj, patch, opts...)
}

func switchoverDispatchClaimMessageForTest(opsRequest *opsv1alpha1.OpsRequest, componentName string,
	switchover opsv1alpha1.Switchover, token string) string {
	return switchoverDispatchClaim{
		opsRequestUID: string(opsRequest.UID),
		componentName: componentName,
		instanceName:  switchover.InstanceName,
		candidateName: switchover.CandidateName,
		token:         token,
	}.message()
}

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
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS, ml)
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
				Cluster:   clusterObj,
				Recorder:  k8sManager.GetEventRecorderFor("opsrequest-controller"),
				APIReader: k8sClient,
			}
		})

		preparePendingSwitchover := func(candidateName string) (client.ObjectKey, string) {
			ops := testops.NewOpsRequestObj("ops-switchover-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace,
				clusterObj.Name, opsv1alpha1.SwitchoverType)
			instanceName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, defaultCompName, 1)
			ops.Spec.SwitchoverList = []opsv1alpha1.Switchover{{
				ComponentName: defaultCompName,
				InstanceName:  instanceName,
				CandidateName: candidateName,
			}}
			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase

			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			key := client.ObjectKeyFromObject(opsRes.OpsRequest)
			stored := &opsv1alpha1.OpsRequest{}
			Expect(k8sClient.Get(ctx, key, stored)).Should(Succeed())
			patch := client.MergeFrom(stored.DeepCopy())
			stored.Status = opsRes.OpsRequest.DeepCopy().Status
			stored.Status.Phase = opsv1alpha1.OpsRunningPhase
			Expect(k8sClient.Status().Patch(ctx, stored, patch)).Should(Succeed())
			opsRes.OpsRequest = stored
			return key, instanceName
		}

		It("persists the Processing dispatch claim before the lifecycle call", func() {
			key, instanceName := preparePendingSwitchover("")
			actionCalls := 0
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(ctx context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					actionCalls++
					Expect(req.Parameters["KB_SWITCHOVER_CURRENT_NAME"]).Should(Equal(instanceName))
					claimed := &opsv1alpha1.OpsRequest{}
					Expect(k8sClient.Get(ctx, key, claimed)).Should(Succeed())
					progressDetail := findStatusProgressDetail(claimed.Status.Components[defaultCompName].ProgressDetails,
						getProgressObjectKey(KBSwitchoverKey, defaultCompName))
					Expect(progressDetail).ShouldNot(BeNil())
					Expect(progressDetail.Status).Should(Equal(opsv1alpha1.ProcessingProgressStatus))
					claim, ok := parseSwitchoverDispatchClaim(progressDetail.Message)
					Expect(ok).Should(BeTrue())
					Expect(claim.matchesIdentity(claimed, defaultCompName, claimed.Spec.SwitchoverList[0])).Should(BeTrue())
					Expect(claim.token).ShouldNot(BeEmpty())
					return kbagentproto.ActionResponse{Message: "mock success"}, nil
				})
			})

			_, _, _, err := handleSwitchovers(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(actionCalls).Should(Equal(1))
		})

		It("retries a transient direct-read failure after the dispatch claim commits", func() {
			preparePendingSwitchover("")
			actionCalls := 0
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(context.Context, kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					actionCalls++
					return kbagentproto.ActionResponse{Message: "mock success"}, nil
				})
			})

			injectedErr := fmt.Errorf("injected transient direct-read failure")
			failedReads := 0
			opsRes.APIReader = &interceptReader{
				Reader: k8sClient,
				get: func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if _, ok := obj.(*opsv1alpha1.OpsRequest); ok && failedReads == 0 {
						failedReads++
						return injectedErr
					}
					return k8sClient.Get(ctx, key, obj, opts...)
				},
			}

			_, _, _, err := handleSwitchovers(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(failedReads).Should(Equal(1))
			Expect(actionCalls).Should(Equal(1))
		})

		It("does not call the lifecycle action when the Processing claim loses a resource-version race", func() {
			key, _ := preparePendingSwitchover("")
			actionCalls := 0
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(context.Context, kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					actionCalls++
					return kbagentproto.ActionResponse{Message: "mock success"}, nil
				})
			})

			injected := false
			conflictingClient := &interceptStatusClient{
				Client: k8sClient,
				patch: func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
					if _, ok := obj.(*opsv1alpha1.OpsRequest); ok && !injected {
						injected = true
						concurrent := &opsv1alpha1.OpsRequest{}
						Expect(k8sClient.Get(ctx, key, concurrent)).Should(Succeed())
						concurrentPatch := client.MergeFrom(concurrent.DeepCopy())
						concurrent.Status.Progress = "99/99"
						Expect(k8sClient.Status().Patch(ctx, concurrent, concurrentPatch)).Should(Succeed())
					}
					return k8sClient.Status().Patch(ctx, obj, patch, opts...)
				},
			}

			_, _, _, err := handleSwitchovers(reqCtx, conflictingClient, opsRes)
			Expect(apierrors.IsConflict(err)).Should(BeTrue())
			Expect(actionCalls).Should(Equal(0))
		})

		It("lets only the writer whose unique token was committed dispatch after ambiguous patch responses", func() {
			preparePendingSwitchover("")
			writerOneOps := *opsRes
			writerOneOps.OpsRequest = opsRes.OpsRequest.DeepCopy()
			writerTwoOps := *opsRes
			writerTwoOps.OpsRequest = opsRes.OpsRequest.DeepCopy()

			writerOneResponseLoss := fmt.Errorf("writer one claim response lost")
			writerTwoResponseLoss := fmt.Errorf("writer two conflict response lost")
			var writerOneClaim, writerTwoClaim string
			actionCalls := 0
			var writerTwoErr error

			writerTwoClient := &interceptStatusClient{
				Client: k8sClient,
				patch: func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
					if writerTwoClaim == "" {
						opsRequest := obj.(*opsv1alpha1.OpsRequest)
						progressDetail := findStatusProgressDetail(opsRequest.Status.Components[defaultCompName].ProgressDetails,
							getProgressObjectKey(KBSwitchoverKey, defaultCompName))
						Expect(progressDetail).ShouldNot(BeNil())
						writerTwoClaim = progressDetail.Message
						conflictErr := k8sClient.Status().Patch(ctx, obj, patch, opts...)
						Expect(apierrors.IsConflict(conflictErr)).Should(BeTrue())
						return writerTwoResponseLoss
					}
					return k8sClient.Status().Patch(ctx, obj, patch, opts...)
				},
			}
			writerOneClient := &interceptStatusClient{
				Client: k8sClient,
				patch: func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
					if writerOneClaim == "" {
						opsRequest := obj.(*opsv1alpha1.OpsRequest)
						progressDetail := findStatusProgressDetail(opsRequest.Status.Components[defaultCompName].ProgressDetails,
							getProgressObjectKey(KBSwitchoverKey, defaultCompName))
						Expect(progressDetail).ShouldNot(BeNil())
						writerOneClaim = progressDetail.Message
						Expect(k8sClient.Status().Patch(ctx, obj, patch, opts...)).Should(Succeed())
						return writerOneResponseLoss
					}
					return k8sClient.Status().Patch(ctx, obj, patch, opts...)
				},
			}

			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(context.Context, kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					actionCalls++
					if actionCalls == 1 {
						_, _, _, writerTwoErr = handleSwitchovers(reqCtx, writerTwoClient, &writerTwoOps)
						live := &opsv1alpha1.OpsRequest{}
						Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(writerOneOps.OpsRequest), live)).Should(Succeed())
						progressDetail := findStatusProgressDetail(live.Status.Components[defaultCompName].ProgressDetails,
							getProgressObjectKey(KBSwitchoverKey, defaultCompName))
						Expect(progressDetail).ShouldNot(BeNil())
						Expect(progressDetail.Message).Should(Equal(writerOneClaim))
						Expect(progressDetail.Message).ShouldNot(Equal(writerTwoClaim))
					}
					return kbagentproto.ActionResponse{Message: "mock success"}, nil
				})
			})

			_, _, _, writerOneErr := handleSwitchovers(reqCtx, writerOneClient, &writerOneOps)
			Expect(writerOneErr).ShouldNot(HaveOccurred())
			Expect(writerTwoErr).Should(MatchError(writerTwoResponseLoss))
			Expect(actionCalls).Should(Equal(1))
			Expect(writerOneClaim).Should(HavePrefix(switchoverDispatchClaimMessagePrefix))
			Expect(writerTwoClaim).Should(HavePrefix(switchoverDispatchClaimMessagePrefix))
			Expect(writerOneClaim).ShouldNot(Equal(writerTwoClaim))
		})

		It("fails closed after restart when a no-candidate dispatch outcome is unknown", func() {
			key, _ := preparePendingSwitchover("")
			fresh := &opsv1alpha1.OpsRequest{}
			Expect(k8sClient.Get(ctx, key, fresh)).Should(Succeed())
			patch := client.MergeFrom(fresh.DeepCopy())
			progressDetail := findStatusProgressDetail(fresh.Status.Components[defaultCompName].ProgressDetails,
				getProgressObjectKey(KBSwitchoverKey, defaultCompName))
			Expect(progressDetail).ShouldNot(BeNil())
			progressDetail.Status = opsv1alpha1.ProcessingProgressStatus
			progressDetail.Message = switchoverDispatchClaimMessageForTest(fresh, defaultCompName,
				fresh.Spec.SwitchoverList[0], "restart-token")
			progressDetail.StartTime = metav1.Now()
			Expect(k8sClient.Status().Patch(ctx, fresh, patch)).Should(Succeed())
			opsRes.OpsRequest = fresh

			actionCalls := 0
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(context.Context, kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					actionCalls++
					return kbagentproto.ActionResponse{}, fmt.Errorf("unexpected redispatch")
				})
			})

			_, completedCount, failedCount, err := handleSwitchovers(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(actionCalls).Should(Equal(0))
			Expect(completedCount).Should(Equal(int32(1)))
			Expect(failedCount).Should(Equal(int32(1)))

			Expect(k8sClient.Get(ctx, key, fresh)).Should(Succeed())
			progressDetail = findStatusProgressDetail(fresh.Status.Components[defaultCompName].ProgressDetails,
				getProgressObjectKey(KBSwitchoverKey, defaultCompName))
			Expect(progressDetail.Status).Should(Equal(opsv1alpha1.FailedProgressStatus))
			Expect(progressDetail.Message).Should(ContainSubstring("outcome is unknown"))
		})

		It("continues candidate role observation without redispatch after restart with a retained dispatch claim", func() {
			candidateName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, defaultCompName, 0)
			key, _ := preparePendingSwitchover(candidateName)
			fresh := &opsv1alpha1.OpsRequest{}
			Expect(k8sClient.Get(ctx, key, fresh)).Should(Succeed())
			patch := client.MergeFrom(fresh.DeepCopy())
			progressDetail := findStatusProgressDetail(fresh.Status.Components[defaultCompName].ProgressDetails,
				getProgressObjectKey(KBSwitchoverKey, defaultCompName))
			Expect(progressDetail).ShouldNot(BeNil())
			progressDetail.Status = opsv1alpha1.ProcessingProgressStatus
			progressDetail.Message = switchoverDispatchClaimMessageForTest(fresh, defaultCompName,
				fresh.Spec.SwitchoverList[0], "restart-with-candidate-token")
			progressDetail.StartTime = metav1.Now()
			Expect(k8sClient.Status().Patch(ctx, fresh, patch)).Should(Succeed())
			opsRes.OpsRequest = fresh

			candidatePod := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: testCtx.DefaultNamespace, Name: candidateName}, candidatePod)).Should(Succeed())
			candidatePod.Labels[constant.RoleLabelKey] = "unexpected-role"
			Expect(k8sClient.Update(ctx, candidatePod)).Should(Succeed())

			actionCalls := 0
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(context.Context, kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					actionCalls++
					return kbagentproto.ActionResponse{}, fmt.Errorf("unexpected redispatch")
				})
			})

			_, completedCount, failedCount, err := handleSwitchovers(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(actionCalls).Should(Equal(0))
			Expect(completedCount).Should(Equal(int32(0)))
			Expect(failedCount).Should(Equal(int32(0)))

			Expect(k8sClient.Get(ctx, key, fresh)).Should(Succeed())
			progressDetail = findStatusProgressDetail(fresh.Status.Components[defaultCompName].ProgressDetails,
				getProgressObjectKey(KBSwitchoverKey, defaultCompName))
			Expect(progressDetail.Status).Should(Equal(opsv1alpha1.ProcessingProgressStatus))
			Expect(progressDetail.Message).Should(ContainSubstring("waiting for candidate pod"))
			Expect(progressDetail.Message).Should(ContainSubstring("unexpected-role"))
		})

		It("fails closed when a persisted dispatch claim no longer matches the request identity", func() {
			key, _ := preparePendingSwitchover("")
			fresh := &opsv1alpha1.OpsRequest{}
			Expect(k8sClient.Get(ctx, key, fresh)).Should(Succeed())
			patch := client.MergeFrom(fresh.DeepCopy())
			progressDetail := findStatusProgressDetail(fresh.Status.Components[defaultCompName].ProgressDetails,
				getProgressObjectKey(KBSwitchoverKey, defaultCompName))
			Expect(progressDetail).ShouldNot(BeNil())
			progressDetail.Status = opsv1alpha1.ProcessingProgressStatus
			progressDetail.Message = switchoverDispatchClaimMessagePrefix + "different-request/default/instance/candidate/foreign-token"
			progressDetail.StartTime = metav1.Now()
			Expect(k8sClient.Status().Patch(ctx, fresh, patch)).Should(Succeed())
			opsRes.OpsRequest = fresh

			actionCalls := 0
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(context.Context, kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					actionCalls++
					return kbagentproto.ActionResponse{}, fmt.Errorf("unexpected redispatch")
				})
			})

			_, completedCount, failedCount, err := handleSwitchovers(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(actionCalls).Should(Equal(0))
			Expect(completedCount).Should(Equal(int32(1)))
			Expect(failedCount).Should(Equal(int32(1)))
			Expect(k8sClient.Get(ctx, key, fresh)).Should(Succeed())
			progressDetail = findStatusProgressDetail(fresh.Status.Components[defaultCompName].ProgressDetails,
				getProgressObjectKey(KBSwitchoverKey, defaultCompName))
			Expect(progressDetail.Message).Should(ContainSubstring("protocol identity changed or is malformed"))
		})

		DescribeTable("fails closed after restart on invalid persisted outcome markers", func(buildMessage func(*opsv1alpha1.OpsRequest) string) {
			key, _ := preparePendingSwitchover("")
			fresh := &opsv1alpha1.OpsRequest{}
			Expect(k8sClient.Get(ctx, key, fresh)).Should(Succeed())
			patch := client.MergeFrom(fresh.DeepCopy())
			progressDetail := findStatusProgressDetail(fresh.Status.Components[defaultCompName].ProgressDetails,
				getProgressObjectKey(KBSwitchoverKey, defaultCompName))
			Expect(progressDetail).ShouldNot(BeNil())
			progressDetail.Status = opsv1alpha1.ProcessingProgressStatus
			progressDetail.Message = buildMessage(fresh)
			progressDetail.StartTime = metav1.Now()
			Expect(k8sClient.Status().Patch(ctx, fresh, patch)).Should(Succeed())
			opsRes.OpsRequest = fresh

			actionCalls := 0
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(context.Context, kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					actionCalls++
					return kbagentproto.ActionResponse{}, fmt.Errorf("unexpected redispatch")
				})
			})

			_, completedCount, failedCount, err := handleSwitchovers(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(completedCount).Should(Equal(int32(1)))
			Expect(failedCount).Should(Equal(int32(1)))
			Expect(actionCalls).Should(Equal(0))

			Expect(k8sClient.Get(ctx, key, fresh)).Should(Succeed())
			progressDetail = findStatusProgressDetail(fresh.Status.Components[defaultCompName].ProgressDetails,
				getProgressObjectKey(KBSwitchoverKey, defaultCompName))
			Expect(progressDetail.Status).Should(Equal(opsv1alpha1.FailedProgressStatus))
			Expect(progressDetail.Message).Should(ContainSubstring("protocol identity changed or is malformed"))
		},
			Entry("malformed outcome", func(*opsv1alpha1.OpsRequest) string {
				return switchoverDispatchOutcomeMessagePrefix + "malformed"
			}),
			Entry("foreign outcome identity and token", func(fresh *opsv1alpha1.OpsRequest) string {
				claim := switchoverDispatchClaim{
					opsRequestUID: "different-request",
					componentName: defaultCompName,
					instanceName:  fresh.Spec.SwitchoverList[0].InstanceName,
					token:         "foreign-token",
				}
				return claim.outcomeMessage("doing switchover")
			}),
		)

		It("does not call the lifecycle action without a live API reader", func() {
			key, _ := preparePendingSwitchover("")
			before := &opsv1alpha1.OpsRequest{}
			Expect(k8sClient.Get(ctx, key, before)).Should(Succeed())
			opsRes.APIReader = nil
			actionCalls := 0
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(context.Context, kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					actionCalls++
					return kbagentproto.ActionResponse{}, fmt.Errorf("unexpected dispatch")
				})
			})

			_, _, _, err := handleSwitchovers(reqCtx, k8sClient, opsRes)
			Expect(err).Should(MatchError("APIReader is required to confirm a switchover dispatch claim"))
			Expect(actionCalls).Should(Equal(0))

			after := &opsv1alpha1.OpsRequest{}
			Expect(k8sClient.Get(ctx, key, after)).Should(Succeed())
			Expect(after.ResourceVersion).Should(Equal(before.ResourceVersion))
			Expect(after.Status).Should(Equal(before.Status))
		})

		It("does not write a dispatch claim without a complete OpsRequest UID", func() {
			key, _ := preparePendingSwitchover("")
			before := &opsv1alpha1.OpsRequest{}
			Expect(k8sClient.Get(ctx, key, before)).Should(Succeed())
			opsRes.OpsRequest.UID = ""
			actionCalls := 0
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(context.Context, kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					actionCalls++
					return kbagentproto.ActionResponse{}, fmt.Errorf("unexpected dispatch")
				})
			})

			_, _, _, err := handleSwitchovers(reqCtx, k8sClient, opsRes)
			Expect(err).Should(MatchError("OpsRequest UID is required to create a switchover dispatch claim"))
			Expect(actionCalls).Should(Equal(0))

			after := &opsv1alpha1.OpsRequest{}
			Expect(k8sClient.Get(ctx, key, after)).Should(Succeed())
			Expect(after.ResourceVersion).Should(Equal(before.ResourceVersion))
			Expect(after.Status).Should(Equal(before.Status))
		})

		It("refetches and retries a known successful outcome after a status conflict", func() {
			key, _ := preparePendingSwitchover("")
			actionCalls := 0
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(context.Context, kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					actionCalls++
					return kbagentproto.ActionResponse{Message: "mock success"}, nil
				})
			})

			conflicted := false
			conflictingClient := &interceptStatusClient{
				Client: k8sClient,
				patch: func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
					if _, ok := obj.(*opsv1alpha1.OpsRequest); ok && actionCalls > 0 && !conflicted {
						conflicted = true
						concurrent := &opsv1alpha1.OpsRequest{}
						Expect(k8sClient.Get(ctx, key, concurrent)).Should(Succeed())
						concurrentPatch := client.MergeFromWithOptions(concurrent.DeepCopy(), client.MergeFromWithOptimisticLock{})
						concurrent.Status.Progress = "99/99"
						Expect(k8sClient.Status().Patch(ctx, concurrent, concurrentPatch)).Should(Succeed())
						conflictErr := k8sClient.Status().Patch(ctx, obj, patch, opts...)
						Expect(apierrors.IsConflict(conflictErr)).Should(BeTrue())
						return conflictErr
					}
					return k8sClient.Status().Patch(ctx, obj, patch, opts...)
				},
			}

			_, _, _, err := handleSwitchovers(reqCtx, conflictingClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(actionCalls).Should(Equal(1))
			Expect(conflicted).Should(BeTrue())

			fresh := &opsv1alpha1.OpsRequest{}
			Expect(k8sClient.Get(ctx, key, fresh)).Should(Succeed())
			progressDetail := findStatusProgressDetail(fresh.Status.Components[defaultCompName].ProgressDetails,
				getProgressObjectKey(KBSwitchoverKey, defaultCompName))
			Expect(progressDetail).ShouldNot(BeNil())
			Expect(progressDetail.Status).Should(Equal(opsv1alpha1.ProcessingProgressStatus))
			outcomeClaim, outcome, ok := parseSwitchoverDispatchOutcomeMessage(progressDetail.Message)
			Expect(ok).Should(BeTrue())
			Expect(outcomeClaim.matchesIdentity(fresh, defaultCompName, fresh.Spec.SwitchoverList[0])).Should(BeTrue())
			Expect(outcomeClaim.token).ShouldNot(BeEmpty())
			Expect(outcome).Should(Equal("doing switchover"))
		})

		It("keeps retrying a known failed outcome beyond the default bounded backoff", func() {
			key, _ := preparePendingSwitchover("")
			actionCalls := 0
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(context.Context, kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					actionCalls++
					return kbagentproto.ActionResponse{}, fmt.Errorf("injected lifecycle failure")
				})
			})

			injectedErr := fmt.Errorf("injected failed-outcome status failure")
			failuresRemaining := retry.DefaultBackoff.Steps + 2
			failedAttempts := 0
			failingClient := &interceptStatusClient{
				Client: k8sClient,
				patch: func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
					if _, ok := obj.(*opsv1alpha1.OpsRequest); ok && actionCalls > 0 && failuresRemaining > 0 {
						failuresRemaining--
						failedAttempts++
						return injectedErr
					}
					return k8sClient.Status().Patch(ctx, obj, patch, opts...)
				},
			}

			_, completedCount, failedCount, err := handleSwitchovers(reqCtx, failingClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(actionCalls).Should(Equal(1))
			Expect(failedAttempts).Should(Equal(retry.DefaultBackoff.Steps + 2))
			Expect(completedCount).Should(Equal(int32(1)))
			Expect(failedCount).Should(Equal(int32(1)))

			fresh := &opsv1alpha1.OpsRequest{}
			Expect(k8sClient.Get(ctx, key, fresh)).Should(Succeed())
			progressDetail := findStatusProgressDetail(fresh.Status.Components[defaultCompName].ProgressDetails,
				getProgressObjectKey(KBSwitchoverKey, defaultCompName))
			Expect(progressDetail).ShouldNot(BeNil())
			Expect(progressDetail.Status).Should(Equal(opsv1alpha1.FailedProgressStatus))
			outcomeClaim, outcome, ok := parseSwitchoverDispatchOutcomeMessage(progressDetail.Message)
			Expect(ok).Should(BeTrue())
			Expect(outcomeClaim.matchesIdentity(fresh, defaultCompName, fresh.Spec.SwitchoverList[0])).Should(BeTrue())
			Expect(outcome).Should(ContainSubstring("injected lifecycle failure"))
		})

		It("stops retrying a known outcome when the caller context ends", func() {
			retryCtx, cancel := context.WithCancel(context.Background())
			calls := 0
			err := retryKnownSwitchoverOutcomeUntilContextDone(retryCtx, func() error {
				calls++
				if calls == 2 {
					cancel()
				}
				return fmt.Errorf("injected retryable status failure")
			})

			Expect(err).Should(MatchError(ContainSubstring("context canceled")))
			Expect(calls).Should(Equal(2))
		})

		DescribeTable("does not retry a known outcome after a permanent API response", func(statusCode int) {
			calls := 0
			permanentErr := apierrors.NewGenericServerResponse(statusCode, "PATCH", schema.GroupResource{
				Group: "apps.kubeblocks.io", Resource: "opsrequests",
			}, "ops", "injected permanent status failure", 0, false)
			err := retryKnownSwitchoverOutcomeUntilContextDone(context.Background(), func() error {
				calls++
				return permanentErr
			})

			Expect(err).Should(MatchError(permanentErr))
			Expect(calls).Should(Equal(1))
		},
			Entry("413 request entity too large", http.StatusRequestEntityTooLarge),
			Entry("400 bad request", http.StatusBadRequest),
			Entry("405 method not supported", http.StatusMethodNotAllowed),
			Entry("415 unsupported media type", http.StatusUnsupportedMediaType),
			Entry("406 not acceptable", http.StatusNotAcceptable),
		)

		It("accepts a known outcome after its committed status response is lost", func() {
			key, _ := preparePendingSwitchover("")
			actionCalls := 0
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(context.Context, kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					actionCalls++
					return kbagentproto.ActionResponse{Message: "mock success"}, nil
				})
			})

			injectedErr := fmt.Errorf("injected committed status response loss")
			lost := false
			responseLossClient := &interceptStatusClient{
				Client: k8sClient,
				patch: func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
					if _, ok := obj.(*opsv1alpha1.OpsRequest); ok && actionCalls > 0 && !lost {
						lost = true
						Expect(k8sClient.Status().Patch(ctx, obj, patch, opts...)).Should(Succeed())
						return injectedErr
					}
					return k8sClient.Status().Patch(ctx, obj, patch, opts...)
				},
			}

			_, _, _, err := handleSwitchovers(reqCtx, responseLossClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(actionCalls).Should(Equal(1))
			Expect(lost).Should(BeTrue())

			fresh := &opsv1alpha1.OpsRequest{}
			Expect(k8sClient.Get(ctx, key, fresh)).Should(Succeed())
			progressDetail := findStatusProgressDetail(fresh.Status.Components[defaultCompName].ProgressDetails,
				getProgressObjectKey(KBSwitchoverKey, defaultCompName))
			Expect(progressDetail).ShouldNot(BeNil())
			Expect(progressDetail.Status).Should(Equal(opsv1alpha1.ProcessingProgressStatus))
			outcomeClaim, outcome, ok := parseSwitchoverDispatchOutcomeMessage(progressDetail.Message)
			Expect(ok).Should(BeTrue())
			Expect(outcomeClaim.matchesIdentity(fresh, defaultCompName, fresh.Spec.SwitchoverList[0])).Should(BeTrue())
			Expect(outcome).Should(Equal("doing switchover"))
		})

		It("does not accept matching status and outcome text without the exact token", func() {
			key, _ := preparePendingSwitchover("")
			actionCalls := 0
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(ctx context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					actionCalls++
					fresh := &opsv1alpha1.OpsRequest{}
					Expect(k8sClient.Get(ctx, key, fresh)).Should(Succeed())
					patch := client.MergeFromWithOptions(fresh.DeepCopy(), client.MergeFromWithOptimisticLock{})
					progressDetail := findStatusProgressDetail(fresh.Status.Components[defaultCompName].ProgressDetails,
						getProgressObjectKey(KBSwitchoverKey, defaultCompName))
					Expect(progressDetail).ShouldNot(BeNil())
					progressDetail.Status = opsv1alpha1.ProcessingProgressStatus
					progressDetail.Message = "doing switchover"
					Expect(k8sClient.Status().Patch(ctx, fresh, patch)).Should(Succeed())
					return kbagentproto.ActionResponse{Message: "mock success"}, nil
				})
			})

			_, _, _, err := handleSwitchovers(reqCtx, k8sClient, opsRes)
			Expect(err).Should(MatchError(ContainSubstring("switchover dispatch claim was lost")))
			Expect(actionCalls).Should(Equal(1))

			fresh := &opsv1alpha1.OpsRequest{}
			Expect(k8sClient.Get(ctx, key, fresh)).Should(Succeed())
			progressDetail := findStatusProgressDetail(fresh.Status.Components[defaultCompName].ProgressDetails,
				getProgressObjectKey(KBSwitchoverKey, defaultCompName))
			Expect(progressDetail).ShouldNot(BeNil())
			Expect(progressDetail.Status).Should(Equal(opsv1alpha1.ProcessingProgressStatus))
			Expect(progressDetail.Message).Should(Equal("doing switchover"))
		})

		It("does not accept a matching failed outcome persisted by a foreign token", func() {
			key, _ := preparePendingSwitchover("")
			actionCalls := 0
			foreignMessage := ""
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(ctx context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					actionCalls++
					fresh := &opsv1alpha1.OpsRequest{}
					Expect(k8sClient.Get(ctx, key, fresh)).Should(Succeed())
					patch := client.MergeFromWithOptions(fresh.DeepCopy(), client.MergeFromWithOptimisticLock{})
					progressDetail := findStatusProgressDetail(fresh.Status.Components[defaultCompName].ProgressDetails,
						getProgressObjectKey(KBSwitchoverKey, defaultCompName))
					Expect(progressDetail).ShouldNot(BeNil())
					foreignClaim, ok := parseSwitchoverDispatchClaim(progressDetail.Message)
					Expect(ok).Should(BeTrue())
					foreignClaim.token = "foreign-token"
					foreignMessage = foreignClaim.outcomeMessage("component " + defaultCompName + " injected lifecycle failure")
					progressDetail.Status = opsv1alpha1.FailedProgressStatus
					progressDetail.Message = foreignMessage
					Expect(k8sClient.Status().Patch(ctx, fresh, patch)).Should(Succeed())
					return kbagentproto.ActionResponse{}, fmt.Errorf("injected lifecycle failure")
				})
			})

			_, _, _, err := handleSwitchovers(reqCtx, k8sClient, opsRes)
			Expect(err).Should(MatchError(ContainSubstring("switchover dispatch claim was lost")))
			Expect(actionCalls).Should(Equal(1))

			fresh := &opsv1alpha1.OpsRequest{}
			Expect(k8sClient.Get(ctx, key, fresh)).Should(Succeed())
			progressDetail := findStatusProgressDetail(fresh.Status.Components[defaultCompName].ProgressDetails,
				getProgressObjectKey(KBSwitchoverKey, defaultCompName))
			Expect(progressDetail).ShouldNot(BeNil())
			Expect(progressDetail.Status).Should(Equal(opsv1alpha1.FailedProgressStatus))
			Expect(progressDetail.Message).Should(Equal(foreignMessage))
		})

		It("does not overwrite a foreign token while persisting a known outcome", func() {
			key, _ := preparePendingSwitchover("")
			actionCalls := 0
			foreignMessage := ""
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(ctx context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					actionCalls++
					fresh := &opsv1alpha1.OpsRequest{}
					Expect(k8sClient.Get(ctx, key, fresh)).Should(Succeed())
					patch := client.MergeFromWithOptions(fresh.DeepCopy(), client.MergeFromWithOptimisticLock{})
					progressDetail := findStatusProgressDetail(fresh.Status.Components[defaultCompName].ProgressDetails,
						getProgressObjectKey(KBSwitchoverKey, defaultCompName))
					Expect(progressDetail).ShouldNot(BeNil())
					foreignMessage = switchoverDispatchClaimMessageForTest(fresh, defaultCompName,
						fresh.Spec.SwitchoverList[0], "foreign-token")
					progressDetail.Message = foreignMessage
					Expect(k8sClient.Status().Patch(ctx, fresh, patch)).Should(Succeed())
					return kbagentproto.ActionResponse{Message: "mock success"}, nil
				})
			})

			_, _, _, err := handleSwitchovers(reqCtx, k8sClient, opsRes)
			Expect(err).Should(MatchError(ContainSubstring("switchover dispatch claim was lost")))
			Expect(actionCalls).Should(Equal(1))

			fresh := &opsv1alpha1.OpsRequest{}
			Expect(k8sClient.Get(ctx, key, fresh)).Should(Succeed())
			progressDetail := findStatusProgressDetail(fresh.Status.Components[defaultCompName].ProgressDetails,
				getProgressObjectKey(KBSwitchoverKey, defaultCompName))
			Expect(progressDetail).ShouldNot(BeNil())
			Expect(progressDetail.Status).Should(Equal(opsv1alpha1.ProcessingProgressStatus))
			Expect(progressDetail.Message).Should(Equal(foreignMessage))
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

		It("fails switchover OpsRequest when source instance does not exist", func() {
			By("create switchover opsRequest with a missing source instance")
			ops := testops.NewOpsRequestObj("ops-switchover-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace,
				clusterObj.Name, opsv1alpha1.SwitchoverType)
			instanceName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, defaultCompName, 99)
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

			By("do switchover action and expect terminal failure")
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsFailedPhase))
		})

		It("fails switchover OpsRequest when source instance disappears before action execution", func() {
			By("create switchover opsRequest with a valid source instance")
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
			key := client.ObjectKeyFromObject(opsRes.OpsRequest)

			By("run precheck while the source pod still exists")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, key)).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("delete the source Pod before the lifecycle action is invoked")
			sourcePod := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: testCtx.DefaultNamespace, Name: instanceName}, sourcePod)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, sourcePod)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Namespace: testCtx.DefaultNamespace, Name: instanceName}, &corev1.Pod{})
				return apierrors.IsNotFound(err)
			}).Should(BeTrue())

			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).Times(0)
			})

			By("do switchover action and expect terminal failure without calling the addon")
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, key)).Should(Equal(opsv1alpha1.OpsFailedPhase))
		})

		It("fails switchover OpsRequest when candidate disappears before action execution", func() {
			By("create switchover opsRequest with a valid source and candidate")
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
			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
			key := client.ObjectKeyFromObject(opsRes.OpsRequest)

			By("run precheck while the candidate pod still exists")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, key)).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("delete the candidate Pod before the lifecycle action is invoked")
			candidatePod := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: testCtx.DefaultNamespace, Name: candidateName}, candidatePod)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, candidatePod)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Namespace: testCtx.DefaultNamespace, Name: candidateName}, &corev1.Pod{})
				return apierrors.IsNotFound(err)
			}).Should(BeTrue())

			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).Times(0)
			})

			By("do switchover action and expect terminal failure without calling the addon")
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, key)).Should(Equal(opsv1alpha1.OpsFailedPhase))
		})

		It("fails switchover OpsRequest when candidate instance does not exist", func() {
			By("create switchover opsRequest with a missing candidate")
			ops := testops.NewOpsRequestObj("ops-switchover-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace,
				clusterObj.Name, opsv1alpha1.SwitchoverType)
			instanceName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, defaultCompName, 1)
			candidateName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, defaultCompName, 99)
			ops.Spec.SwitchoverList = []opsv1alpha1.Switchover{
				{
					ComponentName: defaultCompName,
					InstanceName:  instanceName,
					CandidateName: candidateName,
				},
			}
			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase

			By("mock switchover OpsRequest phase is Creating")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("do switchover action and expect terminal failure")
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsFailedPhase))
		})

		It("fails switchover OpsRequest when candidate only has retained PVC", func() {
			By("create a retained candidate PVC without a candidate Pod")
			ops := testops.NewOpsRequestObj("ops-switchover-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace,
				clusterObj.Name, opsv1alpha1.SwitchoverType)
			instanceName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, defaultCompName, 1)
			candidateName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, defaultCompName, 99)
			testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, "data-"+candidateName,
				clusterObj.Name, defaultCompName, testapps.DataVolumeName).
				SetStorage("1Gi").
				Create(&testCtx)
			ops.Spec.SwitchoverList = []opsv1alpha1.Switchover{
				{
					ComponentName: defaultCompName,
					InstanceName:  instanceName,
					CandidateName: candidateName,
				},
			}
			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase

			By("mock switchover OpsRequest phase is Creating")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("do switchover action and expect terminal failure")
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsFailedPhase))
		})

		It("accepts a real candidate pod with a temporarily empty role label in precheck", func() {
			By("create switchover opsRequest with a roleless real candidate pod")
			ops := testops.NewOpsRequestObj("ops-switchover-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace,
				clusterObj.Name, opsv1alpha1.SwitchoverType)
			instanceName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, defaultCompName, 1)
			candidateName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, defaultCompName, 0)
			candidatePod := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: testCtx.DefaultNamespace, Name: candidateName}, candidatePod)).Should(Succeed())
			delete(candidatePod.Labels, constant.RoleLabelKey)
			Expect(k8sClient.Update(ctx, candidatePod)).Should(Succeed())
			ops.Spec.SwitchoverList = []opsv1alpha1.Switchover{
				{
					ComponentName: defaultCompName,
					InstanceName:  instanceName,
					CandidateName: candidateName,
				},
			}
			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase

			By("mock switchover OpsRequest phase is Creating")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))
		})

		It("keeps switchover OpsRequest creating when source instance temporarily has no role label", func() {
			By("create switchover opsRequest with a roleless source instance")
			ops := testops.NewOpsRequestObj("ops-switchover-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace,
				clusterObj.Name, opsv1alpha1.SwitchoverType)
			instanceName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, defaultCompName, 1)
			candidateName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, defaultCompName, 0)
			sourcePod := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: testCtx.DefaultNamespace, Name: instanceName}, sourcePod)).Should(Succeed())
			delete(sourcePod.Labels, constant.RoleLabelKey)
			Expect(k8sClient.Update(ctx, sourcePod)).Should(Succeed())
			ops.Spec.SwitchoverList = []opsv1alpha1.Switchover{
				{
					ComponentName: defaultCompName,
					InstanceName:  instanceName,
					CandidateName: candidateName,
				},
			}
			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
			key := client.ObjectKeyFromObject(opsRes.OpsRequest)

			By("move the OpsRequest to Creating")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, key)).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("run switchover precheck and expect waiting instead of terminal failure")
			result, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(result).ShouldNot(BeNil())
			Expect(result.RequeueAfter).Should(BeNumerically(">", 0))
			Consistently(testops.GetOpsRequestPhase(&testCtx, key)).Should(Equal(opsv1alpha1.OpsCreatingPhase))
		})

		startProcessingSwitchoverWithCandidate := func() (client.ObjectKey, string) {
			By("create a valid switchover OpsRequest with a candidate")
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
			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase
			key := client.ObjectKeyFromObject(opsRes.OpsRequest)

			By("run precheck and start the switchover action")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, key)).Should(Equal(opsv1alpha1.OpsCreatingPhase))
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(ctx context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					Expect(req.Parameters["KB_SWITCHOVER_CURRENT_NAME"]).Should(Equal(instanceName))
					Expect(req.Parameters["KB_SWITCHOVER_CANDIDATE_NAME"]).Should(Equal(candidateName))
					return kbagentproto.ActionResponse{Message: "mock success"}, nil
				})
			})
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(func(g Gomega) {
				fetched := &opsv1alpha1.OpsRequest{}
				g.Expect(k8sClient.Get(ctx, key, fetched)).Should(Succeed())
				progressDetail := findStatusProgressDetail(fetched.Status.Components[defaultCompName].ProgressDetails, getProgressObjectKey(KBSwitchoverKey, defaultCompName))
				g.Expect(progressDetail).ShouldNot(BeNil())
				g.Expect(progressDetail.Status).Should(Equal(opsv1alpha1.ProcessingProgressStatus))
			}).Should(Succeed())
			return key, candidateName
		}

		expectProcessingCandidateFailure := func(key client.ObjectKey, expectedMessage string) {
			By("reconcile Processing status and expect terminal failure")
			_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(func(g Gomega) {
				fetched := &opsv1alpha1.OpsRequest{}
				g.Expect(k8sClient.Get(ctx, key, fetched)).Should(Succeed())
				g.Expect(fetched.Status.Phase).Should(Equal(opsv1alpha1.OpsFailedPhase))
				progressDetail := findStatusProgressDetail(fetched.Status.Components[defaultCompName].ProgressDetails, getProgressObjectKey(KBSwitchoverKey, defaultCompName))
				g.Expect(progressDetail).ShouldNot(BeNil())
				g.Expect(progressDetail.Status).Should(Equal(opsv1alpha1.FailedProgressStatus))
				g.Expect(progressDetail.Message).Should(ContainSubstring(expectedMessage))
			}).Should(Succeed())
		}

		expectProcessingCandidateWaiting := func(key client.ObjectKey, expectedMessages ...string) {
			By("reconcile Processing status and expect it to keep waiting")
			_, err := GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(func(g Gomega) {
				fetched := &opsv1alpha1.OpsRequest{}
				g.Expect(k8sClient.Get(ctx, key, fetched)).Should(Succeed())
				g.Expect(fetched.Status.Phase).ShouldNot(Equal(opsv1alpha1.OpsFailedPhase))
				progressDetail := findStatusProgressDetail(fetched.Status.Components[defaultCompName].ProgressDetails, getProgressObjectKey(KBSwitchoverKey, defaultCompName))
				g.Expect(progressDetail).ShouldNot(BeNil())
				g.Expect(progressDetail.Status).Should(Equal(opsv1alpha1.ProcessingProgressStatus))
				for _, expectedMessage := range expectedMessages {
					g.Expect(progressDetail.Message).Should(ContainSubstring(expectedMessage))
				}
			}).Should(Succeed())
		}

		waitForCandidatePodGone := func(candidateName string) {
			candidatePod := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: testCtx.DefaultNamespace, Name: candidateName}, candidatePod)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, candidatePod)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Namespace: testCtx.DefaultNamespace, Name: candidateName}, &corev1.Pod{})
				return apierrors.IsNotFound(err)
			}).Should(BeTrue())
		}

		It("fails Processing switchover when candidate instance disappears", func() {
			key, candidateName := startProcessingSwitchoverWithCandidate()
			By("delete the candidate Pod after the switchover action starts")
			waitForCandidatePodGone(candidateName)
			expectProcessingCandidateFailure(key, fmt.Sprintf(`candidate instance "%s" not found`, candidateName))
		})

		It("fails Processing switchover when candidate only has retained PVC", func() {
			key, candidateName := startProcessingSwitchoverWithCandidate()
			By("leave only a retained candidate PVC after the switchover action starts")
			waitForCandidatePodGone(candidateName)
			testapps.NewPersistentVolumeClaimFactory(testCtx.DefaultNamespace, "data-"+candidateName,
				clusterObj.Name, defaultCompName, testapps.DataVolumeName).
				SetStorage("1Gi").
				Create(&testCtx)
			expectProcessingCandidateFailure(key, fmt.Sprintf(`candidate instance "%s" not found`, candidateName))
		})

		It("keeps Processing switchover when candidate temporarily loses role label", func() {
			key, candidateName := startProcessingSwitchoverWithCandidate()
			By("remove the candidate role label after the switchover action starts")
			candidatePod := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: testCtx.DefaultNamespace, Name: candidateName}, candidatePod)).Should(Succeed())
			delete(candidatePod.Labels, constant.RoleLabelKey)
			Expect(k8sClient.Update(ctx, candidatePod)).Should(Succeed())
			expectProcessingCandidateWaiting(key,
				fmt.Sprintf("waiting for candidate pod %s role change", candidateName),
				`current role ""`,
				`expected role "`)
		})

		It("Test switchover OpsRequest with sharding component name", func() {
			const (
				shardingName       = "shard"
				shardingTemplate   = "redis"
				shardComponentName = "shard-abc"
			)

			By("creating a sharding cluster whose template name differs from the sharding name")
			shardingCluster := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
				WithRandomName().
				AddSharding(shardingName, "", compDefObj.Name).
				SetShardingReplicas(2).
				Create(&testCtx).GetObject()
			Expect(testapps.ChangeObj(&testCtx, shardingCluster, func(cluster *appsv1.Cluster) {
				cluster.Spec.Shardings[0].Template.Name = shardingTemplate
			})).Should(Succeed())

			By("creating the concrete shard component")
			testapps.NewComponentFactory(testCtx.DefaultNamespace, shardingCluster.Name+"-"+shardComponentName, compDefObj.Name).
				AddAppManagedByLabel().
				AddAppInstanceLabel(shardingCluster.Name).
				AddAppComponentLabel(shardComponentName).
				AddLabels(constant.KBAppShardingNameLabelKey, shardingName).
				AddAnnotations(constant.KBAppClusterUIDKey, string(shardingCluster.UID)).
				SetReplicas(2).
				Create(&testCtx).
				GetObject()

			By("creating an instanceset and pods for the concrete shard")
			container := corev1.Container{
				Name:            "mock-container-name",
				Image:           testapps.ApeCloudMySQLImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
			}
			its := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				shardingCluster.Name+"-"+shardComponentName, shardingCluster.Name, shardComponentName).
				AddFinalizers([]string{constant.DBClusterFinalizerName}).
				AddContainer(container).
				AddAppInstanceLabel(shardingCluster.Name).
				AddAppComponentLabel(shardComponentName).
				AddLabels(constant.KBAppShardingNameLabelKey, shardingName).
				AddAppManagedByLabel().
				SetReplicas(2).
				Create(&testCtx).GetObject()

			for i := int32(0); i < *its.Spec.Replicas; i++ {
				_ = testapps.NewPodFactory(testCtx.DefaultNamespace, fmt.Sprintf("%s-%d", its.Name, i)).
					AddContainer(container).
					AddLabelsInMap(its.Labels).
					AddRoleLabel(defaultRole(i)).
					Create(&testCtx).GetObject()
			}

			By("mocking the sharding cluster status as Running")
			Expect(testapps.ChangeObjStatus(&testCtx, shardingCluster, func() {
				shardingCluster.Status.Phase = appsv1.RunningClusterPhase
				shardingCluster.Status.Components = map[string]appsv1.ClusterComponentStatus{
					shardingName: {
						Phase: appsv1.RunningComponentPhase,
					},
				}
				shardingCluster.Status.Shardings = map[string]appsv1.ClusterShardingStatus{
					shardingName: {
						Phase: appsv1.RunningComponentPhase,
					},
				}
			})).Should(Succeed())

			opsRes.Cluster = shardingCluster
			ops := testops.NewOpsRequestObj("ops-switchover-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace,
				shardingCluster.Name, opsv1alpha1.SwitchoverType)
			instanceName := fmt.Sprintf("%s-%d", its.Name, 1)
			ops.Spec.SwitchoverList = []opsv1alpha1.Switchover{
				{
					ComponentName: shardingName,
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
			Expect(opsRes.OpsRequest.Status.Components).Should(HaveKey(shardingName))
			Expect(opsRes.OpsRequest.Status.Components).ShouldNot(HaveKey(shardingTemplate))

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

		It("fails sharding switchover OpsRequest when source instance does not exist", func() {
			const (
				shardingName = "shard"
			)

			By("creating a sharding cluster")
			shardingCluster := testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
				WithRandomName().
				AddSharding(shardingName, "", compDefObj.Name).
				SetShardingReplicas(2).
				Create(&testCtx).GetObject()

			By("mocking the sharding cluster status as Running")
			Expect(testapps.ChangeObjStatus(&testCtx, shardingCluster, func() {
				shardingCluster.Status.Phase = appsv1.RunningClusterPhase
				shardingCluster.Status.Shardings = map[string]appsv1.ClusterShardingStatus{
					shardingName: {
						Phase: appsv1.RunningComponentPhase,
					},
				}
			})).Should(Succeed())

			opsRes.Cluster = shardingCluster
			ops := testops.NewOpsRequestObj("ops-switchover-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace,
				shardingCluster.Name, opsv1alpha1.SwitchoverType)
			instanceName := fmt.Sprintf("%s-%s-%d", shardingCluster.Name, shardingName, 99)
			ops.Spec.SwitchoverList = []opsv1alpha1.Switchover{
				{
					ComponentName: shardingName,
					InstanceName:  instanceName,
				},
			}
			opsRes.OpsRequest = testops.CreateOpsRequest(ctx, testCtx, ops)
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase

			By("mock switchover OpsRequest phase is Creating")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			By("do switchover action and expect terminal failure")
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testops.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsFailedPhase))
		})
	})
})
