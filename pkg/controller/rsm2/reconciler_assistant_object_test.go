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

package rsm2

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	rsm1 "github.com/apecloud/kubeblocks/pkg/controller/rsm"
)

var _ = Describe("assistant object reconciler test", func() {
	const (
		namespace = "foo"
		name      = "bar"
	)

	var (
		rsm        *workloads.ReplicatedStateMachine
		reconciler kubebuilderx.Reconciler

		uid = types.UID("rsm-mock-uid")

		selectors = map[string]string{
			constant.AppInstanceLabelKey:    name,
			rsm1.WorkloadsManagedByLabelKey: rsm1.KindReplicatedStateMachine,
		}
		roles = []workloads.ReplicaRole{
			{
				Name:       "leader",
				IsLeader:   true,
				CanVote:    true,
				AccessMode: workloads.ReadWriteMode,
			},
			{
				Name:       "follower",
				IsLeader:   false,
				CanVote:    true,
				AccessMode: workloads.ReadonlyMode,
			},
			{
				Name:       "logger",
				IsLeader:   false,
				CanVote:    true,
				AccessMode: workloads.NoneMode,
			},
			{
				Name:       "learner",
				IsLeader:   false,
				CanVote:    false,
				AccessMode: workloads.ReadonlyMode,
			},
		}
		pod = builder.NewPodBuilder("", "").
			AddContainer(corev1.Container{
				Name:  "foo",
				Image: "bar",
				Ports: []corev1.ContainerPort{
					{
						Name:          "my-svc",
						Protocol:      corev1.ProtocolTCP,
						ContainerPort: 12345,
					},
				},
			}).GetObject()
		template = corev1.PodTemplateSpec{
			ObjectMeta: pod.ObjectMeta,
			Spec:       pod.Spec,
		}

		volumeClaimTemplates = []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "data",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					Resources: corev1.ResourceRequirements{
						Requests: map[corev1.ResourceName]resource.Quantity{
							corev1.ResourceStorage: resource.MustParse("2G"),
						},
					},
				},
			},
		}
	)

	BeforeEach(func() {
		rsm = builder.NewReplicatedStateMachineBuilder(namespace, name).
			SetUID(uid).
			SetReplicas(3).
			AddMatchLabelsInMap(selectors).
			SetTemplate(template).
			SetVolumeClaimTemplates(volumeClaimTemplates...).
			SetRoles(roles).
			GetObject()
	})

	Context("PreCondition & Reconcile", func() {
		It("should work well", func() {
			By("PreCondition")
			rsm.Generation = 1
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(rsm)
			reconciler = NewAssistantObjectReconciler()
			Expect(reconciler.PreCondition(tree)).Should(Equal(kubebuilderx.ResultSatisfied))

			By("do reconcile")
			_, err := reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			// desired: svc: "bar-headless", cm: "bar"
			objects := tree.GetSecondaryObjects()
			Expect(objects).Should(HaveLen(2))
			svc := builder.NewHeadlessServiceBuilder(namespace, name+"-headless").GetObject()
			cm := builder.NewConfigMapBuilder(namespace, name+"-rsm-env").GetObject()
			for _, object := range []client.Object{svc, cm} {
				name, err := model.GetGVKName(object)
				Expect(err).Should(BeNil())
				_, ok := objects[*name]
				Expect(ok).Should(BeTrue())
			}
		})
	})
})
