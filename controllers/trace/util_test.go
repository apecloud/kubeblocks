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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	tracev1 "github.com/apecloud/kubeblocks/apis/trace/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

var _ = Describe("util test", func() {
	Context("objectTypeToGVK", func() {
		It("should work well", func() {
			objType := &tracev1.ObjectType{
				APIVersion: tracev1.SchemeBuilder.GroupVersion.String(),
				Kind:       tracev1.Kind,
			}
			gvk, err := objectTypeToGVK(objType)
			Expect(err).Should(BeNil())
			Expect(*gvk).Should(Equal(schema.GroupVersionKind{
				Group:   tracev1.GroupVersion.Group,
				Version: tracev1.GroupVersion.Version,
				Kind:    tracev1.Kind,
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
				APIVersion: tracev1.SchemeBuilder.GroupVersion.String(),
				Kind:       tracev1.Kind,
			}
			objType := objectReferenceToType(ref)
			Expect(objType).ShouldNot(BeNil())
			Expect(*objType).Should(Equal(tracev1.ObjectType{
				APIVersion: tracev1.SchemeBuilder.GroupVersion.String(),
				Kind:       tracev1.Kind,
			}))

			ref = nil
			objType = objectReferenceToType(ref)
			Expect(objType).Should(BeNil())
		})
	})

	Context("objectReferenceToRef", func() {
		It("should work well", func() {
			ref := &corev1.ObjectReference{
				APIVersion: tracev1.SchemeBuilder.GroupVersion.String(),
				Kind:       tracev1.Kind,
				Namespace:  "foo",
				Name:       "bar",
			}
			objRef := objectReferenceToRef(ref)
			Expect(objRef).ShouldNot(BeNil())
			Expect(*objRef).Should(Equal(model.GVKNObjKey{
				GroupVersionKind: schema.GroupVersionKind{
					Group:   tracev1.GroupVersion.Group,
					Version: tracev1.GroupVersion.Version,
					Kind:    tracev1.Kind,
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
					Group:   tracev1.GroupVersion.Group,
					Version: tracev1.GroupVersion.Version,
					Kind:    tracev1.Kind,
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
				APIVersion:      tracev1.SchemeBuilder.GroupVersion.String(),
				Kind:            tracev1.Kind,
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
					Group:   tracev1.GroupVersion.Group,
					Version: tracev1.GroupVersion.Version,
					Kind:    tracev1.Kind,
				},
				ObjectKey: client.ObjectKey{
					Namespace: "foo",
					Name:      "bar",
				},
			}
			t := objectRefToType(objectRef)
			Expect(t).ShouldNot(BeNil())
			Expect(*t).Should(Equal(tracev1.ObjectType{
				APIVersion: tracev1.SchemeBuilder.GroupVersion.String(),
				Kind:       tracev1.Kind,
			}))
			objectRef = nil
			t = objectRefToType(objectRef)
			Expect(t).Should(BeNil())
		})
	})

	Context("getObjectRef", func() {
		It("should work well", func() {
			obj := builder.NewClusterBuilder(namespace, name).GetObject()
			objectRef, err := getObjectRef(obj, scheme.Scheme)
			Expect(err).Should(BeNil())
			Expect(*objectRef).Should(Equal(model.GVKNObjKey{
				GroupVersionKind: schema.GroupVersionKind{
					Group:   kbappsv1.GroupVersion.Group,
					Version: kbappsv1.GroupVersion.Version,
					Kind:    kbappsv1.ClusterKind,
				},
				ObjectKey: client.ObjectKey{
					Namespace: namespace,
					Name:      name,
				},
			}))
		})
	})

	Context("getObjectReference", func() {
		It("should work well", func() {
			obj := builder.NewClusterBuilder(namespace, name).SetUID(uid).SetResourceVersion(resourceVersion).GetObject()
			ref, err := getObjectReference(obj, scheme.Scheme)
			Expect(err).Should(BeNil())
			Expect(*ref).Should(Equal(corev1.ObjectReference{
				APIVersion:      kbappsv1.APIVersion,
				Kind:            kbappsv1.ClusterKind,
				Namespace:       namespace,
				Name:            name,
				UID:             uid,
				ResourceVersion: resourceVersion,
			}))
		})
	})

	Context("getObjectsByGVK", func() {
		It("should work well", func() {
			gvk := &schema.GroupVersionKind{
				Group:   kbappsv1.GroupVersion.Group,
				Version: kbappsv1.GroupVersion.Version,
				Kind:    kbappsv1.ComponentKind,
			}
			opt := &queryOptions{
				matchOwner: &matchOwner{
					controller: true,
					ownerUID:   uid,
				},
			}
			owner := builder.NewClusterBuilder(namespace, name).SetUID(uid).GetObject()
			compName := "hello"
			fullCompName := fmt.Sprintf("%s-%s", owner.Name, compName)
			owned := builder.NewComponentBuilder(namespace, fullCompName, "").
				SetOwnerReferences(kbappsv1.APIVersion, kbappsv1.ClusterKind, owner).
				GetObject()
			k8sMock.EXPECT().Scheme().Return(scheme.Scheme).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &kbappsv1.ComponentList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *kbappsv1.ComponentList, _ ...client.ListOption) error {
					list.Items = []kbappsv1.Component{*owned}
					return nil
				}).Times(1)

			objects, err := getObjectsByGVK(ctx, k8sMock, gvk, opt)
			Expect(err).Should(BeNil())
			Expect(objects).Should(HaveLen(1))
			Expect(objects[0]).Should(Equal(owned))
		})
	})

	Context("matchOwnerOf", func() {
		It("should work well", func() {
			owner := builder.NewClusterBuilder(namespace, name).SetUID(uid).GetObject()
			compName := "hello"
			fullCompName := fmt.Sprintf("%s-%s", owner.Name, compName)
			owned := builder.NewComponentBuilder(namespace, fullCompName, "").
				SetOwnerReferences(kbappsv1.APIVersion, kbappsv1.ClusterKind, owner).
				GetObject()
			matchOwner := &matchOwner{
				controller: true,
				ownerUID:   uid,
			}
			Expect(matchOwnerOf(matchOwner, owned)).Should(BeTrue())
		})
	})

	Context("parseRevision", func() {
		It("should work well", func() {
			rev := parseRevision(resourceVersion)
			Expect(rev).Should(Equal(int64(612345)))
		})
	})

	Context("parseQueryOptions", func() {
		It("should work well", func() {
			By("selector criteria")
			labels := map[string]string{
				"label1": "value1",
				"label2": "value2",
			}
			primary := builder.NewInstanceSetBuilder(namespace, name).AddLabelsInMap(labels).AddMatchLabelsInMap(labels).GetObject()
			criteria := &OwnershipCriteria{
				SelectorCriteria: &FieldPath{
					Path: "spec.selector.matchLabels",
				},
			}
			opt, err := parseQueryOptions(primary, criteria)
			Expect(err).Should(BeNil())
			Expect(opt).ShouldNot(BeNil())
			Expect(opt.matchLabels).Should(BeEquivalentTo(labels))

			By("label criteria")
			labels = map[string]string{
				"label1": "$(primary)",
				"label2": "$(primary.name)",
				"label3": "value3",
			}
			criteria = &OwnershipCriteria{
				LabelCriteria: labels,
			}
			opt, err = parseQueryOptions(primary, criteria)
			Expect(err).Should(BeNil())
			Expect(opt).ShouldNot(BeNil())
			expectedLabels := map[string]string{
				"label1": primary.Labels["label1"],
				"label2": primary.Name,
				"label3": "value3",
			}
			Expect(opt.matchLabels).Should(BeEquivalentTo(expectedLabels))

			By("specified name criteria")
			criteria = &OwnershipCriteria{
				SpecifiedNameCriteria: &FieldPath{
					Path: "metadata.name",
				},
			}
			opt, err = parseQueryOptions(primary, criteria)
			Expect(err).Should(BeNil())
			Expect(opt).ShouldNot(BeNil())
			Expect(opt.matchFields).Should(BeEquivalentTo(map[string]string{"metadata.name": primary.Name}))

			By("validation type")
			criteria = &OwnershipCriteria{
				Validation: ControllerValidation,
			}
			opt, err = parseQueryOptions(primary, criteria)
			Expect(err).Should(BeNil())
			Expect(opt).ShouldNot(BeNil())
			Expect(opt.matchOwner).ShouldNot(BeNil())
			Expect(*opt.matchOwner).Should(Equal(matchOwner{ownerUID: primary.UID, controller: true}))
		})
	})
})
