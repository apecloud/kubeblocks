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
	"slices"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("update reconciler test", func() {
	var replicas int32

	Context("reconfigure options", func() {
		It("passes workload reconfigure args as lifecycle arguments", func() {
			args := [][]string{{"maxmemory", "1gb"}, {"timeout", "30"}}
			opts := reconfigureOptions(workloads.ConfigTemplate{ReconfigureArgs: args})
			Expect(opts).ShouldNot(BeNil())
			Expect(opts.Arguments).Should(Equal(args))
		})

		It("uses nil options when reconfigure args are empty", func() {
			Expect(reconfigureOptions(workloads.ConfigTemplate{})).Should(BeNil())
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

			By("reconcile with Partition=50% and MaxUnavailable=2")
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
			expectUpdatedPods(partitionTree, []string{"bar-foo-0", "bar-3"})

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
			its.Spec.Template.Spec.Containers = append(its.Spec.Template.Spec.Containers, corev1.Container{
				Name:    "kbagent",
				Image:   "docker.io/apecloud/kubeblocks-tools:1.0.0",
				Command: []string{"/bin/kbagent"},
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
			pod.Status.Phase = corev1.PodRunning
			pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * minReadySeconds * time.Second)),
			})

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
			_, updatedKBAgent := intctrlutil.GetContainerByName(updatedPod.Spec.Containers, "kbagent")
			Expect(updatedKBAgent).ShouldNot(BeNil())
			Expect(updatedKBAgent.Image).Should(Equal("mirror.local/apecloud/kubeblocks-tools:1.1.0"))
			_, option, err := tree.GetWithOption(updatedPod)
			Expect(err).NotTo(HaveOccurred())
			Expect(option.Patch).Should(BeTrue(),
				"KB-managed tools image-only pod updates should use a patch")
			Expect(spy.switchoverCalls).Should(Equal(0),
				"switchover must not be invoked when only KB-managed tools images differ")
		})
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
