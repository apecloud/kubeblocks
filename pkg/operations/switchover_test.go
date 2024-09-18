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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("", func() {
	var (
		randomStr   = testCtx.GetRandomStr()
		compDefName = "test-compdef-" + randomStr
		clusterName = "test-cluster-" + randomStr
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

	patchK8sJobStatus := func(jobStatus batchv1.JobConditionType, key types.NamespacedName) {
		Eventually(testapps.GetAndChangeObjStatus(&testCtx, key, func(job *batchv1.Job) {
			found := false
			for _, cond := range job.Status.Conditions {
				if cond.Type == jobStatus {
					found = true
				}
			}
			if !found {
				jobCondition := batchv1.JobCondition{Type: jobStatus}
				job.Status.Conditions = append(job.Status.Conditions, jobCondition)
			}
		})).Should(Succeed())
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
	}

	BeforeEach(cleanEnv)

	AfterEach(cleanEnv)

	Context("Test OpsRequest", func() {
		BeforeEach(func() {
			By("Create a componentDefinition obj.")
			compDefObj = testapps.NewComponentDefinitionFactory(compDefName).
				SetDefaultSpec().
				Create(&testCtx).
				GetObject()
		})

		// TODO(v1.0): workload and switchover have been removed from CD/CV.
		PIt("Test switchover OpsRequest", func() {
			reqCtx := intctrlutil.RequestCtx{
				Ctx:      testCtx.Ctx,
				Recorder: k8sManager.GetEventRecorderFor("opsrequest-controller"),
			}
			By("Creating a cluster with consensus .")
			clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
				WithRandomName().
				AddComponent(defaultCompName, compDefObj.GetName()).
				SetReplicas(2).
				Create(&testCtx).GetObject()

			By("Creating a statefulSet.")
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
			var (
				leaderPod   *corev1.Pod
				followerPod *corev1.Pod
			)
			for i := int32(0); i < *its.Spec.Replicas; i++ {
				pod := testapps.NewPodFactory(testCtx.DefaultNamespace, fmt.Sprintf("%s-%d", its.Name, i)).
					AddContainer(container).
					AddLabelsInMap(its.Labels).
					AddRoleLabel(defaultRole(i)).
					Create(&testCtx).GetObject()
				if pod.Labels[constant.RoleLabelKey] == constant.Leader {
					leaderPod = pod
				} else {
					followerPod = pod
				}
			}

			opsRes := &OpsResource{
				Cluster:  clusterObj,
				Recorder: k8sManager.GetEventRecorderFor("opsrequest-controller"),
			}
			By("mock cluster is Running and the status operations")
			Expect(testapps.ChangeObjStatus(&testCtx, clusterObj, func() {
				clusterObj.Status.Phase = appsv1.RunningClusterPhase
				clusterObj.Status.Components = map[string]appsv1.ClusterComponentStatus{
					defaultCompName: {
						Phase: appsv1.RunningClusterCompPhase,
					},
				}
			})).Should(Succeed())
			opsRes.Cluster = clusterObj

			By("create switchover opsRequest")
			ops := testapps.NewOpsRequestObj("ops-switchover-"+randomStr, testCtx.DefaultNamespace,
				clusterObj.Name, opsv1alpha1.SwitchoverType)
			ops.Spec.SwitchoverList = []opsv1alpha1.Switchover{
				{
					ComponentOps: opsv1alpha1.ComponentOps{ComponentName: defaultCompName},
					InstanceName: fmt.Sprintf("%s-%s-%d", clusterObj.Name, defaultCompName, 1),
				},
			}
			opsRes.OpsRequest = testapps.CreateOpsRequest(ctx, testCtx, ops)
			// set ops phase to Pending
			opsRes.OpsRequest.Status.Phase = opsv1alpha1.OpsPendingPhase

			By("mock switchover OpsRequest phase is Creating")
			_, err := GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(testapps.GetOpsRequestPhase(&testCtx, client.ObjectKeyFromObject(opsRes.OpsRequest))).Should(Equal(opsv1alpha1.OpsCreatingPhase))

			// do switchover action
			By("do switchover action")
			_, err = GetOpsManager().Do(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())

			By("do reconcile switchoverAction failed because switchover job status failed")
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("requeue to waiting for job"))

			By("mock job status to success.")
			jobName := fmt.Sprintf("%s-%s-%s-%d", KBSwitchoverJobNamePrefix, opsRes.Cluster.Name, defaultCompName, opsRes.Cluster.Generation)
			key := types.NamespacedName{
				Name:      jobName,
				Namespace: clusterObj.Namespace,
			}
			patchK8sJobStatus(batchv1.JobComplete, key)

			By("do reconcile switchoverAction failed because pod role label is not consistency")
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("requeue to waiting for pod role label consistency"))

			By("mock pod role label changed.")
			Expect(testapps.ChangeObj(&testCtx, leaderPod, func(pod *corev1.Pod) {
				pod.Labels[constant.RoleLabelKey] = constant.Follower
			})).Should(Succeed())
			Expect(testapps.ChangeObj(&testCtx, followerPod, func(pod *corev1.Pod) {
				pod.Labels[constant.RoleLabelKey] = constant.Leader
			})).Should(Succeed())
			_, err = GetOpsManager().Reconcile(reqCtx, k8sClient, opsRes)
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
})
