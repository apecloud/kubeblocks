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

package rollout

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	rolloutctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/testutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("replace write order", Ordered, ContinueOnFailure, func() {
	const (
		compDefName          = "test-compdef"
		shardingName         = "sharding"
		compName             = "comp"
		instanceTemplateName = "aaa"
		serviceVersion1      = "1.0.1"
		serviceVersion2      = "1.0.2"
		replicas             = int32(3)
	)

	var (
		localCtx     context.Context
		localClient  client.Client
		localTestCtx testutil.TestContext
	)

	BeforeAll(func() {
		localEnv := &envtest.Environment{
			CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "config", "crd", "bases")},
			ErrorIfCRDPathMissing: true,
			UseExistingCluster:    ptr.To(false),
		}
		DeferCleanup(func() { Expect(localEnv.Stop()).To(Succeed()) })
		localConfig, err := localEnv.Start()
		Expect(err).NotTo(HaveOccurred())
		localScheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(localScheme)).To(Succeed())
		Expect(appsv1.AddToScheme(localScheme)).To(Succeed())
		Expect(appsv1alpha1.AddToScheme(localScheme)).To(Succeed())
		localClient, err = client.New(localConfig, client.Options{Scheme: localScheme})
		Expect(err).NotTo(HaveOccurred())
		var localCancel context.CancelFunc
		localCtx, localCancel = context.WithCancel(context.Background())
		DeferCleanup(localCancel)
		localTestCtx = testutil.NewDefaultTestContext(localCtx, localClient, localEnv)
	})

	createFixture := func(sharding, template bool) (client.ObjectKey, client.ObjectKey, func([]int32, string) []*corev1.Pod, func()) {
		clusterFactory := testapps.NewClusterFactory(localTestCtx.DefaultNamespace, "test-cluster", "").WithRandomName()
		if sharding {
			clusterFactory.AddSharding(shardingName, "", compDefName).
				SetShardingServiceVersion(serviceVersion1).SetShardingReplicas(replicas)
		} else {
			clusterFactory.AddComponent(compName, compDefName).
				SetServiceVersion(serviceVersion1).SetReplicas(replicas)
		}
		instance := appsv1.InstanceTemplate{Name: instanceTemplateName, ServiceVersion: serviceVersion1, Replicas: ptr.To[int32](1)}
		if template {
			if sharding {
				clusterFactory.AddShardingInstances(instance).SetShardingFlatInstanceOrdinal(true)
			} else {
				clusterFactory.AddInstances(compName, instance).SetFlatInstanceOrdinal(true)
			}
		}
		clusterObj := clusterFactory.Create(&localTestCtx).GetObject()
		clusterKey := client.ObjectKeyFromObject(clusterObj)
		componentName := compName
		labels := constant.GetCompLabelsWithDef(clusterKey.Name, compName, compDefName)
		if sharding {
			componentName += "-bvj"
			labels[constant.KBAppShardingNameLabelKey] = shardingName
		}
		componentFactory := testapps.NewComponentFactory(localTestCtx.DefaultNamespace,
			constant.GenerateClusterComponentName(clusterKey.Name, componentName), compDefName).
			AddLabelsInMap(labels).SetServiceVersion(serviceVersion1).SetReplicas(replicas)
		if template {
			instance.CompDef = componentFactory.Get().Name
			componentFactory.AddInstances(instance).SetFlatInstanceOrdinal(true)
		}
		compKey := client.ObjectKeyFromObject(componentFactory.Create(&localTestCtx).GetObject())
		createPods := func(ordinals []int32, templateName string) []*corev1.Pod {
			pods := make([]*corev1.Pod, 0, len(ordinals))
			for _, ordinal := range ordinals {
				podLabels := map[string]string{
					constant.AppManagedByLabelKey:          constant.AppName,
					constant.AppInstanceLabelKey:           clusterKey.Name,
					constant.KBAppComponentLabelKey:        componentName,
					constant.KBAppReleasePhaseKey:          constant.ReleasePhaseStable,
					constant.KBAppInstanceTemplateLabelKey: templateName,
				}
				if sharding {
					podLabels[constant.KBAppShardingNameLabelKey] = shardingName
				}
				pod := testapps.NewPodFactory(localTestCtx.DefaultNamespace, fmt.Sprintf("%s-%d", compKey.Name, ordinal)).
					AddLabelsInMap(podLabels).AddContainer(corev1.Container{Name: "rollout", Image: "rollout"}).
					Create(&localTestCtx).GetObject()
				pods = append(pods, pod)
			}
			slices.SortFunc(pods, func(a, b *corev1.Pod) int { return strings.Compare(b.Name, a.Name) })
			return pods
		}
		running := func() {
			Expect(testapps.GetAndChangeObjStatus(&localTestCtx, compKey, func(comp *appsv1.Component) {
				comp.Status.ObservedGeneration = comp.Generation
				comp.Status.Phase = appsv1.RunningComponentPhase
			})()).To(Succeed())
			Expect(testapps.GetAndChangeObjStatus(&localTestCtx, clusterKey, func(cluster *appsv1.Cluster) {
				cluster.Status.ObservedGeneration = cluster.Generation
				if sharding {
					cluster.Status.Shardings = map[string]appsv1.ClusterShardingStatus{
						shardingName: {Phase: appsv1.RunningComponentPhase},
					}
				} else {
					cluster.Status.Components = map[string]appsv1.ClusterComponentStatus{
						compName: {Phase: appsv1.RunningComponentPhase},
					}
				}
			})()).To(Succeed())
		}
		return clusterKey, compKey, createPods, running
	}

	DescribeTable("recovers persisted scale-down targets", func(sharding, template bool) {
		clusterKey, compKey, createPods, running := createFixture(sharding, template)
		rolloutKey := client.ObjectKey{Namespace: localTestCtx.DefaultNamespace, Name: "manual-" + rand.String(8)}
		strategy := appsv1alpha1.RolloutStrategy{Replace: &appsv1alpha1.RolloutStrategyReplace{}}
		factory := testapps.NewRolloutFactory(rolloutKey.Namespace, rolloutKey.Name).SetClusterName(clusterKey.Name)
		if sharding {
			factory.AddSharding(shardingName).SetShardingCompDef(compDefName).SetShardingServiceVersion(serviceVersion2).SetShardingStrategy(strategy)
		} else {
			factory.AddComponent(compName).SetCompCompDef(compDefName).SetCompServiceVersion(serviceVersion2).SetCompStrategy(strategy).SetCompReplicas(replicas)
		}
		rolloutObj := factory.Create(&localTestCtx).GetObject()
		reconcile := func() ctrl.Result {
			r := &RolloutReconciler{Client: localClient, Scheme: localClient.Scheme(), Recorder: record.NewFakeRecorder(100)}
			result, err := r.Reconcile(localCtx, ctrl.Request{NamespacedName: rolloutKey})
			Expect(err).NotTo(HaveOccurred())
			return result
		}
		readCluster := func() *appsv1.Cluster {
			obj := &appsv1.Cluster{}
			Expect(localClient.Get(localCtx, clusterKey, obj)).To(Succeed())
			return obj
		}
		readRollout := func() *appsv1alpha1.Rollout {
			obj := &appsv1alpha1.Rollout{}
			Expect(localClient.Get(localCtx, rolloutKey, obj)).To(Succeed())
			return obj
		}
		specOf := func(obj *appsv1.Cluster) *appsv1.ClusterComponentSpec {
			if sharding {
				return &obj.Spec.Shardings[0].Template
			}
			return &obj.Spec.ComponentSpecs[0]
		}
		history := func(obj *appsv1alpha1.Rollout) []string {
			if sharding {
				return obj.Status.Shardings[0].ScaleDownInstances
			}
			return obj.Status.Components[0].ScaleDownInstances
		}
		originalTemplate := ""
		if template {
			originalTemplate = instanceTemplateName
		}
		pods := createPods([]int32{0, 1}, "")
		pods = append(createPods([]int32{2}, originalTemplate), pods...)
		reconcile()
		reconcile()
		reconcile()
		running()
		reconcile()
		running()
		before := readCluster()
		specOf(before).OfflineInstances = []string{compKey.Name + "-99"}
		Expect(localClient.Update(localCtx, before)).To(Succeed())
		running()
		before = readCluster()
		build := func() *rolloutPlan {
			b := newRolloutPlanBuilder(rolloutctrlutil.RequestCtx{
				Ctx: localCtx, Req: ctrl.Request{NamespacedName: rolloutKey}, Log: logr.Discard(), Recorder: record.NewFakeRecorder(100),
			}, localClient)
			Expect(b.Init()).To(Succeed())
			plan, err := b.AddTransformer(&rolloutDeletionTransformer{}, &rolloutMetaTransformer{}, &rolloutLoadTransformer{}, &rolloutSetupTransformer{}, &rolloutTearDownTransformer{}, &rolloutInplaceTransformer{}, &rolloutReplaceTransformer{}, &rolloutCreateTransformer{}, &rolloutUpdateTransformer{}, &rolloutStatusTransformer{}).Build()
			Expect(err).NotTo(HaveOccurred())
			return plan.(*rolloutPlan)
		}
		bump := func(obj client.Object) {
			Expect(localClient.Get(localCtx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
			labels := obj.GetLabels()
			if labels == nil {
				labels = map[string]string{}
			}
			labels["write-order-conflict"] = rand.String(8)
			obj.SetLabels(labels)
			Expect(localClient.Update(localCtx, obj)).To(Succeed())
		}
		By("rejecting a stale status write before changing Cluster")
		plan := build()
		bump(&appsv1alpha1.Rollout{ObjectMeta: metav1.ObjectMeta{Name: rolloutKey.Name, Namespace: rolloutKey.Namespace}})
		Expect(plan.Execute()).To(Satisfy(apierrors.IsConflict))
		Expect(readCluster().Spec).To(Equal(before.Spec))
		Expect(readCluster().Generation).To(Equal(before.Generation))
		Expect(history(readRollout())).To(BeEmpty())
		By("persisting intent and resuming with a fresh reconciler after Pod recreation")
		reconcile()
		Expect(history(readRollout())).To(Equal([]string{pods[0].Name}))
		Expect(readCluster().Spec).To(Equal(before.Spec))
		selected := pods[0].DeepCopy()
		Expect(localClient.Delete(localCtx, pods[0])).To(Succeed())
		Expect(reconcile().RequeueAfter).To(BeNumerically(">", 0))
		Expect(readCluster().Spec).To(Equal(before.Spec))
		Expect(history(readRollout())).To(Equal([]string{selected.Name}))
		selected.ResourceVersion = ""
		selected.UID = ""
		Expect(localClient.Create(localCtx, selected)).To(Succeed())
		By("retaining intent after a real Cluster conflict")
		plan = build()
		bump(&appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: clusterKey.Name, Namespace: clusterKey.Namespace}})
		Expect(plan.Execute()).To(Satisfy(apierrors.IsConflict))
		Expect(readCluster().Spec).To(Equal(before.Spec))
		Expect(history(readRollout())).To(Equal([]string{selected.Name}))
		By("retaining cleanup history when status conflicts after the Cluster write")
		plan = build()
		bump(&appsv1alpha1.Rollout{ObjectMeta: metav1.ObjectMeta{Name: rolloutKey.Name, Namespace: rolloutKey.Namespace}})
		Expect(plan.Execute()).To(Satisfy(apierrors.IsConflict))
		down := readCluster()
		Expect(specOf(down).Replicas).To(Equal(replicas))
		Expect(down.Generation).To(Equal(before.Generation + 1))
		Expect(specOf(down).OfflineInstances).To(Equal([]string{compKey.Name + "-99", selected.Name}))
		Expect(history(readRollout())).To(Equal([]string{selected.Name}))
		if template {
			Expect(*specOf(down).Instances[0].Replicas).To(Equal(int32(0)))
		}
		reconcile()
		Expect(readCluster().Spec).To(Equal(down.Spec))
		Expect(readCluster().Generation).To(Equal(down.Generation))
		By("completing replacement without losing any owned cleanup entries")
		for i, oldPod := range pods {
			Expect(localClient.Delete(localCtx, oldPod)).To(Succeed())
			tplName := replaceInstanceTemplateNameSuffix(rolloutObj)
			if template && i == 0 {
				tplName = instanceTemplateName + "-" + tplName
			}
			createPods([]int32{int32(3 + i)}, tplName)
			running()
			if i < 2 {
				reconcile()
				running()
				reconcile()
				Expect(specOf(readCluster()).Replicas).To(Equal(replicas + 1))
				Expect(history(readRollout())).To(HaveLen(i + 2))
				reconcile()
				Expect(specOf(readCluster()).Replicas).To(Equal(replicas))
			}
		}
		beforeCleanup := readCluster()
		Expect(specOf(beforeCleanup).OfflineInstances).To(HaveLen(4))
		Expect(history(readRollout())).To(Equal([]string{pods[0].Name, pods[1].Name, pods[2].Name}))
		By("blocking Succeed on a real cleanup Cluster conflict")
		plan = build()
		bump(&appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: clusterKey.Name, Namespace: clusterKey.Namespace}})
		Expect(plan.Execute()).To(Satisfy(apierrors.IsConflict))
		Expect(readCluster().Spec).To(Equal(beforeCleanup.Spec))
		Expect(readRollout().Status.State).NotTo(Equal(appsv1alpha1.SucceedRolloutState))
		By("retrying a terminal status conflict after cleanup has persisted")
		plan = build()
		bump(&appsv1alpha1.Rollout{ObjectMeta: metav1.ObjectMeta{Name: rolloutKey.Name, Namespace: rolloutKey.Namespace}})
		Expect(plan.Execute()).To(Satisfy(apierrors.IsConflict))
		Expect(readRollout().Status.State).NotTo(Equal(appsv1alpha1.SucceedRolloutState))
		cleaned := readCluster()
		Expect(specOf(cleaned).OfflineInstances).To(Equal([]string{compKey.Name + "-99"}))
		running()
		reconcile()
		Expect(readCluster().Spec).To(Equal(cleaned.Spec))
		Expect(readCluster().Generation).To(Equal(cleaned.Generation))

		Expect(specOf(cleaned).Replicas).To(Equal(replicas))
		Expect(cleaned.Generation).To(Equal(beforeCleanup.Generation + 1))
		Expect(readRollout().Status.State).To(Equal(appsv1alpha1.SucceedRolloutState))
	}, Entry("component", false, false), Entry("component instance template", false, true), Entry("sharding", true, false), Entry("sharding instance template", true, true))
})
