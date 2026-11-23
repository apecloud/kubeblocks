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

package instanceset

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("update reconciler test", func() {
	var replicas int32

	DescribeTable("parse rolling update quotas",
		func(totalReplicas int, replicasValue, maxUnavailableValue intstr.IntOrString, expectedReplicas, expectedMaxUnavailable int) {
			strategy := &workloads.InstanceUpdateStrategy{
				RollingUpdate: &workloads.RollingUpdate{
					Replicas:       &replicasValue,
					MaxUnavailable: &maxUnavailableValue,
				},
			}

			replicas, maxUnavailable, err := parseReplicasNMaxUnavailable(strategy, totalReplicas)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(replicas).Should(Equal(expectedReplicas))
			Expect(maxUnavailable).Should(Equal(expectedMaxUnavailable))
		},
		Entry("rounds replicas up and maxUnavailable down", 3, intstr.FromString("34%"), intstr.FromString("34%"), 2, 1),
		Entry("keeps maxUnavailable at least one", 3, intstr.FromString("10%"), intstr.FromString("10%"), 1, 1),
		Entry("keeps exact percentage results", 4, intstr.FromString("50%"), intstr.FromString("50%"), 2, 2),
		Entry("keeps absolute values", 3, intstr.FromInt32(2), intstr.FromInt32(1), 2, 1),
	)

	Context("reconfigure options", func() {
		It("passes workload reconfigure args as lifecycle arguments", func() {
			args := [][]string{{"maxmemory", "1gb"}, {"timeout", "30"}}
			opts := reconfigureOptions(workloads.ConfigTemplate{ReconfigureArgs: args}, "")
			Expect(opts).ShouldNot(BeNil())
			Expect(opts.Arguments).Should(Equal(args))
		})

		It("uses nil options when reconfigure args are empty", func() {
			Expect(reconfigureOptions(workloads.ConfigTemplate{}, "")).Should(BeNil())
		})
	})

	BeforeEach(func() {
		replicas = 3
		its = builder.NewInstanceSetBuilder(namespace, name).
			SetUID(uid).
			SetReplicas(replicas).
			SetSelectorMatchLabel(selectors).
			SetTemplate(*template.DeepCopy()).
			SetVolumeClaimTemplates(volumeClaimTemplates...).
			SetMinReadySeconds(minReadySeconds).
			GetObject()
	})

	prepareForUpdate := func(tree *kubebuilderx.ObjectTree) {
		By("fix meta")
		reconciler = NewFixMetaReconciler()
		res, err := reconciler.Reconcile(tree)
		Expect(err).Should(BeNil())
		Expect(res).Should(Equal(kubebuilderx.Commit))

		By("update revisions")
		reconciler = NewRevisionUpdateReconciler()
		res, err = reconciler.Reconcile(tree)
		Expect(err).Should(BeNil())
		Expect(res).Should(Equal(kubebuilderx.Continue))

		By("assistant object")
		reconciler = NewAssistantObjectReconciler()
		res, err = reconciler.Reconcile(tree)
		Expect(err).Should(BeNil())
		Expect(res).Should(Equal(kubebuilderx.Continue))

		By("replicas alignment")
		reconciler = NewReplicasAlignmentReconciler()
		res, err = reconciler.Reconcile(tree)
		Expect(err).Should(BeNil())
		Expect(res).Should(Equal(kubebuilderx.Continue))
	}

	expectUpdatedPods := func(tree *kubebuilderx.ObjectTree, names []string) {
		pods := tree.List(&corev1.Pod{})
		Expect(pods).Should(HaveLen(int(replicas) - len(names)))
		for _, name := range names {
			// name should be deleted
			Expect(slices.IndexFunc(pods, func(object client.Object) bool {
				return object.GetName() == name
			})).Should(BeNumerically("<", 0))
		}
	}

	Context("PreCondition & Reconcile", func() {
		getPodReadyCondition := func() corev1.PodCondition {
			return corev1.PodCondition{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * minReadySeconds * time.Second)),
			}
		}

		It("should work well", func() {
			By("PreCondition")
			its.Generation = 1
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			reconciler = NewUpdateReconciler()
			Expect(reconciler.PreCondition(tree)).Should(Equal(kubebuilderx.ConditionSatisfied))

			By("prepare current tree")
			// desired: bar-hello-0, bar-foo-1, bar-foo-0, bar-3, bar-2, bar-1, bar-0
			replicas = int32(7)
			its.Spec.Replicas = &replicas
			its.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
			nameHello := "hello"
			instanceHello := workloads.InstanceTemplate{
				Name: nameHello,
			}
			its.Spec.Instances = append(its.Spec.Instances, instanceHello)
			generateNameFoo := "foo"
			replicasFoo := int32(2)
			instanceFoo := workloads.InstanceTemplate{
				Name:     generateNameFoo,
				Replicas: &replicasFoo,
			}
			its.Spec.Instances = append(its.Spec.Instances, instanceFoo)

			prepareForUpdate(tree)

			By("update all pods to ready with outdated revision")
			pods := tree.List(&corev1.Pod{})
			containersReadyCondition := corev1.PodCondition{
				Type:               corev1.ContainersReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * minReadySeconds * time.Second)),
			}
			makePodAvailableWithOldRevision := func(pod *corev1.Pod) {
				pod.Labels[appsv1.ControllerRevisionHashLabelKey] = "old-revision"
				pod.Status.Phase = corev1.PodRunning
				pod.Status.Conditions = append(pod.Status.Conditions, getPodReadyCondition(), containersReadyCondition)
			}
			for _, object := range pods {
				pod, ok := object.(*corev1.Pod)
				Expect(ok).Should(BeTrue())
				makePodAvailableWithOldRevision(pod)
			}

			makePodLatestRevision := func(pod *corev1.Pod) {
				labels := pod.Labels
				if labels == nil {
					labels = make(map[string]string)
				}
				updateRevisions, err := GetRevisions(its.Status.UpdateRevisions)
				Expect(err).Should(BeNil())
				labels[appsv1.ControllerRevisionHashLabelKey] = updateRevisions[pod.Name]
			}
			reconciler = NewUpdateReconciler()

			By("reconcile with default UpdateStrategy(RollingUpdate, no partition, MaxUnavailable=1)")
			// order: bar-hello-0, bar-foo-1, bar-foo-0, bar-3, bar-2, bar-1, bar-0
			// expected: bar-hello-0 being deleted
			defaultTree, err := tree.DeepCopy()
			Expect(err).Should(BeNil())
			res, err := reconciler.Reconcile(defaultTree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			expectUpdatedPods(defaultTree, []string{"bar-hello-0"})

			By("reconcile with Replicas=3 and MaxUnavailable=2")
			partitionTree, err := tree.DeepCopy()
			Expect(err).Should(BeNil())
			root, ok := partitionTree.GetRoot().(*workloads.InstanceSet)
			Expect(ok).Should(BeTrue())
			updateReplicas := intstr.FromInt32(3)
			maxUnavailable := intstr.FromInt32(2)
			root.Spec.InstanceUpdateStrategy = &workloads.InstanceUpdateStrategy{
				RollingUpdate: &workloads.RollingUpdate{
					Replicas:       &updateReplicas,
					MaxUnavailable: &maxUnavailable,
				},
			}
			// order: bar-hello-0, bar-foo-1, bar-foo-0, bar-3, bar-2, bar-1, bar-0
			// expected: bar-hello-0, bar-foo-1 being deleted
			res, err = reconciler.Reconcile(partitionTree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			expectUpdatedPods(partitionTree, []string{"bar-hello-0", "bar-foo-1"})

			By("update revisions to the updated value")
			partitionTree, err = tree.DeepCopy()
			Expect(err).Should(BeNil())
			root, ok = partitionTree.GetRoot().(*workloads.InstanceSet)
			Expect(ok).Should(BeTrue())
			root.Spec.InstanceUpdateStrategy = &workloads.InstanceUpdateStrategy{
				RollingUpdate: &workloads.RollingUpdate{
					Replicas:       &updateReplicas,
					MaxUnavailable: &maxUnavailable,
				},
			}
			for _, name := range []string{"bar-hello-0", "bar-foo-1"} {
				pod := builder.NewPodBuilder(namespace, name).GetObject()
				object, err := partitionTree.Get(pod)
				Expect(err).Should(BeNil())
				pod, ok = object.(*corev1.Pod)
				Expect(ok).Should(BeTrue())
				makePodLatestRevision(pod)
			}
			res, err = reconciler.Reconcile(partitionTree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			// The first two pods already occupy two positions in the rolling-update
			// window, so only one more pod can be updated.
			expectUpdatedPods(partitionTree, []string{"bar-foo-0"})

			By("reconcile with UpdateStrategy='OnDelete'")
			onDeleteTree, err := tree.DeepCopy()
			Expect(err).Should(BeNil())
			root, ok = onDeleteTree.GetRoot().(*workloads.InstanceSet)
			Expect(ok).Should(BeTrue())
			root.Spec.InstanceUpdateStrategy = &workloads.InstanceUpdateStrategy{
				Type: kbappsv1.OnDeleteStrategyType,
			}
			res, err = reconciler.Reconcile(onDeleteTree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			expectUpdatedPods(onDeleteTree, []string{})

			// order: bar-hello-0, bar-foo-1, bar-foo-0, bar-3, bar-2, bar-1, bar-0
			// expected: bar-hello-0 being deleted
			By("reconcile with PodUpdatePolicy='PreferInPlace'")
			preferInPlaceTree, err := tree.DeepCopy()
			Expect(err).Should(BeNil())
			root, ok = preferInPlaceTree.GetRoot().(*workloads.InstanceSet)
			Expect(ok).Should(BeTrue())
			root.Spec.PodUpdatePolicy = kbappsv1.PreferInPlacePodUpdatePolicyType
			// try to add env to instanceHello to trigger the recreation
			root.Spec.Instances[0].Env = []corev1.EnvVar{
				{
					Name:  "foo",
					Value: "bar",
				},
			}
			res, err = reconciler.Reconcile(preferInPlaceTree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			expectUpdatedPods(preferInPlaceTree, []string{"bar-hello-0"})

			By("reconcile with PodUpdatePolicy='StrictInPlace'")
			strictInPlaceTree, err := tree.DeepCopy()
			Expect(err).Should(BeNil())
			root, ok = strictInPlaceTree.GetRoot().(*workloads.InstanceSet)
			Expect(ok).Should(BeTrue())
			root.Spec.PodUpdatePolicy = kbappsv1.StrictInPlacePodUpdatePolicyType
			// try to add env to instanceHello to trigger the recreation
			root.Spec.Instances[0].Env = []corev1.EnvVar{
				{
					Name:  "foo",
					Value: "bar",
				},
			}
			res, err = reconciler.Reconcile(strictInPlaceTree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			expectUpdatedPods(strictInPlaceTree, []string{})
		})

		It("updates pending pod", func() {
			tree := kubebuilderx.NewObjectTree()
			its.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
			tree.SetRoot(its)

			prepareForUpdate(tree)

			pods := tree.List(&corev1.Pod{})
			Expect(pods).Should(HaveLen(3))
			lastPod := pods[len(pods)-1]
			for i, object := range pods {
				pod, ok := object.(*corev1.Pod)
				Expect(ok).Should(BeTrue())
				// mark the last pod pending with old revision
				if i == len(pods)-1 {
					pod.Labels[appsv1.ControllerRevisionHashLabelKey] = "old-revision"
					pod.Status.Phase = corev1.PodPending
					break
				}
				// mark first two pods available
				pod.Status.Phase = corev1.PodRunning
				pod.Status.Conditions = append(pod.Status.Conditions, getPodReadyCondition())
			}

			reconciler = NewUpdateReconciler()
			res, err := reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			expectUpdatedPods(tree, []string{lastPod.GetName()})
		})

		It("keeps a pending pod outside the rolling-update window untouched", func() {
			tree := kubebuilderx.NewObjectTree()
			its.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
			its.Spec.InstanceUpdateStrategy = &workloads.InstanceUpdateStrategy{
				RollingUpdate: &workloads.RollingUpdate{
					Replicas:       ptr.To(intstr.FromInt32(1)),
					MaxUnavailable: ptr.To(intstr.FromInt32(2)),
				},
			}
			tree.SetRoot(its)

			prepareForUpdate(tree)

			for _, object := range tree.List(&corev1.Pod{}) {
				pod, ok := object.(*corev1.Pod)
				Expect(ok).Should(BeTrue())
				pod.Labels[appsv1.ControllerRevisionHashLabelKey] = "old-revision"
				if pod.Name == "bar-0" {
					pod.Status.Phase = corev1.PodPending
					continue
				}
				pod.Status.Phase = corev1.PodRunning
				pod.Status.Conditions = append(pod.Status.Conditions, getPodReadyCondition())
			}

			reconciler = NewUpdateReconciler()
			res, err := reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			expectUpdatedPods(tree, []string{"bar-2"})

			pending := builder.NewPodBuilder(namespace, "bar-0").GetObject()
			object, err := tree.Get(pending)
			Expect(err).Should(BeNil())
			Expect(object).ShouldNot(BeNil())
			Expect(object.(*corev1.Pod).Status.Phase).Should(Equal(corev1.PodPending))
		})

		It("respects maxUnavailable with pending pods", func() {
			// update order: bar-2, bar-1, bar-0
			tree := kubebuilderx.NewObjectTree()
			its.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
			tree.SetRoot(its)

			prepareForUpdate(tree)

			pods := tree.List(&corev1.Pod{})
			Expect(pods).Should(HaveLen(3))
			for _, object := range pods {
				pod, ok := object.(*corev1.Pod)
				Expect(ok).Should(BeTrue())
				// mark the all pods with old revision and available
				pod.Labels[appsv1.ControllerRevisionHashLabelKey] = "old-revision"
				pod.Status.Phase = corev1.PodRunning
				pod.Status.Conditions = append(pod.Status.Conditions, getPodReadyCondition())
			}

			reconciler = NewUpdateReconciler()
			res, err := reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			expectUpdatedPods(tree, []string{"bar-2"})

			// still, only bar-2 is deleted
			res, err = reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			expectUpdatedPods(tree, []string{"bar-2"})

			// mark pod-2 as pending
			prepareForUpdate(tree)
			pod2 := builder.NewPodBuilder(namespace, "bar-2").GetObject()
			object, err := tree.Get(pod2)
			Expect(err).Should(BeNil())
			pod2, ok := object.(*corev1.Pod)
			Expect(ok).Should(BeTrue())
			pod2.Status.Phase = corev1.PodPending
			Expect(tree.Update(pod2)).Should(BeNil())

			// no pods updated
			reconciler = NewUpdateReconciler()
			res, err = reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			expectUpdatedPods(tree, []string{})

			// mark pod-2 as available
			pod2.Status.Phase = corev1.PodRunning
			pod2.Status.Conditions = append(pod.Status.Conditions, getPodReadyCondition())

			reconciler = NewUpdateReconciler()
			res, err = reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			expectUpdatedPods(tree, []string{"bar-1"})
		})

		testInplacePodVerticalScaling := func(useSubResource bool) {
			oldFeatureGate := viper.GetBool(constant.FeatureGateInPlacePodVerticalScaling)
			defer viper.Set(constant.FeatureGateInPlacePodVerticalScaling, oldFeatureGate)
			viper.Set(constant.FeatureGateInPlacePodVerticalScaling, true)

			// Mock intctrlutil.SupportResizeSubResource
			origSupportResize := intctrlutil.SupportResizeSubResource
			if useSubResource {
				intctrlutil.SupportResizeSubResource = func() (bool, error) { return true, nil }
			} else {
				intctrlutil.SupportResizeSubResource = func() (bool, error) { return false, nil }
			}
			defer func() { intctrlutil.SupportResizeSubResource = origSupportResize }()

			tree := kubebuilderx.NewObjectTree()
			its.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
			its.Spec.Replicas = ptr.To[int32](1)
			tree.SetRoot(its)

			prepareForUpdate(tree)

			pods := tree.List(&corev1.Pod{})
			Expect(pods).Should(HaveLen(1))
			pod := pods[0].(*corev1.Pod)
			// mark available
			pod.Status.Phase = corev1.PodRunning
			pod.Status.Conditions = append(pod.Status.Conditions, getPodReadyCondition())

			its.Spec.Template.Spec.Containers[0].Resources.Limits[corev1.ResourceCPU] = resource.MustParse("1")
			reconciler = NewUpdateReconciler()
			res, err := reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			pods = tree.List(&corev1.Pod{})
			pod = pods[0].(*corev1.Pod)
			Expect(pod.Spec.Containers[0].Resources.Limits[corev1.ResourceCPU]).
				Should(Equal(resource.MustParse("1")))
			_, option, err := tree.GetWithOption(pod)
			Expect(err).NotTo(HaveOccurred())
			if useSubResource {
				Expect(option.SubResource).Should(Equal("resize"))
			} else {
				Expect(option.SubResource).Should(BeEmpty())
			}
		}

		It("inplace updates pod resource", func() {
			testInplacePodVerticalScaling(false)
		})

		It("inplace updates pod resource using resize subresource", func() {
			testInplacePodVerticalScaling(true)
		})

		It("patches pod without calling switchover for metadata-only in-place updates", func() {
			// This test exercises the Reconcile call site (not just the
			// safeMetadataOnlyInPlaceUpdate helper) to assert the contract
			// from PR #10252: when the only difference between the existing
			// pod and the rebuilt pod is non-restart metadata, the pod is
			// patched but the switchover lifecycle action must not be
			// invoked.
			origSupportResize := intctrlutil.SupportResizeSubResource
			intctrlutil.SupportResizeSubResource = func() (bool, error) { return false, nil }
			defer func() { intctrlutil.SupportResizeSubResource = origSupportResize }()

			spy := &lifecycleCallSpy{}
			origNewLifecycleAction := newLifecycleAction
			newLifecycleAction = func(_ *workloads.InstanceSet, _ *kubebuilderx.ObjectTree, _ *corev1.Pod) (lifecycle.Lifecycle, error) {
				return spy, nil
			}
			defer func() { newLifecycleAction = origNewLifecycleAction }()

			tree := kubebuilderx.NewObjectTree()
			its.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
			its.Spec.Replicas = ptr.To[int32](1)
			its.Spec.PodUpdatePolicy = kbappsv1.PreferInPlacePodUpdatePolicyType
			// Configure a switchover action so r.switchover would invoke
			// newLifecycleAction (and thus the spy) if the gate did not
			// suppress it.
			its.Spec.LifecycleActions = &workloads.LifecycleActions{
				Switchover: &kbappsv1.Action{
					Exec: &kbappsv1.ExecAction{Command: []string{"true"}},
				},
			}
			tree.SetRoot(its)

			// Run the revision pipeline against the original template so the
			// generated pod's controller-revision label matches
			// its.Status.UpdateRevisions. This avoids the revision-mismatch
			// branch in getPodUpdatePolicy that would force recreatePolicy.
			prepareForUpdate(tree)

			pods := tree.List(&corev1.Pod{})
			Expect(pods).Should(HaveLen(1))
			pod := pods[0].(*corev1.Pod)
			pod.Status.Phase = corev1.PodRunning
			pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * minReadySeconds * time.Second)),
			})

			// Mutate the template AFTER prepareForUpdate so the rebuilt pod
			// differs from the existing pod only in the config-hash
			// annotation. its.Status.UpdateRevisions is no longer
			// recomputed, so the pod's revision still matches and the
			// reconciler enters the in-place branch via basicUpdate=true.
			if its.Spec.Template.ObjectMeta.Annotations == nil {
				its.Spec.Template.ObjectMeta.Annotations = map[string]string{}
			}
			its.Spec.Template.ObjectMeta.Annotations[constant.CMInsConfigurationHashLabelKey] = "new-hash"

			reconciler = NewUpdateReconciler()
			res, err := reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))

			postPods := tree.List(&corev1.Pod{})
			Expect(postPods).Should(HaveLen(1),
				"pod should be in-place updated, not deleted/recreated")
			updatedPod := postPods[0].(*corev1.Pod)
			Expect(updatedPod.Annotations).Should(HaveKeyWithValue(constant.CMInsConfigurationHashLabelKey, "new-hash"),
				"pod should be patched with the new config-hash annotation even though switchover is skipped")
			_, option, err := tree.GetWithOption(updatedPod)
			Expect(err).NotTo(HaveOccurred())
			Expect(option.Patch).Should(BeTrue(),
				"metadata-only pod updates should use a patch to avoid stale full-update conflicts")
			Expect(spy.switchoverCalls).Should(Equal(0),
				"switchover must not be invoked when only the config-hash annotation differs")
		})

		It("patches pod without calling switchover for KB-managed tools image-only in-place updates", func() {
			oldToolsImage := viper.GetString(constant.KBToolsImage)
			defer viper.Set(constant.KBToolsImage, oldToolsImage)
			viper.Set(constant.KBToolsImage, "docker.io/apecloud/kubeblocks-tools:1.0.0")

			origSupportResize := intctrlutil.SupportResizeSubResource
			intctrlutil.SupportResizeSubResource = func() (bool, error) { return false, nil }
			defer func() { intctrlutil.SupportResizeSubResource = origSupportResize }()

			spy := &lifecycleCallSpy{}
			origNewLifecycleAction := newLifecycleAction
			newLifecycleAction = func(_ *workloads.InstanceSet, _ *kubebuilderx.ObjectTree, _ *corev1.Pod) (lifecycle.Lifecycle, error) {
				return spy, nil
			}
			defer func() { newLifecycleAction = origNewLifecycleAction }()

			its.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
			its.Spec.Replicas = ptr.To[int32](1)
			its.Spec.PodUpdatePolicy = kbappsv1.ReCreatePodUpdatePolicyType
			its.Spec.PodUpgradePolicy = kbappsv1.ReCreatePodUpdatePolicyType
			its.Spec.Template.Spec.ServiceAccountName = "current-sa"
			its.Spec.Template.Spec.InitContainers = append(its.Spec.Template.Spec.InitContainers, corev1.Container{
				Name:    "init-kbagent",
				Image:   "docker.io/apecloud/kubeblocks-tools:1.0.0",
				Command: []string{"cp"},
			})
			its.Spec.Template.Spec.Containers = append(its.Spec.Template.Spec.Containers, corev1.Container{
				Name:    "kbagent",
				Image:   "docker.io/apecloud/kubeblocks-tools:1.0.0",
				Command: []string{kbagent.BinaryPath},
			})
			its.Spec.LifecycleActions = &workloads.LifecycleActions{
				Switchover: &kbappsv1.Action{
					Exec: &kbappsv1.ExecAction{Command: []string{"true"}},
				},
			}

			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			prepareForUpdate(tree)

			pods := tree.List(&corev1.Pod{})
			Expect(pods).Should(HaveLen(1))
			pod := pods[0].(*corev1.Pod)
			pod.UID = "live-pod-uid"
			pod.Spec.AutomountServiceAccountToken = ptr.To(true)
			pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
				Name: "kube-api-access",
				VolumeSource: corev1.VolumeSource{Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{{ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
						Path: "token",
					}}},
				}},
			})
			for i := range pod.Spec.InitContainers {
				if pod.Spec.InitContainers[i].Name == "init-kbagent" {
					pod.Spec.InitContainers[i].ImagePullPolicy = corev1.PullIfNotPresent
					pod.Spec.InitContainers[i].TerminationMessagePath = corev1.TerminationMessagePathDefault
				}
			}
			for i := range pod.Spec.Containers {
				if pod.Spec.Containers[i].Name == "kbagent" {
					pod.Spec.Containers[i].Env = []corev1.EnvVar{{Name: "ADMISSION_INJECTED", Value: "true"}}
					pod.Spec.Containers[i].VolumeMounts = []corev1.VolumeMount{{
						Name:      "kube-api-access",
						MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
						ReadOnly:  true,
					}}
				}
			}
			pod.Status.Phase = corev1.PodRunning
			pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * minReadySeconds * time.Second)),
			})
			if its.Annotations == nil {
				its.Annotations = map[string]string{}
			}
			its.Annotations[constant.ProposedServiceAccountNameAnnotationKey] = "proposed-sa"

			for i := range its.Spec.Template.Spec.InitContainers {
				if its.Spec.Template.Spec.InitContainers[i].Name == "init-kbagent" {
					its.Spec.Template.Spec.InitContainers[i].Image = "mirror.local/apecloud/kubeblocks-tools:1.1.0"
				}
			}
			for i := range its.Spec.Template.Spec.Containers {
				if its.Spec.Template.Spec.Containers[i].Name == "kbagent" {
					its.Spec.Template.Spec.Containers[i].Image = "mirror.local/apecloud/kubeblocks-tools:1.1.0"
				}
			}

			reconciler = NewUpdateReconciler()
			res, err := reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))

			postPods := tree.List(&corev1.Pod{})
			Expect(postPods).Should(HaveLen(1),
				"pod should be in-place updated, not deleted/recreated")
			updatedPod := postPods[0].(*corev1.Pod)
			Expect(updatedPod.UID).Should(Equal(pod.UID), "an image-only update must preserve Pod identity")
			Expect(updatedPod.Spec.ServiceAccountName).Should(Equal("current-sa"),
				"the proposed immutable service account must not be applied by the image patch")
			Expect(updatedPod.Spec.AutomountServiceAccountToken).Should(Equal(ptr.To(true)))
			Expect(updatedPod.Spec.Volumes).Should(ContainElement(HaveField("Name", "kube-api-access")))
			_, updatedInitKBAgent := intctrlutil.GetContainerByName(updatedPod.Spec.InitContainers, "init-kbagent")
			Expect(updatedInitKBAgent).ShouldNot(BeNil())
			Expect(updatedInitKBAgent.Image).Should(Equal("mirror.local/apecloud/kubeblocks-tools:1.1.0"))
			Expect(updatedInitKBAgent.ImagePullPolicy).Should(Equal(corev1.PullIfNotPresent))
			_, updatedKBAgent := intctrlutil.GetContainerByName(updatedPod.Spec.Containers, "kbagent")
			Expect(updatedKBAgent).ShouldNot(BeNil())
			Expect(updatedKBAgent.Image).Should(Equal("mirror.local/apecloud/kubeblocks-tools:1.1.0"))
			Expect(updatedKBAgent.Env).Should(ContainElement(corev1.EnvVar{Name: "ADMISSION_INJECTED", Value: "true"}))
			Expect(updatedKBAgent.VolumeMounts).Should(ContainElement(HaveField("Name", "kube-api-access")))
			_, option, err := tree.GetWithOption(updatedPod)
			Expect(err).NotTo(HaveOccurred())
			Expect(option.Patch).Should(BeTrue(),
				"KB-managed tools image-only pod updates should use a patch")
			Expect(spy.switchoverCalls).Should(Equal(0),
				"switchover must not be invoked when only KB-managed tools images differ")
		})

		It("defers the custom-image init pair while updating the application image in place", func() {
			oldToolsImage := viper.GetString(constant.KBToolsImage)
			defer viper.Set(constant.KBToolsImage, oldToolsImage)
			viper.Set(constant.KBToolsImage, "mirror.local/apecloud/kubeblocks-tools:1.1.0")
			origSupportResize := intctrlutil.SupportResizeSubResource
			intctrlutil.SupportResizeSubResource = func() (bool, error) { return false, nil }
			defer func() { intctrlutil.SupportResizeSubResource = origSupportResize }()

			its.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
			its.Spec.Replicas = ptr.To[int32](1)
			its.Spec.PodUpdatePolicy = kbappsv1.ReCreatePodUpdatePolicyType
			its.Spec.PodUpgradePolicy = kbappsv1.PreferInPlacePodUpdatePolicyType
			its.Spec.Template.Spec.InitContainers = []corev1.Container{{
				Name:    "init-kbagent",
				Image:   "docker.io/apecloud/kubeblocks-tools:1.0.0",
				Command: kbagent.LegacyInitCommand(),
			}}
			its.Spec.Template.Spec.Containers = []corev1.Container{
				{Name: "app", Image: "mysql:8.0"},
				{Name: "kbagent", Image: "custom-action:1.0", Command: []string{kbagent.SharedBinaryPath}},
			}

			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			prepareForUpdate(tree)
			pods := tree.List(&corev1.Pod{})
			Expect(pods).Should(HaveLen(1))
			pod := pods[0].(*corev1.Pod)
			pod.UID = "custom-image-pod-uid"
			pod.Status.Phase = corev1.PodRunning
			pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * minReadySeconds * time.Second)),
			})

			its.Spec.Template.Spec.InitContainers[0].Image = "mirror.local/apecloud/kubeblocks-tools:1.1.0"
			its.Spec.Template.Spec.InitContainers[0].Command = kbagent.InitCommand()
			its.Spec.Template.Spec.Containers[0].Image = "mysql:8.4"
			Expect(its.Spec.Template.Spec.InitContainers[0].Command).Should(Equal(kbagent.InitCommand()),
				"the desired template must retain the new command")

			reconciler := NewUpdateReconciler()
			res, err := reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))

			postPods := tree.List(&corev1.Pod{})
			Expect(postPods).Should(HaveLen(1))
			livePod := postPods[0].(*corev1.Pod)
			Expect(livePod.UID).Should(Equal(pod.UID), "the existing Pod must not be recreated")
			Expect(livePod.Spec.InitContainers[0].Image).Should(Equal("docker.io/apecloud/kubeblocks-tools:1.0.0"))
			Expect(livePod.Spec.InitContainers[0].Command).Should(Equal(kbagent.LegacyInitCommand()),
				"the existing Pod must keep the old image and command as one pair")
			Expect(livePod.Spec.Containers[0].Image).Should(Equal("mysql:8.4"),
				"the application image must still be updated in place")

			itsExt, err := instancetemplate.BuildInstanceSetExt(its, nil)
			Expect(err).ShouldNot(HaveOccurred())
			templates := instancetemplate.BuildInstanceTemplateExt(itsExt)
			Expect(templates).Should(HaveLen(1))
			replacement, err := buildInstancePodByTemplate(pod.Name, templates[0], its, "")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(replacement.Spec.InitContainers[0].Image).Should(Equal("mirror.local/apecloud/kubeblocks-tools:1.1.0"))
			Expect(replacement.Spec.InitContainers[0].Command).Should(Equal(kbagent.InitCommand()),
				"a recreated Pod must copy both kbagent and tini-static")

			if its.Spec.Template.Annotations == nil {
				its.Spec.Template.Annotations = map[string]string{}
			}
			its.Spec.Template.Annotations[constant.RestartAnnotationKey] = "2026-08-11T12:00:00Z"
			_, err = NewRevisionUpdateReconciler().Reconcile(tree)
			Expect(err).ShouldNot(HaveOccurred())
			policy, _, _, err := getPodUpdatePolicyForTest(its, livePod)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(policy).Should(Equal(recreatePolicy), "an explicit restart must still recreate the Pod")
		})

		DescribeTable("keeps recreation policy for revision and application image changes",
			func(mutateTemplate func(*workloads.InstanceSet)) {
				oldToolsImage := viper.GetString(constant.KBToolsImage)
				defer viper.Set(constant.KBToolsImage, oldToolsImage)
				viper.Set(constant.KBToolsImage, "docker.io/apecloud/kubeblocks-tools:1.0.0")

				its.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
				its.Spec.Replicas = ptr.To[int32](1)
				its.Spec.PodUpdatePolicy = kbappsv1.ReCreatePodUpdatePolicyType
				its.Spec.PodUpgradePolicy = kbappsv1.ReCreatePodUpdatePolicyType
				its.Spec.Template.Spec.Containers = append(its.Spec.Template.Spec.Containers, corev1.Container{
					Name:    "kbagent",
					Image:   "docker.io/apecloud/kubeblocks-tools:1.0.0",
					Command: []string{kbagent.BinaryPath},
				})

				tree := kubebuilderx.NewObjectTree()
				tree.SetRoot(its)
				prepareForUpdate(tree)
				mutateTemplate(its)

				res, err := NewRevisionUpdateReconciler().Reconcile(tree)
				Expect(err).Should(BeNil())
				Expect(res).Should(Equal(kubebuilderx.Continue))

				pod := tree.List(&corev1.Pod{})[0].(*corev1.Pod)
				pod.Status.Phase = corev1.PodRunning
				pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
					Type:               corev1.PodReady,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * minReadySeconds * time.Second)),
				})

				res, err = NewUpdateReconciler().Reconcile(tree)
				Expect(err).Should(BeNil())
				Expect(res).Should(Equal(kubebuilderx.Continue))
				Expect(tree.List(&corev1.Pod{})).Should(BeEmpty(), "the Pod must be recreated, not patched")
			},
			Entry("non-image template changes despite a simultaneous tools image update", func(its *workloads.InstanceSet) {
				for i := range its.Spec.Template.Spec.Containers {
					if its.Spec.Template.Spec.Containers[i].Name == "kbagent" {
						its.Spec.Template.Spec.Containers[i].Image = "mirror.local/apecloud/kubeblocks-tools:1.1.0"
						its.Spec.Template.Spec.Containers[i].Env = []corev1.EnvVar{{Name: "TEMPLATE_CHANGE", Value: "true"}}
					}
				}
			}),
			Entry("application image changes", func(its *workloads.InstanceSet) {
				its.Spec.Template.Spec.Containers[0].Image = "example.com/application:new"
			}),
			Entry("a template-owned container is removed while tools images change", func(its *workloads.InstanceSet) {
				its.Spec.Template.Spec.Containers = its.Spec.Template.Spec.Containers[1:]
				its.Spec.Template.Spec.Containers[0].Image = "mirror.local/apecloud/kubeblocks-tools:1.1.0"
			}),
		)
	})
})

// lifecycleCallSpy is a test double for lifecycle.Lifecycle used to assert
// that switchover is or is not invoked during reconciliation. Methods that
// the call-site tests do not exercise return nil to satisfy the interface.
type lifecycleCallSpy struct {
	switchoverCalls  int
	reconfigureCalls int
}

func (s *lifecycleCallSpy) PostProvision(_ context.Context, _ client.Reader, _ *lifecycle.Options) error {
	return nil
}

func (s *lifecycleCallSpy) PreTerminate(_ context.Context, _ client.Reader, _ *lifecycle.Options) error {
	return nil
}

func (s *lifecycleCallSpy) RoleProbe(_ context.Context, _ client.Reader, _ *lifecycle.Options) ([]byte, error) {
	return nil, nil
}

func (s *lifecycleCallSpy) Switchover(_ context.Context, _ client.Reader, _ *lifecycle.Options, _ string) error {
	s.switchoverCalls++
	return nil
}

func (s *lifecycleCallSpy) MemberJoin(_ context.Context, _ client.Reader, _ *lifecycle.Options) error {
	return nil
}

func (s *lifecycleCallSpy) MemberLeave(_ context.Context, _ client.Reader, _ *lifecycle.Options) error {
	return nil
}

func (s *lifecycleCallSpy) Reconfigure(_ context.Context, _ client.Reader, _ *lifecycle.Options, _ map[string]string) error {
	s.reconfigureCalls++
	return nil
}

func (s *lifecycleCallSpy) AccountProvision(_ context.Context, _ client.Reader, _ *lifecycle.Options, _, _, _ string) error {
	return nil
}

func (s *lifecycleCallSpy) UserDefined(_ context.Context, _ client.Reader, _ *lifecycle.Options, _ string, _ *kbappsv1.Action, _ map[string]string) error {
	return nil
}

func TestDirectConfigOverridesUpdateRouting(t *testing.T) {
	cases := []struct {
		name      string
		mutate    func(*workloads.InstanceTemplate)
		wantCalls []string
		deleted   bool
	}{
		{"source switch preserves other bound reload", func(tpl *workloads.InstanceTemplate) {
			tpl.Configs[0].Volume.ConfigMapName = "main-new"
			tpl.Configs[0].ConfigHash = ptr.To("new")
			tpl.Configs[1].ConfigHash = ptr.To("aux-new")
			tpl.Configs[1].Restart = ptr.To(true)
		}, []string{"aux"}, true},
		{"same mount reload then restart", func(tpl *workloads.InstanceTemplate) {
			tpl.Configs[0].ConfigHash = ptr.To("new")
			tpl.Configs[0].Restart = ptr.To(true)
		}, []string{"main"}, true},
		{"same mount reload only", func(tpl *workloads.InstanceTemplate) { tpl.Configs[0].ConfigHash = ptr.To("new") }, []string{"main"}, false},
		{"removed overrides return to inherited mounts", func(tpl *workloads.InstanceTemplate) { tpl.Configs = nil }, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			its, tree, pods, member := newDirectConfigFixture(t, "named")
			spy := installDirectConfigSpy(t)
			mutateDirectTemplate(t, its, tree, tc.mutate)
			its.Generation++
			runDirectReconciler(t, NewRevisionUpdateReconciler(), tree)
			runDirectReconciler(t, NewUpdateReconciler(), tree)
			if !reflect.DeepEqual(spy.configs, tc.wantCalls) {
				t.Fatalf("reload calls %v, want %v", spy.configs, tc.wantCalls)
			}
			obj, err := tree.Get(pods[member])
			if err != nil {
				t.Fatal(err)
			}
			if (obj == nil) != tc.deleted {
				t.Fatalf("deleted=%v, want %v", obj == nil, tc.deleted)
			}
			obj, err = tree.Get(pods["demo-0"])
			if err != nil || obj != pods["demo-0"] {
				t.Fatal("update changed unaffected default member")
			}
			if tc.deleted {
				if spy.switchoverCalls != 1 {
					t.Fatal("recreation bypassed switchover")
				}
				runDirectReconciler(t, NewReplicasAlignmentReconciler(), tree)
				desired, err := instancetemplate.BuildInstanceTemplateExtMap(its, tree)
				if err != nil {
					t.Fatal(err)
				}
				obj, err = tree.Get(pods[member])
				if err != nil || obj == nil {
					t.Fatal("member not recreated")
				}
				recreated := obj.(*corev1.Pod)
				pending, err := configsToUpdate(desired[member].Configs, recreated)
				if err != nil || len(pending) != 0 {
					t.Fatal("recreated member did not start with effective hashes")
				}
				if recreated.Spec.Volumes[0].ConfigMap.Name != desired[member].Spec.Volumes[0].ConfigMap.Name {
					t.Fatal("recreated member has stale mount")
				}
			}
		})
	}
}

func TestDirectConfigOverridePolicyRestrictions(t *testing.T) {
	for _, upgrade := range []bool{false, true} {
		t.Run(map[bool]string{false: "strict update", true: "strict upgrade"}[upgrade], func(t *testing.T) {
			its, tree, pods, member := newDirectConfigFixture(t, "named")
			spy := installDirectConfigSpy(t)
			its.Spec.PodUpdatePolicy = kbappsv1.StrictInPlacePodUpdatePolicyType
			if upgrade {
				its.Spec.PodUpdatePolicy = kbappsv1.PreferInPlacePodUpdatePolicyType
				its.Spec.PodUpgradePolicy = kbappsv1.StrictInPlacePodUpdatePolicyType
			}
			mutateDirectTemplate(t, its, tree, func(tpl *workloads.InstanceTemplate) {
				tpl.Configs[0].Volume.ConfigMapName = "new"
				if upgrade {
					tpl.Images = map[string]string{"db": "mysql:new"}
				}
			})
			runDirectReconciler(t, NewRevisionUpdateReconciler(), tree)
			runDirectReconciler(t, NewUpdateReconciler(), tree)
			obj, err := tree.Get(pods[member])
			if err != nil || obj == nil {
				t.Fatal("strict policy allowed recreation")
			}
			if !meta.IsStatusConditionTrue(its.Status.Conditions, string(workloads.InstanceUpdateRestricted)) {
				t.Fatal("strict policy block was not reported")
			}
			if len(spy.configs) != 0 || spy.switchoverCalls != 0 {
				t.Fatal("strict policy called lifecycle actions")
			}
		})
	}
}

func TestDirectConfigOverrideMemberPlan(t *testing.T) {
	its, tree, pods, member := newDirectConfigFixture(t, "named")
	spy := installDirectConfigSpy(t)
	its.Spec.Roles = []workloads.ReplicaRole{{Name: "follower", UpdatePriority: 1}, {Name: "leader", UpdatePriority: 2}}
	its.Spec.MemberUpdateStrategy = ptr.To(workloads.SerialUpdateStrategy)
	pods[member].Labels[RoleLabelKey] = "follower"
	pods["demo-0"].Labels[RoleLabelKey] = "leader"
	its.Spec.Configs[0].ConfigHash = ptr.To("global-new")
	runDirectReconciler(t, NewRevisionUpdateReconciler(), tree)
	runDirectReconciler(t, NewUpdateReconciler(), tree)
	if !reflect.DeepEqual(spy.configs, []string{"global-main"}) {
		t.Fatalf("member plan used global hashes for overridden member: %v", spy.configs)
	}
}

type directConfigSpy struct {
	lifecycleCallSpy
	configs []string
}

func (s *directConfigSpy) Reconfigure(_ context.Context, _ client.Reader, _ *lifecycle.Options, params map[string]string) error {
	s.configs = append(s.configs, params["id"])
	return nil
}
func installDirectConfigSpy(t *testing.T) *directConfigSpy {
	t.Helper()
	spy := &directConfigSpy{}
	original := newLifecycleAction
	newLifecycleAction = func(_ *workloads.InstanceSet, _ *kubebuilderx.ObjectTree, _ *corev1.Pod) (lifecycle.Lifecycle, error) {
		return spy, nil
	}
	t.Cleanup(func() { newLifecycleAction = original })
	return spy
}

func TestDirectLegacyConfigReloadOnUnrelatedMountChange(t *testing.T) {
	its := legacyInstanceStatusSet(1)
	its.Spec.Instances = nil
	its.Spec.FlatInstanceOrdinal = false
	its.Spec.PodUpdatePolicy = kbappsv1.PreferInPlacePodUpdatePolicyType
	its.Spec.PodUpgradePolicy = kbappsv1.PreferInPlacePodUpdatePolicyType
	action := &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"reload"}}}
	its.Spec.LifecycleActions = &workloads.LifecycleActions{Switchover: action}
	its.Spec.Configs = []workloads.ConfigTemplate{{Name: "aux", ConfigHash: ptr.To("aux-old"), Reconfigure: action, Parameters: map[string]string{"id": "aux"}}}
	its.Spec.Template.Spec.Volumes = []corev1.Volume{
		{Name: "main", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "main-old"}}}},
		{Name: "aux", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "aux"}}}},
	}
	its.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{{Name: "main", MountPath: "/etc/main"}, {Name: "aux", MountPath: "/etc/aux"}}
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(its)
	run := func(rs ...kubebuilderx.Reconciler) {
		t.Helper()
		for _, r := range rs {
			if r.PreCondition(tree) == kubebuilderx.ConditionSatisfied {
				if _, err := r.Reconcile(tree); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	run(NewValidationReconciler(), NewRevisionUpdateReconciler(), NewReplicasAlignmentReconciler())
	objects := tree.List(&corev1.Pod{})
	if len(objects) != 1 {
		t.Fatalf("fixture created %d pods", len(objects))
	}
	pod := objects[0].(*corev1.Pod)
	pod.Status.Phase = corev1.PodRunning
	pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
	run(NewStatusReconciler())
	spy := installDirectConfigSpy(t)
	its.Spec.Template.Spec.Volumes[0].ConfigMap.Name = "main-new"
	its.Spec.Configs[0].ConfigHash = ptr.To("aux-new")
	its.Spec.Configs[0].Restart = ptr.To(true)
	its.Generation++
	run(NewValidationReconciler(), NewStatusReconciler(), NewRevisionUpdateReconciler(), NewUpdateReconciler())
	obj, err := tree.Get(pod)
	if err != nil {
		t.Fatal(err)
	}
	if obj != nil {
		t.Fatal("changed ConfigMap source did not recreate member")
	}
	if !reflect.DeepEqual(spy.configs, []string{"aux"}) {
		t.Fatalf("reloads=%v want inherited aux reload before restart", spy.configs)
	}
}

func TestDirectConfigRollbackPreservesExplicitSameMountReload(t *testing.T) {
	for _, layout := range []string{"named", "compressed flat"} {
		t.Run(layout, func(t *testing.T) {
			its, tree, pods, member := newDirectConfigFixture(t, layout)
			spy := installDirectConfigSpy(t)
			mutateDirectTemplate(t, its, tree, func(tpl *workloads.InstanceTemplate) {
				tpl.Configs = tpl.Configs[1:]
				tpl.Configs[0].ConfigHash = ptr.To("aux-new")
			})
			its.Generation++
			for _, r := range []kubebuilderx.Reconciler{NewValidationReconciler(), NewStatusReconciler(), NewRevisionUpdateReconciler(), NewUpdateReconciler()} {
				if r.PreCondition(tree) == kubebuilderx.ConditionSatisfied {
					runDirectReconciler(t, r, tree)
				}
			}
			if !reflect.DeepEqual(spy.configs, []string{"aux"}) {
				t.Fatalf("calls=%v want aux only", spy.configs)
			}
			if obj, err := tree.Get(pods[member]); err != nil || obj != nil {
				t.Fatalf("source replacement did not delete member: %v %v", obj, err)
			}
		})
	}
}

func TestDirectConfigSourceChangePolicyBoundaries(t *testing.T) {
	for _, policy := range []string{"OnDelete", "Stop", "StrictInPlace"} {
		t.Run(policy, func(t *testing.T) {
			its, tree, pods, member := newDirectConfigFixture(t, "named")
			spy := installDirectConfigSpy(t)
			mutateDirectTemplate(t, its, tree, func(tpl *workloads.InstanceTemplate) {
				tpl.Configs[0].Volume.ConfigMapName = "main-new"
				tpl.Configs[0].ConfigHash = ptr.To("main-new")
			})
			its.Generation++
			switch policy {
			case "OnDelete":
				its.Spec.InstanceUpdateStrategy = &workloads.InstanceUpdateStrategy{Type: kbappsv1.OnDeleteStrategyType}
			case "Stop":
				its.Spec.Stop = ptr.To(true)
			case "StrictInPlace":
				its.Spec.PodUpdatePolicy = kbappsv1.StrictInPlacePodUpdatePolicyType
			}
			for _, r := range []kubebuilderx.Reconciler{NewValidationReconciler(), NewStatusReconciler(), NewRevisionUpdateReconciler(), NewReplicasAlignmentReconciler(), NewUpdateReconciler()} {
				if r.PreCondition(tree) == kubebuilderx.ConditionSatisfied {
					runDirectReconciler(t, r, tree)
				}
			}
			if len(spy.configs) != 0 || spy.switchoverCalls != 0 {
				t.Fatalf("policy executed lifecycle action: configs=%v switchover=%d", spy.configs, spy.switchoverCalls)
			}
			obj, err := tree.Get(pods[member])
			if err != nil || (obj == nil) != (policy == "Stop") {
				t.Fatalf("member existence violates policy: obj=%v err=%v", obj, err)
			}
		})
	}
}

func TestDirectConfigRollbackPreservesInheritedSameMountReload(t *testing.T) {
	for _, layout := range []string{"named", "compressed flat"} {
		t.Run(layout, func(t *testing.T) {
			its, tree, pods, member := newDirectConfigFixture(t, layout)
			// Create the starting member through the production creation path with
			// only A overridden. B has the inherited action and its inherited mount.
			its.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{{Name: "config", MountPath: "/etc/main"}, {Name: "aux", MountPath: "/etc/aux"}}
			mutateDirectTemplate(t, its, tree, func(tpl *workloads.InstanceTemplate) {
				tpl.Configs = tpl.Configs[:1]
			})
			if err := tree.Delete(pods[member]); err != nil {
				t.Fatal(err)
			}
			its.Generation++
			runDirectReconciler(t, NewValidationReconciler(), tree)
			runDirectReconciler(t, NewRevisionUpdateReconciler(), tree)
			runDirectReconciler(t, NewReplicasAlignmentReconciler(), tree)
			obj, err := tree.Get(pods[member])
			if err != nil || obj == nil {
				t.Fatal("starting member creation failed")
			}
			pod := obj.(*corev1.Pod)
			pod.Status.Phase = corev1.PodRunning
			pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
			if pod.Spec.Volumes[0].ConfigMap.Name != "main-a" || pod.Spec.Volumes[1].ConfigMap.Name != "aux" {
				t.Fatal("fixture did not start with overridden A and inherited B mount")
			}
			runDirectReconciler(t, NewStatusReconciler(), tree)
			spy := installDirectConfigSpy(t)
			mutateDirectTemplate(t, its, tree, func(tpl *workloads.InstanceTemplate) { tpl.Configs = nil })
			its.Spec.Configs[1].ConfigHash = ptr.To("aux-new")
			its.Spec.Configs[1].Restart = ptr.To(true)
			its.Generation++
			for _, r := range []kubebuilderx.Reconciler{NewValidationReconciler(), NewStatusReconciler(), NewRevisionUpdateReconciler(), NewUpdateReconciler()} {
				if r.PreCondition(tree) == kubebuilderx.ConditionSatisfied {
					runDirectReconciler(t, r, tree)
				}
			}
			obj, err = tree.Get(pod)
			if err != nil {
				t.Fatal(err)
			}
			if obj != nil {
				t.Fatal("rollback did not recreate member")
			}
			runDirectReconciler(t, NewReplicasAlignmentReconciler(), tree)
			obj, err = tree.Get(pod)
			if err != nil || obj == nil {
				t.Fatal("replacement not created")
			}
			desired, err := instancetemplate.BuildInstanceTemplateExtMap(its, tree)
			if err != nil {
				t.Fatal(err)
			}
			pending, err := configsToUpdate(desired[member].Configs, obj.(*corev1.Pod))
			if err != nil {
				t.Fatal(err)
			}
			if len(pending) != 0 {
				t.Fatal("replacement did not start with desired config hashes")
			}
			if !reflect.DeepEqual(spy.configs, []string{"global-aux"}) {
				t.Fatalf("reloads=%v; want inherited B reload for overridden member before restart", spy.configs)
			}
		})
	}
}

func TestDirectConfigReloadSelectorScope(t *testing.T) {
	for _, tc := range []struct {
		name        string
		selector    kbappsv1.TargetPodSelector
		homogeneous bool
		inherited   bool
		rollback    bool
		scopeOnly   bool
		unmounted   bool
		wantCalls   int
	}{
		{name: "overridden configuration", selector: kbappsv1.AllReplicas, wantCalls: 1},
		{name: "ordinal shared by another template", selector: kbappsv1.OrdinalSelector, wantCalls: 1},
		{name: "inherited configuration with an overridden variant", selector: kbappsv1.AllReplicas, inherited: true, wantCalls: 1},
		{name: "remove last override without changing mounts", selector: kbappsv1.AllReplicas, rollback: true, wantCalls: 1},
		{name: "homogeneous configuration retains selector", selector: kbappsv1.AllReplicas, homogeneous: true, wantCalls: 2},
		{name: "scope alone does not recreate a Pod", selector: kbappsv1.AllReplicas, homogeneous: true, scopeOnly: true},
		{name: "declared ConfigMap volume is not mounted", selector: kbappsv1.AllReplicas, unmounted: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			its, tree, pods, _ := newDirectConfigFixture(t, "named", func(its *workloads.InstanceSet, tpl *workloads.InstanceTemplate) {
				if tc.unmounted {
					its.Spec.Template.Spec.Containers[0].VolumeMounts = nil
				}
				its.Spec.Configs[0].Reconfigure = &kbappsv1.Action{
					Exec:              &kbappsv1.ExecAction{Command: []string{"reload"}},
					TargetPodSelector: tc.selector,
					MatchingKey:       "0",
				}
				tpl.Configs = tpl.Configs[:1]
				tpl.Configs[0].Reconfigure = its.Spec.Configs[0].Reconfigure
				if tc.rollback {
					tpl.Configs[0].Volume = nil
				}
				if tc.homogeneous {
					tpl.Configs = nil
				}
			})
			switch {
			case tc.scopeOnly:
				its.Spec.PodUpdatePolicy = kbappsv1.ReCreatePodUpdatePolicyType
				override := *its.Spec.Configs[0].DeepCopy()
				override.Volume = nil
				override.Parameters = map[string]string{"id": "override"}
				its.Spec.Instances[0].Configs = []workloads.ConfigTemplate{override}
			case tc.rollback:
				its.Spec.Instances[0].Configs = nil
			case tc.homogeneous || tc.inherited:
				its.Spec.Configs[0].ConfigHash = ptr.To("base-new")
			default:
				its.Spec.Instances[0].Configs[0].ConfigHash = ptr.To("override-new")
			}
			agent := &configReloadAgent{}
			kbacli.SetMockClient(agent, nil)
			t.Cleanup(kbacli.UnsetMockClient)

			its.Generation++
			runDirectReconciler(t, NewRevisionUpdateReconciler(), tree)
			runDirectReconciler(t, NewUpdateReconciler(), tree)
			if agent.calls != tc.wantCalls {
				t.Fatalf("reload agent calls = %d, want %d", agent.calls, tc.wantCalls)
			}
			for name, pod := range pods {
				if obj, err := tree.Get(pod); err != nil || obj == nil {
					t.Fatalf("Pod %s was replaced during a same-mount update: %v", name, err)
				}
			}

			if tc.rollback {
				// Finish persisting the restored configuration and then update it
				// as a homogeneous configuration with the original selector.
				runDirectReconciler(t, NewUpdateReconciler(), tree)
				agent.calls = 0
				its.Spec.Configs[0].ConfigHash = ptr.To("base-next")
				its.Generation++
				runDirectReconciler(t, NewRevisionUpdateReconciler(), tree)
				runDirectReconciler(t, NewUpdateReconciler(), tree)
				if agent.calls != 2 {
					t.Fatalf("reload after rollback called %d agents, want both agents from the All selector", agent.calls)
				}
			}
		})
	}
}

func TestDirectConfigReloadRetriesDuringLastOverrideRollback(t *testing.T) {
	its, tree, pods, canaryName := newDirectConfigFixture(t, "named", func(its *workloads.InstanceSet, tpl *workloads.InstanceTemplate) {
		tpl.Configs = tpl.Configs[:1]
		its.Spec.PodUpdatePolicy = kbappsv1.StrictInPlacePodUpdatePolicyType
		its.Spec.LifecycleActions.Switchover = nil
		action := its.Spec.Configs[0].Reconfigure.DeepCopy()
		action.TargetPodSelector = kbappsv1.AllReplicas
		for i := range its.Spec.Configs {
			its.Spec.Configs[i].Reconfigure = action
		}
	})
	const sharedName = "demo-z-0"
	getPod := func(name string) *corev1.Pod {
		t.Helper()
		key := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: its.Namespace, Name: name}}
		object, err := tree.Get(key)
		if err != nil || object == nil {
			t.Fatalf("get Pod %s: object=%v, error=%v", name, object, err)
		}
		return object.(*corev1.Pod)
	}

	publishUpdateRevisions := func() {
		t.Helper()
		its.Generation++
		runDirectReconciler(t, NewRevisionUpdateReconciler(), tree)
	}

	// Give the shared member the first position in the update order.
	its.Spec.Instances = append(its.Spec.Instances, workloads.InstanceTemplate{Name: "z"})
	if err := tree.Delete(pods["demo-0"]); err != nil {
		t.Fatal(err)
	}
	publishUpdateRevisions()
	runDirectReconciler(t, NewReplicasAlignmentReconciler(), tree)
	getPod(sharedName).Status = *getPod(canaryName).Status.DeepCopy()

	agent := &configReloadAgent{}
	kbacli.SetMockClient(agent, nil)
	t.Cleanup(kbacli.UnsetMockClient)

	its.Spec.Instances[0].Configs = nil
	publishUpdateRevisions()
	runDirectReconciler(t, NewUpdateReconciler(), tree)
	if !meta.IsStatusConditionTrue(its.Status.Conditions, string(workloads.InstanceUpdateRestricted)) || agent.calls != 0 {
		t.Fatalf("expected a blocked mount replacement with no reloads, got %d calls", agent.calls)
	}

	// The rollback concerns main. The shared aux configuration still uses All.
	its.Spec.Configs[1].ConfigHash = ptr.To("next-aux")
	publishUpdateRevisions()
	runDirectReconciler(t, NewUpdateReconciler(), tree)
	if agent.calls != 2 {
		t.Fatalf("unrelated shared configuration made %d reloads, want 2", agent.calls)
	}

	oldHash := getPod(sharedName).Annotations[constant.CMInsConfigurationHashLabelKey]
	its.Spec.Configs[0].ConfigHash = ptr.To("next-main")
	publishUpdateRevisions()

	agent.calls = 0
	agent.err = errors.New("shared reload failed")
	if _, err := NewUpdateReconciler().Reconcile(tree); !errors.Is(err, agent.err) {
		t.Fatalf("expected the reload error, got %v", err)
	}
	if agent.calls != 1 {
		t.Fatalf("failed reload made %d calls, want only the shared Pod", agent.calls)
	}
	if getPod(sharedName).Annotations[constant.CMInsConfigurationHashLabelKey] != oldHash {
		t.Fatal("failed reload advanced the observed config hash")
	}

	agent.calls, agent.err = 0, nil
	runDirectReconciler(t, NewUpdateReconciler(), tree)
	if agent.calls != 1 {
		t.Fatalf("retried reload made %d calls, want only the shared Pod", agent.calls)
	}
	if getPod(sharedName).Annotations[constant.CMInsConfigurationHashLabelKey] == oldHash {
		t.Fatal("retried reload did not advance the observed config hash")
	}
	if mounted := getPod(canaryName).Spec.Volumes[0].ConfigMap.Name; mounted != "main-a" {
		t.Fatalf("expected canary to retain main-a during retry, got %q", mounted)
	}

	its.Spec.PodUpdatePolicy = kbappsv1.PreferInPlacePodUpdatePolicyType
	publishUpdateRevisions()
	runDirectReconciler(t, NewUpdateReconciler(), tree)
	runDirectReconciler(t, NewReplicasAlignmentReconciler(), tree)
	canary := getPod(canaryName)
	if mounted := canary.Spec.Volumes[0].ConfigMap.Name; mounted != "main" {
		t.Fatalf("expected canary to restore the shared mount, got %q", mounted)
	}
	canary.Status = *getPod(sharedName).Status.DeepCopy()

	agent.calls = 0
	its.Spec.Configs[0].ConfigHash = ptr.To("main-after-rollback")
	publishUpdateRevisions()
	runDirectReconciler(t, NewUpdateReconciler(), tree)
	if agent.calls != 2 {
		t.Fatalf("reload after rollback made %d calls, want both Pods from the All selector", agent.calls)
	}
}

type configReloadAgent struct {
	calls int
	err   error
}

func (a *configReloadAgent) Close() error { return nil }

func (a *configReloadAgent) Action(context.Context, proto.ActionRequest) (proto.ActionResponse, error) {
	a.calls++
	return proto.ActionResponse{}, a.err
}
