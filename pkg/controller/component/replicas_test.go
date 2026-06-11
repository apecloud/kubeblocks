/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"context"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

type conflictOnceClient struct {
	client.Client
	conflict bool
}

func (c *conflictOnceClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if c.conflict {
		c.conflict = false
		return apierrors.NewConflict(schema.GroupResource{Group: "workloads.kubeblocks.io", Resource: "instancesets"}, obj.GetName(), nil)
	}
	return c.Client.Update(ctx, obj, opts...)
}

var _ = Describe("replicas", func() {
	var (
		its      *workloads.InstanceSet
		replicas []string
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	Context("status", func() {
		BeforeEach(func() {
			its = &workloads.InstanceSet{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:         testCtx.DefaultNamespace,
					Name:              "test-cluster-its",
					CreationTimestamp: metav1.Now(),
					Annotations: map[string]string{
						constant.KubeBlocksGenerationKey: "1",
					},
				},
				Spec: workloads.InstanceSetSpec{
					Replicas: ptr.To[int32](3),
				},
			}
			replicas = []string{"test-cluster-its-0", "test-cluster-its-1", "test-cluster-its-2"}
		})

		It("status init replicas", func() {
			Expect(StatusReplicasStatus(its, replicas, true, true)).Should(Succeed())
			Expect(its.Annotations).Should(HaveKey(replicaStatusAnnotationKey))

			status, err := getReplicasStatus(its)
			Expect(err).Should(BeNil())
			Expect(status.Replicas).Should(Equal(int32(3)))
			Expect(status.Status).Should(HaveLen(int(status.Replicas)))
			for _, s := range status.Status {
				Expect(replicas).Should(ContainElement(s.Name))
				Expect(s.Generation).Should(Equal("1"))
				Expect(s.CreationTimestamp.Equal(its.CreationTimestamp.Time)).Should(BeTrue())
				Expect(s.Provisioned).Should(BeTrue())
				Expect(s.DataLoaded).ShouldNot(BeNil())
				Expect(*s.DataLoaded).Should(BeTrue())
				Expect(s.MemberJoined).ShouldNot(BeNil())
				Expect(*s.MemberJoined).Should(BeTrue())
			}
		})

		It("new replicas", func() {
			Expect(StatusReplicasStatus(its, replicas, true, true)).Should(Succeed())

			its.Annotations[constant.KubeBlocksGenerationKey] = "2"
			its.Spec.Replicas = ptr.To[int32](5)
			newReplicas := []string{"test-cluster-its-3", "test-cluster-its-4"}
			Expect(NewReplicasStatus(its, newReplicas, true, true)).Should(Succeed())

			status, err := getReplicasStatus(its)
			Expect(err).Should(BeNil())
			Expect(status.Replicas).Should(Equal(int32(5)))
			Expect(status.Status).Should(HaveLen(int(status.Replicas)))
			for _, s := range status.Status {
				if slices.Contains(newReplicas, s.Name) {
					Expect(s.Generation).Should(Equal("2"))
					Expect(s.CreationTimestamp.Equal(its.CreationTimestamp.Time)).Should(BeFalse())
					Expect(s.Provisioned).Should(BeFalse())
					Expect(s.DataLoaded).ShouldNot(BeNil())
					Expect(*s.DataLoaded).Should(BeFalse())
					Expect(s.MemberJoined).ShouldNot(BeNil())
					Expect(*s.MemberJoined).Should(BeFalse())
				}
			}
		})

		It("delete replicas", func() {
			Expect(StatusReplicasStatus(its, replicas, true, true)).Should(Succeed())

			its.Annotations[constant.KubeBlocksGenerationKey] = "2"
			its.Spec.Replicas = ptr.To[int32](2)
			deleteReplicas := []string{"test-cluster-its-2"}
			Expect(DeleteReplicasStatus(its, deleteReplicas, func(s ReplicaStatus) {
				Expect(s.Provisioned).Should(BeTrue())
				Expect(s.DataLoaded).ShouldNot(BeNil())
				Expect(*s.DataLoaded).Should(BeTrue())
				Expect(s.MemberJoined).ShouldNot(BeNil())
				Expect(*s.MemberJoined).Should(BeTrue())
			})).Should(Succeed())

			status, err := getReplicasStatus(its)
			Expect(err).Should(BeNil())
			Expect(status.Replicas).Should(Equal(int32(2)))
			Expect(status.Status).Should(HaveLen(int(status.Replicas)))
		})

		It("status new replicas", func() {
			Expect(StatusReplicasStatus(its, replicas, true, true)).Should(Succeed())

			its.Annotations[constant.KubeBlocksGenerationKey] = "2"
			its.Spec.Replicas = ptr.To[int32](5)
			newReplicas := []string{"test-cluster-its-3", "test-cluster-its-4"}
			Expect(NewReplicasStatus(its, newReplicas, true, true)).Should(Succeed())

			replicas = append(replicas, "test-cluster-its-3")
			Expect(StatusReplicasStatus(its, replicas, true, true)).Should(Succeed())

			status, err := getReplicasStatus(its)
			Expect(err).Should(BeNil())
			for _, s := range status.Status {
				if s.Name == "test-cluster-its-3" {
					Expect(s.Provisioned).Should(BeTrue()) // provisioned
					Expect(s.DataLoaded).ShouldNot(BeNil())
					Expect(*s.DataLoaded).Should(BeFalse()) // not loaded
					Expect(s.MemberJoined).ShouldNot(BeNil())
					Expect(*s.MemberJoined).Should(BeFalse()) // not joined
				}
			}
		})

		It("delete new replicas", func() {
			Expect(StatusReplicasStatus(its, replicas, true, true)).Should(Succeed())

			its.Annotations[constant.KubeBlocksGenerationKey] = "2"
			its.Spec.Replicas = ptr.To[int32](5)
			newReplicas := []string{"test-cluster-its-3", "test-cluster-its-4"}
			Expect(NewReplicasStatus(its, newReplicas, true, true)).Should(Succeed())

			its.Annotations[constant.KubeBlocksGenerationKey] = "3"
			its.Spec.Replicas = ptr.To[int32](4)
			deleteReplicas := []string{"test-cluster-its-4"}
			Expect(DeleteReplicasStatus(its, deleteReplicas, func(s ReplicaStatus) {
				Expect(s.Provisioned).Should(BeFalse())
				Expect(s.DataLoaded).ShouldNot(BeNil())
				Expect(*s.DataLoaded).Should(BeFalse())
				Expect(s.MemberJoined).ShouldNot(BeNil())
				Expect(*s.MemberJoined).Should(BeFalse())
			})).Should(Succeed())
		})

		It("retries member-joined status update on conflict", func() {
			key := types.NamespacedName{Namespace: testCtx.DefaultNamespace, Name: "test-cluster-its-member-join-retry"}
			_ = k8sClient.Delete(ctx, &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name}})

			its.Name = key.Name
			its.Namespace = key.Namespace
			its.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"app": key.Name}}
			its.Spec.Template = corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": key.Name}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "test", Image: "test"}},
				},
			}
			Expect(StatusReplicasStatus(its, replicas, true, true)).Should(Succeed())
			its.Spec.Replicas = ptr.To[int32](4)
			newReplica := "test-cluster-its-3"
			Expect(NewReplicasStatus(its, []string{newReplica}, true, false)).Should(Succeed())
			Expect(k8sClient.Create(ctx, its)).Should(Succeed())

			cli := &conflictOnceClient{Client: k8sClient, conflict: true}
			Expect(UpdateReplicaStatusWithRetry(ctx, cli, key, newReplica, func(status *ReplicaStatus) error {
				status.MemberJoined = ptr.To(true)
				return nil
			})).Should(Succeed())

			got := &workloads.InstanceSet{}
			Expect(k8sClient.Get(ctx, key, got)).Should(Succeed())
			status, err := getReplicasStatus(got)
			Expect(err).Should(BeNil())
			i := slices.IndexFunc(status.Status, func(s ReplicaStatus) bool {
				return s.Name == newReplica
			})
			Expect(i).Should(BeNumerically(">=", 0))
			Expect(status.Status[i].MemberJoined).ShouldNot(BeNil())
			Expect(*status.Status[i].MemberJoined).Should(BeTrue())
		})

		// It("task event for new replicas - succeed", func() {
		//	Expect(StatusReplicasStatus(its, replicas, true, true)).Should(Succeed())
		//
		//	its.Annotations[constant.KubeBlocksGenerationKey] = "2"
		//	its.Spec.Replicas = ptr.To[int32](5)
		//	newReplicas := []string{"test-cluster-its-3", "test-cluster-its-4"}
		//	Expect(NewReplicasStatus(its, newReplicas, true, true)).Should(Succeed())
		//
		//	cli := testutil.NewK8sMockClient()
		//  cli.MockGetMethod(testutil.WithGetReturned(func(key client.ObjectKey, obj client.Object) error {
		//	    // TODO: mock
		//	    return fmt.Errorf("not found")
		//  }, testutil.WithAnyTimes()))
		//	cli.MockUpdateMethod(testutil.WithSucceed(testutil.WithAnyTimes()))
		//	event := proto.TaskEvent{
		//		Instance: "test-cluster-its",
		//		Replica:  "test-cluster-its-3",
		//		EndTime:  time.Now(),
		//		Code:     0,
		//	}
		//	Expect(handleNewReplicaTaskEvent(logger, testCtx.Ctx, cli.Client(), testCtx.DefaultNamespace, event)).Should(Succeed())
		//
		//	status, err := getReplicasStatus(its)
		//	Expect(err).Should(BeNil())
		//	for _, s := range status.Status {
		//		if s.Name == "test-cluster-its-3" {
		//			Expect(s.Provisioned).Should(BeTrue()) // provisioned
		//			Expect(s.DataLoaded).ShouldNot(BeNil())
		//			Expect(*s.DataLoaded).Should(BeTrue()) // loaded
		//			Expect(s.MemberJoined).ShouldNot(BeNil())
		//			Expect(*s.MemberJoined).Should(BeFalse()) // not joined
		//		}
		//	}
		// })
		//
		// It("task event for new replicas - failed", func() {
		//	Expect(StatusReplicasStatus(its, replicas, true, true)).Should(Succeed())
		//
		//	its.Annotations[constant.KubeBlocksGenerationKey] = "2"
		//	its.Spec.Replicas = ptr.To[int32](5)
		//	newReplicas := []string{"test-cluster-its-3", "test-cluster-its-4"}
		//	Expect(NewReplicasStatus(its, newReplicas, true, true)).Should(Succeed())
		//
		//	cli := testutil.NewK8sMockClient()
		//  cli.MockGetMethod(testutil.WithGetReturned(func(key client.ObjectKey, obj client.Object) error {
		//	    // TODO: mock
		//	    return fmt.Errorf("not found")
		//  }, testutil.WithAnyTimes()))
		//	cli.MockUpdateMethod(testutil.WithSucceed(testutil.WithAnyTimes()))
		//	event := proto.TaskEvent{
		//		Instance: "test-cluster-its",
		//		Replica:  "test-cluster-its-3",
		//		EndTime:  time.Now(),
		//		Code:     -1,
		//		Message:  "failed",
		//	}
		//	Expect(handleNewReplicaTaskEvent(logger, testCtx.Ctx, cli.Client(), testCtx.DefaultNamespace, event)).Should(Succeed())
		//
		//	status, err := getReplicasStatus(its)
		//	Expect(err).Should(BeNil())
		//	for _, s := range status.Status {
		//		if s.Name == "test-cluster-its-3" {
		//			Expect(s.Provisioned).Should(BeTrue()) // provisioned
		//			Expect(s.DataLoaded).ShouldNot(BeNil())
		//			Expect(*s.DataLoaded).Should(BeFalse()) // not loaded
		//			Expect(s.MemberJoined).ShouldNot(BeNil())
		//			Expect(*s.MemberJoined).Should(BeFalse()) // not joined
		//			Expect(s.Message).Should(Equal("failed"))
		//		}
		//	}
		// })
		//
		// It("task event for new replicas - in progress", func() {
		//	Expect(StatusReplicasStatus(its, replicas, true, true)).Should(Succeed())
		//
		//	its.Annotations[constant.KubeBlocksGenerationKey] = "2"
		//	its.Spec.Replicas = ptr.To[int32](5)
		//	newReplicas := []string{"test-cluster-its-3", "test-cluster-its-4"}
		//	Expect(NewReplicasStatus(its, newReplicas, true, true)).Should(Succeed())
		//
		//  cli := testutil.NewK8sMockClient()
		//  cli.MockGetMethod(testutil.WithGetReturned(func(key client.ObjectKey, obj client.Object) error {
		//	    // TODO: mock
		//	    return fmt.Errorf("not found")
		//  }, testutil.WithAnyTimes()))
		//	cli.MockUpdateMethod(testutil.WithSucceed(testutil.WithAnyTimes()))
		//	event := proto.TaskEvent{
		//		Instance: "test-cluster-its",
		//		Replica:  "test-cluster-its-3",
		//		// EndTime: time.Now(),
		//		Code:    0,
		//		Message: "90",
		//	}
		//	Expect(handleNewReplicaTaskEvent(logger, testCtx.Ctx, cli.Client(), testCtx.DefaultNamespace, event)).Should(Succeed())
		//
		//	status, err := getReplicasStatus(its)
		//	Expect(err).Should(BeNil())
		//	for _, s := range status.Status {
		//		if s.Name == "test-cluster-its-3" {
		//			Expect(s.Provisioned).Should(BeTrue()) // provisioned
		//			Expect(s.DataLoaded).ShouldNot(BeNil())
		//			Expect(*s.DataLoaded).Should(BeFalse()) // not loaded
		//			Expect(s.MemberJoined).ShouldNot(BeNil())
		//			Expect(*s.MemberJoined).Should(BeFalse()) // not joined
		//			Expect(s.Message).Should(Equal("90"))
		//		}
		//	}
		// })
	})
})
