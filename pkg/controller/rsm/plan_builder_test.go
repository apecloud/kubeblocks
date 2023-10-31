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

package rsm

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	mockclient "github.com/apecloud/kubeblocks/pkg/testutil/k8s/mocks"
)

var _ = Describe("plan builder test", func() {
	Context("rsmWalkFunc function", func() {
		var rsmBuilder *PlanBuilder

		BeforeEach(func() {
			cli := k8sMock
			reqCtx := intctrlutil.RequestCtx{
				Ctx: ctx,
				Log: logger,
			}
			req := ctrl.Request{}
			planBuilder := NewRSMPlanBuilder(reqCtx, cli, req)
			rsmBuilder, _ = planBuilder.(*PlanBuilder)

			rsm = builder.NewReplicatedStateMachineBuilder(namespace, name).
				AddFinalizers([]string{getFinalizer(&workloads.ReplicatedStateMachine{})}).
				GetObject()
		})

		It("should create object", func() {
			v := &model.ObjectVertex{
				Obj:    rsm,
				Action: model.ActionCreatePtr(),
			}
			k8sMock.EXPECT().
				Create(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, obj *workloads.ReplicatedStateMachine, _ ...client.CreateOption) error {
					Expect(obj).ShouldNot(BeNil())
					Expect(obj.Namespace).Should(Equal(rsm.Namespace))
					Expect(obj.Name).Should(Equal(rsm.Name))
					Expect(obj.Finalizers).Should(Equal(rsm.Finalizers))
					return nil
				}).Times(1)
			Expect(rsmBuilder.rsmWalkFunc(v)).Should(Succeed())
		})

		It("should update sts object", func() {
			stsOrig := builder.NewStatefulSetBuilder(namespace, name).SetReplicas(3).GetObject()
			sts := stsOrig.DeepCopy()
			replicas := int32(5)
			sts.Spec.Replicas = &replicas
			v := &model.ObjectVertex{
				OriObj: stsOrig,
				Obj:    sts,
				Action: model.ActionUpdatePtr(),
			}
			k8sMock.EXPECT().
				Update(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, obj *apps.StatefulSet, _ ...client.UpdateOption) error {
					Expect(obj).ShouldNot(BeNil())
					Expect(obj.Namespace).Should(Equal(sts.Namespace))
					Expect(obj.Name).Should(Equal(sts.Name))
					Expect(obj.Spec.Replicas).Should(Equal(sts.Spec.Replicas))
					Expect(obj.Spec.Template).Should(Equal(sts.Spec.Template))
					Expect(obj.Spec.UpdateStrategy).Should(Equal(sts.Spec.UpdateStrategy))
					return nil
				}).Times(1)
			Expect(rsmBuilder.rsmWalkFunc(v)).Should(Succeed())
		})

		It("should update svc object", func() {
			svcOrig := builder.NewServiceBuilder(namespace, name).SetType(corev1.ServiceTypeLoadBalancer).GetObject()
			svc := svcOrig.DeepCopy()
			svc.Spec.Selector = map[string]string{"foo": "bar"}
			v := &model.ObjectVertex{
				OriObj: svcOrig,
				Obj:    svc,
				Action: model.ActionUpdatePtr(),
			}
			k8sMock.EXPECT().
				Update(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, obj *corev1.Service, _ ...client.UpdateOption) error {
					Expect(obj).ShouldNot(BeNil())
					Expect(obj.Namespace).Should(Equal(svc.Namespace))
					Expect(obj.Name).Should(Equal(svc.Name))
					Expect(obj.Spec).Should(Equal(svc.Spec))
					return nil
				}).Times(1)
			Expect(rsmBuilder.rsmWalkFunc(v)).Should(Succeed())
		})

		It("should update pvc object", func() {
			pvcOrig := builder.NewPVCBuilder(namespace, name).GetObject()
			pvc := pvcOrig.DeepCopy()
			pvc.Spec.Resources = corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: resource.MustParse("500m"),
				},
			}
			v := &model.ObjectVertex{
				OriObj: pvcOrig,
				Obj:    pvc,
				Action: model.ActionUpdatePtr(),
			}
			k8sMock.EXPECT().
				Update(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, obj *corev1.PersistentVolumeClaim, _ ...client.UpdateOption) error {
					Expect(obj).ShouldNot(BeNil())
					Expect(obj.Namespace).Should(Equal(pvc.Namespace))
					Expect(obj.Name).Should(Equal(pvc.Name))
					Expect(obj.Spec.Resources).Should(Equal(pvc.Spec.Resources))
					return nil
				}).Times(1)
			Expect(rsmBuilder.rsmWalkFunc(v)).Should(Succeed())
		})

		It("should delete object", func() {
			v := &model.ObjectVertex{
				Obj:    rsm,
				Action: model.ActionDeletePtr(),
			}
			k8sMock.EXPECT().
				Update(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, obj *workloads.ReplicatedStateMachine, _ ...client.UpdateOption) error {
					Expect(obj).ShouldNot(BeNil())
					Expect(obj.Finalizers).Should(HaveLen(0))
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				Delete(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, obj *workloads.ReplicatedStateMachine, _ ...client.DeleteOption) error {
					Expect(obj).ShouldNot(BeNil())
					Expect(obj.Namespace).Should(Equal(rsm.Namespace))
					Expect(obj.Name).Should(Equal(rsm.Name))
					Expect(obj.Finalizers).Should(HaveLen(0))
					return nil
				}).Times(1)
			Expect(rsmBuilder.rsmWalkFunc(v)).Should(Succeed())
		})

		It("should update object status", func() {
			rsm.Generation = 2
			rsm.Status.ObservedGeneration = 2
			rsmOrig := rsm.DeepCopy()
			rsmOrig.Status.ObservedGeneration = 1

			v := &model.ObjectVertex{
				Obj:    rsm,
				OriObj: rsmOrig,
				Action: model.ActionStatusPtr(),
			}
			ct := gomock.NewController(GinkgoT())
			statusWriter := mockclient.NewMockStatusWriter(ct)

			gomock.InOrder(
				k8sMock.EXPECT().Status().Return(statusWriter),
				statusWriter.EXPECT().
					Update(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, obj *workloads.ReplicatedStateMachine, _ ...client.UpdateOption) error {
						Expect(obj).ShouldNot(BeNil())
						Expect(obj.Namespace).Should(Equal(rsm.Namespace))
						Expect(obj.Name).Should(Equal(rsm.Name))
						Expect(obj.Status.ObservedGeneration).Should(Equal(rsm.Status.ObservedGeneration))
						return nil
					}).Times(1),
			)
			Expect(rsmBuilder.rsmWalkFunc(v)).Should(Succeed())
		})

		It("should return error if no action set", func() {
			v := &model.ObjectVertex{}
			err := rsmBuilder.rsmWalkFunc(v)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("vertex action can't be nil"))
		})

		It("should return nil and do nothing if action is Noop", func() {
			v := &model.ObjectVertex{
				Action: model.ActionNoopPtr(),
			}
			Expect(rsmBuilder.rsmWalkFunc(v)).Should(Succeed())
		})
	})
})
