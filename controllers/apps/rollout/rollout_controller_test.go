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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("rollout controller", func() {
	const (
		compDefName          = "test-compdef"
		clusterName          = "test-cluster"
		shardingName         = "sharding"
		compName             = "comp"
		instanceTemplateName = "aaa"
		serviceVersion1      = "1.0.1"
		serviceVersion2      = "1.0.2"
		rolloutName          = "test-rollout"
		replicas             = int32(3)
		seed                 = 1670750000
	)

	var (
		clusterObj                      *appsv1.Cluster
		compObj                         *appsv1.Component
		rolloutObj                      *appsv1alpha1.Rollout
		clusterKey, compKey, rolloutKey client.ObjectKey

		// first 10 ids
		shardIDs = []string{"bvj", "g7c", "gpz", "w8b", "dng", "rhk", "rzn", "ql8", "929", "99n"}
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
			SetServiceVersion(serviceVersion1).
			SetReplicas(replicas).
			Create(&testCtx).
			GetObject()
		compKey = client.ObjectKeyFromObject(compObj)
	}

	createClusterNCompObjWithInstanceTemplate := func() {
		By("creating a cluster object")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
			WithRandomName().
			AddComponent(compName, compDefName).
			SetServiceVersion(serviceVersion1).
			SetReplicas(replicas).
			AddInstances(compName, appsv1.InstanceTemplate{
				Name:           instanceTemplateName,
				ServiceVersion: serviceVersion1,
				Replicas:       ptr.To[int32](1),
			}).
			SetFlatInstanceOrdinal(true).
			Create(&testCtx).
			GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("creating a component object")
		compObjName := constant.GenerateClusterComponentName(clusterKey.Name, compName)
		compObj = testapps.NewComponentFactory(testCtx.DefaultNamespace, compObjName, compDefName).
			AddLabelsInMap(constant.GetCompLabelsWithDef(clusterKey.Name, compName, compDefName)).
			SetServiceVersion(serviceVersion1).
			SetReplicas(replicas).
			AddInstances(appsv1.InstanceTemplate{
				Name:           instanceTemplateName,
				ServiceVersion: serviceVersion1,
				CompDef:        compObjName,
				Replicas:       ptr.To[int32](1),
			}).
			SetFlatInstanceOrdinal(true).
			Create(&testCtx).
			GetObject()
		compKey = client.ObjectKeyFromObject(compObj)
	}

	createClusterNShardingObj := func() {
		By("creating a cluster object")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
			WithRandomName().
			AddSharding(shardingName, "", compDefName).
			SetShardingServiceVersion(serviceVersion1).
			SetShardingReplicas(replicas).
			Create(&testCtx).
			GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("creating a component object")
		compObjName := constant.GenerateClusterComponentName(clusterKey.Name, fmt.Sprintf("%s-%s", compName, shardIDs[0]))
		compObj = testapps.NewComponentFactory(testCtx.DefaultNamespace, compObjName, compDefName).
			AddLabelsInMap(constant.GetCompLabelsWithDef(clusterKey.Name, compName, compDefName, map[string]string{
				constant.KBAppShardingNameLabelKey: shardingName,
			})).
			SetServiceVersion(serviceVersion1).
			SetReplicas(replicas).
			Create(&testCtx).
			GetObject()
		compKey = client.ObjectKeyFromObject(compObj)
	}

	createClusterNShardingObjWithInstanceTemplate := func() {
		By("creating a cluster object")
		clusterObj = testapps.NewClusterFactory(testCtx.DefaultNamespace, clusterName, "").
			WithRandomName().
			AddSharding(shardingName, "", compDefName).
			SetShardingServiceVersion(serviceVersion1).
			SetShardingReplicas(replicas).
			AddShardingInstances(appsv1.InstanceTemplate{
				Name:           instanceTemplateName,
				ServiceVersion: serviceVersion1,
				Replicas:       ptr.To[int32](1),
			}).
			SetShardingFlatInstanceOrdinal(true).
			Create(&testCtx).
			GetObject()
		clusterKey = client.ObjectKeyFromObject(clusterObj)

		By("creating a component object")
		compObjName := constant.GenerateClusterComponentName(clusterKey.Name, fmt.Sprintf("%s-%s", compName, shardIDs[0]))
		compObj = testapps.NewComponentFactory(testCtx.DefaultNamespace, compObjName, compDefName).
			AddLabelsInMap(constant.GetCompLabelsWithDef(clusterKey.Name, compName, compDefName, map[string]string{
				constant.KBAppShardingNameLabelKey: shardingName,
			})).
			SetServiceVersion(serviceVersion1).
			SetReplicas(replicas).
			AddInstances(appsv1.InstanceTemplate{
				Name:           instanceTemplateName,
				ServiceVersion: serviceVersion1,
				CompDef:        compObjName,
				Replicas:       ptr.To[int32](1),
			}).
			SetFlatInstanceOrdinal(true).
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

	mockClusterNShardingRunning := func() {
		By("mock cluster & component as running")
		Expect(testapps.GetAndChangeObjStatus(&testCtx, compKey, func(comp *appsv1.Component) {
			comp.Status.ObservedGeneration = comp.Generation
			comp.Status.Phase = appsv1.RunningComponentPhase
		})()).Should(Succeed())
		Expect(testapps.GetAndChangeObjStatus(&testCtx, clusterKey, func(cluster *appsv1.Cluster) {
			cluster.Status.ObservedGeneration = cluster.Generation
			cluster.Status.Shardings = map[string]appsv1.ClusterShardingStatus{
				shardingName: {
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
					Name:      fmt.Sprintf("%s-%d", compKey.Name, ordinal),
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

	mockCreatePods4Sharding := func(ordinals []int32, tplName string) []*corev1.Pod {
		pods := make([]*corev1.Pod, 0)
		for _, ordinal := range ordinals {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testCtx.DefaultNamespace,
					Name:      fmt.Sprintf("%s-%d", compKey.Name, ordinal),
					Labels: map[string]string{
						constant.AppManagedByLabelKey:          constant.AppName,
						constant.AppInstanceLabelKey:           clusterKey.Name,
						constant.KBAppComponentLabelKey:        fmt.Sprintf("%s-%s", compName, shardIDs[0]),
						constant.KBAppShardingNameLabelKey:     shardingName,
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

	createRolloutObj4Sharding := func(processor func(*testapps.MockRolloutFactory)) {
		By("creating a rollout object")
		f := testapps.NewRolloutFactory(testCtx.DefaultNamespace, rolloutName).
			WithRandomName().
			SetClusterName(clusterKey.Name).
			AddSharding(shardingName)
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
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, intctrlutil.PodSignature, true, inNS, ml)
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

		It("rollout label - concurrent", func() {
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

		It("rollout status", func() {
			By("checking the replicas in rollout status")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				comps := rollout.Status.Components
				g.Expect(comps).Should(HaveLen(1))
				g.Expect(comps[0].Name).Should(Equal(compName))
				g.Expect(comps[0].Replicas).Should(Equal(replicas))
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

	Context("inplace - instance template", func() {
		var (
			defaultInplaceStrategy = appsv1alpha1.RolloutStrategy{
				Inplace: &appsv1alpha1.RolloutStrategyInplace{},
			}
		)

		BeforeEach(func() {
			createClusterNCompObjWithInstanceTemplate()
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
				g.Expect(cluster.Spec.ComponentSpecs[0].Instances[0].ServiceVersion).Should(Equal(serviceVersion2))
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

	Context("inplace - sharding", func() {
		var (
			defaultInplaceStrategy = appsv1alpha1.RolloutStrategy{
				Inplace: &appsv1alpha1.RolloutStrategyInplace{},
			}
		)

		BeforeEach(func() {
			rand.Seed(seed)
			createClusterNShardingObj()
		})

		It("rolling", func() {
			createRolloutObj4Sharding(func(f *testapps.MockRolloutFactory) {
				f.SetShardingServiceVersion(serviceVersion2).
					SetShardingStrategy(defaultInplaceStrategy)
			})

			By("checking the rollout state")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.PendingRolloutState))
				// TODO: check message
			})).Should(Succeed())

			mockClusterNShardingRunning()

			By("checking the cluster spec been updated")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				g.Expect(cluster.Spec.Shardings[0].Template.ServiceVersion).Should(Equal(serviceVersion2))
			})).Should(Succeed())

			By("checking the rollout state as rolling")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.RollingRolloutState))
			})).Should(Succeed())
		})

		It("succeed", func() {
			createRolloutObj4Sharding(func(f *testapps.MockRolloutFactory) {
				f.SetShardingServiceVersion(serviceVersion2).
					SetShardingStrategy(defaultInplaceStrategy)
			})

			mockClusterNShardingRunning()

			By("checking the rollout state as rolling")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.RollingRolloutState))
			})).Should(Succeed())

			mockClusterNShardingRunning()

			By("checking the rollout state as succeed")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.SucceedRolloutState))
			})).Should(Succeed())
		})
	})

	Context("inplace - sharding + instance template", func() {
		var (
			defaultInplaceStrategy = appsv1alpha1.RolloutStrategy{
				Inplace: &appsv1alpha1.RolloutStrategyInplace{},
			}
		)

		BeforeEach(func() {
			rand.Seed(seed)
			createClusterNShardingObjWithInstanceTemplate()
		})

		It("rolling", func() {
			createRolloutObj4Sharding(func(f *testapps.MockRolloutFactory) {
				f.SetShardingServiceVersion(serviceVersion2).
					SetShardingStrategy(defaultInplaceStrategy)
			})

			By("checking the rollout state")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.PendingRolloutState))
				// TODO: check message
			})).Should(Succeed())

			mockClusterNShardingRunning()

			By("checking the cluster spec been updated")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				g.Expect(cluster.Spec.Shardings[0].Template.ServiceVersion).Should(Equal(serviceVersion2))
				g.Expect(cluster.Spec.Shardings[0].Template.Instances[0].ServiceVersion).Should(Equal(serviceVersion2))
			})).Should(Succeed())

			By("checking the rollout state as rolling")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.RollingRolloutState))
			})).Should(Succeed())
		})

		It("succeed", func() {
			createRolloutObj4Sharding(func(f *testapps.MockRolloutFactory) {
				f.SetShardingServiceVersion(serviceVersion2).
					SetShardingStrategy(defaultInplaceStrategy)
			})

			mockClusterNShardingRunning()

			By("checking the rollout state as rolling")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.RollingRolloutState))
			})).Should(Succeed())

			mockClusterNShardingRunning()

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
			By("creating pods for the component")
			pods := mockCreatePods([]int32{0, 1, 2}, "")

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
			By("creating pods for the component")
			pods := mockCreatePods([]int32{0, 1, 2}, "")

			createRolloutObj(func(f *testapps.MockRolloutFactory) {
				f.SetCompServiceVersion(serviceVersion2).
					SetCompStrategy(defaultReplaceStrategy).
					SetCompReplicas(replicas)
			})

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
			By("creating pods for the component")
			pods := mockCreatePods([]int32{0, 1, 2}, "")

			createRolloutObj(func(f *testapps.MockRolloutFactory) {
				f.SetCompServiceVersion(serviceVersion2).
					SetCompStrategy(defaultReplaceStrategy).
					SetCompReplicas(replicas)
			})

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

	Context("replace - instance template", func() {
		var (
			defaultReplaceStrategy = appsv1alpha1.RolloutStrategy{
				Replace: &appsv1alpha1.RolloutStrategyReplace{},
			}
		)

		BeforeEach(func() {
			createClusterNCompObjWithInstanceTemplate()
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
				g.Expect(spec.Instances).Should(HaveLen(3)) // aaa, prefix, prefix-aaa
				g.Expect(spec.Instances[0].Name).Should(Equal(instanceTemplateName))
				g.Expect(spec.Instances[0].ServiceVersion).Should(Equal(serviceVersion1)) // hasn't been updated
				g.Expect(*spec.Instances[0].Replicas).Should(Equal(int32(1)))
				prefix := replaceInstanceTemplateNamePrefix(rolloutObj)
				for _, i := range []int{1, 2} {
					tpl := spec.Instances[i]
					g.Expect(tpl.ServiceVersion).Should(Equal(serviceVersion2))
					g.Expect(tpl.Name).Should(HavePrefix(prefix))
					if tpl.Name == prefix {
						g.Expect(*tpl.Replicas).Should(Equal(int32(1)))
					} else {
						g.Expect(*tpl.Replicas).Should(Equal(int32(0)))
					}
				}
				g.Expect(spec.FlatInstanceOrdinal).Should(BeTrue())
				g.Expect(cluster.Generation).Should(Equal(cluster.Status.ObservedGeneration + 1))
			})).Should(Succeed())

			By("checking the rollout state as rolling")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.RollingRolloutState))
			})).Should(Succeed())
		})

		It("scale down", func() {
			By("creating pods for the component")
			pods := mockCreatePods([]int32{0}, instanceTemplateName)  // pod 0 is the instance template pod
			pods = append(mockCreatePods([]int32{1, 2}, ""), pods...) // 2, 1, 0

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
				g.Expect(spec.Instances[0].Name).Should(Equal(instanceTemplateName))
				g.Expect(spec.Instances[0].ServiceVersion).Should(Equal(serviceVersion1)) // hasn't been updated
				g.Expect(*spec.Instances[0].Replicas).Should(Equal(int32(1)))             // hasn't been scaled down
				prefix := replaceInstanceTemplateNamePrefix(rolloutObj)
				for _, i := range []int{1, 2} {
					tpl := spec.Instances[i]
					g.Expect(tpl.ServiceVersion).Should(Equal(serviceVersion2))
					g.Expect(tpl.Name).Should(HavePrefix(prefix))
					if tpl.Name == prefix {
						g.Expect(*tpl.Replicas).Should(Equal(int32(1)))
					} else {
						g.Expect(*tpl.Replicas).Should(Equal(int32(0)))
					}
				}
			})).Should(Succeed())
		})

		It("scale down - instance template pod", func() {
			By("creating pods for the component")
			pods := mockCreatePods([]int32{0, 1}, "")
			pods = append(mockCreatePods([]int32{2}, instanceTemplateName), pods...) // pod 2 is the instance template pod

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
				g.Expect(spec.Instances[0].Name).Should(Equal(instanceTemplateName))
				g.Expect(spec.Instances[0].ServiceVersion).Should(Equal(serviceVersion1)) // hasn't been updated
				g.Expect(*spec.Instances[0].Replicas).Should(Equal(int32(0)))             // should be scaled down
				prefix := replaceInstanceTemplateNamePrefix(rolloutObj)
				for _, i := range []int{1, 2} {
					tpl := spec.Instances[i]
					g.Expect(tpl.ServiceVersion).Should(Equal(serviceVersion2))
					g.Expect(tpl.Name).Should(HavePrefix(prefix))
					if tpl.Name == prefix {
						g.Expect(*tpl.Replicas).Should(Equal(int32(0)))
					} else {
						g.Expect(*tpl.Replicas).Should(Equal(int32(1)))
					}
				}
			})).Should(Succeed())
		})

		It("succeed", func() {
			By("creating pods for the component")
			pods := mockCreatePods([]int32{0, 1}, "")
			pods = append(mockCreatePods([]int32{2}, instanceTemplateName), pods...) // pod 2 is the instance template pod

			createRolloutObj(func(f *testapps.MockRolloutFactory) {
				f.SetCompServiceVersion(serviceVersion2).
					SetCompStrategy(defaultReplaceStrategy).
					SetCompReplicas(replicas)
			})

			for i := int32(0); i < replicas; i++ {
				prefix := replaceInstanceTemplateNamePrefix(rolloutObj)
				tplName := pods[i].Labels[constant.KBAppInstanceTemplateLabelKey]
				newTplName := prefix
				if tplName != "" {
					newTplName = fmt.Sprintf("%s-%s", prefix, tplName)
				}

				mockClusterNCompRunning() // to up

				By("checking the cluster spec after roll up")
				Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
					spec := cluster.Spec.ComponentSpecs[0]
					g.Expect(spec.Replicas).Should(Equal(replicas + 1))
					g.Expect(spec.ServiceVersion).Should(Equal(serviceVersion1))
					for _, tpl := range spec.Instances {
						if tpl.Name == newTplName {
							if tplName == "" {
								g.Expect(*tpl.Replicas).Should(Equal(i))
							} else {
								g.Expect(*tpl.Replicas).Should(Equal(int32(1)))
							}
						}
					}
				})).Should(Succeed())

				By("creating the new pod")
				if tplName == "" {
					mockCreatePods([]int32{i + 10}, prefix)
				} else {
					mockCreatePods([]int32{i + 10}, fmt.Sprintf("%s-%s", prefix, tplName))
				}

				mockClusterNCompRunning() // to down

				By("checking the cluster spec after scale down")
				Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
					spec := cluster.Spec.ComponentSpecs[0]
					g.Expect(spec.Replicas).Should(Equal(replicas))
					g.Expect(*spec.Instances[0].Replicas).Should(Equal(int32(0)))
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
				g.Expect(*spec.Instances[0].Replicas).Should(Equal(int32(0)))
				g.Expect(*spec.Instances[1].Replicas + *spec.Instances[2].Replicas).Should(Equal(replicas))
			})).Should(Succeed())

			mockClusterNCompRunning() // tear down will update the cluster spec

			By("checking the rollout state as succeed")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.SucceedRolloutState))
			})).Should(Succeed())
		})

		It("tear down", func() {
			By("creating pods for the component")
			pods := mockCreatePods([]int32{0, 1}, "")
			pods = append(mockCreatePods([]int32{2}, instanceTemplateName), pods...) // pod 2 is the instance template pod

			createRolloutObj(func(f *testapps.MockRolloutFactory) {
				f.SetCompServiceVersion(serviceVersion2).
					SetCompStrategy(defaultReplaceStrategy).
					SetCompReplicas(replicas)
			})

			for i := int32(0); i < replicas; i++ {
				prefix := replaceInstanceTemplateNamePrefix(rolloutObj)
				tplName := pods[i].Labels[constant.KBAppInstanceTemplateLabelKey]
				newTplName := prefix
				if tplName != "" {
					newTplName = fmt.Sprintf("%s-%s", prefix, tplName)
				}

				mockClusterNCompRunning() // to up

				By("checking the cluster spec after roll up")
				Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
					spec := cluster.Spec.ComponentSpecs[0]
					g.Expect(spec.Replicas).Should(Equal(replicas + 1))
					g.Expect(spec.ServiceVersion).Should(Equal(serviceVersion1))
					for _, tpl := range spec.Instances {
						if tpl.Name == newTplName {
							if tplName == "" {
								g.Expect(*tpl.Replicas).Should(Equal(i))
							} else {
								g.Expect(*tpl.Replicas).Should(Equal(int32(1)))
							}
						}
					}
				})).Should(Succeed())

				By("creating the new pod")
				if tplName == "" {
					mockCreatePods([]int32{i + 10}, prefix)
				} else {
					mockCreatePods([]int32{i + 10}, fmt.Sprintf("%s-%s", prefix, tplName))
				}

				mockClusterNCompRunning() // to down

				By("checking the cluster spec after scale down")
				Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
					spec := cluster.Spec.ComponentSpecs[0]
					g.Expect(spec.Replicas).Should(Equal(replicas))
					g.Expect(*spec.Instances[0].Replicas).Should(Equal(int32(0)))
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

	Context("replace - sharding", func() {
		var (
			defaultReplaceStrategy = appsv1alpha1.RolloutStrategy{
				Replace: &appsv1alpha1.RolloutStrategyReplace{},
			}
		)

		BeforeEach(func() {
			rand.Seed(seed)
			createClusterNShardingObj()
		})

		It("rolling", func() {
			createRolloutObj4Sharding(func(f *testapps.MockRolloutFactory) {
				f.SetShardingServiceVersion(serviceVersion2).
					SetShardingStrategy(defaultReplaceStrategy)
			})

			By("checking the rollout state")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.PendingRolloutState))
				// TODO: check message
			})).Should(Succeed())

			mockClusterNShardingRunning()

			By("checking the cluster spec & status been updated")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				spec := cluster.Spec.Shardings[0]
				g.Expect(spec.Template.Replicas).Should(Equal(replicas + 1))
				g.Expect(spec.Template.ServiceVersion).Should(Equal(serviceVersion1))
				g.Expect(spec.Template.Instances).Should(HaveLen(1))
				g.Expect(spec.Template.Instances[0].Name).Should(Equal(string(rolloutObj.UID[:8])))
				g.Expect(spec.Template.Instances[0].ServiceVersion).Should(Equal(serviceVersion2))
				g.Expect(*spec.Template.Instances[0].Replicas).Should(Equal(int32(1)))
				g.Expect(spec.Template.FlatInstanceOrdinal).Should(BeTrue())
				g.Expect(cluster.Generation).Should(Equal(cluster.Status.ObservedGeneration + 1))
			})).Should(Succeed())

			By("checking the rollout state as rolling")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.RollingRolloutState))
			})).Should(Succeed())
		})

		It("scale down", func() {
			By("creating pods for the shard")
			pods := mockCreatePods4Sharding([]int32{0, 1, 2}, "")

			createRolloutObj4Sharding(func(f *testapps.MockRolloutFactory) {
				f.SetShardingServiceVersion(serviceVersion2).
					SetShardingStrategy(defaultReplaceStrategy)
			})

			mockClusterNShardingRunning() // to up

			By("checking the rollout state as rolling")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.RollingRolloutState))
			})).Should(Succeed())

			mockClusterNShardingRunning() // to down

			By("checking the rollout state as rolling, and one instance is scaled down")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.RollingRolloutState))
				g.Expect(rollout.Status.Shardings).Should(HaveLen(1))
				g.Expect(rollout.Status.Shardings[0].ScaleDownInstances).Should(HaveLen(1))
				g.Expect(rollout.Status.Shardings[0].ScaleDownInstances[0]).Should(Equal(pods[0].Name))
			})).Should(Succeed())

			By("checking the cluster spec after scale down")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				spec := cluster.Spec.Shardings[0]
				g.Expect(spec.Template.Replicas).Should(Equal(replicas))
				g.Expect(spec.Template.OfflineInstances).Should(HaveLen(1))
				g.Expect(spec.Template.OfflineInstances[0]).Should(Equal(pods[0].Name))
			})).Should(Succeed())
		})

		It("succeed", func() {
			By("creating pods for the component")
			pods := mockCreatePods4Sharding([]int32{0, 1, 2}, "")

			createRolloutObj4Sharding(func(f *testapps.MockRolloutFactory) {
				f.SetShardingServiceVersion(serviceVersion2).
					SetShardingStrategy(defaultReplaceStrategy)
			})

			for i := int32(0); i < replicas; i++ {
				mockClusterNShardingRunning() // to up

				By("checking the cluster spec after roll up")
				Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
					spec := cluster.Spec.Shardings[0]
					g.Expect(spec.Template.Replicas).Should(Equal(replicas + 1))
					g.Expect(spec.Template.ServiceVersion).Should(Equal(serviceVersion1))
					g.Expect(*spec.Template.Instances[0].Replicas).Should(Equal(i + 1))
				})).Should(Succeed())

				By("creating the new pod")
				mockCreatePods4Sharding([]int32{i + 10}, string(rolloutObj.UID[:8]))

				mockClusterNShardingRunning() // to down

				By("checking the cluster spec after scale down")
				Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
					spec := cluster.Spec.Shardings[0]
					g.Expect(spec.Template.Replicas).Should(Equal(replicas))
					g.Expect(spec.Template.OfflineInstances).Should(HaveLen(int(i + 1)))
					for j := int32(0); j < i+1; j++ {
						g.Expect(spec.Template.OfflineInstances[j]).Should(Equal(pods[j].Name))
					}
				})).Should(Succeed())

				By("deleting the scaled down pod")
				Expect(testCtx.Cli.Delete(testCtx.Ctx, pods[i])).Should(Succeed())
			}

			mockClusterNShardingRunning() // all old pods are deleted

			By("checking the cluster spec finally")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				spec := cluster.Spec.Shardings[0]
				g.Expect(spec.Template.Replicas).Should(Equal(replicas))
				g.Expect(*spec.Template.Instances[0].Replicas).Should(Equal(replicas))
			})).Should(Succeed())

			mockClusterNShardingRunning() // tear down will update the cluster spec

			By("checking the rollout state as succeed")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.SucceedRolloutState))
			})).Should(Succeed())
		})

		It("tear down", func() {
			By("creating pods for the component")
			pods := mockCreatePods4Sharding([]int32{0, 1, 2}, "")

			createRolloutObj4Sharding(func(f *testapps.MockRolloutFactory) {
				f.SetShardingServiceVersion(serviceVersion2).
					SetShardingStrategy(defaultReplaceStrategy)
			})

			for i := int32(0); i < replicas; i++ {
				mockClusterNShardingRunning() // to up

				By("checking the cluster spec after roll up")
				Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
					spec := cluster.Spec.Shardings[0]
					g.Expect(spec.Template.Replicas).Should(Equal(replicas + 1))
					g.Expect(spec.Template.ServiceVersion).Should(Equal(serviceVersion1))
					g.Expect(*spec.Template.Instances[0].Replicas).Should(Equal(i + 1))
				})).Should(Succeed())

				By("creating the new pod")
				mockCreatePods4Sharding([]int32{i + 10}, string(rolloutObj.UID[:8]))

				mockClusterNShardingRunning() // to down

				By("checking the cluster spec after scale down")
				Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
					spec := cluster.Spec.Shardings[0]
					g.Expect(spec.Template.Replicas).Should(Equal(replicas))
					g.Expect(spec.Template.OfflineInstances).Should(HaveLen(int(i + 1)))
					for j := int32(0); j < i+1; j++ {
						g.Expect(spec.Template.OfflineInstances[j]).Should(Equal(pods[j].Name))
					}
				})).Should(Succeed())

				By("deleting the scaled down pod")
				Expect(testCtx.Cli.Delete(testCtx.Ctx, pods[i])).Should(Succeed())
			}

			mockClusterNShardingRunning() // all old pods are deleted

			By("checking the cluster spec updated finally")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				spec := cluster.Spec.Shardings[0]
				g.Expect(spec.Template.ServiceVersion).Should(Equal(serviceVersion2))
				g.Expect(spec.Template.OfflineInstances).Should(BeEmpty())
			})).Should(Succeed())

			mockClusterNShardingRunning() // tear down will update the cluster spec

			By("checking the rollout state as succeed")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.SucceedRolloutState))
			})).Should(Succeed())
		})
	})

	Context("replace - sharding + instance template", func() {
		var (
			defaultReplaceStrategy = appsv1alpha1.RolloutStrategy{
				Replace: &appsv1alpha1.RolloutStrategyReplace{},
			}
		)

		BeforeEach(func() {
			rand.Seed(seed)
			createClusterNShardingObjWithInstanceTemplate()
		})

		It("rolling", func() {
			createRolloutObj4Sharding(func(f *testapps.MockRolloutFactory) {
				f.SetShardingServiceVersion(serviceVersion2).
					SetShardingStrategy(defaultReplaceStrategy)
			})

			By("checking the rollout state")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.PendingRolloutState))
				// TODO: check message
			})).Should(Succeed())

			mockClusterNShardingRunning()

			By("checking the cluster spec & status been updated")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				spec := cluster.Spec.Shardings[0]
				g.Expect(spec.Template.Replicas).Should(Equal(replicas + 1))
				g.Expect(spec.Template.ServiceVersion).Should(Equal(serviceVersion1))
				g.Expect(spec.Template.Instances).Should(HaveLen(3)) // aaa, prefix, prefix-aaa
				g.Expect(spec.Template.Instances[0].Name).Should(Equal(instanceTemplateName))
				g.Expect(spec.Template.Instances[0].ServiceVersion).Should(Equal(serviceVersion1)) // hasn't been updated
				g.Expect(*spec.Template.Instances[0].Replicas).Should(Equal(int32(1)))
				prefix := replaceInstanceTemplateNamePrefix(rolloutObj)
				for _, i := range []int{1, 2} {
					tpl := spec.Template.Instances[i]
					g.Expect(tpl.ServiceVersion).Should(Equal(serviceVersion2))
					g.Expect(tpl.Name).Should(HavePrefix(prefix))
					if tpl.Name == prefix {
						g.Expect(*tpl.Replicas).Should(Equal(int32(1)))
					} else {
						g.Expect(*tpl.Replicas).Should(Equal(int32(0)))
					}
				}
				g.Expect(spec.Template.FlatInstanceOrdinal).Should(BeTrue())
				g.Expect(cluster.Generation).Should(Equal(cluster.Status.ObservedGeneration + 1))
			})).Should(Succeed())

			By("checking the rollout state as rolling")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.RollingRolloutState))
			})).Should(Succeed())
		})

		It("scale down", func() {
			By("creating pods for the component")
			pods := mockCreatePods4Sharding([]int32{0}, instanceTemplateName)  // pod 0 is the instance template pod
			pods = append(mockCreatePods4Sharding([]int32{1, 2}, ""), pods...) // 2, 1, 0

			createRolloutObj4Sharding(func(f *testapps.MockRolloutFactory) {
				f.SetShardingServiceVersion(serviceVersion2).
					SetShardingStrategy(defaultReplaceStrategy)
			})

			mockClusterNShardingRunning() // to up

			By("checking the rollout state as rolling")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.RollingRolloutState))
			})).Should(Succeed())

			mockClusterNShardingRunning() // to down

			By("checking the rollout state as rolling, and one instance is scaled down")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.RollingRolloutState))
				g.Expect(rollout.Status.Shardings).Should(HaveLen(1))
				g.Expect(rollout.Status.Shardings[0].ScaleDownInstances).Should(HaveLen(1))
				g.Expect(rollout.Status.Shardings[0].ScaleDownInstances[0]).Should(Equal(pods[0].Name))
			})).Should(Succeed())

			By("checking the cluster spec after scale down")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				spec := cluster.Spec.Shardings[0]
				g.Expect(spec.Template.Replicas).Should(Equal(replicas))
				g.Expect(spec.Template.OfflineInstances).Should(HaveLen(1))
				g.Expect(spec.Template.OfflineInstances[0]).Should(Equal(pods[0].Name))
				g.Expect(spec.Template.Instances[0].Name).Should(Equal(instanceTemplateName))
				g.Expect(spec.Template.Instances[0].ServiceVersion).Should(Equal(serviceVersion1)) // hasn't been updated
				g.Expect(*spec.Template.Instances[0].Replicas).Should(Equal(int32(1)))             // hasn't been scaled down
				prefix := replaceInstanceTemplateNamePrefix(rolloutObj)
				for _, i := range []int{1, 2} {
					tpl := spec.Template.Instances[i]
					g.Expect(tpl.ServiceVersion).Should(Equal(serviceVersion2))
					g.Expect(tpl.Name).Should(HavePrefix(prefix))
					if tpl.Name == prefix {
						g.Expect(*tpl.Replicas).Should(Equal(int32(1)))
					} else {
						g.Expect(*tpl.Replicas).Should(Equal(int32(0)))
					}
				}
			})).Should(Succeed())
		})

		It("scale down - instance template pod", func() {
			By("creating pods for the component")
			pods := mockCreatePods4Sharding([]int32{0, 1}, "")
			pods = append(mockCreatePods4Sharding([]int32{2}, instanceTemplateName), pods...) // pod 2 is the instance template pod

			createRolloutObj4Sharding(func(f *testapps.MockRolloutFactory) {
				f.SetShardingServiceVersion(serviceVersion2).
					SetShardingStrategy(defaultReplaceStrategy)
			})

			mockClusterNShardingRunning() // to up

			By("checking the rollout state as rolling")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.RollingRolloutState))
			})).Should(Succeed())

			mockClusterNShardingRunning() // to down

			By("checking the rollout state as rolling, and one instance is scaled down")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.RollingRolloutState))
				g.Expect(rollout.Status.Shardings).Should(HaveLen(1))
				g.Expect(rollout.Status.Shardings[0].ScaleDownInstances).Should(HaveLen(1))
				g.Expect(rollout.Status.Shardings[0].ScaleDownInstances[0]).Should(Equal(pods[0].Name))
			})).Should(Succeed())

			By("checking the cluster spec after scale down")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				spec := cluster.Spec.Shardings[0]
				g.Expect(spec.Template.Replicas).Should(Equal(replicas))
				g.Expect(spec.Template.OfflineInstances).Should(HaveLen(1))
				g.Expect(spec.Template.OfflineInstances[0]).Should(Equal(pods[0].Name))
				g.Expect(spec.Template.Instances[0].Name).Should(Equal(instanceTemplateName))
				g.Expect(spec.Template.Instances[0].ServiceVersion).Should(Equal(serviceVersion1)) // hasn't been updated
				g.Expect(*spec.Template.Instances[0].Replicas).Should(Equal(int32(0)))             // should be scaled down
				prefix := replaceInstanceTemplateNamePrefix(rolloutObj)
				for _, i := range []int{1, 2} {
					tpl := spec.Template.Instances[i]
					g.Expect(tpl.ServiceVersion).Should(Equal(serviceVersion2))
					g.Expect(tpl.Name).Should(HavePrefix(prefix))
					if tpl.Name == prefix {
						g.Expect(*tpl.Replicas).Should(Equal(int32(0)))
					} else {
						g.Expect(*tpl.Replicas).Should(Equal(int32(1)))
					}
				}
			})).Should(Succeed())
		})

		It("succeed", func() {
			By("creating pods for the component")
			pods := mockCreatePods4Sharding([]int32{0, 1}, "")
			pods = append(mockCreatePods4Sharding([]int32{2}, instanceTemplateName), pods...) // pod 2 is the instance template pod

			createRolloutObj4Sharding(func(f *testapps.MockRolloutFactory) {
				f.SetShardingServiceVersion(serviceVersion2).
					SetShardingStrategy(defaultReplaceStrategy)
			})

			for i := int32(0); i < replicas; i++ {
				prefix := replaceInstanceTemplateNamePrefix(rolloutObj)
				tplName := pods[i].Labels[constant.KBAppInstanceTemplateLabelKey]
				newTplName := prefix
				if tplName != "" {
					newTplName = fmt.Sprintf("%s-%s", prefix, tplName)
				}

				mockClusterNShardingRunning() // to up

				By("checking the cluster spec after roll up")
				Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
					spec := cluster.Spec.Shardings[0]
					g.Expect(spec.Template.Replicas).Should(Equal(replicas + 1))
					g.Expect(spec.Template.ServiceVersion).Should(Equal(serviceVersion1))
					for _, tpl := range spec.Template.Instances {
						if tpl.Name == newTplName {
							if tplName == "" {
								g.Expect(*tpl.Replicas).Should(Equal(i))
							} else {
								g.Expect(*tpl.Replicas).Should(Equal(int32(1)))
							}
						}
					}
				})).Should(Succeed())

				By("creating the new pod")
				if tplName == "" {
					mockCreatePods4Sharding([]int32{i + 10}, prefix)
				} else {
					mockCreatePods4Sharding([]int32{i + 10}, fmt.Sprintf("%s-%s", prefix, tplName))
				}

				mockClusterNShardingRunning() // to down

				By("checking the cluster spec after scale down")
				Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
					spec := cluster.Spec.Shardings[0]
					g.Expect(spec.Template.Replicas).Should(Equal(replicas))
					g.Expect(*spec.Template.Instances[0].Replicas).Should(Equal(int32(0)))
					g.Expect(spec.Template.OfflineInstances).Should(HaveLen(int(i + 1)))
					for j := int32(0); j < i+1; j++ {
						g.Expect(spec.Template.OfflineInstances[j]).Should(Equal(pods[j].Name))
					}
				})).Should(Succeed())

				By("deleting the scaled down pod")
				Expect(testCtx.Cli.Delete(testCtx.Ctx, pods[i])).Should(Succeed())
			}

			mockClusterNShardingRunning() // all old pods are deleted

			By("checking the cluster spec finally")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				spec := cluster.Spec.Shardings[0]
				g.Expect(spec.Template.Replicas).Should(Equal(replicas))
				g.Expect(*spec.Template.Instances[0].Replicas).Should(Equal(int32(0)))
				g.Expect(*spec.Template.Instances[1].Replicas + *spec.Template.Instances[2].Replicas).Should(Equal(replicas))
			})).Should(Succeed())

			mockClusterNShardingRunning() // tear down will update the cluster spec

			By("checking the rollout state as succeed")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.SucceedRolloutState))
			})).Should(Succeed())
		})

		It("tear down", func() {
			By("creating pods for the component")
			pods := mockCreatePods4Sharding([]int32{0, 1}, "")
			pods = append(mockCreatePods4Sharding([]int32{2}, instanceTemplateName), pods...) // pod 2 is the instance template pod

			createRolloutObj4Sharding(func(f *testapps.MockRolloutFactory) {
				f.SetShardingServiceVersion(serviceVersion2).
					SetShardingStrategy(defaultReplaceStrategy)
			})

			for i := int32(0); i < replicas; i++ {
				prefix := replaceInstanceTemplateNamePrefix(rolloutObj)
				tplName := pods[i].Labels[constant.KBAppInstanceTemplateLabelKey]
				newTplName := prefix
				if tplName != "" {
					newTplName = fmt.Sprintf("%s-%s", prefix, tplName)
				}

				mockClusterNShardingRunning() // to up

				By("checking the cluster spec after roll up")
				Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
					spec := cluster.Spec.Shardings[0]
					g.Expect(spec.Template.Replicas).Should(Equal(replicas + 1))
					g.Expect(spec.Template.ServiceVersion).Should(Equal(serviceVersion1))
					for _, tpl := range spec.Template.Instances {
						if tpl.Name == newTplName {
							if tplName == "" {
								g.Expect(*tpl.Replicas).Should(Equal(i))
							} else {
								g.Expect(*tpl.Replicas).Should(Equal(int32(1)))
							}
						}
					}
				})).Should(Succeed())

				By("creating the new pod")
				if tplName == "" {
					mockCreatePods4Sharding([]int32{i + 10}, prefix)
				} else {
					mockCreatePods4Sharding([]int32{i + 10}, fmt.Sprintf("%s-%s", prefix, tplName))
				}

				mockClusterNShardingRunning() // to down

				By("checking the cluster spec after scale down")
				Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
					spec := cluster.Spec.Shardings[0]
					g.Expect(spec.Template.Replicas).Should(Equal(replicas))
					g.Expect(*spec.Template.Instances[0].Replicas).Should(Equal(int32(0)))
					g.Expect(spec.Template.OfflineInstances).Should(HaveLen(int(i + 1)))
					for j := int32(0); j < i+1; j++ {
						g.Expect(spec.Template.OfflineInstances[j]).Should(Equal(pods[j].Name))
					}
				})).Should(Succeed())

				By("deleting the scaled down pod")
				Expect(testCtx.Cli.Delete(testCtx.Ctx, pods[i])).Should(Succeed())
			}

			mockClusterNShardingRunning() // all old pods are deleted

			By("checking the cluster spec updated finally")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				spec := cluster.Spec.Shardings[0]
				g.Expect(spec.Template.ServiceVersion).Should(Equal(serviceVersion2))
				g.Expect(spec.Template.OfflineInstances).Should(BeEmpty())
			})).Should(Succeed())

			mockClusterNShardingRunning() // tear down will update the cluster spec

			By("checking the rollout state as succeed")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.SucceedRolloutState))
			})).Should(Succeed())
		})
	})

	Context("create", func() {
		var (
			defaultCreateStrategy = appsv1alpha1.RolloutStrategy{
				Create: &appsv1alpha1.RolloutStrategyCreate{
					Canary: ptr.To(true),
				},
			}
		)

		BeforeEach(func() {
			createClusterNCompObj()
		})

		It("basic create strategy without promotion", func() {
			By("creating rollout with create strategy")
			createRolloutObj(func(f *testapps.MockRolloutFactory) {
				f.SetCompServiceVersion(serviceVersion2).
					SetCompStrategy(defaultCreateStrategy).
					SetCompReplicas(int32(1))
			})

			By("checking rollout state is pending initially")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.PendingRolloutState))
			})).Should(Succeed())

			By("mocking cluster and component as running")
			mockClusterNCompRunning()

			By("checking rollout state transitions to rolling")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.RollingRolloutState))
			})).Should(Succeed())

			By("checking canary instance template created in cluster spec")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				spec := cluster.Spec.ComponentSpecs[0]
				prefix := replaceInstanceTemplateNamePrefix(rolloutObj)
				found := false
				for _, tpl := range spec.Instances {
					if strings.HasPrefix(tpl.Name, prefix) {
						found = true
						g.Expect(tpl.Canary).ShouldNot(BeNil())
						g.Expect(*tpl.Canary).Should(BeTrue())
						g.Expect(tpl.ServiceVersion).Should(Equal(serviceVersion2))
						break
					}
				}
				g.Expect(found).Should(BeTrue())
			})).Should(Succeed())
		})

		It("create strategy with auto promotion", func() {
			By("creating rollout with create strategy and auto promotion")
			createRolloutObj(func(f *testapps.MockRolloutFactory) {
				f.SetCompServiceVersion(serviceVersion2).
					SetCompStrategy(appsv1alpha1.RolloutStrategy{
						Create: &appsv1alpha1.RolloutStrategyCreate{
							Canary: ptr.To(true),
							Promotion: &appsv1alpha1.RolloutPromotion{
								Auto:         ptr.To(true),
								DelaySeconds: ptr.To[int32](1), // short delay for test
							},
						},
					}).
					SetCompReplicas(int32(1))
			})

			By("mocking cluster and component as running")
			mockClusterNCompRunning()

			By("checking rollout state is rolling initially")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.RollingRolloutState))
			})).Should(Succeed())

			By("creating canary pods to reach target replicas")
			prefix := replaceInstanceTemplateNamePrefix(rolloutObj)
			_ = mockCreatePods([]int32{3, 4, 5}, prefix) // create canary pods

			By("waiting for promotion delay and checking state transitions to succeed")
			Eventually(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.SucceedRolloutState))
			})).WithTimeout(5 * time.Second).Should(Succeed())

			By("checking canary instance template marked as non-canary after promotion")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				spec := cluster.Spec.ComponentSpecs[0]
				for _, tpl := range spec.Instances {
					if strings.HasPrefix(tpl.Name, prefix) {
						g.Expect(tpl.Canary).ShouldNot(BeNil())
						g.Expect(*tpl.Canary).Should(BeFalse())
						break
					}
				}
			})).Should(Succeed())
		})

		It("create strategy with promotion delay", func() {
			By("creating rollout with create strategy and promotion delay")
			createRolloutObj(func(f *testapps.MockRolloutFactory) {
				f.SetCompServiceVersion(serviceVersion2).
					SetCompStrategy(appsv1alpha1.RolloutStrategy{
						Create: &appsv1alpha1.RolloutStrategyCreate{
							Canary: ptr.To(true),
							Promotion: &appsv1alpha1.RolloutPromotion{
								Auto:         ptr.To(true),
								DelaySeconds: ptr.To[int32](30), // longer delay
							},
						},
					}).
					SetCompReplicas(int32(1))
			})

			By("mocking cluster and component as running")
			mockClusterNCompRunning()

			By("creating canary pods")
			prefix := replaceInstanceTemplateNamePrefix(rolloutObj)
			_ = mockCreatePods([]int32{3, 4, 5}, prefix)

			By("checking rollout stays in rolling state during promotion delay")
			Consistently(testapps.CheckObj(&testCtx, rolloutKey, func(g Gomega, rollout *appsv1alpha1.Rollout) {
				g.Expect(rollout.Status.State).Should(Equal(appsv1alpha1.RollingRolloutState))
			})).WithTimeout(2 * time.Second).Should(Succeed())

			By("checking canary instance template still marked as canary")
			Eventually(testapps.CheckObj(&testCtx, clusterKey, func(g Gomega, cluster *appsv1.Cluster) {
				spec := cluster.Spec.ComponentSpecs[0]
				for _, tpl := range spec.Instances {
					if strings.HasPrefix(tpl.Name, prefix) {
						g.Expect(tpl.Canary).ShouldNot(BeNil())
						g.Expect(*tpl.Canary).Should(BeTrue())
						break
					}
				}
			})).Should(Succeed())
		})
	})
})
