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

package workloads

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instance"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	workloadlifecycle "github.com/apecloud/kubeblocks/pkg/controller/workloads/lifecycle"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("InstanceSet Controller 2", func() {
	var (
		itsName  = "test-cluster-its"
		itsObj   *workloads.InstanceSet
		itsKey   client.ObjectKey
		replicas = int32(3)
	)

	cleanEnv := func() {
		// must wait till resources deleted and no longer existed before the testcases start,
		// otherwise if later it needs to create some new resource objects with the same name,
		// in race conditions, it will find the existence of old objects, resulting failure to
		// create the new objects.
		By("clean resources")

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}
		// namespaced
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.InstanceSetSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.InstanceSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ServiceSignature, true, inNS, ml)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	createITSObj := func(name string, processors ...func(factory *testapps.MockInstanceSetFactory)) {
		By("create a ITS object")
		container := corev1.Container{
			Name:  "foo",
			Image: "bar",
		}
		f := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace, name, "test-cluster", "comp").
			WithRandomName().
			AddContainer(container).
			SetReplicas(replicas).
			SetEnableInstanceAPI(ptr.To(true)).
			SetPodManagementPolicy(appsv1.ParallelPodManagement)
		for _, processor := range processors {
			if processor != nil {
				processor(f)
			}
		}
		itsObj = f.Create(&testCtx).GetObject()
		itsKey = client.ObjectKeyFromObject(itsObj)

		Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, set *workloads.InstanceSet) {
			g.Expect(set.Status.ObservedGeneration).Should(BeEquivalentTo(1))
		}),
		).Should(Succeed())
	}

	podName := func(ordinal int32) string {
		return fmt.Sprintf("%s-%d", itsKey.Name, ordinal)
	}

	mockPVCCapacities := func() {
		By("report PVC capacities")
		for i := int32(0); i < replicas; i++ {
			for _, claim := range itsObj.Spec.VolumeClaimTemplates {
				key := client.ObjectKey{
					Namespace: itsObj.Namespace,
					Name:      intctrlutil.ComposePVCName(claim, itsObj.Name, podName(i)),
				}
				Eventually(testapps.GetAndChangeObjStatus(&testCtx, key, func(pvc *corev1.PersistentVolumeClaim) {
					pvc.Status.Capacity = pvc.Spec.Resources.Requests.DeepCopy()
				})).Should(Succeed())
			}
		}
	}

	mockPodsReady := func() {
		for i := int32(0); i < replicas; i++ {
			mockPodReady(itsObj.Namespace, podName(i))
		}
	}

	mockPodsReadyNAvailableWithRole := func() {
		for i := int32(0); i < replicas; i++ {
			mockPodReadyNAvailableWithRole(itsObj.Namespace, podName(i), "leader", 0)
		}
	}

	Context("provision", func() {
		It("create & delete", func() {
			createITSObj(itsName, nil)

			Expect(k8sClient.Delete(ctx, itsObj)).Should(Succeed())
			Eventually(testapps.CheckObjExists(&testCtx, itsKey, &workloads.InstanceSet{}, false)).Should(Succeed())
		})

		It("status", func() {
			createITSObj(itsName, nil)

			By("check its not ready")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.IsInstanceSetReady()).Should(BeFalse())
			})).Should(Succeed())

			mockPodsReady()

			By("check its ready")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.IsInstanceSetReady()).Should(BeTrue())
			})).Should(Succeed())
		})

		It("instance status", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.SetRoles([]workloads.ReplicaRole{
					{
						Name:                 "leader",
						UpdatePriority:       2,
						ParticipatesInQuorum: true,
					},
					{
						Name:                 "follower",
						UpdatePriority:       1,
						ParticipatesInQuorum: true,
					},
				})
			})

			By("check its not ready")
			Consistently(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.IsInstanceSetReady()).Should(BeFalse())
			})).Should(Succeed())

			mockPodsReady()

			By("check its not ready")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.IsInstancesReady()).Should(BeTrue())
				g.Expect(its.IsInstanceSetReady()).Should(BeFalse())
			})).Should(Succeed())

			mockPodsReadyNAvailableWithRole()

			By("check its ready")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.IsInstanceSetReady()).Should(BeTrue())
				g.Expect(len(its.Status.InstanceStatus)).Should(Equal(int(replicas)))
				for i := int32(0); i < replicas; i++ {
					g.Expect(its.Status.InstanceStatus[i].Role).Should(Equal("leader"))
				}
			})).Should(Succeed())
		})

		It("instance status - configs", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.AddConfigs(
					workloads.ConfigTemplate{
						Name:       "log",
						ConfigHash: ptr.To("123456"),
					},
					workloads.ConfigTemplate{
						Name:       "server",
						ConfigHash: ptr.To("654321"),
					},
				)
			})

			mockPodsReady()

			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(int(replicas)))
				for i := range its.Status.InstanceStatus {
					g.Expect(its.Status.InstanceStatus[i].Configs).Should(Equal([]workloads.InstanceConfigStatus{
						{
							Name:       "log",
							ConfigHash: ptr.To("123456"),
						},
						{
							Name:       "server",
							ConfigHash: ptr.To("654321"),
						},
					}))
				}
			})).Should(Succeed())
		})

		It("pod management - ordered ready", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.SetPodManagementPolicy(appsv1.OrderedReadyPodManagement)
			})

			for i := int32(0); i < replicas; i++ {
				By(fmt.Sprintf("check its not ready: %d", i))
				Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
					g.Expect(its.IsInstanceSetReady()).Should(BeFalse())
				})).Should(Succeed())

				mockPodReady(itsObj.Namespace, podName(i))
			}

			By("check its ready")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.IsInstanceSetReady()).Should(BeTrue())
			})).Should(Succeed())
		})
	})

	Context("update", func() {
		var (
			pvc = corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testCtx.DefaultNamespace,
					Name:      "data",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1Gi"),
						},
					},
				},
			}
		)

		It("rolling", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.SetInstanceUpdateStrategy(&workloads.InstanceUpdateStrategy{
					Type: kbappsv1.RollingUpdateStrategyType,
					RollingUpdate: &workloads.RollingUpdate{
						MaxUnavailable: &intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: 1, // one instance at a time
						},
					},
				})
			})

			mockPodsReady()

			By("check its ready")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.IsInstanceSetReady()).Should(BeTrue())
			})).Should(Succeed())

			By("update its spec")
			beforeUpdate := time.Now()
			time.Sleep(1 * time.Second)
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
			})()).ShouldNot(HaveOccurred())

			for i := replicas; i > 0; i-- {
				instName := podName(i - 1)
				By(fmt.Sprintf("check instance updated: %s", instName))
				instKey := types.NamespacedName{
					Namespace: itsObj.Namespace,
					Name:      instName,
				}
				Eventually(testapps.CheckObj(&testCtx, instKey, func(g Gomega, inst *workloads.Instance) {
					g.Expect(inst.Spec.Template.Spec.DNSPolicy).Should(Equal(corev1.DNSClusterFirstWithHostNet))
				})).Should(Succeed())

				By("wait new pod created")
				podKey := instKey
				Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
					g.Expect(pod.CreationTimestamp.After(beforeUpdate)).Should(BeTrue())
				})).Should(Succeed())

				// mock new pod ready
				mockPodReady(itsObj.Namespace, instName)

				By(fmt.Sprintf("check instance ready: %s", instName))
				Eventually(testapps.CheckObj(&testCtx, instKey, func(g Gomega, inst *workloads.Instance) {
					g.Expect(intctrlutil.IsInstanceReady(inst)).Should(BeTrue())
				})).Should(Succeed())

				By(fmt.Sprintf("check its status updated: %s", instName))
				Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
					g.Expect(its.Status.UpdatedReplicas).Should(Equal(replicas - i + 1))
				})).Should(Succeed())
			}

			By("check its ready")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.IsInstanceSetReady()).Should(BeTrue())
			})).Should(Succeed())
		})

		It("limits rolling update to replicas", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.SetInstanceUpdateStrategy(&workloads.InstanceUpdateStrategy{
					Type: kbappsv1.RollingUpdateStrategyType,
					RollingUpdate: &workloads.RollingUpdate{
						Replicas:       ptr.To(intstr.FromInt32(1)),
						MaxUnavailable: ptr.To(intstr.FromInt32(2)),
					},
				})
			})

			mockPodsReady()

			By("check its ready")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.IsInstanceSetReady()).Should(BeTrue())
			})).Should(Succeed())

			By("update its spec")
			beforeUpdate := time.Now()
			time.Sleep(1 * time.Second)
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
			})()).ShouldNot(HaveOccurred())

			updatedInstanceKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      podName(replicas - 1),
			}
			By("check the first instance updated")
			Eventually(testapps.CheckObj(&testCtx, updatedInstanceKey, func(g Gomega, inst *workloads.Instance) {
				g.Expect(inst.Spec.Template.Spec.DNSPolicy).Should(Equal(corev1.DNSClusterFirstWithHostNet))
			})).Should(Succeed())

			By("wait for its new pod and mock it ready")
			Eventually(testapps.CheckObj(&testCtx, updatedInstanceKey, func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.CreationTimestamp.After(beforeUpdate)).Should(BeTrue())
			})).Should(Succeed())
			mockPodReady(itsObj.Namespace, updatedInstanceKey.Name)
			Eventually(testapps.CheckObj(&testCtx, updatedInstanceKey, func(g Gomega, inst *workloads.Instance) {
				g.Expect(intctrlutil.IsInstanceReady(inst)).Should(BeTrue())
			})).Should(Succeed())

			By("keep the remaining instances outside the rolling-update window")
			Consistently(func(g Gomega) {
				for i := int32(0); i < replicas-1; i++ {
					inst := &workloads.Instance{}
					key := types.NamespacedName{Namespace: itsObj.Namespace, Name: podName(i)}
					g.Expect(testCtx.Cli.Get(testCtx.Ctx, key, inst)).Should(Succeed())
					g.Expect(inst.Spec.Template.Spec.DNSPolicy).ShouldNot(Equal(corev1.DNSClusterFirstWithHostNet))
				}
			}).Should(Succeed())
		})

		It("uses status role order for a roleful rolling-update window", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.SetRoles([]workloads.ReplicaRole{
					{
						Name:                 "follower",
						UpdatePriority:       1,
						ParticipatesInQuorum: true,
					},
					{
						Name:                 "leader",
						UpdatePriority:       2,
						ParticipatesInQuorum: true,
					},
				}).SetInstanceUpdateStrategy(&workloads.InstanceUpdateStrategy{
					Type: kbappsv1.RollingUpdateStrategyType,
					RollingUpdate: &workloads.RollingUpdate{
						Replicas:       ptr.To(intstr.FromInt32(1)),
						MaxUnavailable: ptr.To(intstr.FromInt32(1)),
					},
				})
			})

			// The follower has the lowest name but the highest update precedence.
			mockPodReadyNAvailableWithRole(itsObj.Namespace, podName(0), "follower", 0)
			for i := int32(1); i < replicas; i++ {
				mockPodReadyNAvailableWithRole(itsObj.Namespace, podName(i), "leader", 0)
			}

			By("check its ready with observed roles")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.IsInstanceSetReady()).Should(BeTrue())
			})).Should(Succeed())

			By("update its spec")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
			})()).ShouldNot(HaveOccurred())

			followerKey := types.NamespacedName{Namespace: itsObj.Namespace, Name: podName(0)}
			By("update the follower inside the role-ordered window")
			Eventually(testapps.CheckObj(&testCtx, followerKey, func(g Gomega, inst *workloads.Instance) {
				g.Expect(inst.Spec.Template.Spec.DNSPolicy).Should(Equal(corev1.DNSClusterFirstWithHostNet))
			})).Should(Succeed())

			By("keep leaders outside the rolling-update window")
			Consistently(func(g Gomega) {
				for i := int32(1); i < replicas; i++ {
					inst := &workloads.Instance{}
					key := types.NamespacedName{Namespace: itsObj.Namespace, Name: podName(i)}
					g.Expect(testCtx.Cli.Get(testCtx.Ctx, key, inst)).Should(Succeed())
					g.Expect(inst.Spec.Template.Spec.DNSPolicy).ShouldNot(Equal(corev1.DNSClusterFirstWithHostNet))
				}
			}).Should(Succeed())
		})

		It("scale-in", func() {
			createITSObj(itsName)

			mockPodsReady()

			By("check its ready")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.IsInstanceSetReady()).Should(BeTrue())
			})).Should(Succeed())

			By("scale in")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Replicas = ptr.To(replicas - 1)
			})()).ShouldNot(HaveOccurred())

			By("check its updated and ready")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.Replicas).Should(Equal(replicas - 1))
				g.Expect(its.Status.ReadyReplicas).Should(Equal(replicas - 1))
				g.Expect(its.IsInstanceSetReady()).Should(BeTrue())
			})).Should(Succeed())
		})

		It("scale-in - delete pvc", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.AddVolumeClaimTemplate(pvc).
					SetPVCRetentionPolicy(&workloads.PersistentVolumeClaimRetentionPolicy{
						WhenScaled: kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType,
					})
			})

			mockPVCCapacities()
			mockPodsReady()

			By("check its ready")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.IsInstanceSetReady()).Should(BeTrue())
			})).Should(Succeed())

			By("scale in")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Replicas = ptr.To(replicas - 1)
			})()).ShouldNot(HaveOccurred())

			By("check pods deleted")
			podKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      podName(replicas - 1),
			}
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())

			By("check PVCs deleted, but the pvc-protection finalizer prevent the pvc to be deleted physically")
			pvcKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s", pvc.Name, podKey.Name),
			}
			Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, pvc *corev1.PersistentVolumeClaim) {
				g.Expect(pvc.DeletionTimestamp).ShouldNot(BeNil())
				g.Expect(pvc.Finalizers).To(HaveLen(1))
				g.Expect(pvc.Finalizers[0]).To(Equal("kubernetes.io/pvc-protection"))
			})).Should(Succeed())
		})

		It("scale-in - retain pvc", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.AddVolumeClaimTemplate(pvc).
					SetPVCRetentionPolicy(&workloads.PersistentVolumeClaimRetentionPolicy{
						WhenScaled: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType,
					})
			})

			mockPVCCapacities()
			mockPodsReady()

			By("check its ready")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.IsInstanceSetReady()).Should(BeTrue())
			})).Should(Succeed())

			By("scale in")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Replicas = ptr.To(replicas - 1)
			})()).ShouldNot(HaveOccurred())

			By("check pods deleted")
			podKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      podName(replicas - 1),
			}
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())

			By("check PVCs retained and not deleted")
			pvcKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s", pvc.Name, podKey.Name),
			}
			Consistently(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, pvc *corev1.PersistentVolumeClaim) {
				g.Expect(pvc.DeletionTimestamp).Should(BeNil())
			})).Should(Succeed())
		})

		It("scale-out", func() {
			createITSObj(itsName)

			mockPodsReady()

			By("check its ready")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.IsInstanceSetReady()).Should(BeTrue())
			})).Should(Succeed())

			By("scale out")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Replicas = ptr.To(replicas + 1)
			})()).ShouldNot(HaveOccurred())

			By("check its updated and not ready")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.IsInstanceSetReady()).Should(BeFalse())
				g.Expect(its.Status.Replicas).Should(Equal(replicas + 1))
				g.Expect(its.Status.ReadyReplicas).Should(Equal(replicas))
			})).Should(Succeed())

			// mock new replicas ready
			mockPodReady(itsObj.Namespace, podName(replicas))

			By("check its ready")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.IsInstanceSetReady()).Should(BeTrue())
				g.Expect(its.Status.Replicas).Should(Equal(replicas + 1))
				g.Expect(its.Status.ReadyReplicas).Should(Equal(replicas + 1))
			})).Should(Succeed())
		})
	})
})

type its2LifecycleClient struct {
	client.Client
	failStatus bool
}
type its2LifecycleStatus struct {
	client.SubResourceWriter
	owner *its2LifecycleClient
}

func (c *its2LifecycleClient) Status() client.SubResourceWriter {
	return &its2LifecycleStatus{SubResourceWriter: c.Client.Status(), owner: c}
}
func (s *its2LifecycleStatus) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	if _, ok := obj.(*workloads.InstanceSet); ok && s.owner.failStatus {
		s.owner.failStatus = false
		return apierrors.NewConflict(schema.GroupResource{Group: workloads.SchemeGroupVersion.Group, Resource: "instancesets"}, obj.GetName(), fmt.Errorf("injected status conflict"))
	}
	return s.SubResourceWriter.Update(ctx, obj, opts...)
}

type its2LifecycleAgent struct{ calls []string }

func (a *its2LifecycleAgent) Close() error { return nil }
func (a *its2LifecycleAgent) Action(_ context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
	a.calls = append(a.calls, req.Action)
	return proto.ActionResponse{}, nil
}

func its2LifecycleFixture(t *testing.T, replicas int32, data bool, provided ...client.Client) (*workloads.InstanceSet, *its2LifecycleClient, *InstanceSetReconciler2) {
	t.Helper()
	action := &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"true"}}}
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "lifecycle", Namespace: "default", UID: "its-uid", Generation: 1, Finalizers: []string{"instanceset.workloads.kubeblocks.io/finalizer"}}, Spec: workloads.InstanceSetSpec{EnableInstanceAPI: ptr.To(true), Replicas: ptr.To(replicas), PodManagementPolicy: appsv1.ParallelPodManagement, Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "lifecycle"}}, Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "lifecycle"}}, Spec: corev1.PodSpec{ServiceAccountName: "data-worker", Containers: []corev1.Container{{Name: "database", Image: "database:v1"}}}}, LifecycleActions: &workloads.LifecycleActions{MemberJoin: action, MemberLeave: action, Worker: &corev1.Container{Name: "data-worker", Image: "agent:v1"}}}}
	if data {
		its.Spec.LifecycleActions.DataDump = action
		its.Spec.LifecycleActions.DataLoad = action
		its.Spec.LifecycleActions.DataVolume = "data"
		its.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}, Spec: corev1.PersistentVolumeClaimSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}}}}}
	}
	var storage client.Client
	if len(provided) > 0 && provided[0] != nil {
		storage = provided[0]
		if err := storage.Create(context.Background(), its); err != nil {
			t.Fatal(err)
		}
	} else {
		storage = fake.NewClientBuilder().WithScheme(model.GetScheme()).WithStatusSubresource(&workloads.InstanceSet{}, &workloads.Instance{}, &corev1.Pod{}).WithObjects(its).Build()
	}
	cli := &its2LifecycleClient{Client: storage}
	return its, cli, &InstanceSetReconciler2{Client: cli, Scheme: model.GetScheme(), Recorder: record.NewFakeRecorder(100)}
}
func its2LifecycleRound(t *testing.T, r *InstanceSetReconciler2, its *workloads.InstanceSet) {
	t.Helper()
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(its)}); err != nil {
		t.Fatal(err)
	}
}
func its2LifecycleRead(t *testing.T, cli client.Client, its *workloads.InstanceSet) *workloads.InstanceSet {
	t.Helper()
	observed := &workloads.InstanceSet{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(its), observed); err != nil {
		t.Fatal(err)
	}
	return observed
}
func its2LifecycleInstance(t *testing.T, cli client.Client, name string) *workloads.Instance {
	t.Helper()
	inst := &workloads.Instance{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, inst); err != nil {
		t.Fatal(err)
	}
	return inst
}
func TestITS2LifecycleRegistrationCommit(t *testing.T) { checkITS2LifecycleRegistrationCommit(t, nil) }
func checkITS2LifecycleRegistrationCommit(t *testing.T, storage client.Client) {
	its, cli, r := its2LifecycleFixture(t, 2, true, storage)
	cli.failStatus = true
	its2LifecycleRound(t, r, its)
	stored := its2LifecycleRead(t, cli, its)
	list := &workloads.InstanceList{}
	if err := cli.List(context.Background(), list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 0 || len(stored.Status.InstanceStatus) != 0 || stored.Status.ObservedGeneration != 0 {
		t.Fatalf("failed registration created resources or advanced status: %#v %#v", stored.Status, list.Items)
	}
	its2LifecycleRound(t, r, its)
	stored = its2LifecycleRead(t, cli, its)
	if len(stored.Status.InstanceStatus) != 2 || stored.Status.ObservedGeneration != 0 {
		t.Fatalf("registration did not precede revision: %#v", stored.Status)
	}
	for _, status := range stored.Status.InstanceStatus {
		if status.DataLoaded != nil || status.MemberJoined != nil {
			t.Fatalf("initial instances must keep unknown engine bootstrap results: %#v", status)
		}
	}
	if err := cli.List(context.Background(), list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 0 {
		t.Fatal("registration round created Instance")
	}
	// Restart and expand before initial Instance resources exist.
	stored.Spec.Replicas = ptr.To[int32](3)
	stored.Generation = 2
	if err := cli.Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	r = &InstanceSetReconciler2{Client: cli, Scheme: model.GetScheme(), Recorder: record.NewFakeRecorder(100)}
	its2LifecycleRound(t, r, its)
	stored = its2LifecycleRead(t, cli, its)
	status := stored.Status.InstanceStatus[2]
	if status.DataLoaded == nil || *status.DataLoaded || status.MemberJoined == nil || *status.MemberJoined {
		t.Fatalf("new allocation was not registered as expansion: %#v", status)
	}
	if err := cli.List(context.Background(), list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 0 {
		t.Fatal("expansion registration created Instance")
	}
}
func TestITS2LifecycleLeaveCommitAndSpecReversal(t *testing.T) {
	checkITS2LifecycleLeaveCommitAndSpecReversal(t, nil)
}
func checkITS2LifecycleLeaveCommitAndSpecReversal(t *testing.T, storage client.Client) {
	its, cli, r := its2LifecycleFixture(t, 1, false, storage)
	its2LifecycleRound(t, r, its)
	its2LifecycleRound(t, r, its)
	inst := its2LifecycleInstance(t, cli, "lifecycle-0")
	inst.Status.CurrentState = workloads.InstanceCurrentStatePresent
	inst.Status.Ready = true
	inst.Status.Available = true
	if err := cli.Status().Update(context.Background(), inst); err != nil {
		t.Fatal(err)
	}
	pod, err := instance.BuildPod(inst)
	if err != nil {
		t.Fatal(err)
	}
	pod.Status = corev1.PodStatus{Phase: corev1.PodRunning, PodIP: "127.0.0.1", Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}
	savedStatus := pod.Status
	if err := cli.Create(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	pod.Status = savedStatus
	if err := cli.Status().Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	its2LifecycleRound(t, r, its)
	stored := its2LifecycleRead(t, cli, its)
	stored.Spec.Replicas = ptr.To[int32](0)
	stored.Generation++
	if err := cli.Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	agent := &its2LifecycleAgent{}
	kbacli.SetMockClient(agent, nil)
	defer kbacli.UnsetMockClient()
	cli.failStatus = true
	its2LifecycleRound(t, r, its)
	inst = its2LifecycleInstance(t, cli, "lifecycle-0")
	if ptr.Deref(inst.Spec.ScaledDown, false) || !inst.DeletionTimestamp.IsZero() {
		t.Fatal("Leave status conflict started Instance deletion")
	}
	if len(agent.calls) != 1 || agent.calls[0] != "memberLeave" {
		t.Fatalf("expected actual Leave call: %v", agent.calls)
	}
	its2LifecycleRound(t, r, its)
	stored = its2LifecycleRead(t, cli, its)
	if len(stored.Status.InstanceStatus) != 1 || stored.Status.InstanceStatus[0].MemberJoined == nil || *stored.Status.InstanceStatus[0].MemberJoined {
		t.Fatalf("Leave result not committed: %#v", stored.Status.InstanceStatus)
	}
	inst = its2LifecycleInstance(t, cli, "lifecycle-0")
	if ptr.Deref(inst.Spec.ScaledDown, false) {
		t.Fatal("successful Leave round prematurely marked ScaledDown")
	}
	stored.Spec.Replicas = ptr.To[int32](1)
	stored.Generation++
	if err := cli.Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	its2LifecycleRound(t, r, its)
	stored = its2LifecycleRead(t, cli, its)
	inst = its2LifecycleInstance(t, cli, "lifecycle-0")
	if !ptr.Deref(stored.Status.InstanceStatus[0].MemberJoined, false) || ptr.Deref(inst.Spec.ScaledDown, false) {
		t.Fatalf("spec reversal did not rejoin retained member: %#v %#v", stored.Status, inst.Spec)
	}
	if len(agent.calls) != 3 || agent.calls[2] != "memberJoin" {
		t.Fatalf("unexpected member calls %v", agent.calls)
	}
}

func its2TaskInput(inst *workloads.Instance) (string, string) {
	guard, task := "", ""
	for _, c := range inst.Spec.Template.Spec.InitContainers {
		for _, e := range c.Env {
			if e.Name == "KB_AGENT_TASK" {
				task = e.Value
			}
			if e.Name == "KB_AGENT_DATA_LOAD_RESULT" {
				guard = e.Value
			}
		}
	}
	return guard, task
}
func TestITS2LifecycleDataCleanupAndFreshPVC(t *testing.T) {
	checkITS2LifecycleDataCleanupAndFreshPVC(t, nil)
}
func TestITS2LifecycleOnDeleteDataCleanupAndFreshPVC(t *testing.T) {
	checkITS2LifecycleDataRecovery(t, nil, true)
}
func checkITS2LifecycleDataCleanupAndFreshPVC(t *testing.T, storage client.Client) {
	checkITS2LifecycleDataRecovery(t, storage, false)
}
func checkITS2LifecycleDataRecovery(t *testing.T, storage client.Client, onDelete bool) {
	its, cli, r := its2LifecycleFixture(t, 1, true, storage)
	its2LifecycleRound(t, r, its)
	its2LifecycleRound(t, r, its)
	initial := its2LifecycleInstance(t, cli, "lifecycle-0")
	source, err := instance.BuildPod(initial)
	if err != nil {
		t.Fatal(err)
	}
	source.Status = corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{Name: "database", Image: "database:v1"}, {Name: kbagent.ContainerName, Image: "agent:v1"}}, Phase: corev1.PodRunning, PodIP: "10.0.0.1", Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}
	savedStatus := source.Status
	if err := cli.Create(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	source.Status = savedStatus
	if err := cli.Status().Update(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	initial.Status.CurrentState = workloads.InstanceCurrentStatePresent
	initial.Status.Ready = true
	initial.Status.Available = true
	if err := cli.Status().Update(context.Background(), initial); err != nil {
		t.Fatal(err)
	}
	stored := its2LifecycleRead(t, cli, its)
	stored.Spec.Replicas = ptr.To[int32](2)
	stored.Generation++
	if err := cli.Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	its2LifecycleRound(t, r, its)
	its2LifecycleRound(t, r, its)
	target := its2LifecycleInstance(t, cli, "lifecycle-1")
	if onDelete {
		target.Spec.InstanceUpdateStrategyType = ptr.To(kbappsv1.OnDeleteStrategyType)
		if err := cli.Update(context.Background(), target); err != nil {
			t.Fatal(err)
		}
	}
	guard, task := its2TaskInput(target)
	if guard == "" || task == "" {
		t.Fatalf("expansion did not receive worker guard and task: %#v", target.Spec.Template.Spec.InitContainers)
	}
	var targetResult proto.DataLoadResult
	if err := json.Unmarshal([]byte(guard), &targetResult); err != nil {
		t.Fatal(err)
	}
	snapshotTasks := []proto.Task{{UID: "own-result", NewReplica: &proto.NewReplicaTask{DataLoadResult: &targetResult}}, {UID: "other-result", NewReplica: &proto.NewReplicaTask{DataLoadResult: &proto.DataLoadResult{Namespace: targetResult.Namespace, PVCName: "other-pvc"}}}, {UID: "other-namespace", NewReplica: &proto.NewReplicaTask{DataLoadResult: &proto.DataLoadResult{Namespace: "other", PVCName: targetResult.PVCName}}}}
	snapshotEnv, err := kbagent.BuildEnv4Worker(snapshotTasks)
	if err != nil {
		t.Fatal(err)
	}
	target.Spec.InstanceAssistantObjects = []workloads.InstanceAssistantObject{{ConfigMap: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "target-task", Namespace: target.Namespace}, Data: map[string]string{snapshotEnv.Name: snapshotEnv.Value}}}}
	if err := cli.Update(context.Background(), target); err != nil {
		t.Fatal(err)
	}
	beforeRevision, err := instance.BuildPodRevision(target)
	if err != nil {
		t.Fatal(err)
	}
	ir := &InstanceReconciler{Client: cli, Scheme: model.GetScheme(), Recorder: record.NewFakeRecorder(100)}
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(target)}
	for i := 0; i < 2; i++ {
		if _, err := ir.Reconcile(context.Background(), req); err != nil {
			t.Fatal(err)
		}
	}
	pvcName := intctrlutil.ComposePVCName(its.Spec.VolumeClaimTemplates[0], its.Name, target.Name)
	pvc := &corev1.PersistentVolumeClaim{}
	pvcKey := client.ObjectKey{Namespace: its.Namespace, Name: pvcName}
	if err := cli.Get(context.Background(), pvcKey, pvc); err != nil {
		t.Fatal(err)
	}
	pvc.Annotations = map[string]string{proto.DataLoadedAnnotationKey: "true", "unrelated": "preserved"}
	if err := cli.Update(context.Background(), pvc); err != nil {
		t.Fatal(err)
	}
	stored = its2LifecycleRead(t, cli, its)
	stored.Spec.LifecycleActions.Worker.Image = "agent:v2"
	stored.Spec.LifecycleActions.DataLoad.Exec.Command = []string{"new-load"}
	stored.Generation++
	if err := cli.Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	cli.failStatus = true
	its2LifecycleRound(t, r, its)
	target = its2LifecycleInstance(t, cli, target.Name)
	_, task = its2TaskInput(target)
	if task == "" {
		t.Fatal("status conflict cleaned task before data result committed")
	}
	its2LifecycleRound(t, r, its)
	stored = its2LifecycleRead(t, cli, its)
	if !ptr.Deref(stored.Status.InstanceStatus[1].DataLoaded, false) {
		t.Fatal("actual PVC result not persisted")
	}
	its2LifecycleRound(t, r, its)
	target = its2LifecycleInstance(t, cli, target.Name)
	guard, task = its2TaskInput(target)
	if guard == "" || task != "" {
		t.Fatalf("cleanup lost guard or kept completed task: %q %q", guard, task)
	}
	if target.Spec.LifecycleActions.Worker.Image != "agent:v1" || target.Spec.LifecycleActions.DataLoad.Exec.Command[0] != "true" {
		t.Fatal("cleanup bypassed normal rollout admission for worker/actions")
	}
	for _, worker := range target.Spec.Template.Spec.InitContainers {
		if worker.Name == kbagent.ContainerName && worker.Image != "agent:v1" {
			t.Fatal("cleanup bypassed normal worker rollout")
		}
	}
	var remainingTasks []proto.Task
	if err := json.Unmarshal([]byte(target.Spec.InstanceAssistantObjects[0].ConfigMap.Data[snapshotEnv.Name]), &remainingTasks); err != nil {
		t.Fatal(err)
	}
	if len(remainingTasks) != 2 || remainingTasks[0].UID != "other-result" || remainingTasks[1].UID != "other-namespace" {
		t.Fatalf("parent snapshot cleanup removed other target tasks: %#v", remainingTasks)
	}
	afterRevision, err := instance.BuildPodRevision(target)
	if err != nil {
		t.Fatal(err)
	}
	if beforeRevision != afterRevision {
		t.Fatalf("cleanup changed persistent Pod revision: %s %s", beforeRevision, afterRevision)
	}
	pod := &corev1.Pod{}
	podKey := client.ObjectKeyFromObject(target)
	if err := cli.Get(context.Background(), podKey, pod); err != nil {
		t.Fatal(err)
	}
	pod.Finalizers = nil
	if err := cli.Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	if err := cli.Delete(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	if _, err := ir.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if err := cli.Get(context.Background(), podKey, pod); err != nil {
		t.Fatal(err)
	}
	rebuilt := &workloads.Instance{Spec: workloads.InstanceSpec{Template: corev1.PodTemplateSpec{Spec: pod.Spec}}}
	guard, task = its2TaskInput(rebuilt)
	if guard == "" || task != "" {
		t.Fatal("Instance Pod rebuild restored stale task or lost success guard")
	}
	// Same-name replacement has no result. Its guard-only Pending Pod must receive a current task.
	pvc.Finalizers = nil
	if err := cli.Update(context.Background(), pvc); err != nil {
		t.Fatal(err)
	}
	if err := cli.Delete(context.Background(), pvc); err != nil {
		t.Fatal(err)
	}
	pvc.ResourceVersion = ""
	pvc.UID = ""
	pvc.Annotations = nil
	if err := cli.Create(context.Background(), pvc); err != nil {
		t.Fatal(err)
	}
	pod.Status.Phase = corev1.PodPending
	if err := cli.Status().Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	its2LifecycleRound(t, r, its)
	its2LifecycleRound(t, r, its)
	target = its2LifecycleInstance(t, cli, target.Name)
	_, task = its2TaskInput(target)
	if task == "" {
		t.Fatal("fresh PVC did not regenerate current data task")
	}
	pod.Finalizers = nil
	if err := cli.Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	if _, err := ir.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if err := cli.Get(context.Background(), podKey, &corev1.Pod{}); !apierrors.IsNotFound(err) {
		t.Fatalf("guard-only Pending Pod was not replaced: %v", err)
	}
	if _, err := ir.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if err := cli.Get(context.Background(), podKey, pod); err != nil {
		t.Fatal(err)
	}
	rebuilt.Spec.Template.Spec = pod.Spec
	_, task = its2TaskInput(rebuilt)
	if task == "" {
		t.Fatal("regenerated Pod has no current data task")
	}
}

func TestITS2LifecycleAPIServer(t *testing.T) {
	scenarios := []struct {
		name  string
		check func(*testing.T, client.Client)
	}{
		{"registration", checkITS2LifecycleRegistrationCommit},
		{"leave-reversal", checkITS2LifecycleLeaveCommitAndSpecReversal},
		{"data-recovery", checkITS2LifecycleDataCleanupAndFreshPVC},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			env := &envtest.Environment{CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")}, ErrorIfCRDPathMissing: true}
			cfg, err := env.Start()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := env.Stop(); err != nil {
					t.Error(err)
				}
			}()
			storage, err := client.New(cfg, client.Options{Scheme: model.GetScheme()})
			if err != nil {
				t.Fatal(err)
			}
			scenario.check(t, storage)
		})
	}
}

func TestITS2LifecycleOfflineDoesNotLeaveMember(t *testing.T) {
	its, cli, r := its2LifecycleFixture(t, 1, false)
	its2LifecycleRound(t, r, its)
	its2LifecycleRound(t, r, its)
	stored := its2LifecycleRead(t, cli, its)
	stored.Status.InstanceStatus[0].MemberJoined = ptr.To(true)
	stored.Status.InstanceStatus[0].Provisioned = true
	if err := cli.Status().Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	stored.Spec.OfflineInstances = []string{"lifecycle-0"}
	stored.Generation++
	if err := cli.Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	agent := &its2LifecycleAgent{}
	kbacli.SetMockClient(agent, nil)
	defer kbacli.UnsetMockClient()
	its2LifecycleRound(t, r, its)
	its2LifecycleRound(t, r, its)
	stored = its2LifecycleRead(t, cli, its)
	if len(agent.calls) != 0 {
		t.Fatalf("offline transition called member action: %v", agent.calls)
	}
	var offline *workloads.InstanceStatus
	for i := range stored.Status.InstanceStatus {
		if stored.Status.InstanceStatus[i].PodName == "lifecycle-0" {
			offline = &stored.Status.InstanceStatus[i]
		}
	}
	if offline == nil || offline.EffectiveDesiredState() != workloads.InstanceDesiredStateOffline || !ptr.Deref(offline.MemberJoined, false) {
		t.Fatalf("offline transition lost known membership: %#v", stored.Status.InstanceStatus)
	}
	if cond := meta.FindStatusCondition(stored.Status.Conditions, "InstanceLifecycle"); cond != nil {
		t.Fatalf("offline member was blocked on leave: %#v", cond)
	}
}
func TestITS2LifecycleAbsentBootstrapMemberStillNeedsLeave(t *testing.T) {
	its, cli, r := its2LifecycleFixture(t, 1, false)
	its2LifecycleRound(t, r, its)
	its2LifecycleRound(t, r, its)
	stored := its2LifecycleRead(t, cli, its)
	stored.Status.InstanceStatus[0].Provisioned = true
	if err := cli.Status().Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	stored.Spec.Replicas = ptr.To[int32](0)
	stored.Generation++
	if err := cli.Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	its2LifecycleRound(t, r, its)
	its2LifecycleRound(t, r, its)
	stored = its2LifecycleRead(t, cli, its)
	if len(stored.Status.InstanceStatus) != 1 || !stored.Status.InstanceStatus[0].Provisioned || stored.Status.InstanceStatus[0].MemberJoined != nil {
		t.Fatalf("absent unknown bootstrap member was discarded: %#v", stored.Status.InstanceStatus)
	}
	inst := its2LifecycleInstance(t, cli, "lifecycle-0")
	if ptr.Deref(inst.Spec.ScaledDown, false) || !inst.DeletionTimestamp.IsZero() {
		t.Fatal("absent member was scaled down without Leave")
	}
}

func TestITS2LifecycleStopDoesNotWaitForDataSource(t *testing.T) {
	its, cli, r := its2LifecycleFixture(t, 1, true)
	its2LifecycleRound(t, r, its)
	its2LifecycleRound(t, r, its)
	stored := its2LifecycleRead(t, cli, its)
	stored.Status.InstanceStatus[0].DataLoaded = ptr.To(false)
	if err := cli.Status().Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	stored.Spec.Stop = ptr.To(true)
	stored.Generation++
	if err := cli.Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	its2LifecycleRound(t, r, its)
	stored = its2LifecycleRead(t, cli, its)
	if stored.Status.ObservedGeneration != stored.Generation {
		t.Fatalf("stop did not advance: %#v", stored.Status)
	}
	if cond := meta.FindStatusCondition(stored.Status.Conditions, "InstanceLifecycle"); cond != nil {
		t.Fatalf("stop blocked on data source: %#v", cond)
	}
}
func TestITS2LifecycleBootstrapPodWithoutLeafStatusNeedsLeave(t *testing.T) {
	its, cli, r := its2LifecycleFixture(t, 1, false)
	its2LifecycleRound(t, r, its)
	its2LifecycleRound(t, r, its)
	inst := its2LifecycleInstance(t, cli, "lifecycle-0")
	pod, err := instance.BuildPod(inst)
	if err != nil {
		t.Fatal(err)
	}
	if err := cli.Create(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	pod.Status.PodIP = "10.0.0.1"
	if err := cli.Status().Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	stored := its2LifecycleRead(t, cli, its)
	stored.Spec.Replicas = ptr.To[int32](0)
	stored.Generation++
	if err := cli.Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	agent := &its2LifecycleAgent{}
	kbacli.SetMockClient(agent, nil)
	defer kbacli.UnsetMockClient()
	its2LifecycleRound(t, r, its)
	stored = its2LifecycleRead(t, cli, its)
	if len(agent.calls) != 1 || agent.calls[0] != "memberLeave" {
		t.Fatalf("observed bootstrap Pod skipped Leave: %v %#v", agent.calls, stored.Status.InstanceStatus)
	}
	if len(stored.Status.InstanceStatus) != 1 || !stored.Status.InstanceStatus[0].Provisioned {
		t.Fatalf("observed bootstrap identity not retained: %#v", stored.Status.InstanceStatus)
	}
	inst = its2LifecycleInstance(t, cli, inst.Name)
	if ptr.Deref(inst.Spec.ScaledDown, false) {
		t.Fatal("ScaledDown before Leave status commit")
	}
}

func TestITS2LifecycleReconcileSeparatesSameNamePVCResultsByPlacement(t *testing.T) {
	makeClient := func() client.Client {
		return fake.NewClientBuilder().WithScheme(model.GetScheme()).WithStatusSubresource(&workloads.InstanceSet{}, &workloads.Instance{}, &corev1.Pod{}).Build()
	}
	control, workerA, workerB := makeClient(), makeClient(), makeClient()
	multi := multicluster.NewClient(control, map[string]client.Client{"worker-a": workerA, "worker-b": workerB})
	its, cli, r := its2LifecycleFixture(t, 2, true, multi)
	stored := its2LifecycleRead(t, cli, its)
	stored.Annotations = map[string]string{constant.KBAppMultiClusterPlacementKey: "worker-a,worker-b"}
	stored.Spec.VolumeClaimTemplates = nil
	stored.Spec.Template.Spec.Volumes = []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "shared-name"}}}}
	if err := cli.Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	its2LifecycleRound(t, r, its)
	its2LifecycleRound(t, r, its)
	// Each real Instance is assigned its own cluster by the normal controller.
	for i, worker := range []client.Client{workerA, workerB} {
		inst := its2LifecycleInstance(t, worker, fmt.Sprintf("lifecycle-%d", i))
		if inst.Annotations[constant.KBAppMultiClusterPlacementKey] != []string{"worker-a", "worker-b"}[i] {
			t.Fatal("fixture did not create placed Instances")
		}
	}
	stored = its2LifecycleRead(t, cli, its)
	for i := range stored.Status.InstanceStatus {
		stored.Status.InstanceStatus[i].DataLoaded = ptr.To(false)
	}
	if err := cli.Status().Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	unmarked := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "shared-name", Namespace: its.Namespace, Annotations: map[string]string{"untouched": "b"}}}
	marked := unmarked.DeepCopy()
	marked.Annotations = map[string]string{proto.DataLoadedAnnotationKey: "true", "untouched": "a"}
	if err := workerA.Create(context.Background(), marked); err != nil {
		t.Fatal(err)
	}
	if err := workerB.Create(context.Background(), unmarked); err != nil {
		t.Fatal(err)
	}
	its2LifecycleRound(t, r, its)
	stored = its2LifecycleRead(t, cli, its)
	results := map[string]bool{}
	for _, status := range stored.Status.InstanceStatus {
		results[status.PodName] = ptr.Deref(status.DataLoaded, false)
	}
	if !results["lifecycle-0"] || results["lifecycle-1"] {
		t.Fatalf("placement results were mixed: %#v", results)
	}
	for i, worker := range []client.Client{workerA, workerB} {
		pvc := &corev1.PersistentVolumeClaim{}
		if err := worker.Get(context.Background(), client.ObjectKeyFromObject(unmarked), pvc); err != nil {
			t.Fatal(err)
		}
		expected := unmarked
		if i == 0 {
			expected = marked
		}
		if !reflect.DeepEqual(pvc.Annotations, expected.Annotations) || pvc.ResourceVersion != expected.ResourceVersion {
			t.Fatalf("parent changed runtime PVC in cluster %d: %#v", i, pvc)
		}
	}
}

func TestInstanceOnDeleteRecoversTaskOnExternalPVC(t *testing.T) {
	its, cli, r := its2LifecycleFixture(t, 1, true)
	its2LifecycleRound(t, r, its)
	its2LifecycleRound(t, r, its)
	inst := its2LifecycleInstance(t, cli, "lifecycle-0")
	result := &proto.DataLoadResult{Namespace: its.Namespace, PVCName: "external-data"}
	guard, err := kbagent.BuildEnv4DataLoadResult(result)
	if err != nil {
		t.Fatal(err)
	}
	task, err := kbagent.BuildEnv4Worker([]proto.Task{{UID: "current-data-task", NewReplica: &proto.NewReplicaTask{Remote: "10.0.0.1", DataLoadResult: result}}})
	if err != nil {
		t.Fatal(err)
	}
	inst.Spec.VolumeClaimTemplates = nil
	inst.Spec.InstanceUpdateStrategyType = ptr.To(kbappsv1.OnDeleteStrategyType)
	inst.Spec.Template.Spec.Volumes = []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: result.PVCName}}}}
	inst.Spec.Template.Spec.InitContainers = []corev1.Container{{Name: kbagent.ContainerName4Worker, Image: "agent:v1", Env: []corev1.EnvVar{{Name: "KB_AGENT_DATA_VOLUME", Value: "data"}, *guard, *task}}}
	if err := cli.Update(context.Background(), inst); err != nil {
		t.Fatal(err)
	}
	pod, err := instance.BuildPod(inst)
	if err != nil {
		t.Fatal(err)
	}
	pod.Finalizers = nil
	pod.Spec.InitContainers[0].Env = pod.Spec.InitContainers[0].Env[:2]
	if err := cli.Create(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	pod.Status.Phase = corev1.PodPending
	if err := cli.Status().Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: result.PVCName, Namespace: result.Namespace, Annotations: map[string]string{"external": "preserved"}}}
	if err := cli.Create(context.Background(), pvc); err != nil {
		t.Fatal(err)
	}
	ir := &InstanceReconciler{Client: cli, Scheme: model.GetScheme(), Recorder: record.NewFakeRecorder(100)}
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(inst)}
	for i := 0; i < 2; i++ {
		if _, err := ir.Reconcile(context.Background(), req); err != nil {
			t.Fatal(err)
		}
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(pod), &corev1.Pod{}); !apierrors.IsNotFound(err) {
		t.Fatalf("external PVC recovery did not replace guard-only Pending Pod: %v", err)
	}
	if _, err := ir.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	rebuilt := &corev1.Pod{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(pod), rebuilt); err != nil {
		t.Fatal(err)
	}
	generated, err := workloadlifecycle.GeneratedTask(&rebuilt.Spec)
	if err != nil || generated == "" {
		t.Fatalf("rebuilt external-PVC Pod has no regenerated task: %q %v", generated, err)
	}
	observed := &corev1.PersistentVolumeClaim{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(pvc), observed); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pvc.Annotations, observed.Annotations) || len(observed.OwnerReferences) != 0 || pvc.ResourceVersion != observed.ResourceVersion {
		t.Fatal("Instance took ownership of external result PVC")
	}
}
