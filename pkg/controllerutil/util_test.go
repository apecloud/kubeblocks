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

package controllerutil

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

var _ = Describe("utils test", func() {
	Context("MergeList", func() {
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
			MergeList(&src, &dst, func(v corev1.Volume) func(corev1.Volume) bool {
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
})

func TestGetUncachedObjects(t *testing.T) {
	GetUncachedObjects()
}

func TestRequestCtxMisc(t *testing.T) {
	itFuncs := func(reqCtx *RequestCtx) {
		reqCtx.Event(nil, "type", "reason", "msg")
		reqCtx.Eventf(nil, "type", "reason", "%s", "arg")
		if reqCtx != nil {
			reqCtx.UpdateCtxValue("key", "value")
			reqCtx.WithValue("key", "value")
		}
	}
	itFuncs(nil)
	itFuncs(&RequestCtx{
		Ctx:      context.Background(),
		Recorder: record.NewFakeRecorder(100),
	})
}
