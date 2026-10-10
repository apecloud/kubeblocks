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

package workloads

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/generics"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("InstanceSet Stop path parity", func() {
	for _, enableInstanceAPI := range []bool{false, true} {
		It(fmt.Sprintf("preserves data and resumes the latest template with enableInstanceAPI=%t", enableInstanceAPI), func() {
			factory := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace, "stop-parity", "stop-parity", "db").
				WithRandomName().
				AddContainer(corev1.Container{Name: "db", Image: "db:v1"}).
				SetReplicas(1).
				SetEnableInstanceAPI(ptr.To(enableInstanceAPI)).
				SetPodManagementPolicy(appsv1.ParallelPodManagement).
				AddVolumeClaimTemplate(corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{Name: "data"},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1Gi"),
						}},
					},
				})
			factory.Get().Spec.MinReadySeconds = 3600
			its := factory.Create(&testCtx).GetObject()
			itsKey := client.ObjectKeyFromObject(its)
			podKey := client.ObjectKey{Namespace: its.Namespace, Name: its.Name + "-0"}
			pvcKey := client.ObjectKey{Namespace: its.Namespace, Name: "data-" + podKey.Name}
			DeferCleanup(func() {
				Eventually(func() error {
					return client.IgnoreNotFound(testapps.GetAndChangeObj(&testCtx, podKey, func(p *corev1.Pod) { p.Finalizers = nil })())
				}).Should(Succeed())
				inNS := client.InNamespace(its.Namespace)
				ml := client.MatchingLabels{constant.AppInstanceLabelKey: "stop-parity"}
				testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.InstanceSetSignature, true, inNS, ml)
				testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.InstanceSignature, true, inNS, ml)
				testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PodSignature, true, inNS, ml)
				testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS, ml)
				testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ServiceSignature, true, inNS, ml)
			})

			pod := &corev1.Pod{}
			claim := &corev1.PersistentVolumeClaim{}
			Eventually(func() error { return k8sClient.Get(ctx, podKey, pod) }).Should(Succeed())
			Eventually(func() error { return k8sClient.Get(ctx, pvcKey, claim) }).Should(Succeed())
			oldPodUID, pvcUID := pod.UID, claim.UID
			Eventually(testapps.GetAndChangeObj(&testCtx, podKey, func(p *corev1.Pod) {
				p.Finalizers = append(p.Finalizers, "test.kubeblocks.io/stop-parity")
			})).Should(Succeed())
			Eventually(func() error { return k8sClient.Get(ctx, pvcKey, claim) }).Should(Succeed())
			pvcVersion, pvcSpec, pvcOwners := claim.ResourceVersion, claim.Spec.DeepCopy(), claim.OwnerReferences

			Eventually(testapps.GetAndChangeObj(&testCtx, itsKey, func(set *workloads.InstanceSet) {
				set.Spec.Stop = ptr.To(true)
			})).Should(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, podKey, pod)).Should(Succeed())
				g.Expect(pod.DeletionTimestamp.IsZero()).Should(BeFalse())
				g.Expect(k8sClient.Get(ctx, itsKey, its)).Should(Succeed())
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(its.Status.InstanceStatus[0].CurrentState).Should(Equal(workloads.InstanceCurrentStateTerminating))
				g.Expect(its.Status.InstanceStatus[0].UpToDate).Should(BeFalse())
				g.Expect(its.Status.Replicas).Should(Equal(int32(1)))
			}).Should(Succeed())

			Eventually(testapps.GetAndChangeObj(&testCtx, itsKey, func(set *workloads.InstanceSet) {
				set.Spec.Template.Spec.Containers[0].Image = "db:v2"
				set.Spec.VolumeClaimTemplates = nil
			})).Should(Succeed())
			Consistently(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, pvcKey, claim)).Should(Succeed())
				g.Expect(claim.ResourceVersion).Should(Equal(pvcVersion))
				g.Expect(claim.Spec).Should(Equal(*pvcSpec))
				g.Expect(claim.OwnerReferences).Should(Equal(pvcOwners))
			}, time.Second).Should(Succeed())

			Eventually(testapps.GetAndChangeObj(&testCtx, itsKey, func(set *workloads.InstanceSet) {
				set.Spec.Stop = nil
				set.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{{
					ObjectMeta: metav1.ObjectMeta{Name: "data", Annotations: map[string]string{"test.kubeblocks.io/resume": "latest"}},
					Spec:       *pvcSpec,
				}}
			})).Should(Succeed())
			Consistently(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, podKey, pod)).Should(Succeed())
				g.Expect(pod.UID).Should(Equal(oldPodUID))
				g.Expect(pod.Spec.Containers[0].Image).Should(Equal("db:v1"))
				g.Expect(k8sClient.Get(ctx, pvcKey, claim)).Should(Succeed())
				g.Expect(claim.ResourceVersion).Should(Equal(pvcVersion))
			}, time.Second).Should(Succeed())
			Eventually(testapps.GetAndChangeObj(&testCtx, podKey, func(p *corev1.Pod) { p.Finalizers = nil })).Should(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, podKey, pod)).Should(Succeed())
				g.Expect(pod.UID).ShouldNot(Equal(oldPodUID))
				g.Expect(pod.Spec.Containers[0].Image).Should(Equal("db:v2"))
				g.Expect(k8sClient.Get(ctx, pvcKey, claim)).Should(Succeed())
				g.Expect(claim.UID).Should(Equal(pvcUID))
				g.Expect(claim.Spec).Should(Equal(*pvcSpec))
				g.Expect(claim.OwnerReferences).Should(Equal(pvcOwners))
				g.Expect(claim.Annotations["test.kubeblocks.io/resume"]).Should(Equal("latest"))
			}).Should(Succeed())

			Eventually(testapps.GetAndChangeObj(&testCtx, itsKey, func(set *workloads.InstanceSet) { set.Spec.Stop = ptr.To(true) })).Should(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, podKey, &corev1.Pod{}))).Should(BeTrue())
				g.Expect(k8sClient.Get(ctx, itsKey, its)).Should(Succeed())
				g.Expect(its.Status.Replicas).Should(BeZero())
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(its.Status.InstanceStatus[0].CurrentState).Should(Equal(workloads.InstanceCurrentStateAbsent))
				g.Expect(k8sClient.Get(ctx, pvcKey, claim)).Should(Succeed())
				g.Expect(claim.UID).Should(Equal(pvcUID))
			}).Should(Succeed())
		})
	}
})
