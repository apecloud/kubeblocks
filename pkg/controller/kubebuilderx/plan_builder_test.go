/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package kubebuilderx

import (
	"context"
	"reflect"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	mockclient "github.com/apecloud/kubeblocks/pkg/testutil/k8s/mocks"
)

var _ = Describe("plan builder test", func() {
	Context("defaultWalkFunc function", func() {
		const (
			namespace = "foo"
			name      = "bar"
			finalizer = "test"
		)

		var (
			planBuilder *PlanBuilder
			its         *workloads.InstanceSet
		)

		BeforeEach(func() {
			bldr := NewPlanBuilder(ctx, k8sMock, nil, nil, nil, logger)
			planBuilder, _ = bldr.(*PlanBuilder)
			its = builder.NewInstanceSetBuilder(namespace, name).
				AddFinalizers([]string{finalizer}).
				GetObject()
		})

		It("should create object", func() {
			v := &model.ObjectVertex{
				Obj:    its,
				Action: model.ActionCreatePtr(),
			}
			k8sMock.EXPECT().
				Create(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, obj *workloads.InstanceSet, _ ...client.CreateOption) error {
					Expect(obj).ShouldNot(BeNil())
					Expect(obj.Namespace).Should(Equal(its.Namespace))
					Expect(obj.Name).Should(Equal(its.Name))
					Expect(obj.Finalizers).Should(Equal(its.Finalizers))
					return nil
				}).Times(1)
			Expect(planBuilder.defaultWalkFunc(v)).Should(Succeed())
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
			Expect(planBuilder.defaultWalkFunc(v)).Should(Succeed())
		})

		It("should update pvc object", func() {
			pvcOrig := builder.NewPVCBuilder(namespace, name).GetObject()
			pvc := pvcOrig.DeepCopy()
			pvc.Spec.Resources = corev1.VolumeResourceRequirements{
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
			Expect(planBuilder.defaultWalkFunc(v)).Should(Succeed())
		})

		It("should delete object", func() {
			v := &model.ObjectVertex{
				Obj:    its,
				Action: model.ActionDeletePtr(),
			}
			// k8sMock.EXPECT().
			//	Update(gomock.Any(), gomock.Any(), gomock.Any()).
			//	DoAndReturn(func(_ context.Context, obj *workloads.InstanceSet, _ ...client.UpdateOption) error {
			//		Expect(obj).ShouldNot(BeNil())
			//		Expect(obj.Finalizers).Should(HaveLen(0))
			//		return nil
			//	}).Times(1)
			k8sMock.EXPECT().
				Delete(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, obj *workloads.InstanceSet, _ ...client.DeleteOption) error {
					Expect(obj).ShouldNot(BeNil())
					Expect(obj.Namespace).Should(Equal(its.Namespace))
					Expect(obj.Name).Should(Equal(its.Name))
					Expect(obj.Finalizers).Should(HaveLen(1))
					Expect(obj.Finalizers[0]).Should(Equal(finalizer))
					return nil
				}).Times(1)
			Expect(planBuilder.defaultWalkFunc(v)).Should(Succeed())
		})

		It("should update object status", func() {
			its.Generation = 2
			its.Status.ObservedGeneration = 2
			itsOrig := its.DeepCopy()
			itsOrig.Status.ObservedGeneration = 1

			v := &model.ObjectVertex{
				Obj:    its,
				OriObj: itsOrig,
				Action: model.ActionStatusPtr(),
			}
			ct := gomock.NewController(GinkgoT())
			statusWriter := mockclient.NewMockStatusWriter(ct)

			gomock.InOrder(
				k8sMock.EXPECT().Status().Return(statusWriter),
				statusWriter.EXPECT().
					Update(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, obj *workloads.InstanceSet, _ ...client.UpdateOption) error {
						Expect(obj).ShouldNot(BeNil())
						Expect(obj.Namespace).Should(Equal(its.Namespace))
						Expect(obj.Name).Should(Equal(its.Name))
						Expect(obj.Status.ObservedGeneration).Should(Equal(its.Status.ObservedGeneration))
						return nil
					}).Times(1),
			)
			Expect(planBuilder.defaultWalkFunc(v)).Should(Succeed())
		})

		It("should return error if no action set", func() {
			v := &model.ObjectVertex{}
			err := planBuilder.defaultWalkFunc(v)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("vertex action can't be nil"))
		})
	})

	Context("buildOrderedVertices", func() {
		const (
			namespace = "foo"
			name      = "bar"
		)

		var (
			its         *workloads.InstanceSet
			currentTree *ObjectTree
			desiredTree *ObjectTree
		)

		BeforeEach(func() {
			its = builder.NewInstanceSetBuilder(namespace, name).
				AddLabels(constant.AppComponentLabelKey, name).
				SetReplicas(3).
				GetObject()
			currentTree = NewObjectTree()
			desiredTree = NewObjectTree()
		})

		Context("buildOrderedVertices", func() {
			It("should work well", func() {
				newVertex := func(oldObj, newObj client.Object, action *model.Action) *model.ObjectVertex {
					return &model.ObjectVertex{
						Obj:    newObj,
						OriObj: oldObj,
						Action: action,
					}
				}

				pod := builder.NewPodBuilder(namespace, name).GetObject()
				headlessSvc := builder.NewHeadlessServiceBuilder(namespace, name+"-headless").GetObject()
				svc := builder.NewServiceBuilder(namespace, name).GetObject()
				env := builder.NewConfigMapBuilder(namespace, name+"-env").GetObject()

				var verticesExpected []*model.ObjectVertex
				itsCopy := its.DeepCopy()
				itsCopy.Status.Replicas = *itsCopy.Spec.Replicas
				verticesExpected = append(verticesExpected, newVertex(its, itsCopy, model.ActionStatusPtr()))
				verticesExpected = append(verticesExpected, newVertex(nil, pod, model.ActionCreatePtr()))
				verticesExpected = append(verticesExpected, newVertex(nil, headlessSvc, model.ActionCreatePtr()))
				verticesExpected = append(verticesExpected, newVertex(nil, svc, model.ActionCreatePtr()))
				verticesExpected = append(verticesExpected, newVertex(nil, env, model.ActionCreatePtr()))

				// build ordered vertices
				currentTree.SetRoot(its)
				desiredTree.SetRoot(itsCopy)
				Expect(desiredTree.Add(pod, headlessSvc, svc, env)).Should(Succeed())
				vertices := buildOrderedVertices(ctx, currentTree, desiredTree)

				// compare vertices
				Expect(vertices).Should(HaveLen(len(verticesExpected)))
				for _, vertex := range vertices {
					Expect(slices.IndexFunc(verticesExpected, func(v *model.ObjectVertex) bool {
						if reflect.DeepEqual(v.Obj, vertex.Obj) && *v.Action == *vertex.Action {
							return true
						}
						return false
					})).Should(BeNumerically(">=", 0))
				}
			})
		})
	})
})
