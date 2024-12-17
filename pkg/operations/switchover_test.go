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

package operations

import (
	"context"
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
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
			_ = testapps.NewComponentFactory(testCtx.DefaultNamespace, clusterObj.Name+"-"+defaultCompName, compDefObj.Name).
				AddAppManagedByLabel().
				AddAppInstanceLabel(clusterObj.Name).
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

		It("Test switchover OpsRequest", func() {
			By("create switchover opsRequest")
			ops := testops.NewOpsRequestObj("ops-switchover-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace,
				clusterObj.Name, opsv1alpha1.SwitchoverType)
			instanceName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, defaultCompName, 1)
			ops.Spec.SwitchoverList = []opsv1alpha1.Switchover{
				{
					ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
					InstanceName: instanceName,
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

		It("Test switchover OpsRequest with candidate", func() {
			By("create switchover opsRequest")
			ops := testops.NewOpsRequestObj("ops-switchover-"+testCtx.GetRandomStr(), testCtx.DefaultNamespace,
				clusterObj.Name, opsv1alpha1.SwitchoverType)
			instanceName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, defaultCompName, 1)
			candidateName := fmt.Sprintf("%s-%s-%d", clusterObj.Name, defaultCompName, 0)
			ops.Spec.SwitchoverList = []opsv1alpha1.Switchover{
				{
					ComponentOps:  opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
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
		})
	})
})
