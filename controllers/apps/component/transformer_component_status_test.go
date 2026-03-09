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

package component

import (
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

var _ = Describe("component status transformer conditions", func() {
	const (
		compDefName = "test-compdef-status"
		clusterName = "test-cluster-status"
		compName    = "comp-status"
	)

	var (
		transCtx      *componentTransformContext
		transformer   *componentStatusTransformer
		comp          *appsv1.Component
		compDef       *appsv1.ComponentDefinition
		runningITS    *workloads.InstanceSet
		protoITS      *workloads.InstanceSet
		eventRecorder record.EventRecorder
	)

	newReadyITS := func(generation int64, replicas int32, roles []workloads.ReplicaRole) *workloads.InstanceSet {
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:  testCtx.DefaultNamespace,
				Name:       constant.GenerateWorkloadNamePattern(clusterName, compName),
				Generation: generation,
				Annotations: map[string]string{
					constant.KubeBlocksGenerationKey: strconv.FormatInt(comp.Generation, 10),
				},
			},
			Spec: workloads.InstanceSetSpec{
				Replicas: ptr.To(replicas),
				Roles:    roles,
			},
			Status: workloads.InstanceSetStatus{
				ObservedGeneration: generation,
				Replicas:           replicas,
				ReadyReplicas:      replicas,
				UpdatedReplicas:    replicas,
				InitReplicas:       replicas,
				ReadyInitReplicas:  replicas,
			},
		}
		if len(roles) > 0 {
			for i := int32(0); i < replicas; i++ {
				its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{
					PodName: "pod-" + strconv.Itoa(int(i)),
					Role:    roles[i%int32(len(roles))].Name,
				})
			}
		}
		return its
	}

	BeforeEach(func() {
		eventRecorder = record.NewFakeRecorder(100)

		compDef = &appsv1.ComponentDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: compDefName,
			},
			Spec: appsv1.ComponentDefinitionSpec{},
		}

		comp = &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:  testCtx.DefaultNamespace,
				Name:       constant.GenerateClusterComponentName(clusterName, compName),
				Generation: 1,
				Labels: map[string]string{
					constant.AppManagedByLabelKey:   constant.AppName,
					constant.AppInstanceLabelKey:    clusterName,
					constant.KBAppComponentLabelKey: compName,
				},
				Annotations: map[string]string{
					constant.KBAppClusterUIDKey: string(uuid.NewUUID()),
				},
			},
			Spec: appsv1.ComponentSpec{
				CompDef:  compDef.Name,
				Replicas: 3,
			},
			Status: appsv1.ComponentStatus{
				Phase: appsv1.RunningComponentPhase,
			},
		}

		runningITS = newReadyITS(1, 3, nil)
		protoITS = runningITS.DeepCopy()

		reader := &appsutil.MockReader{
			Objects: []client.Object{compDef, comp},
		}
		graphCli := model.NewGraphClient(reader)

		transCtx = &componentTransformContext{
			Context:       ctx,
			Client:        graphCli,
			EventRecorder: eventRecorder,
			Logger:        logger,
			CompDef:       compDef,
			Component:     comp,
			ComponentOrig: comp.DeepCopy(),
			SynthesizeComponent: &component.SynthesizedComponent{
				Namespace:   testCtx.DefaultNamespace,
				ClusterName: clusterName,
				Name:        compName,
			},
			RunningWorkload: runningITS,
			ProtoWorkload:   protoITS,
		}

		transformer = &componentStatusTransformer{}
		transformer.comp = comp
		transformer.runningITS = runningITS
		transformer.protoITS = protoITS
		transformer.synthesizeComp = transCtx.SynthesizeComponent
	})

	Context("reconcileHealthyCondition", func() {
		It("should be unhealthy when runningITS is nil", func() {
			transformer.runningITS = nil
			err := transformer.reconcileHealthyCondition(transCtx)
			Expect(err).Should(BeNil())

			cond := meta.FindStatusCondition(comp.Status.Conditions, appsv1.ConditionTypeHealthy)
			Expect(cond).ShouldNot(BeNil())
			Expect(cond.Status).Should(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).Should(Equal("WorkloadNotExist"))
		})

		It("should be unhealthy when workload generation not matching", func() {
			runningITS.Annotations[constant.KubeBlocksGenerationKey] = "999"
			err := transformer.reconcileHealthyCondition(transCtx)
			Expect(err).Should(BeNil())

			cond := meta.FindStatusCondition(comp.Status.Conditions, appsv1.ConditionTypeHealthy)
			Expect(cond).ShouldNot(BeNil())
			Expect(cond.Status).Should(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).Should(Equal("WorkloadNotUpdated"))
		})

		It("should be unhealthy when instances are not ready", func() {
			runningITS.Status.ReadyReplicas = 1
			err := transformer.reconcileHealthyCondition(transCtx)
			Expect(err).Should(BeNil())

			cond := meta.FindStatusCondition(comp.Status.Conditions, appsv1.ConditionTypeHealthy)
			Expect(cond).ShouldNot(BeNil())
			Expect(cond.Status).Should(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).Should(Equal("WorkloadNotReady"))
		})

		It("should be unhealthy when role probe not done", func() {
			roles := []workloads.ReplicaRole{{Name: "leader"}, {Name: "follower"}}
			runningITS.Spec.Roles = roles
			// no instance status with roles -> role probe not done
			runningITS.Status.InstanceStatus = nil
			err := transformer.reconcileHealthyCondition(transCtx)
			Expect(err).Should(BeNil())

			cond := meta.FindStatusCondition(comp.Status.Conditions, appsv1.ConditionTypeHealthy)
			Expect(cond).ShouldNot(BeNil())
			Expect(cond.Status).Should(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).Should(Equal("RoleProbeNotDone"))
		})

		It("should be healthy when everything is ready (no roles)", func() {
			err := transformer.reconcileHealthyCondition(transCtx)
			Expect(err).Should(BeNil())

			cond := meta.FindStatusCondition(comp.Status.Conditions, appsv1.ConditionTypeHealthy)
			Expect(cond).ShouldNot(BeNil())
			Expect(cond.Status).Should(Equal(metav1.ConditionTrue))
			Expect(cond.Reason).Should(Equal("Healthy"))
		})

		It("should be healthy when everything is ready (with roles)", func() {
			roles := []workloads.ReplicaRole{{Name: "leader"}, {Name: "follower"}}
			its := newReadyITS(1, 3, roles)
			transformer.runningITS = its
			transCtx.RunningWorkload = its
			err := transformer.reconcileHealthyCondition(transCtx)
			Expect(err).Should(BeNil())

			cond := meta.FindStatusCondition(comp.Status.Conditions, appsv1.ConditionTypeHealthy)
			Expect(cond).ShouldNot(BeNil())
			Expect(cond.Status).Should(Equal(metav1.ConditionTrue))
			Expect(cond.Reason).Should(Equal("Healthy"))
		})
	})

	Context("reconcileProgressingCondition", func() {
		It("should not be progressing when nothing is in progress", func() {
			err := transformer.reconcileProgressingCondition(transCtx)
			Expect(err).Should(BeNil())

			cond := meta.FindStatusCondition(comp.Status.Conditions, appsv1.ConditionTypeProgressing)
			Expect(cond).ShouldNot(BeNil())
			Expect(cond.Status).Should(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).Should(Equal("NotProgressing"))
		})

		It("should be progressing when volume expansion is running", func() {
			runningITS.Status.InstanceStatus = []workloads.InstanceStatus{
				{PodName: "pod-0", VolumeExpansion: true},
			}
			err := transformer.reconcileProgressingCondition(transCtx)
			Expect(err).Should(BeNil())

			cond := meta.FindStatusCondition(comp.Status.Conditions, appsv1.ConditionTypeProgressing)
			Expect(cond).ShouldNot(BeNil())
			Expect(cond.Status).Should(Equal(metav1.ConditionTrue))
			Expect(cond.Reason).Should(Equal("VolumeExpansionRunning"))
		})

		It("should be progressing when post-provision is not done", func() {
			transCtx.SynthesizeComponent.LifecycleActions = component.SynthesizedLifecycleActions{
				ComponentLifecycleActions: &appsv1.ComponentLifecycleActions{
					PostProvision: &appsv1.Action{
						Exec: &appsv1.ExecAction{
							Command: []string{"echo", "hello"},
						},
					},
				},
			}

			err := transformer.reconcileProgressingCondition(transCtx)
			Expect(err).Should(BeNil())

			cond := meta.FindStatusCondition(comp.Status.Conditions, appsv1.ConditionTypeProgressing)
			Expect(cond).ShouldNot(BeNil())
			Expect(cond.Status).Should(Equal(metav1.ConditionTrue))
			Expect(cond.Reason).Should(Equal("PostProvisioning"))

			// set post-provision-done annotation
			comp.Annotations[kbCompPostProvisionDoneKey] = time.Now().Format(time.RFC3339Nano)
			err = transformer.reconcileProgressingCondition(transCtx)
			Expect(err).Should(BeNil())
			cond = meta.FindStatusCondition(comp.Status.Conditions, appsv1.ConditionTypeProgressing)
			Expect(cond).ShouldNot(BeNil())
			Expect(cond.Status).Should(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).Should(Equal("NotProgressing"))
		})

		It("should be progressing when scale out is running", func() {
			err := component.NewReplicasStatus(protoITS, []string{"pod-3"}, true, false)
			Expect(err).Should(BeNil())
			transformer.protoITS = protoITS

			err = transformer.reconcileProgressingCondition(transCtx)
			Expect(err).Should(BeNil())

			cond := meta.FindStatusCondition(comp.Status.Conditions, appsv1.ConditionTypeProgressing)
			Expect(cond).ShouldNot(BeNil())
			Expect(cond.Status).Should(Equal(metav1.ConditionTrue))
			Expect(cond.Reason).Should(Equal("ScaleOutRunning"))

			// when scale out is done
			// TODO
		})
	})

	Context("reconcileAvailableCondition", func() {
		It("should be available when no available policy is defined", func() {
			comp.Status.Phase = appsv1.UpdatingComponentPhase
			err := transformer.reconcileAvailableCondition(transCtx)
			Expect(err).Should(BeNil())

			cond := meta.FindStatusCondition(comp.Status.Conditions, appsv1.ConditionTypeAvailable)
			Expect(cond).ShouldNot(BeNil())
			Expect(cond.Status).Should(Equal(metav1.ConditionTrue))
			Expect(cond.Reason).Should(Equal("Available"))
		})

		Context("WithPhases policy", func() {
			BeforeEach(func() {
				compDef.Spec.Available = &appsv1.ComponentAvailable{
					WithPhases: ptr.To("Running,Updating"),
				}
			})

			It("should be available when phase matches", func() {
				comp.Status.Phase = appsv1.RunningComponentPhase
				err := transformer.reconcileAvailableCondition(transCtx)
				Expect(err).Should(BeNil())

				cond := meta.FindStatusCondition(comp.Status.Conditions, appsv1.ConditionTypeAvailable)
				Expect(cond).ShouldNot(BeNil())
				Expect(cond.Status).Should(Equal(metav1.ConditionTrue))
				Expect(cond.Reason).Should(Equal("Available"))
			})

			It("should not be available when phase does not match", func() {
				comp.Status.Phase = appsv1.FailedComponentPhase
				err := transformer.reconcileAvailableCondition(transCtx)
				Expect(err).Should(BeNil())

				cond := meta.FindStatusCondition(comp.Status.Conditions, appsv1.ConditionTypeAvailable)
				Expect(cond).ShouldNot(BeNil())
				Expect(cond.Status).Should(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).Should(Equal("PhaseCheckFail"))
			})

			It("should be unknown when phase is empty", func() {
				comp.Status.Phase = ""
				err := transformer.reconcileAvailableCondition(transCtx)
				Expect(err).Should(BeNil())

				cond := meta.FindStatusCondition(comp.Status.Conditions, appsv1.ConditionTypeAvailable)
				Expect(cond).ShouldNot(BeNil())
				Expect(cond.Status).Should(Equal(metav1.ConditionUnknown))
			})
		})

		Context("WithRole policy", func() {
			BeforeEach(func() {
				compDef.Spec.Available = &appsv1.ComponentAvailable{
					WithRole: ptr.To("leader"),
				}
			})

			It("should be available when role is present", func() {
				runningITS.Status.InstanceStatus = []workloads.InstanceStatus{
					{PodName: "pod-0", Role: "leader"},
					{PodName: "pod-1", Role: "follower"},
				}
				transCtx.RunningWorkload = runningITS
				err := transformer.reconcileAvailableCondition(transCtx)
				Expect(err).Should(BeNil())

				cond := meta.FindStatusCondition(comp.Status.Conditions, appsv1.ConditionTypeAvailable)
				Expect(cond).ShouldNot(BeNil())
				Expect(cond.Status).Should(Equal(metav1.ConditionTrue))
				Expect(cond.Reason).Should(Equal("Available"))
			})

			It("should not be available when role is not present", func() {
				runningITS.Status.InstanceStatus = []workloads.InstanceStatus{
					{PodName: "pod-0", Role: "follower"},
				}
				transCtx.RunningWorkload = runningITS
				err := transformer.reconcileAvailableCondition(transCtx)
				Expect(err).Should(BeNil())

				cond := meta.FindStatusCondition(comp.Status.Conditions, appsv1.ConditionTypeAvailable)
				Expect(cond).ShouldNot(BeNil())
				Expect(cond.Status).Should(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).Should(Equal("RoleCheckFail"))
			})

			It("should not be available when workload is nil", func() {
				transCtx.RunningWorkload = nil
				err := transformer.reconcileAvailableCondition(transCtx)
				Expect(err).Should(BeNil())

				cond := meta.FindStatusCondition(comp.Status.Conditions, appsv1.ConditionTypeAvailable)
				Expect(cond).ShouldNot(BeNil())
				Expect(cond.Status).Should(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).Should(Equal("RoleCheckFail"))
			})
		})

		Context("WithPhases and WithRole combined policy", func() {
			BeforeEach(func() {
				compDef.Spec.Available = &appsv1.ComponentAvailable{
					WithPhases: ptr.To("Running"),
					WithRole:   ptr.To("leader"),
				}
			})

			It("should be available when both checks pass", func() {
				comp.Status.Phase = appsv1.RunningComponentPhase
				runningITS.Status.InstanceStatus = []workloads.InstanceStatus{
					{PodName: "pod-0", Role: "leader"},
				}
				transCtx.RunningWorkload = runningITS
				err := transformer.reconcileAvailableCondition(transCtx)
				Expect(err).Should(BeNil())

				cond := meta.FindStatusCondition(comp.Status.Conditions, appsv1.ConditionTypeAvailable)
				Expect(cond).ShouldNot(BeNil())
				Expect(cond.Status).Should(Equal(metav1.ConditionTrue))
			})

			It("should not be available when phase check fails", func() {
				comp.Status.Phase = appsv1.FailedComponentPhase
				runningITS.Status.InstanceStatus = []workloads.InstanceStatus{
					{PodName: "pod-0", Role: "leader"},
				}
				transCtx.RunningWorkload = runningITS
				err := transformer.reconcileAvailableCondition(transCtx)
				Expect(err).Should(BeNil())

				cond := meta.FindStatusCondition(comp.Status.Conditions, appsv1.ConditionTypeAvailable)
				Expect(cond).ShouldNot(BeNil())
				Expect(cond.Status).Should(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).Should(Equal("PhaseCheckFail"))
			})

			It("should not be available when role check fails", func() {
				comp.Status.Phase = appsv1.RunningComponentPhase
				runningITS.Status.InstanceStatus = []workloads.InstanceStatus{
					{PodName: "pod-0", Role: "follower"},
				}
				transCtx.RunningWorkload = runningITS
				err := transformer.reconcileAvailableCondition(transCtx)
				Expect(err).Should(BeNil())

				cond := meta.FindStatusCondition(comp.Status.Conditions, appsv1.ConditionTypeAvailable)
				Expect(cond).ShouldNot(BeNil())
				Expect(cond.Status).Should(Equal(metav1.ConditionFalse))
				Expect(cond.Reason).Should(Equal("RoleCheckFail"))
			})
		})

		It("should skip setting condition when neither WithPhases nor WithRole is set", func() {
			compDef.Spec.Available = &appsv1.ComponentAvailable{}
			err := transformer.reconcileAvailableCondition(transCtx)
			Expect(err).Should(BeNil())

			cond := meta.FindStatusCondition(comp.Status.Conditions, appsv1.ConditionTypeAvailable)
			Expect(cond).Should(BeNil())
		})
	})
})
