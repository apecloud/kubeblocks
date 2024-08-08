/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("instance util test", func() {
	Context("filterInPlaceFields", func() {
		It("should work well", func() {
			pod := buildRandomPod()
			restartTime := (metav1.Time{Time: time.Now()}).Format(time.RFC3339)
			pod.Annotations[constant.RestartAnnotationKey] = restartTime
			reconfigureKey := "config.kubeblocks.io/restart-foo-bar-config"
			reconfigureValue := "7cdb79ffdb"
			pod.Annotations[reconfigureKey] = reconfigureValue
			podTemplate := &corev1.PodTemplateSpec{
				ObjectMeta: pod.ObjectMeta,
				Spec:       pod.Spec,
			}

			result := filterInPlaceFields(podTemplate)
			Expect(result.Annotations).Should(HaveKey(constant.RestartAnnotationKey))
			Expect(result.Annotations[constant.RestartAnnotationKey]).Should(Equal(restartTime))
			Expect(result.Annotations).Should(HaveKey(reconfigureKey))
			Expect(result.Annotations[reconfigureKey]).Should(Equal(reconfigureValue))
			Expect(result.Labels).Should(BeNil())
			Expect(result.Spec.ActiveDeadlineSeconds).Should(BeNil())
			Expect(result.Spec.Tolerations).Should(BeNil())
			Expect(result.Spec.InitContainers).Should(HaveLen(1))
			Expect(result.Spec.InitContainers[0].Image).Should(BeEmpty())
			Expect(result.Spec.Containers).Should(HaveLen(1))
			Expect(result.Spec.Containers[0].Image).Should(BeEmpty())
			Expect(result.Spec.Containers[0].Resources.Requests).ShouldNot(HaveKey(corev1.ResourceCPU))
			Expect(result.Spec.Containers[0].Resources.Requests).ShouldNot(HaveKey(corev1.ResourceMemory))
			Expect(result.Spec.Containers[0].Resources.Limits).ShouldNot(HaveKey(corev1.ResourceCPU))
			Expect(result.Spec.Containers[0].Resources.Limits).ShouldNot(HaveKey(corev1.ResourceMemory))
		})
	})

	Context("mergeInPlaceFields & equalXFields", func() {
		It("should work well", func() {
			ignorePodVerticalScaling := viper.GetBool(FeatureGateIgnorePodVerticalScaling)
			defer viper.Set(FeatureGateIgnorePodVerticalScaling, ignorePodVerticalScaling)
			viper.Set(FeatureGateIgnorePodVerticalScaling, false)

			By("build new pod without tolerations")
			oldPod := buildRandomPod()
			newPod := buildRandomPod()
			newPod.Spec.Tolerations = nil
			mergeInPlaceFields(newPod, oldPod)
			Expect(equalBasicInPlaceFields(oldPod, newPod)).Should(BeTrue())
			Expect(equalResourcesInPlaceFields(oldPod, newPod)).Should(BeTrue())

			By("build new pod with tolerations")
			oldPod = buildRandomPod()
			newPod = buildRandomPod()
			newPod.Spec.Tolerations = append(newPod.Spec.Tolerations, oldPod.Spec.Tolerations...)
			mergeInPlaceFields(newPod, oldPod)
			Expect(equalBasicInPlaceFields(oldPod, newPod)).Should(BeTrue())
		})
	})

	Context("getPodUpdatePolicy", func() {
		It("should work well", func() {
			By("build an updated pod")
			randStr := rand.String(16)
			key := randStr
			podTemplate := template.DeepCopy()
			mergeMap(&map[string]string{key: randStr}, &podTemplate.Annotations)
			mergeMap(&map[string]string{key: randStr}, &podTemplate.Labels)
			its = builder.NewInstanceSetBuilder(namespace, name).
				SetUID(uid).
				AddAnnotations(randStr, randStr).
				AddLabels(randStr, randStr).
				SetReplicas(3).
				AddMatchLabelsInMap(selectors).
				SetTemplate(*podTemplate).
				SetVolumeClaimTemplates(volumeClaimTemplates...).
				SetMinReadySeconds(minReadySeconds).
				SetRoles(roles).
				SetPodManagementPolicy(appsv1.ParallelPodManagement).
				GetObject()
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			var reconciler kubebuilderx.Reconciler
			By("update revisions")
			reconciler = NewRevisionUpdateReconciler()
			res, err := reconciler.Reconcile(tree)
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
			objects := tree.List(&corev1.Pod{})
			Expect(objects).Should(HaveLen(3))
			pod1, ok := objects[0].(*corev1.Pod)
			Expect(ok).Should(BeTrue())
			policy, err := getPodUpdatePolicy(its, pod1)
			Expect(err).Should(BeNil())
			Expect(policy).Should(Equal(NoOpsPolicy))

			By("build a pod with revision updated")
			pod2 := pod1.DeepCopy()
			pod2.Spec.Containers = append(pod2.Spec.Containers, corev1.Container{
				Name:  "foo2",
				Image: "bar2",
				Ports: []corev1.ContainerPort{
					{
						Name:          "my-svc",
						Protocol:      corev1.ProtocolTCP,
						ContainerPort: 54321,
					},
				},
			})
			pod2.Labels[appsv1.ControllerRevisionHashLabelKey] = "new-revision"
			its.Status.UpdateRevisions[pod2.Name] = getPodRevision(pod2)
			policy, err = getPodUpdatePolicy(its, pod2)
			Expect(err).Should(BeNil())
			Expect(policy).Should(Equal(RecreatePolicy))

			By("build a pod without revision updated, with basic mutable fields updated")
			pod3 := pod1.DeepCopy()
			randStr = rand.String(16)
			mergeMap(&map[string]string{key: randStr}, &pod3.Annotations)
			policy, err = getPodUpdatePolicy(its, pod3)
			Expect(err).Should(BeNil())
			Expect(policy).Should(Equal(InPlaceUpdatePolicy))

			By("build a pod without revision updated, with basic mutable and resources fields updated")
			pod4 := pod3.DeepCopy()
			randInt := rand.Int()
			requests := corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse(fmt.Sprintf("%dm", randInt)),
			}
			pod4.Spec.Containers[0].Resources.Requests = requests
			policy, err = getPodUpdatePolicy(its, pod4)
			Expect(err).Should(BeNil())
			Expect(policy).Should(Equal(RecreatePolicy))

			By("build a pod without revision updated, with resources fields updated")
			pod5 := pod1.DeepCopy()
			randInt = rand.Int()
			requests = corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse(fmt.Sprintf("%dm", randInt)),
			}
			pod5.Spec.Containers[0].Resources.Requests = requests
			policy, err = getPodUpdatePolicy(its, pod5)
			Expect(err).Should(BeNil())
			Expect(policy).Should(Equal(RecreatePolicy))

			By("build a pod without revision updated, with resources fields updated, with IgnorePodVerticalScaling enabled")
			ignorePodVerticalScaling := viper.GetBool(FeatureGateIgnorePodVerticalScaling)
			defer viper.Set(FeatureGateIgnorePodVerticalScaling, ignorePodVerticalScaling)
			viper.Set(FeatureGateIgnorePodVerticalScaling, true)
			policy, err = getPodUpdatePolicy(its, pod5)
			Expect(err).Should(BeNil())
			Expect(policy).Should(Equal(NoOpsPolicy))

			By("build a pod without revision updated, with IgnorePodVerticalScaling disabled")
			pod6 := pod1.DeepCopy()
			pod6.Spec.Containers = append(pod6.Spec.Containers, corev1.Container{
				Name:  "sidecar1",
				Image: "bar2",
				Ports: []corev1.ContainerPort{
					{
						Name:          "my-svc",
						Protocol:      corev1.ProtocolTCP,
						ContainerPort: 54321,
					},
				},
			})
			randStr = rand.String(16)
			mergeMap(&map[string]string{key: randStr}, &pod6.Annotations)
			ignorePodVerticalScaling = viper.GetBool(FeatureGateIgnorePodVerticalScaling)
			defer viper.Set(FeatureGateIgnorePodVerticalScaling, ignorePodVerticalScaling)
			viper.Set(FeatureGateIgnorePodVerticalScaling, false)
			policy, err = getPodUpdatePolicy(its, pod6)
			Expect(err).Should(BeNil())
			Expect(policy).Should(Equal(InPlaceUpdatePolicy))
		})
	})
})
