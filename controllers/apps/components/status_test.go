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
	"strconv"
	"time"

	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	"github.com/apecloud/kubeblocks/pkg/testutil/apps"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
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

		clusterDef *appsv1alpha1.ClusterDefinition
		cluster    *appsv1alpha1.Cluster
		component  Component
		rsm        *workloads.ReplicatedStateMachine
		reqCtx     *controllerutil.RequestCtx
		dag        *graph.DAG
		err        error
	)

	cleanAll := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// clear rest resources
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}

		// non-namespaced resources
		apps.ClearResources(&testCtx, generics.ClusterDefinitionSignature, inNS, ml)

		// namespaced resources
		apps.ClearResources(&testCtx, generics.ClusterSignature, inNS, ml)
		apps.ClearResources(&testCtx, generics.StatefulSetSignature, inNS, ml)
		apps.ClearResources(&testCtx, generics.DeploymentSignature, inNS, ml)
		if controllerutil.IsRSMEnabled() {
			apps.ClearResources(&testCtx, generics.RSMSignature, inNS, ml)
		}

		apps.ClearResources(&testCtx, generics.PodSignature, inNS, ml, client.GracePeriodSeconds(0))
	}

	BeforeEach(cleanAll)

	AfterEach(cleanAll)

	Context("with stateless component", func() {
		BeforeEach(func() {
			clusterDef = apps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(apps.StatelessNginxComponent, compDefName).
				GetObject()

			cluster = apps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(compName, compDefName).
				SetReplicas(1).
				GetObject()

			reqCtx = &controllerutil.RequestCtx{
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
				deployment = apps.NewDeploymentFactory(testCtx.DefaultNamespace, deploymentName, clusterName, compName).
					AddAnnotations(constant.KubeBlocksGenerationKey, strconv.FormatInt(cluster.Generation, 10)).
					SetMinReadySeconds(int32(10)).
					SetReplicas(int32(1)).
					AddContainer(corev1.Container{Name: apps.DefaultNginxContainerName, Image: apps.NginxImage}).
					Create(&testCtx).GetObject()
				if controllerutil.IsRSMEnabled() {
					rsm = apps.NewRSMFactory(testCtx.DefaultNamespace, deploymentName, clusterName, compName).
						AddAnnotations(constant.KubeBlocksGenerationKey, strconv.FormatInt(cluster.Generation, 10)).
						SetReplicas(int32(1)).
						AddContainer(corev1.Container{Name: apps.DefaultNginxContainerName, Image: apps.NginxImage}).
						Create(&testCtx).GetObject()
				}

				podName := fmt.Sprintf("%s-%s-%s", clusterName, compName, testCtx.GetRandomStr())
				if controllerutil.IsRSMEnabled() {
					podName = rsm.Name + "-0"
				}
				pod = apps.NewPodFactory(testCtx.DefaultNamespace, podName).
					SetOwnerReferences("apps/v1", constant.DeploymentKind, deployment).
					AddAppInstanceLabel(clusterName).
					AddAppComponentLabel(compName).
					AddAppManangedByLabel().
					AddContainer(corev1.Container{Name: apps.DefaultNginxContainerName, Image: apps.NginxImage}).
					Create(&testCtx).GetObject()
			})

			It("should set component status to failed if container is not ready and have error message", func() {
				Expect(mockContainerError(pod)).Should(Succeed())

				Expect(component.Status(*reqCtx, testCtx.Cli)).Should(Succeed())
				Expect(cluster.Status.Components[compName].Phase).Should(Equal(appsv1alpha1.FailedClusterCompPhase))
			})

			It("should set component status to running if container is ready", func() {
				Expect(apps.ChangeObjStatus(&testCtx, deployment, func() {
					testutil.MockDeploymentReady(deployment, NewRSAvailableReason, deployment.Name)
				})).Should(Succeed())
				if controllerutil.IsRSMEnabled() {
					Expect(apps.ChangeObjStatus(&testCtx, rsm, func() {
						testutil.MockRSMReady(rsm)
					})).Should(Succeed())
				}

				Expect(component.Status(*reqCtx, testCtx.Cli)).Should(Succeed())
				Expect(cluster.Status.Components[compName].Phase).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
			})
		})
	})

	Context("with statefulset component", func() {
		BeforeEach(func() {
			clusterDef = apps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(apps.StatefulMySQLComponent, compDefName).
				GetObject()

			cluster = apps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(compName, compDefName).
				SetReplicas(int32(3)).
				GetObject()

			reqCtx = &controllerutil.RequestCtx{
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
				statefulset = apps.NewStatefulSetFactory(testCtx.DefaultNamespace, stsName, clusterName, compName).
					AddAnnotations(constant.KubeBlocksGenerationKey, strconv.FormatInt(cluster.Generation, 10)).
					SetReplicas(int32(3)).
					AddContainer(corev1.Container{Name: apps.DefaultMySQLContainerName, Image: apps.ApeCloudMySQLImage}).
					Create(&testCtx).GetObject()
				// init statefulset status
				testutil.InitStatefulSetStatus(testCtx, statefulset, controllerRevision)
				for i := 0; i < 3; i++ {
					podName := fmt.Sprintf("%s-%s-%d", clusterName, compName, i)
					pod := apps.NewPodFactory(testCtx.DefaultNamespace, podName).
						SetOwnerReferences("apps/v1", constant.StatefulSetKind, statefulset).
						AddAppInstanceLabel(clusterName).
						AddAppComponentLabel(compName).
						AddAppManangedByLabel().
						AddControllerRevisionHashLabel(controllerRevision).
						AddContainer(corev1.Container{Name: apps.DefaultMySQLContainerName, Image: apps.ApeCloudMySQLImage}).
						Create(&testCtx).GetObject()
					Expect(apps.ChangeObjStatus(&testCtx, pod, func() {
						pod.Status.Conditions = []corev1.PodCondition{{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						}}
					})).Should(Succeed())
					pods = append(pods, pod)
				}
				if controllerutil.IsRSMEnabled() {
					rsm = apps.NewRSMFactory(testCtx.DefaultNamespace, stsName, clusterName, compName).
						AddAnnotations(constant.KubeBlocksGenerationKey, strconv.FormatInt(cluster.Generation, 10)).
						SetReplicas(int32(3)).
						AddContainer(corev1.Container{Name: apps.DefaultMySQLContainerName, Image: apps.ApeCloudMySQLImage}).
						Create(&testCtx).GetObject()
					// init rsm status
					testutil.InitRSMStatus(testCtx, rsm, controllerRevision)
				}
			})

			It("should set component status to failed if container is not ready and have error message", func() {
				Expect(mockContainerError(pods[0])).Should(Succeed())
				Expect(mockContainerError(pods[1])).Should(Succeed())

				Expect(component.Status(*reqCtx, testCtx.Cli)).Should(Succeed())
				Expect(cluster.Status.Components[compName].Phase).Should(Equal(appsv1alpha1.FailedClusterCompPhase))
			})

			It("should set component status to running if container is ready", func() {
				Expect(apps.ChangeObjStatus(&testCtx, statefulset, func() {
					testutil.MockStatefulSetReady(statefulset)
				})).Should(Succeed())
				if controllerutil.IsRSMEnabled() {
					Expect(apps.ChangeObjStatus(&testCtx, rsm, func() {
						testutil.MockRSMReady(rsm)
					})).Should(Succeed())
				}

				Expect(component.Status(*reqCtx, testCtx.Cli)).Should(Succeed())
				Expect(cluster.Status.Components[compName].Phase).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
			})
		})
	})

	Context("with consensusset component", func() {
		BeforeEach(func() {
			clusterDef = apps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(apps.ConsensusMySQLComponent, compDefName).
				Create(&testCtx).GetObject()

			cluster = apps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(compName, compDefName).
				SetReplicas(int32(3)).
				Create(&testCtx).GetObject()

			reqCtx = &controllerutil.RequestCtx{
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
				statefulset = apps.NewStatefulSetFactory(testCtx.DefaultNamespace, stsName, clusterName, compName).
					AddAnnotations(constant.KubeBlocksGenerationKey, strconv.FormatInt(cluster.Generation, 10)).
					SetReplicas(int32(3)).
					AddContainer(corev1.Container{Name: apps.DefaultMySQLContainerName, Image: apps.ApeCloudMySQLImage}).
					Create(&testCtx).GetObject()
				testutil.InitStatefulSetStatus(testCtx, statefulset, controllerRevision)
				for i := 0; i < 3; i++ {
					podName := fmt.Sprintf("%s-%s-%d", clusterName, compName, i)
					pod := apps.NewPodFactory(testCtx.DefaultNamespace, podName).
						SetOwnerReferences("apps/v1", constant.StatefulSetKind, statefulset).
						AddAppInstanceLabel(clusterName).
						AddAppComponentLabel(compName).
						AddAppManangedByLabel().
						AddControllerRevisionHashLabel(controllerRevision).
						AddContainer(corev1.Container{Name: apps.DefaultMySQLContainerName, Image: apps.ApeCloudMySQLImage}).
						Create(&testCtx).GetObject()
					Expect(apps.ChangeObjStatus(&testCtx, pod, func() {
						pod.Status.Conditions = []corev1.PodCondition{{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						}}
					})).Should(Succeed())
					pods = append(pods, pod)
				}
				if controllerutil.IsRSMEnabled() {
					rsm = apps.NewRSMFactory(testCtx.DefaultNamespace, stsName, clusterName, compName).
						AddAnnotations(constant.KubeBlocksGenerationKey, strconv.FormatInt(cluster.Generation, 10)).
						SetReplicas(int32(3)).
						AddContainer(corev1.Container{Name: apps.DefaultMySQLContainerName, Image: apps.ApeCloudMySQLImage}).
						Create(&testCtx).GetObject()
					testutil.InitRSMStatus(testCtx, rsm, controllerRevision)
				}
			})

			It("should set component status to failed if container is not ready and have error message", func() {
				Expect(mockContainerError(pods[0])).Should(Succeed())

				Expect(component.Status(*reqCtx, testCtx.Cli)).Should(Succeed())
				Expect(cluster.Status.Components[compName].Phase).Should(Equal(appsv1alpha1.FailedClusterCompPhase))
			})

			It("should set component status to running if container is ready", func() {
				Expect(apps.ChangeObjStatus(&testCtx, statefulset, func() {
					testutil.MockStatefulSetReady(statefulset)
				})).Should(Succeed())
				if controllerutil.IsRSMEnabled() {
					Expect(apps.ChangeObjStatus(&testCtx, rsm, func() {
						testutil.MockRSMReady(rsm)
					})).Should(Succeed())
				}

				Expect(setPodRole(pods[0], "leader")).Should(Succeed())
				Expect(setPodRole(pods[1], "follower")).Should(Succeed())
				Expect(setPodRole(pods[2], "follower")).Should(Succeed())

				Expect(component.Status(*reqCtx, testCtx.Cli)).Should(Succeed())
				Expect(cluster.Status.Components[compName].Phase).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
			})
		})
	})

	Context("with replicationset component", func() {
		BeforeEach(func() {
			clusterDef = apps.NewClusterDefFactory(clusterDefName).
				AddComponentDef(apps.ReplicationRedisComponent, compDefName).
				Create(&testCtx).GetObject()

			cluster = apps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, clusterDefName, clusterVersionName).
				AddComponent(compName, compDefName).
				SetReplicas(2).
				Create(&testCtx).GetObject()

			reqCtx = &controllerutil.RequestCtx{
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
				statefulset = apps.NewStatefulSetFactory(testCtx.DefaultNamespace, stsName, clusterName, compName).
					AddAnnotations(constant.KubeBlocksGenerationKey, strconv.FormatInt(cluster.Generation, 10)).
					SetReplicas(int32(replicas)).
					AddContainer(corev1.Container{Name: apps.DefaultRedisContainerName, Image: apps.DefaultRedisImageName}).
					Create(&testCtx).GetObject()
				testutil.InitStatefulSetStatus(testCtx, statefulset, controllerRevision)
				for i := 0; i < replicas; i++ {
					podName := fmt.Sprintf("%s-%d", stsName, i)
					podRole := "primary"
					if i > 0 {
						podRole = "secondary"
					}
					pod := apps.NewPodFactory(testCtx.DefaultNamespace, podName).
						SetOwnerReferences("apps/v1", constant.StatefulSetKind, statefulset).
						AddAppInstanceLabel(clusterName).
						AddAppComponentLabel(compName).
						AddAppManangedByLabel().
						AddRoleLabel(podRole).
						AddControllerRevisionHashLabel(controllerRevision).
						AddContainer(corev1.Container{Name: apps.DefaultRedisContainerName, Image: apps.DefaultRedisImageName}).
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
				if controllerutil.IsRSMEnabled() {
					rsm = apps.NewRSMFactory(testCtx.DefaultNamespace, stsName, clusterName, compName).
						AddAnnotations(constant.KubeBlocksGenerationKey, strconv.FormatInt(cluster.Generation, 10)).
						SetReplicas(int32(replicas)).
						AddContainer(corev1.Container{Name: apps.DefaultRedisContainerName, Image: apps.DefaultRedisImageName}).
						Create(&testCtx).GetObject()
					testutil.InitRSMStatus(testCtx, rsm, controllerRevision)
				}
			})

			It("should set component status to failed if container is not ready and have error message", func() {
				Expect(mockContainerError(pods[0])).Should(Succeed())

				Expect(component.Status(*reqCtx, testCtx.Cli)).Should(Succeed())
				Expect(cluster.Status.Components[compName].Phase).Should(Equal(appsv1alpha1.FailedClusterCompPhase))
			})

			It("should set component status to running if container is ready", func() {
				Expect(apps.ChangeObjStatus(&testCtx, statefulset, func() {
					testutil.MockStatefulSetReady(statefulset)
				})).Should(Succeed())
				if controllerutil.IsRSMEnabled() {
					Expect(apps.ChangeObjStatus(&testCtx, rsm, func() {
						testutil.MockRSMReady(rsm)
					})).Should(Succeed())
				}

				Expect(component.Status(*reqCtx, testCtx.Cli)).Should(Succeed())
				Expect(cluster.Status.Components[compName].Phase).Should(Equal(appsv1alpha1.RunningClusterCompPhase))
			})
		})
	})
})

func mockContainerError(pod *corev1.Pod) error {
	return apps.ChangeObjStatus(&testCtx, pod, func() {
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
	return apps.ChangeObj(&testCtx, pod, func(lpod *corev1.Pod) {
		lpod.Labels[constant.RoleLabelKey] = role
	})
}
