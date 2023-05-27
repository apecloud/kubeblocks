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
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/controllers/apps/components/stateless"
	"github.com/apecloud/kubeblocks/controllers/apps/components/types"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	testk8s "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("ComponentStatusSynchronizer", func() {
	const (
		compName    = "comp"
		compDefName = "comp"
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
		testapps.ClearResources(&testCtx, generics.ClusterDefinitionSignature, inNS, ml)

		// namespaced resources
		testapps.ClearResources(&testCtx, generics.ClusterSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.StatefulSetSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.DeploymentSignature, inNS, ml)
		testapps.ClearResources(&testCtx, generics.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	Context("with stateless component", func() {
		var (
			clusterDef *appsv1alpha1.ClusterDefinition
			cluster    *appsv1alpha1.Cluster
			component  types.Component
			reqCtx     *intctrlutil.RequestCtx
			dag        *graph.DAG
			err        error
		)

		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatelessNginxComponent, compDefName).
				GetObject()

			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(compName, compDefName).
				SetReplicas(1).
				GetObject()

			reqCtx = &intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: log.FromContext(ctx).WithValues("cluster", clusterDef.Name),
			}
			dag = graph.NewDAG()
			component, err = NewComponent(*reqCtx, testCtx.Cli, clusterDef, nil, cluster, compName, dag)
			Expect(err).Should(Succeed())
			Expect(component).ShouldNot(BeNil())
		})

		It("should not change component if no deployment or pod exists", func() {
			Expect(component.Status(*reqCtx, testCtx.Cli)).Should(Succeed())
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

				Expect(component.Status(*reqCtx, testCtx.Cli)).Should(Succeed())
				Expect(cluster.Status.Components[compName].Phase).Should(Equal(appsv1alpha1.FailedClusterCompPhase))
			})

			It("should set component status to running if container is ready", func() {
				Expect(testapps.ChangeObjStatus(&testCtx, deployment, func() {
					testk8s.MockDeploymentReady(deployment, stateless.NewRSAvailableReason, deployment.Name)
				})).Should(Succeed())

				Expect(component.Status(*reqCtx, testCtx.Cli)).Should(Succeed())
				Expect(cluster.Status.Components[compName].Phase).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
			})
		})
	})

	Context("with statefulset component", func() {
		var (
			clusterDef *appsv1alpha1.ClusterDefinition
			cluster    *appsv1alpha1.Cluster
			component  types.Component
			reqCtx     *intctrlutil.RequestCtx
			dag        *graph.DAG
			err        error
		)

		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.StatefulMySQLComponent, compDefName).
				GetObject()

			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(compName, compDefName).
				SetReplicas(int32(3)).
				GetObject()

			reqCtx = &intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: log.FromContext(ctx).WithValues("cluster", clusterDef.Name),
			}
			dag = graph.NewDAG()
			component, err = NewComponent(*reqCtx, testCtx.Cli, clusterDef, nil, cluster, compName, dag)
			Expect(err).Should(Succeed())
			Expect(component).ShouldNot(BeNil())
		})

		It("should not change component if no statefulset or pod exists", func() {
			Expect(component.Status(*reqCtx, testCtx.Cli)).Should(Succeed())
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

				Expect(component.Status(*reqCtx, testCtx.Cli)).Should(Succeed())
				Expect(cluster.Status.Components[compName].Phase).Should(Equal(appsv1alpha1.FailedClusterCompPhase))
			})

			It("should set component status to running if container is ready", func() {
				Expect(testapps.ChangeObjStatus(&testCtx, statefulset, func() {
					testk8s.MockStatefulSetReady(statefulset)
				})).Should(Succeed())

				Expect(component.Status(*reqCtx, testCtx.Cli)).Should(Succeed())
				Expect(cluster.Status.Components[compName].Phase).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
			})
		})
	})

	Context("with consensusset component", func() {
		var (
			clusterDef *appsv1alpha1.ClusterDefinition
			cluster    *appsv1alpha1.Cluster
			component  types.Component
			reqCtx     *intctrlutil.RequestCtx
			dag        *graph.DAG
			err        error
		)

		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.ConsensusMySQLComponent, compDefName).
				Create(&testCtx).GetObject()

			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(compName, compDefName).
				SetReplicas(int32(3)).
				Create(&testCtx).GetObject()

			reqCtx = &intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: log.FromContext(ctx).WithValues("cluster", clusterDef.Name),
			}
			dag = graph.NewDAG()
			component, err = NewComponent(*reqCtx, testCtx.Cli, clusterDef, nil, cluster, compName, dag)
			Expect(err).Should(Succeed())
			Expect(component).ShouldNot(BeNil())
		})

		It("should not change component if no statefulset or pod exists", func() {
			Expect(component.Status(*reqCtx, testCtx.Cli)).Should(Succeed())
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

				Expect(component.Status(*reqCtx, testCtx.Cli)).Should(Succeed())
				Expect(cluster.Status.Components[compName].Phase).Should(Equal(appsv1alpha1.FailedClusterCompPhase))
			})

			It("should set component status to running if container is ready", func() {
				Expect(testapps.ChangeObjStatus(&testCtx, statefulset, func() {
					testk8s.MockStatefulSetReady(statefulset)
				})).Should(Succeed())

				Expect(setPodRole(pods[0], "leader")).Should(Succeed())
				Expect(setPodRole(pods[1], "follower")).Should(Succeed())
				Expect(setPodRole(pods[2], "follower")).Should(Succeed())

				Expect(component.Status(*reqCtx, testCtx.Cli)).Should(Succeed())
				Expect(cluster.Status.Components[compName].Phase).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
			})
		})
	})

	Context("with replicationset component", func() {
		var (
			clusterDef *appsv1alpha1.ClusterDefinition
			cluster    *appsv1alpha1.Cluster
			component  types.Component
			reqCtx     *intctrlutil.RequestCtx
			dag        *graph.DAG
			err        error
		)

		BeforeEach(func() {
			clusterDef = testapps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(testapps.ReplicationRedisComponent, compDefName).
				GetObject()

			cluster = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(compName, compDefName).
				SetReplicas(2).
				GetObject()

			reqCtx = &intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: log.FromContext(ctx).WithValues("cluster", clusterDef.Name),
			}
			dag = graph.NewDAG()
			component, err = NewComponent(*reqCtx, testCtx.Cli, clusterDef, nil, cluster, compName, dag)
			Expect(err).Should(Succeed())
			Expect(component).ShouldNot(BeNil())
		})

		It("should not change component if no deployment or pod exists", func() {
			Expect(component.Status(*reqCtx, testCtx.Cli)).Should(Succeed())
			Expect(cluster.Status.Components[compName].Phase).Should(BeEmpty())
		})

		Context("and with mocked statefulset & pod", func() {
			const (
				replicas = 2
			)
			var (
				statefulset *appsv1.StatefulSet
				pods        []*corev1.Pod
			)

			BeforeEach(func() {
				stsName := clusterName + "-" + compName
				statefulset = testapps.NewStatefulSetFactory(testCtx.DefaultNamespace, stsName, clusterName, compName).
					SetReplicas(int32(replicas)).
					AddContainer(corev1.Container{Name: testapps.DefaultRedisContainerName, Image: testapps.DefaultRedisImageName}).
					Create(&testCtx).GetObject()
				testk8s.InitStatefulSetStatus(testCtx, statefulset, controllerRevision)
				for i := 0; i < replicas; i++ {
					podName := fmt.Sprintf("%s-%d", stsName, i)
					podRole := "primary"
					if i > 0 {
						podRole = "secondary"
					}
					pod := testapps.NewPodFactory(testCtx.DefaultNamespace, podName).
						SetOwnerReferences("apps/v1", constant.StatefulSetKind, statefulset).
						AddAppInstanceLabel(clusterName).
						AddAppComponentLabel(compName).
						AddAppManangedByLabel().
						AddRoleLabel(podRole).
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

				Expect(component.Status(*reqCtx, testCtx.Cli)).Should(Succeed())
				Expect(cluster.Status.Components[compName].Phase).Should(Equal(appsv1alpha1.FailedClusterCompPhase))
			})

			It("should set component status to running if container is ready", func() {
				Expect(testapps.ChangeObjStatus(&testCtx, statefulset, func() {
					testk8s.MockStatefulSetReady(statefulset)
				})).Should(Succeed())

				Expect(component.Status(*reqCtx, testCtx.Cli)).Should(Succeed())
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
	return testapps.ChangeObj(&testCtx, pod, func(lpod *corev1.Pod) {
		lpod.Labels[constant.RoleLabelKey] = role
	})
}
