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
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("utils test", func() {
	Context("mergeList", func() {
		It("should work well", func() {
			src := []corev1.Volume{
				{
					Name: "pvc1",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "pvc1-pod-0",
						},
					},
				},
				{
					Name: "pvc2",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "pvc2-pod-0",
						},
					},
				},
			}
			dst := []corev1.Volume{
				{
					Name: "pvc0",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "pvc0-pod-0",
						},
					},
				},
				{
					Name: "pvc1",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "pvc-pod-0",
						},
					},
				},
			}
			mergeList(&src, &dst, func(v corev1.Volume) func(corev1.Volume) bool {
				return func(volume corev1.Volume) bool {
					return v.Name == volume.Name
				}
			})

			Expect(dst).Should(HaveLen(3))
			slices.SortStableFunc(dst, func(a, b corev1.Volume) bool {
				return a.Name < b.Name
			})
			Expect(dst[0].Name).Should(Equal("pvc0"))
			Expect(dst[1].Name).Should(Equal("pvc1"))
			Expect(dst[1].PersistentVolumeClaim).ShouldNot(BeNil())
			Expect(dst[1].PersistentVolumeClaim.ClaimName).Should(Equal("pvc1-pod-0"))
			Expect(dst[2].Name).Should(Equal("pvc2"))
		})
	})

	Context("mergeMap", func() {
		It("should work well", func() {
			src := map[string]string{
				"foo1": "bar1",
				"foo2": "bar2",
			}
			dst := map[string]string{
				"foo0": "bar0",
				"foo1": "bar",
			}
			mergeMap(&src, &dst)

			Expect(dst).Should(HaveLen(3))
			Expect(dst).Should(HaveKey("foo0"))
			Expect(dst).Should(HaveKey("foo1"))
			Expect(dst).Should(HaveKey("foo2"))
			Expect(dst["foo1"]).Should(Equal("bar1"))
		})
	})

	Context("CurrentProvider", func() {
		It("should work well", func() {
			if viper.IsSet(FeatureGateRSMReplicaProvider) {
				provider := viper.GetString(FeatureGateRSMReplicaProvider)
				defer func() {
					viper.Set(FeatureGateRSMReplicaProvider, provider)
				}()
			}

			controller, k8sMock := testutil.SetupK8sMock()
			defer controller.Finish()
			root := builder.NewStatefulSetBuilder("foo", "bar").GetObject()

			By("No StatefulSet found")
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &appsv1.StatefulSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *appsv1.StatefulSet, _ ...client.GetOption) error {
					return apierrors.NewNotFound(schema.GroupResource{}, "bar")
				}).Times(1)
			provider, err := CurrentReplicaProvider(context.Background(), k8sMock, client.ObjectKeyFromObject(root))
			Expect(err).Should(BeNil())
			Expect(provider).Should(Equal(defaultReplicaProvider))

			By("ReplicaProvider set")
			viper.Set(FeatureGateRSMReplicaProvider, string(StatefulSetProvider))
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &appsv1.StatefulSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *appsv1.StatefulSet, _ ...client.GetOption) error {
					return apierrors.NewNotFound(schema.GroupResource{}, "bar")
				}).Times(1)
			provider, err = CurrentReplicaProvider(context.Background(), k8sMock, client.ObjectKeyFromObject(root))
			Expect(err).Should(BeNil())
			Expect(provider).Should(Equal(StatefulSetProvider))

			By("StatefulSet found")
			viper.Set(FeatureGateRSMReplicaProvider, string(PodProvider))
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &appsv1.StatefulSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *appsv1.StatefulSet, _ ...client.GetOption) error {
					*obj = *root
					return nil
				}).Times(1)
			provider, err = CurrentReplicaProvider(context.Background(), k8sMock, client.ObjectKeyFromObject(root))
			Expect(err).Should(BeNil())
			Expect(provider).Should(Equal(StatefulSetProvider))
		})
	})
})
