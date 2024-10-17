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

package trace

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	vsv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

var _ = Describe("reconciler_tree test", func() {
	Context("Testing reconciler_tree", func() {
		var (
			mClient        client.Client
			mRecorder      record.EventRecorder
			reconcilerTree ReconcilerTree
		)

		reconcileN := func(n int) error {
			for i := 0; i < n; i++ {
				if err := reconcilerTree.Run(); err != nil {
					return err
				}
			}
			return nil
		}

		BeforeEach(func() {
			i18n := builder.NewConfigMapBuilder(namespace, name).SetData(
				map[string]string{"en": "apps.kubeblocks.io/v1/Component/Creation=Component %s/%s is created."},
			).GetObject()
			store := newChangeCaptureStore(scheme.Scheme, buildDescriptionFormatter(i18n, defaultLocale, nil))
			k8sMock.EXPECT().Scheme().Return(scheme.Scheme).AnyTimes()
			var err error
			mClient, err = newMockClient(k8sMock, store, getKBOwnershipRules())
			Expect(err).ToNot(HaveOccurred())
			mRecorder = newMockEventRecorder(store)

			reconcilerTree, err = newReconcilerTree(ctx, mClient, mRecorder, getKBOwnershipRules())
			Expect(err).ToNot(HaveOccurred())
		})

		It("reconcile with nothing", func() {
			Expect(reconcileN(1)).Should(Succeed())
		})

		It("reconcile a Pod", func() {
			container := corev1.Container{
				Name:  "test",
				Image: "busybox",
			}
			pod := builder.NewPodBuilder(namespace, name).AddContainer(container).GetObject()
			Expect(mClient.Create(ctx, pod)).Should(Succeed())
			Expect(reconcileN(10)).Should(Succeed())
			podRes := &corev1.Pod{}
			Expect(mClient.Get(ctx, client.ObjectKeyFromObject(pod), podRes)).Should(Succeed())
			Expect(podRes.Status.Phase).Should(Equal(corev1.PodRunning))
		})

		It("reconcile PVC&PV", func() {
			resources := corev1.VolumeResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			}
			pvc := builder.NewPVCBuilder(namespace, name).SetResources(resources).GetObject()
			Expect(mClient.Create(ctx, pvc)).Should(Succeed())
			Expect(reconcileN(10)).Should(Succeed())
			pvcRes := &corev1.PersistentVolumeClaim{}
			pvRes := &corev1.PersistentVolume{}
			Expect(mClient.Get(ctx, client.ObjectKeyFromObject(pvc), pvcRes)).Should(Succeed())
			Expect(pvcRes.Status.Phase).Should(Equal(corev1.ClaimBound))
			pvKey := client.ObjectKey{Name: pvc.Name + "-pv"}
			Expect(mClient.Get(ctx, pvKey, pvRes)).Should(Succeed())
			Expect(pvRes.Status.Phase).Should(Equal(corev1.VolumeBound))
		})

		It("reconcile a Job", func() {
			container := corev1.Container{
				Name:  "test",
				Image: "busybox",
			}
			pod := builder.NewPodBuilder(namespace, name).AddContainer(container).GetObject()
			job := builder.NewJobBuilder(namespace, name).SetPodTemplateSpec(corev1.PodTemplateSpec{Spec: pod.Spec}).GetObject()

			By("create the Job")
			Expect(mClient.Create(ctx, job)).Should(Succeed())

			By("verify the Pod been created")
			Expect(reconcileN(1)).Should(Succeed())
			key := client.ObjectKey{Namespace: job.Namespace, Name: job.Name + "-0"}
			Expect(mClient.Get(ctx, key, &corev1.Pod{})).Should(Succeed())

			By("pod succeed")
			Expect(reconcileN(10)).Should(Succeed())
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &corev1.Pod{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *corev1.Pod, _ ...client.GetOption) error {
					return apierrors.NewNotFound(corev1.Resource(constant.PodKind), objKey.Name)
				}).Times(1)
			err := mClient.Get(ctx, key, &corev1.Pod{})
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsNotFound(err)).To(BeTrue())

			By("job succeed")
			Expect(mClient.Get(ctx, client.ObjectKeyFromObject(job), job)).Should(Succeed())
			Expect(job.Status.Succeeded).Should(BeEquivalentTo(1))
		})

		It("reconcile a StatefulSet", func() {
			container := corev1.Container{
				Name:  "test",
				Image: "busybox",
			}
			pod := builder.NewPodBuilder(namespace, name).AddLabels("hello", "world").AddContainer(container).GetObject()
			sts := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				},
				Spec: appsv1.StatefulSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"hello": "world",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: pod.ObjectMeta,
						Spec:       pod.Spec,
					},
					Replicas: pointer.Int32(1),
				},
			}
			Expect(mClient.Create(ctx, sts)).Should(Succeed())
			Expect(reconcileN(10)).Should(Succeed())
			Expect(mClient.Get(ctx, client.ObjectKeyFromObject(sts), sts)).Should(Succeed())
			Expect(sts.Status.ReadyReplicas).Should(BeEquivalentTo(*sts.Spec.Replicas))
		})

		It("reconcile VolumeSnapshot v1", func() {
			reconciler := newVolumeSnapshotV1Reconciler(mClient, mRecorder)
			v1 := &vsv1.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      name,
				},
			}
			Expect(mClient.Create(ctx, v1)).Should(Succeed())

			key := client.ObjectKeyFromObject(v1)
			for i := 0; i < 10; i++ {
				_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: key})
				Expect(err).ToNot(HaveOccurred())
			}
			Expect(mClient.Get(ctx, key, v1)).Should(Succeed())
			Expect(v1.Status).ShouldNot(BeNil())
			Expect(v1.Status.ReadyToUse).ShouldNot(BeNil())
			Expect(*v1.Status.ReadyToUse).Should(BeTrue())
		})
	})
})
