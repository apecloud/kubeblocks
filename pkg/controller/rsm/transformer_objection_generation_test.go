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
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
)

var _ = Describe("object generation transformer test.", func() {
	BeforeEach(func() {
		rsm = builder.NewReplicatedStateMachineBuilder(namespace, name).
			SetUID(uid).
			AddLabels(constant.AppComponentLabelKey, name).
			SetReplicas(3).
			AddMatchLabelsInMap(selectors).
			SetServiceName(headlessSvcName).
			SetRoles(roles).
			SetService(service).
			SetCredential(credential).
			SetTemplate(template).
			SetCustomHandler(observeActions).
			GetObject()

		transCtx = &rsmTransformContext{
			Context:       ctx,
			Client:        graphCli,
			EventRecorder: nil,
			Logger:        logger,
			rsmOrig:       rsm.DeepCopy(),
			rsm:           rsm,
		}

		transformer = &ObjectGenerationTransformer{}
	})

	Context("Transform function", func() {
		It("should work well", func() {
			sts := builder.NewStatefulSetBuilder(namespace, name).GetObject()
			headlessSvc := builder.NewHeadlessServiceBuilder(name, getHeadlessSvcName(*rsm)).GetObject()
			svc := builder.NewServiceBuilder(name, name).GetObject()
			env := builder.NewConfigMapBuilder(name, name+"-rsm-env").GetObject()
			k8sMock.EXPECT().
				List(gomock.Any(), &apps.StatefulSetList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *apps.StatefulSetList, _ ...client.ListOption) error {
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.ServiceList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.ServiceList, _ ...client.ListOption) error {
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.ConfigMapList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.ConfigMapList, _ ...client.ListOption) error {
					return nil
				}).Times(1)

			dagExpected := mockDAG()
			graphCli.Create(dagExpected, sts)
			graphCli.Create(dagExpected, headlessSvc)
			graphCli.Create(dagExpected, svc)
			graphCli.Create(dagExpected, env)
			graphCli.DependOn(dagExpected, sts, headlessSvc, svc, env)

			// do Transform
			dag := mockDAG()
			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())

			// compare DAGs
			Expect(dag.Equals(dagExpected, less)).Should(BeTrue())

			By("set svc and alternative svcs to nil")
			rsm.Spec.Service = nil
			rsm.Spec.AlternativeServices = nil
			k8sMock.EXPECT().
				List(gomock.Any(), &apps.StatefulSetList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *apps.StatefulSetList, _ ...client.ListOption) error {
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.ServiceList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.ServiceList, _ ...client.ListOption) error {
					return nil
				}).Times(1)
			k8sMock.EXPECT().
				List(gomock.Any(), &corev1.ConfigMapList{}, gomock.Any()).
				DoAndReturn(func(_ context.Context, list *corev1.ConfigMapList, _ ...client.ListOption) error {
					return nil
				}).Times(1)
			dag = mockDAG()
			Expect(transformer.Transform(transCtx, dag)).Should(Succeed())
		})
	})

	Context("buildEnvConfigData function", func() {
		It("should work well", func() {
			By("build env config data")
			rsm.Status.MembersStatus = []workloads.MemberStatus{
				{
					PodName:     getPodName(rsm.Name, 1),
					ReplicaRole: workloads.ReplicaRole{Name: "leader", IsLeader: true},
				},
				{
					PodName:     getPodName(rsm.Name, 0),
					ReplicaRole: workloads.ReplicaRole{Name: "follower", CanVote: true},
				},
				{
					PodName:     getPodName(rsm.Name, 2),
					ReplicaRole: workloads.ReplicaRole{Name: "follower", CanVote: true},
				},
			}
			requiredKeys := []string{
				"KB_REPLICA_COUNT",
				"KB_0_HOSTNAME",
				"KB_CLUSTER_UID",
			}
			cfg := buildEnvConfigData(*rsm)

			By("builds Env Config correctly")
			Expect(cfg).ShouldNot(BeNil())
			for _, k := range requiredKeys {
				_, ok := cfg[k]
				Expect(ok).Should(BeTrue())
			}

			By("builds env config with resources recreate")
			Expect(cfg["KB_CLUSTER_UID"]).Should(BeEquivalentTo(uid))

			By("builds Env Config with ConsensusSet status correctly")
			toCheckKeys := append(requiredKeys, []string{
				"KB_LEADER",
				"KB_FOLLOWERS",
			}...)
			for _, k := range toCheckKeys {
				_, ok := cfg[k]
				Expect(ok).Should(BeTrue())
			}
		})
	})

	Context("well-known service labels", func() {
		It("should work well", func() {
			svc := buildSvc(*rsm)
			Expect(svc).ShouldNot(BeNil())
			for k, ev := range service.Labels {
				v, ok := svc.Labels[k]
				Expect(ok).Should(BeTrue())
				Expect(v).Should(Equal(ev))
			}
		})
	})

	Context("injectRoleProbeBaseContainer function", func() {
		It("should reuse container 'kb-checkrole' if exists", func() {
			templateCopy := template.DeepCopy()
			templateCopy.Spec.Containers = append(templateCopy.Spec.Containers, corev1.Container{
				Name:  constant.RoleProbeContainerName,
				Image: "bar",
			})
			injectRoleProbeBaseContainer(*rsm, templateCopy, "", nil)
			Expect(len(templateCopy.Spec.Containers)).Should(Equal(2))
			probeContainer := templateCopy.Spec.Containers[1]
			Expect(probeContainer.ReadinessProbe).ShouldNot(BeNil())
			Expect(len(probeContainer.Ports)).Should(Equal(1))
			Expect(probeContainer.Ports[0].ContainerPort).Should(BeElementOf([]int32{int32(defaultRoleProbeGRPCPort), int32(defaultRoleProbeDaemonPort)}))
		})

		It("should not use default grpcPort in case of 'lorry-grpc-port' existence", func() {
			rsm.Spec.RoleProbe.RoleUpdateMechanism = workloads.ReadinessProbeEventUpdate
			templateCopy := template.DeepCopy()
			templateCopy.Spec.Containers = append(templateCopy.Spec.Containers, corev1.Container{
				Name:  constant.RoleProbeContainerName,
				Image: "bar",
				Ports: []corev1.ContainerPort{
					{
						Name:          constant.LorryGRPCPortName,
						ContainerPort: 9555,
					},
				},
			})
			injectRoleProbeBaseContainer(*rsm, templateCopy, "", nil)
			Expect(len(templateCopy.Spec.Containers)).Should(Equal(2))
			probeContainer := templateCopy.Spec.Containers[1]
			Expect(len(probeContainer.Ports)).Should(Equal(1))
			Expect(probeContainer.Ports[0].ContainerPort).Should(Equal(int32(9555)))
		})

		It("container.ports nil", func() {
			rsm.Spec.RoleProbe.RoleUpdateMechanism = workloads.ReadinessProbeEventUpdate
			templateCopy := template.DeepCopy()
			templateCopy.Spec.Containers = append(templateCopy.Spec.Containers, corev1.Container{
				Name:  constant.RoleProbeContainerName,
				Image: "bar",
				Ports: nil,
			})
			injectRoleProbeBaseContainer(*rsm, templateCopy, "", nil)
			Expect(len(templateCopy.Spec.Containers)).Should(Equal(2))
			probeContainer := templateCopy.Spec.Containers[1]
			Expect(len(probeContainer.Ports)).Should(Equal(1))
			Expect(probeContainer.Ports[0].ContainerPort).Should(Equal(int32(defaultRoleProbeGRPCPort)))
		})

		It("container.ports.containerPort negative", func() {
			rsm.Spec.RoleProbe.RoleUpdateMechanism = workloads.ReadinessProbeEventUpdate
			templateCopy := template.DeepCopy()
			templateCopy.Spec.Containers = append(templateCopy.Spec.Containers, corev1.Container{
				Name:  constant.RoleProbeContainerName,
				Image: "bar",
				Ports: []corev1.ContainerPort{
					{
						Name:          constant.LorryGRPCPortName,
						ContainerPort: -9999,
					},
				},
			})
			injectRoleProbeBaseContainer(*rsm, templateCopy, "", nil)
			Expect(len(templateCopy.Spec.Containers)).Should(Equal(2))
			probeContainer := templateCopy.Spec.Containers[1]
			Expect(len(probeContainer.Ports)).Should(Equal(1))
			Expect(probeContainer.Ports[0].ContainerPort).Should(Equal(int32(defaultRoleProbeGRPCPort)))
		})

	})

})
