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

package dbaas

import (
	"context"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/dbaas/components/util"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	testdbaas "github.com/apecloud/kubeblocks/internal/testutil/dbaas"
)

var _ = Describe("Cluster Controller with Consensus Component", func() {
	const clusterDefName = "test-clusterdef"
	const clusterVersionName = "test-clusterversion"
	const clusterNamePrefix = "test-cluster"

	const statefulCompName = "mysql-test"
	const statefulCompType = "replicasets"

	const dataVolumeName = "data"

	const replicas = 3
	const leader = "leader"
	const follower = "follower"

	ctx := context.Background()

	cleanEnv := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete cluster(and all dependent sub-resources), clusterversion and clusterdef
		testdbaas.ClearClusterResources(&testCtx)

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testdbaas.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml)
	}

	var (
		clusterDefObj     *dbaasv1alpha1.ClusterDefinition
		clusterVersionObj *dbaasv1alpha1.ClusterVersion
		clusterObj        *dbaasv1alpha1.Cluster
		clusterKey        types.NamespacedName
	)

	BeforeEach(func() {
		cleanEnv()

		By("Create a clusterDef obj")
		clusterDefObj = testdbaas.NewClusterDefFactory(&testCtx, clusterDefName, testdbaas.MySQLType).
			AddComponent(testdbaas.ConsensusMySQL, statefulCompType).
			Create().GetClusterDef()

		By("Create a clusterVersion obj")
		clusterVersionObj = testdbaas.NewClusterVersionFactory(&testCtx, clusterVersionName, clusterDefObj.GetName()).
			AddComponent(statefulCompType).AddContainerShort("mysql", testdbaas.ApeCloudMySQLImage).
			Create().GetClusterVersion()

		By("Mock a cluster obj")
		pvcSpec := &corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		}
		clusterObj = testdbaas.NewClusterFactory(&testCtx, clusterNamePrefix,
			clusterDefObj.Name, clusterVersionObj.Name).WithRandomName().
			AddComponent(statefulCompName, statefulCompType).
			SetReplicas(replicas).AddVolumeClaim(dataVolumeName, pvcSpec).
			Create().GetCluster()
		clusterKey = client.ObjectKeyFromObject(clusterObj)
	})

	AfterEach(func() {
		cleanEnv()
	})

	listAndCheckStatefulSet := func(key types.NamespacedName) *appsv1.StatefulSetList {
		By("Check statefulset workload has been created")
		stsList := &appsv1.StatefulSetList{}
		Eventually(func() bool {
			Expect(k8sClient.List(ctx, stsList, client.MatchingLabels{
				intctrlutil.AppInstanceLabelKey: key.Name,
			}, client.InNamespace(key.Namespace))).Should(Succeed())
			return len(stsList.Items) > 0
		}).Should(BeTrue())
		return stsList
	}

	mockPodsForConsensusTest := func(cluster *dbaasv1alpha1.Cluster, number int) []corev1.Pod {
		componentName := cluster.Spec.Components[0].Name
		clusterName := cluster.Name
		stsName := cluster.Name + "-" + componentName
		pods := make([]corev1.Pod, 0)
		for i := 0; i < number; i++ {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      stsName + "-" + strconv.Itoa(i),
					Namespace: testCtx.DefaultNamespace,
					Labels: map[string]string{
						intctrlutil.AppInstanceLabelKey:  clusterName,
						intctrlutil.AppComponentLabelKey: componentName,
						"controller-revision-hash":       "mock-version",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "mock-container",
						Image: "mock-container",
					}},
				},
			}
			pods = append(pods, *pod)
		}
		return pods
	}

	mockRoleChangedEvent := func(key types.NamespacedName, sts *appsv1.StatefulSet) []corev1.Event {
		pods, err := util.GetPodListByStatefulSet(ctx, k8sClient, sts)
		Expect(err).To(Succeed())

		events := make([]corev1.Event, 0)
		for _, pod := range pods {
			event := corev1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pod.Name + "-event",
					Namespace: testCtx.DefaultNamespace,
				},
				Reason:  "Unhealthy",
				Message: `Readiness probe failed: {"event":"roleUnchanged","originalRole":"Leader","role":"Follower"}`,
				InvolvedObject: corev1.ObjectReference{
					Name:      pod.Name,
					Namespace: testCtx.DefaultNamespace,
					UID:       pod.UID,
					FieldPath: "spec.containers{kb-rolechangedcheck}",
				},
			}
			events = append(events, event)
		}
		events[0].Message = `Readiness probe failed: {"event":"roleUnchanged","originalRole":"Leader","role":"Leader"}`
		return events
	}

	getStsPodsName := func(sts *appsv1.StatefulSet) []string {
		pods, err := util.GetPodListByStatefulSet(ctx, k8sClient, sts)
		Expect(err).To(Succeed())

		names := make([]string, 0)
		for _, pod := range pods {
			names = append(names, pod.Name)
		}
		return names
	}

	Context("When creating cluster with componentType = Consensus", func() {

		It("Should success with: "+
			"1 pod with 'leader' role label set, "+
			"2 pods with 'follower' role label set,"+
			"1 service routes to 'leader' pod", func() {
			By("Waiting for cluster creation")
			Eventually(testdbaas.GetClusterObservedGeneration(&testCtx, clusterKey)).Should(BeEquivalentTo(1))
			stsList := listAndCheckStatefulSet(clusterKey)
			sts := &stsList.Items[0]

			By("Creating mock pods in StatefulSet")
			pods := mockPodsForConsensusTest(clusterObj, replicas)
			for _, pod := range pods {
				Expect(testCtx.CreateObj(testCtx.Ctx, &pod)).Should(Succeed())
				// mock the status to pass the isReady(pod) check in consensus_set
				pod.Status.Conditions = []corev1.PodCondition{{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				}}
				Expect(k8sClient.Status().Update(ctx, &pod)).Should(Succeed())
			}

			By("Creating mock role changed events")
			// pod.Labels[intctrlutil.RoleLabelKey] will be filled with the role
			events := mockRoleChangedEvent(clusterKey, sts)
			for _, event := range events {
				Expect(testCtx.CreateObj(ctx, &event)).Should(Succeed())
			}

			By("Checking pods' role are changed accordingly")
			Eventually(func(g Gomega) {
				pods, err := util.GetPodListByStatefulSet(ctx, k8sClient, sts)
				g.Expect(err).To(Succeed())
				// should have 3 pods
				g.Expect(len(pods)).To(Equal(3))
				// 1 leader
				// 2 followers
				leaderCount, followerCount := 0, 0
				for _, pod := range pods {
					switch pod.Labels[intctrlutil.RoleLabelKey] {
					case leader:
						leaderCount++
					case follower:
						followerCount++
					}
				}
				g.Expect(leaderCount).Should(Equal(1))
				g.Expect(followerCount).Should(Equal(2))
			}).Should(Succeed())

			By("Updating StatefulSet's status")
			sts.Status.UpdateRevision = "mock-version"
			sts.Status.Replicas = int32(replicas)
			sts.Status.AvailableReplicas = int32(replicas)
			sts.Status.CurrentReplicas = int32(replicas)
			sts.Status.ReadyReplicas = int32(replicas)
			sts.Status.ObservedGeneration = sts.Generation
			Expect(k8sClient.Status().Update(ctx, sts)).Should(Succeed())

			By("Checking pods' role are updated in cluster status")
			Eventually(func(g Gomega) {
				fetched := &dbaasv1alpha1.Cluster{}
				g.Expect(k8sClient.Get(ctx, clusterKey, fetched)).To(Succeed())
				compName := fetched.Spec.Components[0].Name
				g.Expect(fetched.Status.Components != nil).To(BeTrue())
				g.Expect(fetched.Status.Components).To(HaveKey(compName))
				consensusStatus := fetched.Status.Components[compName].ConsensusSetStatus
				g.Expect(consensusStatus != nil).To(BeTrue())
				g.Expect(consensusStatus.Leader.Pod).To(BeElementOf(getStsPodsName(sts)))
				g.Expect(len(consensusStatus.Followers) == 2).To(BeTrue())
				g.Expect(consensusStatus.Followers[0].Pod).To(BeElementOf(getStsPodsName(sts)))
				g.Expect(consensusStatus.Followers[1].Pod).To(BeElementOf(getStsPodsName(sts)))
			}).Should(Succeed())

			By("Waiting the cluster be running")
			Eventually(testdbaas.GetClusterPhase(&testCtx, clusterKey)).Should(Equal(dbaasv1alpha1.RunningPhase))
		})
	})
})
