/*
Copyright ApeCloud Inc.

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
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("ReplicationSet Util", func() {

	var (
		randomStr           = testCtx.GetRandomStr()
		clusterName         = "cluster-replication" + randomStr
		clusterDefName      = "cluster-def-replication-" + randomStr
		clusterVersionName  = "cluster-version-replication-" + randomStr
		replicationCompName = "replication"
	)

	cleanupObjects := func() {
		err := k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterDefinition{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.ClusterVersion{}, client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &dbaasv1alpha1.Cluster{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &appsv1.StatefulSet{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(testCtx.DefaultNamespace), client.HasLabels{testCtx.TestObjLabelKey},
			client.GracePeriodSeconds(0))
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeEach(func() {
		cleanupObjects()
	})

	AfterEach(func() {
		cleanupObjects()
	})

	Context("test replicationSet util", func() {
		It("", func() {
			_, _, cluster := testdbaas.InitReplicationRedis(ctx, testCtx, clusterDefName, clusterVersionName, clusterName, replicationCompName)
			By("init cluster status")
			componentName := "rsts-comp"
			patch := client.MergeFrom(cluster.DeepCopy())
			cluster.Status.Phase = dbaasv1alpha1.RunningPhase
			cluster.Status.Components = map[string]dbaasv1alpha1.ClusterStatusComponent{
				componentName: {
					Phase: dbaasv1alpha1.RunningPhase,
					ReplicationSetStatus: &dbaasv1alpha1.ReplicationSetStatus{
						Primary: dbaasv1alpha1.ReplicationMemberStatus{
							Pod: clusterName + componentName + "-0-0",
						},
						Secondaries: []dbaasv1alpha1.ReplicationMemberStatus{
							{
								Pod: clusterName + componentName + "-1-0",
							},
							{
								Pod: clusterName + componentName + "-2-0",
							},
						},
					},
				},
			}
			Expect(k8sClient.Status().Patch(context.Background(), cluster, patch)).Should(Succeed())

			By("testing sync cluster status with add pod")
			var podList []*corev1.Pod
			set := testk8s.NewFakeStatefulSet(clusterName+componentName+"-3", 3)
			pod := testk8s.NewFakeStatefulSetPod(set, 0)
			pod.Labels = make(map[string]string, 0)
			pod.Labels[intctrlutil.RoleLabelKey] = "secondary"
			podList = append(podList, pod)
			Expect(needUpdateReplicationSetStatus(cluster.Status.Components[componentName].ReplicationSetStatus, podList)).Should(BeTrue())

			By("testing sync cluster status with remove pod")
			var podRemoveList []corev1.Pod
			set = testk8s.NewFakeStatefulSet(clusterName+componentName+"-2", 3)
			pod = testk8s.NewFakeStatefulSetPod(set, 0)
			pod.Labels = make(map[string]string, 0)
			pod.Labels[intctrlutil.RoleLabelKey] = "secondary"
			podRemoveList = append(podRemoveList, *pod)
			Expect(needRemoveReplicationSetStatus(cluster.Status.Components[componentName].ReplicationSetStatus, podRemoveList)).Should(BeTrue())
		})
	})
})
