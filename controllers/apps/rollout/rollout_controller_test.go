/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package rollout

import (
	"fmt"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("rollout controller", func() {
	const (
		compDefName     = "test-compdef"
		clusterName     = "test-cluster"
		compName        = "comp"
		serviceVersion1 = "1.0.1"
		serviceVersion2 = "1.0.2"
		rolloutName     = "test-rollout"
		replicas        = int32(3)
	)

	var (
		clusterObj                      *appsv1.Cluster
		compObj                         *appsv1.Component
		rolloutObj                      *appsv1alpha1.Rollout
		clusterKey, compKey, rolloutKey client.ObjectKey
	)

	createClusterNCompObj := func() {
		By("creating a cluster object")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
			WithRandomName().
			AddComponent(compName, compDefName).
			SetServiceVersion(serviceVersion1).
			SetReplicas(replicas).
			Create(&testCtx).
			GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("creating a component object")
		compObjName := constant.GenerateClusterComponentName(clusterKey.Name, compName)
		compObj = testapps.NewComponentFactory(testCtx.DefaultNamespace, compObjName, compDefName).
			AddLabelsInMap(constant.GetCompLabelsWithDef(clusterKey.Name, compName, compDefName)).
			SetReplicas(replicas).
			Create(&testCtx).
			GetObject()
		compKey = client.ObjectKeyFromObject(compObj)
	}

	mockClusterNCompRunning := func() {
		By("mock cluster & component as running")
		Expect(testapps.GetAndChangeObjStatus(&testCtx, compKey, func(comp *appsv1.Component) {
			comp.Status.ObservedGeneration = comp.Generation
			comp.Status.Phase = appsv1.RunningComponentPhase
		})()).Should(Succeed())
		Expect(testapps.GetAndChangeObjStatus(&testCtx, clusterKey, func(cluster *appsv1.Cluster) {
			cluster.Status.ObservedGeneration = cluster.Generation
			cluster.Status.Components = map[string]appsv1.ClusterComponentStatus{
				compName: {
					Phase: appsv1.RunningComponentPhase,
				},
			}
		})()).Should(Succeed())
	}

	mockCreatePods := func(ordinals []int32, tplName string) []*corev1.Pod {
		pods := make([]*corev1.Pod, 0)
		for _, ordinal := range ordinals {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testCtx.DefaultNamespace,
					Name:      fmt.Sprintf("%s-%d", constant.GenerateWorkloadNamePattern(clusterKey.Name, compName), ordinal),
					Labels: map[string]string{
						constant.AppManagedByLabelKey:          constant.AppName,
						constant.AppInstanceLabelKey:           clusterKey.Name,
						constant.KBAppComponentLabelKey:        compName,
						constant.KBAppReleasePhaseKey:          constant.ReleasePhaseStable,
						constant.KBAppInstanceTemplateLabelKey: tplName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "rollout",
							Image: "rollout",
						},
					},
				},
			}
			Expect(testCtx.CheckedCreateObj(testCtx.Ctx, pod)).Should(Succeed())
			pods = append(pods, pod)
		}
		slices.SortFunc(pods, func(a, b *corev1.Pod) int {
			return strings.Compare(a.Name, b.Name) * -1
		})
		return pods
	}

	createRolloutObj := func(processor func(*testapps.MockRolloutFactory)) {
		By("creating a rollout object")
		f := testapps.NewRolloutFactory(testCtx.DefaultNamespace, rolloutName).
			WithRandomName().
			SetClusterName(clusterKey.Name).
			AddComponent(compName)
		if processor != nil {
			processor(f)
		}
		rolloutObj = f.Create(&testCtx).GetObject()
		rolloutKey = client.ObjectKeyFromObject(rolloutObj)
	}

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}

		// namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.RolloutSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.ClusterSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.ComponentSignature, true, inNS, ml)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("rollout", func() {
		BeforeEach(func() {
			createClusterNCompObj()
			createRolloutObj(func(f *testapps.MockRolloutFactory) {
				f.SetCompServiceVersion(serviceVersion2).
					SetCompStrategy(appsv1alpha1.RolloutStrategy{
						Replace: &appsv1alpha1.RolloutStrategyReplace{},
					}).
					SetCompReplicas(replicas)
			})
		})

		It("finalizer", func() {
			By("checking the finalizer")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				finalizers := rollout.GetFinalizers()
				g.Expect(finalizers).Should(HaveLen(1))
				g.Expect(finalizers[0]).Should(Equal(constant.RolloutFinalizerName))
			})).Should(Succeed())
		})

		It("rollout label", func() {
			By("checking the rollout label in cluster")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				labels := cluster.GetLabels()
				g.Expect(labels).Should(HaveKeyWithValue(rolloutNameClusterLabel, rolloutKey.Name))
			})).Should(Succeed())
		})

		It("rollout label - after deletion", func() {
			By("checking the rollout label in cluster")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				labels := cluster.GetLabels()
				g.Expect(labels).Should(HaveKeyWithValue(rolloutNameClusterLabel, rolloutKey.Name))
			})).Should(Succeed())

			By("deleting the rollout object")
			Expect(testCtx.Cli.Delete(testCtx.Ctx, rolloutObj)).Should(Succeed())

			By("checking the rollout label in cluster after deletion")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				labels := cluster.GetLabels()
				g.Expect(labels).ShouldNot(HaveKeyWithValue(rolloutNameClusterLabel, rolloutKey.Name))
			})).Should(Succeed())
		})

		It("replicas in status", func() {
			By("checking the replicas in rollout status")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				comps := rollout.Status.Components
				g.Expect(comps).Should(HaveLen(1))
				g.Expect(comps[0].Name).Should(Equal(compName))
				g.Expect(comps[0].Replicas).Should(Equal(replicas))
			})).Should(Succeed())
		})

		It("concurrent rollout", func() {
			By("checking the rollout label in cluster")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				labels := cluster.GetLabels()
				g.Expect(labels).Should(HaveKeyWithValue(rolloutNameClusterLabel, rolloutKey.Name))
			})).Should(Succeed())

			By("checking the rollout state")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.PendingRolloutState))
			})).Should(Succeed())

			By("creating a new rollout object")
			createRolloutObj(func(f *testapps.MockRolloutFactory) {
				f.SetCompServiceVersion(serviceVersion2).
					SetCompStrategy(appsv1alpha1.RolloutStrategy{
						Replace: &appsv1alpha1.RolloutStrategyReplace{},
					}).
					SetCompReplicas(replicas)
			})

			By("checking the rollout state of new object")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.ErrorRolloutState))
			})).Should(Succeed())
		})
	})

	Context("inplace", func() {
		var (
			defaultInplaceStrategy = appsv1alpha1.RolloutStrategy{
				Inplace: &appsv1alpha1.RolloutStrategyInplace{},
			}
		)

		BeforeEach(func() {
			createClusterNCompObj()
		})

		It("rolling", func() {
			createRolloutObj(func(f *testapps.MockRolloutFactory) {
				f.SetCompServiceVersion(serviceVersion2).
					SetCompStrategy(defaultInplaceStrategy).
					SetCompReplicas(replicas)
			})

			By("checking the rollout state")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.PendingRolloutState))
				// TODO: check message
			})).Should(Succeed())

			mockClusterNCompRunning()

			By("checking the cluster spec been updated")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				g.Expect(cluster.Spec.ComponentSpecs[0].ServiceVersion).Should(Equal(serviceVersion2))
			})).Should(Succeed())

			By("checking the rollout state as rolling")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.RollingRolloutState))
			})).Should(Succeed())
		})

		It("succeed", func() {
			createRolloutObj(func(f *testapps.MockRolloutFactory) {
				f.SetCompServiceVersion(serviceVersion2).
					SetCompStrategy(defaultInplaceStrategy).
					SetCompReplicas(replicas)
			})

			mockClusterNCompRunning()

			By("checking the rollout state as rolling")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.RollingRolloutState))
			})).Should(Succeed())

			mockClusterNCompRunning()

			By("checking the rollout state as succeed")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.SucceedRolloutState))
			})).Should(Succeed())
		})
	})

	Context("replace", func() {
		var (
			defaultReplaceStrategy = appsv1alpha1.RolloutStrategy{
				Replace: &appsv1alpha1.RolloutStrategyReplace{},
			}
		)

		BeforeEach(func() {
			createClusterNCompObj()
		})

		It("rolling", func() {
			createRolloutObj(func(f *testapps.MockRolloutFactory) {
				f.SetCompServiceVersion(serviceVersion2).
					SetCompStrategy(defaultReplaceStrategy).
					SetCompReplicas(replicas)
			})

			By("checking the rollout state")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.PendingRolloutState))
				// TODO: check message
			})).Should(Succeed())

			mockClusterNCompRunning()

			By("checking the cluster spec & status been updated")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				spec := cluster.Spec.ComponentSpecs[0]
				g.Expect(spec.Replicas).Should(Equal(replicas + 1))
				g.Expect(spec.ServiceVersion).Should(Equal(serviceVersion1))
				g.Expect(spec.Instances).Should(HaveLen(1))
				g.Expect(spec.Instances[0].Name).Should(Equal(string(rolloutObj.UID[:8])))
				g.Expect(spec.Instances[0].ServiceVersion).Should(Equal(serviceVersion2))
				g.Expect(*spec.Instances[0].Replicas).Should(Equal(int32(1)))
				g.Expect(spec.FlatInstanceOrdinal).Should(BeTrue())
				g.Expect(cluster.Generation).Should(Equal(cluster.Status.ObservedGeneration + 1))
			})).Should(Succeed())

			By("checking the rollout state as rolling")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.RollingRolloutState))
			})).Should(Succeed())
		})

		It("scale down", func() {
			createRolloutObj(func(f *testapps.MockRolloutFactory) {
				f.SetCompServiceVersion(serviceVersion2).
					SetCompStrategy(defaultReplaceStrategy).
					SetCompReplicas(replicas)
			})

			mockClusterNCompRunning() // to up

			By("checking the rollout state as rolling")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.RollingRolloutState))
			})).Should(Succeed())

			By("creating pods for the component")
			pods := mockCreatePods([]int32{0, 1, 2}, "")

			mockClusterNCompRunning() // to down

			By("checking the rollout state as rolling, and one instance is scaled down")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.RollingRolloutState))
				g.Expect(rollout.Status.Components).Should(HaveLen(1))
				g.Expect(rollout.Status.Components[0].ScaleDownInstances).Should(HaveLen(1))
				g.Expect(rollout.Status.Components[0].ScaleDownInstances[0]).Should(Equal(pods[0].Name))
			})).Should(Succeed())

			By("checking the cluster spec after scale down")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				spec := cluster.Spec.ComponentSpecs[0]
				g.Expect(spec.Replicas).Should(Equal(replicas))
				g.Expect(spec.OfflineInstances).Should(HaveLen(1))
				g.Expect(spec.OfflineInstances[0]).Should(Equal(pods[0].Name))
			})).Should(Succeed())
		})

		It("succeed", func() {
			createRolloutObj(func(f *testapps.MockRolloutFactory) {
				f.SetCompServiceVersion(serviceVersion2).
					SetCompStrategy(defaultReplaceStrategy).
					SetCompReplicas(replicas)
			})

			By("creating pods for the component")
			pods := mockCreatePods([]int32{0, 1, 2}, "")

			for i := int32(0); i < replicas; i++ {
				mockClusterNCompRunning() // to up

				By("checking the cluster spec after roll up")
				Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
					spec := cluster.Spec.ComponentSpecs[0]
					g.Expect(spec.Replicas).Should(Equal(replicas + 1))
					g.Expect(spec.ServiceVersion).Should(Equal(serviceVersion1))
					g.Expect(*spec.Instances[0].Replicas).Should(Equal(i + 1))
				})).Should(Succeed())

				By("creating the new pod")
				mockCreatePods([]int32{i + 10}, string(rolloutObj.UID[:8]))

				mockClusterNCompRunning() // to down

				By("checking the cluster spec after scale down")
				Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
					spec := cluster.Spec.ComponentSpecs[0]
					g.Expect(spec.Replicas).Should(Equal(replicas))
					g.Expect(spec.OfflineInstances).Should(HaveLen(int(i + 1)))
					for j := int32(0); j < i+1; j++ {
						g.Expect(spec.OfflineInstances[j]).Should(Equal(pods[j].Name))
					}
				})).Should(Succeed())

				By("deleting the scaled down pod")
				Expect(testCtx.Cli.Delete(testCtx.Ctx, pods[i])).Should(Succeed())
			}

			mockClusterNCompRunning() // all old pods are deleted

			By("checking the cluster spec finally")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				spec := cluster.Spec.ComponentSpecs[0]
				g.Expect(spec.Replicas).Should(Equal(replicas))
				g.Expect(*spec.Instances[0].Replicas).Should(Equal(replicas))
			})).Should(Succeed())

			mockClusterNCompRunning() // tear down will update the cluster spec

			By("checking the rollout state as succeed")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.SucceedRolloutState))
			})).Should(Succeed())
		})

		It("tear down", func() {
			createRolloutObj(func(f *testapps.MockRolloutFactory) {
				f.SetCompServiceVersion(serviceVersion2).
					SetCompStrategy(defaultReplaceStrategy).
					SetCompReplicas(replicas)
			})

			By("creating pods for the component")
			pods := mockCreatePods([]int32{0, 1, 2}, "")

			for i := int32(0); i < replicas; i++ {
				mockClusterNCompRunning() // to up

				By("checking the cluster spec after roll up")
				Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
					spec := cluster.Spec.ComponentSpecs[0]
					g.Expect(spec.Replicas).Should(Equal(replicas + 1))
					g.Expect(spec.ServiceVersion).Should(Equal(serviceVersion1))
					g.Expect(*spec.Instances[0].Replicas).Should(Equal(i + 1))
				})).Should(Succeed())

				By("creating the new pod")
				mockCreatePods([]int32{i + 10}, string(rolloutObj.UID[:8]))

				mockClusterNCompRunning() // to down

				By("checking the cluster spec after scale down")
				Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
					spec := cluster.Spec.ComponentSpecs[0]
					g.Expect(spec.Replicas).Should(Equal(replicas))
					g.Expect(spec.OfflineInstances).Should(HaveLen(int(i + 1)))
					for j := int32(0); j < i+1; j++ {
						g.Expect(spec.OfflineInstances[j]).Should(Equal(pods[j].Name))
					}
				})).Should(Succeed())

				By("deleting the scaled down pod")
				Expect(testCtx.Cli.Delete(testCtx.Ctx, pods[i])).Should(Succeed())
			}

			mockClusterNCompRunning() // all old pods are deleted

			By("checking the cluster spec updated finally")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				spec := cluster.Spec.ComponentSpecs[0]
				g.Expect(spec.ServiceVersion).Should(Equal(serviceVersion2))
				g.Expect(spec.OfflineInstances).Should(BeEmpty())
			})).Should(Succeed())

			mockClusterNCompRunning() // tear down will update the cluster spec

			By("checking the rollout state as succeed")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.SucceedRolloutState))
			})).Should(Succeed())
		})
	})

	// Context("create", func() {
	//	It("auto promotion", func() {
	//	})
	//
	//	It("partly", func() {
	//	})
	//
	//	It("partly - done", func() {
	//	})
	//
	//	It("promote condition", func() {
	//	})
	//
	//	It("scale down", func() {
	//	})
	//
	//	It("tear down", func() {
	//	})
	// })
})
