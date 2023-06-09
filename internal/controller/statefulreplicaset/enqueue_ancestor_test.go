/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package statefulreplicaset

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/model"
)

func init() {
	model.AddScheme(workloads.AddToScheme)
}

var _ = Describe("enqueue ancestor", func() {
	const namespace = "foo"

	scheme := model.GetScheme()
	ctx := context.Background()
	var handler *EnqueueRequestForAncestor

	buildAncestorTree := func() (*workloads.StatefulReplicaSet, *appsv1.StatefulSet, *corev1.Pod) {
		ancestorL2APIVersion := "workloads.kubeblocks.io/v1alpha1"
		ancestorL2Kind := "StatefulReplicaSet"
		ancestorL2Name := "ancestor-level-2"
		ancestorL1APIVersion := "apps/v1"
		ancestorL1Kind := "StatefulSet"
		ancestorL1Name := "ancestor-level-1"
		objectName := ancestorL1Name + "-0"

		ancestorLevel2 := builder.NewStatefulReplicaSetBuilder(namespace, ancestorL2Name).GetObject()
		ancestorLevel2.APIVersion = ancestorL2APIVersion
		ancestorLevel2.Kind = ancestorL2Kind
		ancestorLevel1 := builder.NewStatefulSetBuilder(namespace, ancestorL1Name).
			SetOwnerReferences(ancestorL2APIVersion, ancestorL2Kind, ancestorLevel2).
			GetObject()
		ancestorLevel1.APIVersion = ancestorL1APIVersion
		ancestorLevel1.Kind = ancestorL1Kind
		object := builder.NewPodBuilder(namespace, objectName).
			SetOwnerReferences(ancestorL1APIVersion, ancestorL1Kind, ancestorLevel1).
			GetObject()

		return ancestorLevel2, ancestorLevel1, object
	}

	BeforeEach(func() {
		handler = &EnqueueRequestForAncestor{
			Client:    k8sMock,
			OwnerType: &workloads.StatefulReplicaSet{},
			UpToLevel: 2,
			InTypes:   []runtime.Object{&appsv1.StatefulSet{}},
		}
	})

	Context("parseOwnerTypeGroupKind", func() {
		It("should work well", func() {
			Expect(handler.parseOwnerTypeGroupKind(scheme)).Should(Succeed())
			Expect(handler.groupKind.Group).Should(Equal("workloads.kubeblocks.io"))
			Expect(handler.groupKind.Kind).Should(Equal("StatefulReplicaSet"))
		})
	})

	Context("parseInTypesGroupKind", func() {
		It("should work well", func() {
			Expect(handler.parseInTypesGroupKind(scheme)).Should(Succeed())
			Expect(len(handler.ancestorGroupKinds)).Should(Equal(1))
			Expect(handler.ancestorGroupKinds[0].Group).Should(Equal("apps"))
			Expect(handler.ancestorGroupKinds[0].Kind).Should(Equal("StatefulSet"))
		})
	})

	Context("getObjectByOwnerRef", func() {
		BeforeEach(func() {
			Expect(handler.InjectScheme(scheme)).Should(Succeed())
			Expect(handler.InjectMapper(newFakeMapper())).Should(Succeed())
		})

		It("should return err if groupVersion parsing error", func() {
			wrongAPIVersion := "wrong/group/version"
			ownerRef := metav1.OwnerReference{
				APIVersion: wrongAPIVersion,
			}
			_, err := handler.getObjectByOwnerRef(ctx, namespace, ownerRef, *scheme)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring(wrongAPIVersion))
		})

		It("should return nil if ancestor's type out of range", func() {
			ownerRef := metav1.OwnerReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "foo",
				UID:        "bar",
			}
			object, err := handler.getObjectByOwnerRef(ctx, namespace, ownerRef, *scheme)
			Expect(err).Should(BeNil())
			Expect(object).Should(BeNil())
		})

		It("should return the owner object", func() {
			ownerName := "foo"
			ownerUID := types.UID("bar")
			ownerRef := metav1.OwnerReference{
				APIVersion: "apps/v1",
				Kind:       "StatefulSet",
				Name:       ownerName,
				UID:        ownerUID,
			}
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &appsv1.StatefulSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *appsv1.StatefulSet, _ ...client.ListOption) error {
					obj.Name = ownerName
					obj.UID = ownerUID
					return nil
				}).Times(1)
			object, err := handler.getObjectByOwnerRef(ctx, namespace, ownerRef, *scheme)
			Expect(err).Should(BeNil())
			Expect(object).ShouldNot(BeNil())
			Expect(object.GetName()).Should(Equal(ownerName))
			Expect(object.GetUID()).Should(Equal(ownerUID))
		})
	})

	Context("getOwnerUpTo", func() {
		BeforeEach(func() {
			Expect(handler.InjectScheme(scheme)).Should(Succeed())
			Expect(handler.InjectMapper(newFakeMapper())).Should(Succeed())
		})

		It("should work well", func() {
			By("set upToLevel to 0")
			ownerRef, err := handler.getOwnerUpTo(ctx, nil, 0, *scheme)
			Expect(err).Should(BeNil())
			Expect(ownerRef).Should(BeNil())

			By("set object to nil")
			ownerRef, err = handler.getOwnerUpTo(ctx, nil, handler.UpToLevel, *scheme)
			Expect(err).Should(BeNil())
			Expect(ownerRef).Should(BeNil())

			By("builder ancestor tree")
			ancestorLevel2, ancestorLevel1, object := buildAncestorTree()

			By("set upToLevel to 1")
			ownerRef, err = handler.getOwnerUpTo(ctx, object, 1, *scheme)
			Expect(err).Should(BeNil())
			Expect(ownerRef).ShouldNot(BeNil())
			Expect(ownerRef.APIVersion).Should(Equal(ancestorLevel1.APIVersion))
			Expect(ownerRef.Kind).Should(Equal(ancestorLevel1.Kind))
			Expect(ownerRef.Name).Should(Equal(ancestorLevel1.Name))
			Expect(ownerRef.UID).Should(Equal(ancestorLevel1.UID))

			By("set upToLevel to 2")
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &appsv1.StatefulSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, sts *appsv1.StatefulSet, _ ...client.ListOptions) error {
					sts.Namespace = objKey.Namespace
					sts.Name = objKey.Name
					sts.OwnerReferences = ancestorLevel1.OwnerReferences
					return nil
				}).Times(1)
			ownerRef, err = handler.getOwnerUpTo(ctx, object, handler.UpToLevel, *scheme)
			Expect(err).Should(BeNil())
			Expect(ownerRef).ShouldNot(BeNil())
			Expect(ownerRef.APIVersion).Should(Equal(ancestorLevel2.APIVersion))
			Expect(ownerRef.Kind).Should(Equal(ancestorLevel2.Kind))
			Expect(ownerRef.Name).Should(Equal(ancestorLevel2.Name))
			Expect(ownerRef.UID).Should(Equal(ancestorLevel2.UID))
		})
	})

	Context("getSourceObject", func() {
		BeforeEach(func() {
			Expect(handler.InjectScheme(scheme)).Should(Succeed())
			Expect(handler.InjectMapper(newFakeMapper())).Should(Succeed())
		})

		It("should work well", func() {
			By("build a non-event object")
			name := "foo"
			uid := types.UID("bar")
			object1 := builder.NewPodBuilder(namespace, name).SetUID(uid).GetObject()
			objectSrc1, err := handler.getSourceObject(object1)
			Expect(err).Should(BeNil())
			Expect(objectSrc1 == object1).Should(BeTrue())

			By("build an event object")
			handler.InTypes = append(handler.InTypes, &corev1.Pod{})
			Expect(handler.InjectScheme(scheme)).Should(Succeed())
			objectRef := corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Namespace:  namespace,
				Name:       object1.Name,
				UID:        object1.UID,
			}
			object2 := builder.NewEventBuilder(namespace, "foo").
				SetInvolvedObject(objectRef).
				GetObject()
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &corev1.Pod{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, obj *corev1.Pod, _ ...client.ListOptions) error {
					obj.Name = objKey.Name
					obj.Namespace = objKey.Namespace
					obj.UID = objectRef.UID
					return nil
				}).Times(1)
			objectSrc2, err := handler.getSourceObject(object2)
			Expect(err).Should(BeNil())
			Expect(objectSrc2).ShouldNot(BeNil())
			Expect(objectSrc2.GetName()).Should(Equal(object1.Name))
			Expect(objectSrc2.GetNamespace()).Should(Equal(object1.Namespace))
			Expect(objectSrc2.GetUID()).Should(Equal(object1.UID))
		})
	})

	Context("getOwnerReconcileRequest", func() {
		BeforeEach(func() {
			Expect(handler.InjectScheme(scheme)).Should(Succeed())
			Expect(handler.InjectMapper(newFakeMapper())).Should(Succeed())
		})

		It("should work well", func() {
			By("build ancestor tree")
			ancestorLevel2, ancestorLevel1, object := buildAncestorTree()

			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &appsv1.StatefulSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, sts *appsv1.StatefulSet, _ ...client.ListOptions) error {
					sts.Namespace = objKey.Namespace
					sts.Name = objKey.Name
					sts.OwnerReferences = ancestorLevel1.OwnerReferences
					return nil
				}).Times(1)

			By("get object with ancestors")
			result := make(map[reconcile.Request]empty)
			handler.getOwnerReconcileRequest(object, result)
			Expect(len(result)).Should(Equal(1))
			for request := range result {
				Expect(request.Namespace).Should(Equal(ancestorLevel2.Namespace))
				Expect(request.Name).Should(Equal(ancestorLevel2.Name))
			}

			By("set obj not exist")
			wrongAPIVersion := "wrong/api/version"
			object.OwnerReferences[0].APIVersion = wrongAPIVersion
			result = make(map[reconcile.Request]empty)
			handler.getOwnerReconcileRequest(object, result)
			Expect(len(result)).Should(Equal(0))

			By("set level 1 ancestor's owner not exist")
			object.OwnerReferences[0].APIVersion = ancestorLevel1.APIVersion
			k8sMock.EXPECT().
				Get(gomock.Any(), gomock.Any(), &appsv1.StatefulSet{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, objKey client.ObjectKey, sts *appsv1.StatefulSet, _ ...client.ListOptions) error {
					sts.Namespace = objKey.Namespace
					sts.Name = objKey.Name
					return nil
				}).Times(1)
			result = make(map[reconcile.Request]empty)
			handler.getOwnerReconcileRequest(object, result)
			Expect(len(result)).Should(Equal(0))
		})
	})

	Context("handler interface", func() {
		BeforeEach(func() {
			Expect(handler.InjectScheme(scheme)).Should(Succeed())
			Expect(handler.InjectMapper(newFakeMapper())).Should(Succeed())
		})

		It("should work well", func() {
			By("build events and queue")
			queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "enqueue-ancestor-test")
			ancestorLevel2, ancestorLevel1, object := buildAncestorTree()
			createEvent := event.CreateEvent{Object: object}
			updateEvent := event.UpdateEvent{ObjectOld: object, ObjectNew: object}
			deleteEvent := event.DeleteEvent{Object: object}
			genericEvent := event.GenericEvent{Object: object}

			cases := []struct {
				name     string
				testFunc func()
				getTimes int
			}{
				{
					name:     "Create",
					testFunc: func() { handler.Create(createEvent, queue) },
					getTimes: 1,
				},
				{
					name:     "Update",
					testFunc: func() { handler.Update(updateEvent, queue) },
					getTimes: 2,
				},
				{
					name:     "Delete",
					testFunc: func() { handler.Delete(deleteEvent, queue) },
					getTimes: 1,
				},
				{
					name:     "Generic",
					testFunc: func() { handler.Generic(genericEvent, queue) },
					getTimes: 1,
				},
			}
			for _, c := range cases {
				By(fmt.Sprintf("test %s interface", c.name))
				k8sMock.EXPECT().
					Get(gomock.Any(), gomock.Any(), &appsv1.StatefulSet{}, gomock.Any()).
					DoAndReturn(func(_ context.Context, objKey client.ObjectKey, sts *appsv1.StatefulSet, _ ...client.ListOptions) error {
						sts.Namespace = objKey.Namespace
						sts.Name = objKey.Name
						sts.OwnerReferences = ancestorLevel1.OwnerReferences
						return nil
					}).Times(c.getTimes)
				c.testFunc()
				item, shutdown := queue.Get()
				Expect(shutdown).Should(BeFalse())
				request, ok := item.(reconcile.Request)
				Expect(ok).Should(BeTrue())
				Expect(request.Namespace).Should(Equal(ancestorLevel2.Namespace))
				Expect(request.Name).Should(Equal(ancestorLevel2.Name))
				queue.Done(item)
				queue.Forget(item)
			}

			queue.ShutDown()
		})
	})
})

type fakeMapper struct{}

func (f *fakeMapper) KindFor(resource schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}

func (f *fakeMapper) KindsFor(resource schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	return nil, nil
}

func (f *fakeMapper) ResourceFor(input schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	return schema.GroupVersionResource{}, nil
}

func (f *fakeMapper) ResourcesFor(input schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	return nil, nil
}

func (f *fakeMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	return &meta.RESTMapping{Scope: meta.RESTScopeNamespace}, nil
}

func (f *fakeMapper) RESTMappings(gk schema.GroupKind, versions ...string) ([]*meta.RESTMapping, error) {
	return nil, nil
}

func (f *fakeMapper) ResourceSingularizer(resource string) (singular string, err error) {
	return "", nil
}

func newFakeMapper() meta.RESTMapper {
	return &fakeMapper{}
}
