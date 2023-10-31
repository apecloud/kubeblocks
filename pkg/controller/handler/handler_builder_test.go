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

package handler

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

var _ = Describe("handler builder test.", func() {
	Context("CRUD events", func() {
		It("should work well", func() {
			By("build resource tree")
			namespace := "foo"
			clusterName := "bar"
			componentName := "test"
			rsmName := fmt.Sprintf("%s-%s", clusterName, componentName)
			stsName := rsmName
			podName := stsName + "-0"
			eventName := podName + ".123456"
			labels := map[string]string{
				constant.AppManagedByLabelKey:   constant.AppName,
				constant.AppNameLabelKey:        clusterName + "def",
				constant.AppComponentLabelKey:   componentName + "def",
				constant.AppInstanceLabelKey:    clusterName,
				constant.KBAppComponentLabelKey: componentName,
			}
			rsm := builder.NewReplicatedStateMachineBuilder(namespace, rsmName).
				AddLabelsInMap(labels).
				GetObject()
			sts := builder.NewStatefulSetBuilder(namespace, stsName).
				AddLabelsInMap(labels).
				GetObject()
			pod := builder.NewPodBuilder(namespace, podName).
				SetOwnerReferences("apps/v1", "StatefulSet", sts).
				GetObject()
			objectRef := corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Namespace:  namespace,
				Name:       pod.Name,
				UID:        pod.UID,
			}
			evt := builder.NewEventBuilder(namespace, eventName).
				SetInvolvedObject(objectRef).
				GetObject()

			finderCtx := &FinderContext{
				Context: ctx,
				Reader:  k8sMock,
				Scheme:  *model.GetScheme(),
			}
			handler := NewBuilder(finderCtx).
				AddFinder(NewInvolvedObjectFinder(&corev1.Pod{})).
				AddFinder(NewOwnerFinder(&appsv1.StatefulSet{})).
				AddFinder(NewDelegatorFinder(&workloads.ReplicatedStateMachine{},
					[]string{constant.AppInstanceLabelKey, constant.KBAppComponentLabelKey})).
				Build()

			By("build events and queue")
			queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "handler-builder-test")
			createEvent := event.CreateEvent{Object: evt}
			updateEvent := event.UpdateEvent{ObjectOld: evt, ObjectNew: evt}
			deleteEvent := event.DeleteEvent{Object: evt}
			genericEvent := event.GenericEvent{Object: evt}

			cases := []struct {
				name     string
				testFunc func()
				getTimes int
			}{
				{
					name:     "Create",
					testFunc: func() { handler.Create(ctx, createEvent, queue) },
					getTimes: 1,
				},
				{
					name:     "Update",
					testFunc: func() { handler.Update(ctx, updateEvent, queue) },
					getTimes: 2,
				},
				{
					name:     "Delete",
					testFunc: func() { handler.Delete(ctx, deleteEvent, queue) },
					getTimes: 1,
				},
				{
					name:     "Generic",
					testFunc: func() { handler.Generic(ctx, genericEvent, queue) },
					getTimes: 1,
				},
			}
			for _, c := range cases {
				By(fmt.Sprintf("test %s interface", c.name))
				k8sMock.EXPECT().
					Get(gomock.Any(), gomock.Any(), &appsv1.StatefulSet{}, gomock.Any()).
					DoAndReturn(func(_ context.Context, objKey client.ObjectKey, stsTmp *appsv1.StatefulSet, _ ...client.ListOptions) error {
						*stsTmp = *sts
						return nil
					}).Times(c.getTimes)
				k8sMock.EXPECT().
					Get(gomock.Any(), gomock.Any(), &corev1.Pod{}, gomock.Any()).
					DoAndReturn(func(_ context.Context, objKey client.ObjectKey, podTmp *corev1.Pod, _ ...client.ListOptions) error {
						*podTmp = *pod
						return nil
					}).Times(c.getTimes)
				c.testFunc()
				item, shutdown := queue.Get()
				Expect(shutdown).Should(BeFalse())
				request, ok := item.(reconcile.Request)
				Expect(ok).Should(BeTrue())
				Expect(request.Namespace).Should(Equal(rsm.Namespace))
				Expect(request.Name).Should(Equal(rsm.Name))
				queue.Done(item)
				queue.Forget(item)
			}

			queue.ShutDown()
		})
	})
})
