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

package model

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("transform utils test", func() {
	const (
		namespace = "foo"
		name      = "bar"
	)

	Context("FindRootVertex function", func() {
		It("should work well", func() {
			dag := graph.NewDAG()
			_, err := FindRootVertex(dag)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("root vertex not found"))

			root := builder.NewInstanceSetBuilder(namespace, name).GetObject()
			obj0 := builder.NewPodBuilder(namespace, name+"-0").GetObject()
			obj1 := builder.NewPodBuilder(namespace, name+"-1").GetObject()
			obj2 := builder.NewPodBuilder(namespace, name+"-2").GetObject()
			dag.AddVertex(&ObjectVertex{Obj: root})
			dag.AddConnectRoot(&ObjectVertex{Obj: obj0})
			dag.AddConnectRoot(&ObjectVertex{Obj: obj1})
			dag.AddConnectRoot(&ObjectVertex{Obj: obj2})

			rootVertex, err := FindRootVertex(dag)
			Expect(err).Should(BeNil())
			Expect(rootVertex.Obj).Should(Equal(root))
		})
	})

	Context("IsOwnerOf function", func() {
		It("should work well", func() {
			ownerAPIVersion := "workloads.kubeblocks.io/v1"
			ownerKind := "InstanceSet"
			objectName := name + "-0"
			owner := builder.NewInstanceSetBuilder(namespace, name).GetObject()
			object := builder.NewPodBuilder(namespace, objectName).
				SetOwnerReferences(ownerAPIVersion, ownerKind, owner).
				GetObject()
			Expect(IsOwnerOf(owner, object)).Should(BeTrue())

		})
	})

	Context("NewRequeueError function", func() {
		It("should work well", func() {
			after := 17 * time.Second
			reason := "something really bad happens"
			err := NewRequeueError(after, reason)
			reqErr, ok := err.(RequeueError)
			Expect(ok).Should(BeTrue())
			Expect(reqErr.RequeueAfter()).Should(Equal(after))
			Expect(reqErr.Reason()).Should(Equal(reason))
			Expect(err.Error()).Should(ContainSubstring("requeue after:"))
		})
	})

	Context("test IsObjectDoing", func() {
		It("should work well", func() {
			object := &apps.StatefulSet{}
			By("set generation equal")
			object.Generation = 1
			object.Status.ObservedGeneration = 1
			Expect(IsObjectUpdating(object)).Should(BeFalse())
			Expect(IsObjectStatusUpdating(object)).Should(BeTrue())

			By("set generation not equal")
			object.Generation = 2
			object.Status.ObservedGeneration = 1
			Expect(IsObjectUpdating(object)).Should(BeTrue())

			By("set deletionTimestamp")
			ts := metav1.NewTime(time.Now())
			object.DeletionTimestamp = &ts
			Expect(IsObjectDeleting(object)).Should(BeTrue())

			By("set fields not exist")
			object2 := &corev1.Secret{}
			Expect(IsObjectUpdating(object2)).Should(BeFalse())
		})
	})

	Context("ReadCacheSnapshot function", func() {
		It("should work well", func() {
			controller, k8sMock := testutil.SetupK8sMock()
			defer controller.Finish()

			root := builder.NewInstanceSetBuilder(namespace, name).GetObject()
			obj0 := builder.NewPodBuilder(namespace, name+"-0").GetObject()
			obj1 := builder.NewPodBuilder(namespace, name+"-1").GetObject()
			obj2 := builder.NewPodBuilder(namespace, name+"-2").GetObject()

			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.PodList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.PodList, _ ...client.ListOption) error {
					Expect(list).ShouldNot(BeNil())
					list.Items = []corev1.Pod{*obj0, *obj1, *obj2}
					return nil
				}).Times(1)
			transCtx := &testTransCtx{
				Context:     context.Background(),
				GraphClient: NewGraphClient(k8sMock),
			}
			snapshot, err := ReadCacheSnapshot(transCtx, root, nil, &corev1.PodList{})
			Expect(err).Should(BeNil())
			Expect(snapshot).Should(HaveLen(3))
			objList := []*corev1.Pod{obj0, obj1, obj2}
			for _, pod := range objList {
				gvk, err := GetGVKName(pod)
				Expect(err).Should(BeNil())
				obj, ok := snapshot[*gvk]
				Expect(ok).Should(BeTrue())
				p, ok := obj.(*corev1.Pod)
				Expect(ok).Should(BeTrue())
				Expect(p).Should(Equal(pod))
			}
		})
	})
})

type testTransCtx struct {
	context.Context
	GraphClient
}

var _ graph.TransformContext = &testTransCtx{}

func (t *testTransCtx) GetContext() context.Context {
	return t.Context
}

func (t *testTransCtx) GetClient() client.Reader {
	return t.GraphClient
}

func (t *testTransCtx) GetRecorder() record.EventRecorder {
	// TODO implement me
	panic("implement me")
}

func (t *testTransCtx) GetLogger() logr.Logger {
	// TODO implement me
	panic("implement me")
}
