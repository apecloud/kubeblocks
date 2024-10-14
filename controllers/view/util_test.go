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

package view

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	viewv1 "github.com/apecloud/kubeblocks/apis/view/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

var _ = Describe("util test", func() {
	Context("objectTypeToGVK", func() {
		It("should work well", func() {
			objType := &viewv1.ObjectType{
				APIVersion: viewv1.SchemeBuilder.GroupVersion.String(),
				Kind:       viewv1.Kind,
			}
			gvk, err := objectTypeToGVK(objType)
			Expect(err).Should(BeNil())
			Expect(*gvk).Should(Equal(schema.GroupVersionKind{
				Group:   viewv1.GroupVersion.Group,
				Version: viewv1.GroupVersion.Version,
				Kind:    viewv1.Kind,
			}))

			objType = nil
			gvk, err = objectTypeToGVK(objType)
			Expect(err).Should(BeNil())
			Expect(gvk).Should(BeNil())
		})
	})

	Context("objectReferenceToType", func() {
		It("should work well", func() {
			ref := &corev1.ObjectReference{
				APIVersion: viewv1.SchemeBuilder.GroupVersion.String(),
				Kind:       viewv1.Kind,
			}
			objType := objectReferenceToType(ref)
			Expect(objType).ShouldNot(BeNil())
			Expect(*objType).Should(Equal(viewv1.ObjectType{
				APIVersion: viewv1.SchemeBuilder.GroupVersion.String(),
				Kind:       viewv1.Kind,
			}))

			ref = nil
			objType = objectReferenceToType(ref)
			Expect(objType).Should(BeNil())
		})
	})

	Context("objectReferenceToRef", func() {
		It("should work well", func() {
			ref := &corev1.ObjectReference{
				APIVersion: viewv1.SchemeBuilder.GroupVersion.String(),
				Kind:       viewv1.Kind,
				Namespace:  "foo",
				Name:       "bar",
			}
			objRef := objectReferenceToRef(ref)
			Expect(objRef).ShouldNot(BeNil())
			Expect(*objRef).Should(Equal(model.GVKNObjKey{
				GroupVersionKind: schema.GroupVersionKind{
					Group:   viewv1.GroupVersion.Group,
					Version: viewv1.GroupVersion.Version,
					Kind:    viewv1.Kind,
				},
				ObjectKey: client.ObjectKey{
					Namespace: "foo",
					Name:      "bar",
				},
			}))

			ref = nil
			objRef = objectReferenceToRef(ref)
			Expect(objRef).Should(BeNil())
		})
	})

	Context("objectRefToReference", func() {
		It("should work well", func() {
			objectRef := model.GVKNObjKey{
				GroupVersionKind: schema.GroupVersionKind{
					Group:   viewv1.GroupVersion.Group,
					Version: viewv1.GroupVersion.Version,
					Kind:    viewv1.Kind,
				},
				ObjectKey: client.ObjectKey{
					Namespace: "foo",
					Name:      "bar",
				},
			}
			uid := uuid.NewUUID()
			resourceVersion := "123456"
			ref := objectRefToReference(objectRef, uid, resourceVersion)
			Expect(ref).ShouldNot(BeNil())
			Expect(*ref).Should(Equal(corev1.ObjectReference{
				APIVersion:      viewv1.SchemeBuilder.GroupVersion.String(),
				Kind:            viewv1.Kind,
				Namespace:       "foo",
				Name:            "bar",
				UID:             uid,
				ResourceVersion: resourceVersion,
			}))
		})
	})

	Context("objectRefToType", func() {
		It("should work well", func() {
			objectRef := &model.GVKNObjKey{
				GroupVersionKind: schema.GroupVersionKind{
					Group:   viewv1.GroupVersion.Group,
					Version: viewv1.GroupVersion.Version,
					Kind:    viewv1.Kind,
				},
				ObjectKey: client.ObjectKey{
					Namespace: "foo",
					Name:      "bar",
				},
			}
			t := objectRefToType(objectRef)
			Expect(t).ShouldNot(BeNil())
			Expect(*t).Should(Equal(viewv1.ObjectType{
				APIVersion: viewv1.SchemeBuilder.GroupVersion.String(),
				Kind:       viewv1.Kind,
			}))
			objectRef = nil
			t = objectRefToType(objectRef)
			Expect(t).Should(BeNil())
		})
	})

	Context("getObjectRef", func() {
		It("should work well", func() {

		})
	})
})
