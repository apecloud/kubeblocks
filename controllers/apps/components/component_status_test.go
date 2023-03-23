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

package components

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateless"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("ComponentStatusSynchronizer", func() {
	const (
		compName = "comp"
		compType = "comp"
	)

	var (
		clusterDefName     = "test-clusterdef"
		clusterVersionName = "test-clusterversion"
		clusterName        = "test-cluster"
		controllerRevision = fmt.Sprintf("%s-%s-%s", clusterName, compName, "6fdd48d9cd1")
	)

	cleanAll := func() {
		// must wait until resources deleted and no longer exist before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// clear rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}

		// non-namespaced resources
		testapps.ClearResources(&testCtx, intctrlutil.ClusterDefinitionSignature, inNS, ml)

		// namespaced resources
		testapps.ClearResources(&testCtx, intctrlutil.ClusterSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.StatefulSetSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.DeploymentSignature, inNS, ml)
		testapps.ClearResources(&testCtx, intctrlutil.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	Context("with stateless component", func() {
		var (
			clusterDef *appsv1alpha1.ClusterDefinition
			cluster    *appsv1alpha1.Cluster
			component  types.Component
			err        error
		)

		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.StatelessNginxComponent, compType).
				GetObject()

			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(compName, compType).
				SetReplicas(1).
				GetObject()

			component, err = NewComponentByType(testCtx.Cli, cluster,
				cluster.GetComponentByName(compName), *clusterDef.GetComponentDefByName(compName))
			Expect(err).Should(Succeed())
			Expect(component).ShouldNot(BeNil())
		})

		It("should not change component if no deployment or pod exists", func() {
			synchronizer, err := newClusterStatusSynchronizer(testCtx.Ctx, testCtx.Cli, cluster,
				cluster.GetComponentByName(compName), component)
			Expect(err).Should(Succeed())
			Expect(synchronizer).ShouldNot(BeNil())

			hasFailedAndTimeoutPod, hasFailedPod := synchronizer.hasFailedAndTimedOutPod()
			Expect(hasFailedAndTimeoutPod).Should(BeFalse())
			Expect(hasFailedPod).Should(BeFalse())

			podsAreReady := false
			Expect(synchronizer.updateComponentsPhase(ctx, false, &podsAreReady, hasFailedAndTimeoutPod)).Should(Succeed())
			Expect(cluster.Status.Components[compName].Phase).Should(BeEmpty())
		})

		Context("and with mocked deployment & pod", func() {
			var (
				deployment *appsv1.Deployment
				pod        *corev1.Pod
			)

			BeforeEach(func() {
				deploymentName := clusterName + "-" + compName
				deployment = testapps.NewDeploymentFactory(testCtx.DefaultNamespace, deploymentName, clusterName, compName).
					SetMinReadySeconds(int32(10)).
					SetReplicas(int32(1)).
					AddContainer(corev1.Container{Name: testapps.DefaultNginxContainerName, Image: testapps.NginxImage}).
					Create(&testCtx).GetObject()

				podName := fmt.Sprintf("%s-%s-%s", clusterName, compName, testCtx.GetRandomStr())
				pod = testapps.NewPodFactory(testCtx.DefaultNamespace, podName).
					SetOwnerReferences("apps/v1", constant.DeploymentKind, deployment).
					AddAppInstanceLabel(clusterName).
					AddAppComponentLabel(compName).
					AddAppManangedByLabel().
					AddContainer(corev1.Container{Name: testapps.DefaultNginxContainerName, Image: testapps.NginxImage}).
					Create(&testCtx).GetObject()
			})

			It("should set component status to failed if container is not ready and have error message", func() {
				Expect(mockContainerError(pod)).Should(Succeed())

				synchronizer, err := newClusterStatusSynchronizer(testCtx.Ctx, testCtx.Cli, cluster,
					cluster.GetComponentByName(compName), component)
				Expect(err).Should(Succeed())
				Expect(synchronizer).ShouldNot(BeNil())

				hasFailedAndTimeoutPod, hasFailedPod := synchronizer.hasFailedAndTimedOutPod()
				Expect(hasFailedAndTimeoutPod).Should(BeTrue())
				Expect(hasFailedPod).Should(BeTrue())

				isPodReady, err := component.PodsReady(ctx, deployment)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(isPodReady).Should(BeFalse())
				isRunning, err := component.IsRunning(ctx, deployment)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(isRunning).Should(BeFalse())

				Expect(synchronizer.updateComponentsPhase(ctx, isRunning, &isPodReady, hasFailedAndTimeoutPod)).Should(Succeed())
				Expect(cluster.Status.Components[compName].Phase).Should(Equal(appsv1alpha1.FailedClusterCompPhase))
			})

			It("should set component status to running if container is ready", func() {
				Expect(testapps.ChangeObjStatus(&testCtx, deployment, func() {
					testk8s.MockDeploymentReady(deployment, stateless.NewRSAvailableReason, deployment.Name)
				})).Should(Succeed())

				synchronizer, err := newClusterStatusSynchronizer(testCtx.Ctx, testCtx.Cli, cluster,
					cluster.GetComponentByName(compName), component)
				Expect(err).Should(Succeed())
				Expect(synchronizer).ShouldNot(BeNil())

				hasFailedAndTimeoutPod, hasFailedPod := synchronizer.hasFailedAndTimedOutPod()
				Expect(hasFailedAndTimeoutPod).Should(BeFalse())
				Expect(hasFailedPod).Should(BeFalse())

				isPodReady, err := component.PodsReady(ctx, deployment)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(isPodReady).Should(BeTrue())
				isRunning, err := component.IsRunning(ctx, deployment)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(isRunning).Should(BeTrue())

				Expect(synchronizer.updateComponentsPhase(ctx, isRunning, &isPodReady, hasFailedAndTimeoutPod)).Should(Succeed())
				Expect(cluster.Status.Components[compName].Phase).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
			})
		})
	})

	Context("with statefulset component", func() {
		var (
			clusterDef *appsv1alpha1.ClusterDefinition
			cluster    *appsv1alpha1.Cluster
			component  types.Component
			err        error
		)

		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.StatefulMySQLComponent, compType).
				GetObject()

			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(compName, compType).
				SetReplicas(int32(3)).
				GetObject()

			component, err = NewComponentByType(testCtx.Cli, cluster,
				cluster.GetComponentByName(compName), *clusterDef.GetComponentDefByName(compName))
			Expect(err).Should(Succeed())
			Expect(component).ShouldNot(BeNil())
		})

		It("should not change component if no statefulset or pod exists", func() {
			synchronizer, err := newClusterStatusSynchronizer(testCtx.Ctx, testCtx.Cli, cluster,
				cluster.GetComponentByName(compName), component)
			Expect(err).Should(Succeed())
			Expect(synchronizer).ShouldNot(BeNil())

			hasFailedAndTimeoutPod, hasFailedPod := synchronizer.hasFailedAndTimedOutPod()
			Expect(hasFailedAndTimeoutPod).Should(BeFalse())
			Expect(hasFailedPod).Should(BeFalse())

			podsAreReady := false
			Expect(synchronizer.updateComponentsPhase(ctx, false, &podsAreReady, hasFailedAndTimeoutPod)).Should(Succeed())
			Expect(cluster.Status.Components[compName].Phase).Should(BeEmpty())
		})

		Context("and with mocked statefulset & pod", func() {
			var (
				statefulset *appsv1.StatefulSet
				pods        []*corev1.Pod
			)

			BeforeEach(func() {
				stsName := clusterName + "-" + compName
				statefulset = testapps.NewStatefulSetFactory(testCtx.DefaultNamespace, stsName, clusterName, compName).
					SetReplicas(int32(3)).
					AddContainer(corev1.Container{Name: testapps.DefaultMySQLContainerName, Image: testapps.ApeCloudMySQLImage}).
					Create(&testCtx).GetObject()
				// init statefulset status
				testk8s.InitStatefulSetStatus(testCtx, statefulset, controllerRevision)
				for i := 0; i < 3; i++ {
					podName := fmt.Sprintf("%s-%s-%d", clusterName, compName, i)
					pod := testapps.NewPodFactory(testCtx.DefaultNamespace, podName).
						SetOwnerReferences("apps/v1", constant.StatefulSetKind, statefulset).
						AddAppInstanceLabel(clusterName).
						AddAppComponentLabel(compName).
						AddAppManangedByLabel().
						AddControllerRevisionHashLabel(controllerRevision).
						AddContainer(corev1.Container{Name: testapps.DefaultMySQLContainerName, Image: testapps.ApeCloudMySQLImage}).
						Create(&testCtx).GetObject()
					Expect(testapps.ChangeObjStatus(&testCtx, pod, func() {
						pod.Status.Conditions = []corev1.PodCondition{{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						}}
					})).Should(Succeed())
					pods = append(pods, pod)
				}
			})

			It("should set component status to failed if container is not ready and have error message", func() {
				Expect(mockContainerError(pods[0])).Should(Succeed())
				Expect(mockContainerError(pods[1])).Should(Succeed())

				synchronizer, err := newClusterStatusSynchronizer(testCtx.Ctx, testCtx.Cli, cluster,
					cluster.GetComponentByName(compName), component)
				Expect(err).Should(Succeed())
				Expect(synchronizer).ShouldNot(BeNil())

				hasFailedAndTimeoutPod, hasFailedPod := synchronizer.hasFailedAndTimedOutPod()
				Expect(hasFailedAndTimeoutPod).Should(BeTrue())
				// two pod failed message
				Expect(len(cluster.Status.Components[compName].Message)).Should(Equal(2))
				Expect(hasFailedPod).Should(BeTrue())

				isPodReady, err := component.PodsReady(ctx, statefulset)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(isPodReady).Should(BeFalse())
				isRunning, err := component.IsRunning(ctx, statefulset)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(isRunning).Should(BeFalse())

				Expect(synchronizer.updateComponentsPhase(ctx, isRunning, &isPodReady, hasFailedAndTimeoutPod)).Should(Succeed())
				Expect(cluster.Status.Components[compName].Phase).Should(Equal(appsv1alpha1.FailedClusterCompPhase))
			})

			It("should set component status to running if container is ready", func() {
				Expect(testapps.ChangeObjStatus(&testCtx, statefulset, func() {
					testk8s.MockStatefulSetReady(statefulset)
				})).Should(Succeed())

				synchronizer, err := newClusterStatusSynchronizer(testCtx.Ctx, testCtx.Cli, cluster,
					cluster.GetComponentByName(compName), component)
				Expect(err).Should(Succeed())
				Expect(synchronizer).ShouldNot(BeNil())

				hasFailedAndTimeoutPod, hasFailedPod := synchronizer.hasFailedAndTimedOutPod()
				Expect(hasFailedAndTimeoutPod).Should(BeFalse())
				Expect(hasFailedPod).Should(BeFalse())

				isPodReady, err := component.PodsReady(ctx, statefulset)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(isPodReady).Should(BeTrue())
				isRunning, err := component.IsRunning(ctx, statefulset)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(isRunning).Should(BeTrue())

				Expect(synchronizer.updateComponentsPhase(ctx, isRunning, &isPodReady, hasFailedAndTimeoutPod)).Should(Succeed())
				Expect(cluster.Status.Components[compName].Phase).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
			})
		})
	})

	Context("with consensusset component", func() {
		var (
			clusterDef *appsv1alpha1.ClusterDefinition
			cluster    *appsv1alpha1.Cluster
			component  types.Component
			err        error
		)

		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.ConsensusMySQLComponent, compType).
				Create(&testCtx).GetObject()

			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(compName, compType).
				SetReplicas(int32(3)).
				Create(&testCtx).GetObject()

			component, err = NewComponentByType(testCtx.Cli, cluster,
				cluster.GetComponentByName(compName), *clusterDef.GetComponentDefByName(compName))
			Expect(err).Should(Succeed())
			Expect(component).ShouldNot(BeNil())
		})

		It("should not change component if no statefulset or pod exists", func() {
			synchronizer, err := newClusterStatusSynchronizer(testCtx.Ctx, testCtx.Cli, cluster,
				cluster.GetComponentByName(compName), component)
			Expect(err).Should(Succeed())
			Expect(synchronizer).ShouldNot(BeNil())

			hasFailedAndTimeoutPod, hasFailedPod := synchronizer.hasFailedAndTimedOutPod()
			Expect(hasFailedAndTimeoutPod).Should(BeFalse())
			Expect(hasFailedPod).Should(BeFalse())

			podsAreReady := false
			Expect(synchronizer.updateComponentsPhase(ctx, false, &podsAreReady, hasFailedAndTimeoutPod)).Should(Succeed())
			Expect(cluster.Status.Components[compName].Phase).Should(BeEmpty())
		})

		Context("and with mocked statefulset & pod", func() {
			var (
				statefulset *appsv1.StatefulSet
				pods        []*corev1.Pod
			)

			BeforeEach(func() {
				stsName := clusterName + "-" + compName
				statefulset = testapps.NewStatefulSetFactory(testCtx.DefaultNamespace, stsName, clusterName, compName).
					SetReplicas(int32(3)).
					AddContainer(corev1.Container{Name: testapps.DefaultMySQLContainerName, Image: testapps.ApeCloudMySQLImage}).
					Create(&testCtx).GetObject()
				testk8s.InitStatefulSetStatus(testCtx, statefulset, controllerRevision)
				for i := 0; i < 3; i++ {
					podName := fmt.Sprintf("%s-%s-%d", clusterName, compName, i)
					pod := testapps.NewPodFactory(testCtx.DefaultNamespace, podName).
						SetOwnerReferences("apps/v1", constant.StatefulSetKind, statefulset).
						AddAppInstanceLabel(clusterName).
						AddAppComponentLabel(compName).
						AddAppManangedByLabel().
						AddControllerRevisionHashLabel(controllerRevision).
						AddContainer(corev1.Container{Name: testapps.DefaultMySQLContainerName, Image: testapps.ApeCloudMySQLImage}).
						Create(&testCtx).GetObject()
					Expect(testapps.ChangeObjStatus(&testCtx, pod, func() {
						pod.Status.Conditions = []corev1.PodCondition{{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						}}
					})).Should(Succeed())
					pods = append(pods, pod)
				}
			})

			It("should set component status to failed if container is not ready and have error message", func() {
				Expect(mockContainerError(pods[0])).Should(Succeed())

				synchronizer, err := newClusterStatusSynchronizer(testCtx.Ctx, testCtx.Cli, cluster,
					cluster.GetComponentByName(compName), component)
				Expect(err).Should(Succeed())
				Expect(synchronizer).ShouldNot(BeNil())

				hasFailedAndTimeoutPod, hasFailedPod := synchronizer.hasFailedAndTimedOutPod()
				Expect(hasFailedAndTimeoutPod).Should(BeTrue())
				Expect(hasFailedPod).Should(BeTrue())

				isPodReady, err := component.PodsReady(ctx, statefulset)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(isPodReady).Should(BeFalse())
				isRunning, err := component.IsRunning(ctx, statefulset)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(isRunning).Should(BeFalse())

				Expect(synchronizer.updateComponentsPhase(ctx, isRunning, &isPodReady, hasFailedAndTimeoutPod)).Should(Succeed())
				Expect(cluster.Status.Components[compName].Phase).Should(Equal(appsv1alpha1.FailedClusterCompPhase))
			})

			It("should set component status to running if container is ready", func() {
				Expect(testapps.ChangeObjStatus(&testCtx, statefulset, func() {
					testk8s.MockStatefulSetReady(statefulset)
				})).Should(Succeed())

				Expect(setPodRole(pods[0], "leader")).Should(Succeed())
				Expect(setPodRole(pods[1], "follower")).Should(Succeed())
				Expect(setPodRole(pods[2], "follower")).Should(Succeed())

				synchronizer, err := newClusterStatusSynchronizer(testCtx.Ctx, testCtx.Cli, cluster,
					cluster.GetComponentByName(compName), component)
				Expect(err).Should(Succeed())
				Expect(synchronizer).ShouldNot(BeNil())

				hasFailedAndTimeoutPod, hasFailedPod := synchronizer.hasFailedAndTimedOutPod()
				Expect(hasFailedAndTimeoutPod).Should(BeFalse())
				Expect(hasFailedPod).Should(BeFalse())

				isPodReady, err := component.PodsReady(ctx, statefulset)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(isPodReady).Should(BeTrue())
				isRunning, err := component.IsRunning(ctx, statefulset)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(isRunning).Should(BeTrue())

				Expect(synchronizer.updateComponentsPhase(ctx, isRunning, &isPodReady, hasFailedAndTimeoutPod)).Should(Succeed())
				Expect(cluster.Status.Components[compName].Phase).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
			})
		})
	})

	Context("with replicationset component", func() {
		var (
			clusterDef *appsv1alpha1.ClusterDefinition
			cluster    *appsv1alpha1.Cluster
			component  types.Component
			err        error
		)

		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponent(testapps.ReplicationRedisComponent, compType).
				GetObject()

			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(compName, compType).
				SetReplicas(2).
				GetObject()

			component, err = NewComponentByType(testCtx.Cli, cluster,
				cluster.GetComponentByName(compName), *clusterDef.GetComponentDefByName(compName))
			Expect(err).Should(Succeed())
			Expect(component).ShouldNot(BeNil())
		})

		It("should not change component if no deployment or pod exists", func() {
			synchronizer, err := newClusterStatusSynchronizer(testCtx.Ctx, testCtx.Cli, cluster,
				cluster.GetComponentByName(compName), component)
			Expect(err).Should(Succeed())
			Expect(synchronizer).ShouldNot(BeNil())

			hasFailedAndTimeoutPod, hasFailedPod := synchronizer.hasFailedAndTimedOutPod()
			Expect(hasFailedAndTimeoutPod).Should(BeFalse())
			Expect(hasFailedPod).Should(BeFalse())

			podsAreReady := false
			Expect(synchronizer.updateComponentsPhase(ctx, false, &podsAreReady, hasFailedAndTimeoutPod)).Should(Succeed())
			Expect(cluster.Status.Components[compName].Phase).Should(BeEmpty())
		})

		Context("and with mocked statefulset & pod", func() {
			var (
				statefulset *appsv1.StatefulSet
				pods        []*corev1.Pod
			)

			BeforeEach(func() {
				stsName := clusterName + "-" + compName
				statefulset = testapps.NewStatefulSetFactory(testCtx.DefaultNamespace, stsName, clusterName, compName).
					SetReplicas(int32(2)).
					AddContainer(corev1.Container{Name: testapps.DefaultRedisContainerName, Image: testapps.DefaultRedisImageName}).
					Create(&testCtx).GetObject()
				testk8s.InitStatefulSetStatus(testCtx, statefulset, controllerRevision)
				for i := 0; i < 2; i++ {
					podName := fmt.Sprintf("%s-%s-%d", clusterName, compName, i)
					pod := testapps.NewPodFactory(testCtx.DefaultNamespace, podName).
						SetOwnerReferences("apps/v1", constant.StatefulSetKind, statefulset).
						AddAppInstanceLabel(clusterName).
						AddAppComponentLabel(compName).
						AddAppManangedByLabel().
						AddRoleLabel("leader").
						AddControllerRevisionHashLabel(controllerRevision).
						AddContainer(corev1.Container{Name: testapps.DefaultRedisContainerName, Image: testapps.DefaultRedisImageName}).
						Create(&testCtx).GetObject()
					patch := client.MergeFrom(pod.DeepCopy())
					pod.Status.Conditions = []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					}
					Expect(testCtx.Cli.Status().Patch(testCtx.Ctx, pod, patch)).Should(Succeed())
					pods = append(pods, pod)
				}
			})

			It("should set component status to failed if container is not ready and have error message", func() {
				Expect(mockContainerError(pods[0])).Should(Succeed())

				synchronizer, err := newClusterStatusSynchronizer(testCtx.Ctx, testCtx.Cli, cluster,
					cluster.GetComponentByName(compName), component)
				Expect(err).Should(Succeed())
				Expect(synchronizer).ShouldNot(BeNil())

				hasFailedAndTimeoutPod, hasFailedPod := synchronizer.hasFailedAndTimedOutPod()
				Expect(hasFailedAndTimeoutPod).Should(BeTrue())
				Expect(hasFailedPod).Should(BeTrue())

				isPodReady, err := component.PodsReady(ctx, statefulset)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(isPodReady).Should(BeFalse())
				isRunning, err := component.IsRunning(ctx, statefulset)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(isRunning).Should(BeFalse())

				Expect(synchronizer.updateComponentsPhase(ctx, isRunning, &isPodReady, hasFailedAndTimeoutPod)).Should(Succeed())
				Expect(cluster.Status.Components[compName].Phase).Should(Equal(appsv1alpha1.FailedClusterCompPhase))
			})

			It("should set component status to running if container is ready", func() {
				Expect(testapps.ChangeObjStatus(&testCtx, statefulset, func() {
					testk8s.MockStatefulSetReady(statefulset)
				})).Should(Succeed())

				synchronizer, err := newClusterStatusSynchronizer(testCtx.Ctx, testCtx.Cli, cluster,
					cluster.GetComponentByName(compName), component)
				Expect(err).Should(Succeed())
				Expect(synchronizer).ShouldNot(BeNil())

				hasFailedAndTimeoutPod, hasFailedPod := synchronizer.hasFailedAndTimedOutPod()
				Expect(hasFailedAndTimeoutPod).Should(BeFalse())
				Expect(hasFailedPod).Should(BeFalse())

				isPodReady, err := component.PodsReady(ctx, statefulset)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(isPodReady).Should(BeTrue())
				isRunning, err := component.IsRunning(ctx, statefulset)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(isRunning).Should(BeTrue())

				Expect(synchronizer.updateComponentsPhase(ctx, isRunning, &isPodReady, hasFailedAndTimeoutPod)).Should(Succeed())
				Expect(cluster.Status.Components[compName].Phase).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
			})
		})
	})
})

func mockContainerError(pod *corev1.Pod) error {
	return testapps.ChangeObjStatus(&testCtx, pod, func() {
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{
			{
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason:  "ImagePullBackOff",
						Message: "Back-off pulling image",
					},
				},
			},
		}
		pod.Status.Conditions = []corev1.PodCondition{
			{
				Type:               corev1.ContainersReady,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: metav1.NewTime(time.Now().Add(-2 * time.Minute)),
			},
		}
	})
}

func setPodRole(pod *corev1.Pod, role string) error {
	return testapps.ChangeObj(&testCtx, pod, func() {
		pod.Labels[constant.RoleLabelKey] = role
	})
}
