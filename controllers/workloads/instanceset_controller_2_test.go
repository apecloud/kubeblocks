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
	"errors"
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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
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

type setStopClient struct {
	client.Client
	failChildWriteAt    int
	childWrites         int
	failResumeWriteAt   int
	resumeWrites        int
	delayPodObservation bool
	beforePodDelete     func() error
}

var (
	errSetStopChildWrite  = errors.New("injected child write failure")
	errSetStopResumeWrite = errors.New("injected resume write failure")
	errSetStopStatusWrite = errors.New("injected parent status failure")
)

type failSetStopStatusClient struct {
	client.Client
	fail bool
}

func (c *failSetStopStatusClient) Status() client.SubResourceWriter {
	return &failSetStopStatusWriter{SubResourceWriter: c.Client.Status(), owner: c}
}

type failSetStopStatusWriter struct {
	client.SubResourceWriter
	owner *failSetStopStatusClient
}

func (w *failSetStopStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	if _, ok := obj.(*workloads.InstanceSet); ok && w.owner.fail {
		w.owner.fail = false
		return errSetStopStatusWrite
	}
	return w.SubResourceWriter.Update(ctx, obj, opts...)
}

func TestInstanceSetStopRecoversAfterParentStatusFailure(t *testing.T) {
	for _, stopping := range []bool{true, false} {
		t.Run(fmt.Sprintf("stop=%t", stopping), func(t *testing.T) {
			its := setStopFixture()
			cli := newSetStopClient(t, its)
			reconcileSetStop(t, cli, 5)
			claims := bindSetStopClaims(t, cli)
			markSetStopPodsReady(t, cli)
			reconcileSetStop(t, cli, 3)
			if !stopping {
				mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
				reconcileSetStop(t, cli, 8)
				if len(listSetStopPods(t, cli)) != 0 || readSetStop(t, cli).Status.Replicas != 0 {
					t.Fatal("resume fixture did not finish Stop")
				}
			}
			if !stopping {
				mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(false) })
			}
			mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
				its.Spec.MinReadySeconds = 3600
			})
			if stopping {
				mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
			} else {
				mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Template.Spec.Containers[0].Image = "db:latest" })
			}
			beforeStatus := readSetStop(t, cli).Status
			faulty := &failSetStopStatusClient{Client: cli, fail: true}
			if _, err := reconcileSetStopParent(faulty); !errors.Is(err, errSetStopStatusWrite) || faulty.fail {
				t.Fatalf("expected parent status fault after child handoff, got %v", err)
			}
			if !reflect.DeepEqual(readSetStop(t, cli).Status, beforeStatus) {
				t.Fatal("failed parent status write changed the persisted status")
			}
			assertOneHandoff := func() {
				t.Helper()
				handoffs := 0
				for _, child := range listSetStopChildren(t, cli) {
					if ptr.Deref(child.Spec.Stop, false) == stopping {
						handoffs++
						if child.Generation == child.Status.ObservedGeneration {
							t.Fatal("persisted child handoff did not invalidate the prior observation")
						}
						if !stopping && child.Spec.Template.Spec.Containers[0].Image != "db:latest" {
							t.Fatal("persisted resume did not contain the latest payload")
						}
					}
				}
				if handoffs != 1 {
					t.Fatalf("parent fault/retry admitted %d children into a one-slot budget", handoffs)
				}
			}
			assertOneHandoff()
			if _, err := reconcileSetStopParent(cli); err != nil {
				t.Fatal(err)
			}
			assertOneHandoff()
			if stopping {
				reconcileSetStop(t, cli, 10)
				if len(listSetStopPods(t, cli)) != 0 || readSetStop(t, cli).Status.Replicas != 0 {
					t.Fatal("parent status failure did not converge to stopped runtime")
				}
				for _, child := range listSetStopChildren(t, cli) {
					if !ptr.Deref(child.Spec.Stop, false) || child.Generation != child.Status.ObservedGeneration || child.Status.CurrentState != workloads.InstanceCurrentStateAbsent {
						t.Fatal("Stop left an unacknowledged child")
					}
				}
			} else {
				for i := 0; i < 8; i++ {
					reconcileSetStop(t, cli, 1)
					markSetStopPodsReady(t, cli)
				}
				reconcileSetStop(t, cli, 2)
				pods := listSetStopPods(t, cli)
				status := readSetStop(t, cli).Status
				if len(pods) != 2 || status.Replicas != 2 || status.ReadyReplicas != 2 || status.AvailableReplicas != 2 {
					t.Fatalf("parent status failure did not converge to available runtime: %#v", status)
				}
				for _, child := range listSetStopChildren(t, cli) {
					if ptr.Deref(child.Spec.Stop, false) || child.Spec.Template.Spec.Containers[0].Image != "db:latest" {
						t.Fatal("resume left a stopped or obsolete child")
					}
				}
				for _, pod := range pods {
					if pod.Spec.Containers[0].Image != "db:latest" {
						t.Fatal("resume used an obsolete Pod payload")
					}
				}
			}
			currentClaims := &corev1.PersistentVolumeClaimList{}
			if err := cli.List(context.Background(), currentClaims); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(currentClaims.Items, claims) {
				t.Fatal("parent status failure/recovery changed storage")
			}
		})
	}
}

func (c *setStopClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if inst, ok := obj.(*workloads.Instance); ok {
		old := &workloads.Instance{}
		if err := c.Get(ctx, client.ObjectKeyFromObject(inst), old); err != nil {
			return err
		}
		if !reflect.DeepEqual(old.Spec, inst.Spec) {
			c.childWrites++
			if ptr.Deref(old.Spec.Stop, false) && !ptr.Deref(inst.Spec.Stop, false) {
				c.resumeWrites++
				if c.failResumeWriteAt > 0 && c.resumeWrites == c.failResumeWriteAt {
					return errSetStopResumeWrite
				}
			}
			if c.failChildWriteAt > 0 && c.childWrites == c.failChildWriteAt {
				return errSetStopChildWrite
			}
			inst.Generation = old.Generation + 1
		}
	}
	return c.Client.Update(ctx, obj, opts...)
}

func (c *setStopClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if inst, ok := obj.(*workloads.Instance); ok && inst.Generation == 0 {
		inst.Generation = 1
	}
	return c.Client.Create(ctx, obj, opts...)
}

func (c *setStopClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if err := c.Client.List(ctx, list, opts...); err != nil {
		return err
	}
	if pods, ok := list.(*corev1.PodList); ok && c.delayPodObservation {
		pods.Items = nil
	}
	return nil
}

func (c *setStopClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if _, ok := obj.(*corev1.Pod); ok && c.beforePodDelete != nil {
		hook := c.beforePodDelete
		c.beforePodDelete = nil
		if err := hook(); err != nil {
			return err
		}
	}
	return c.Client.Delete(ctx, obj, opts...)
}

func TestInstanceSetStopResumeDuringPodObservationDelay(t *testing.T) {
	its := setStopFixture()
	its.Spec.Replicas = ptr.To(int32(1))
	cli := newSetStopClient(t, its)
	for i := 0; i < 3; i++ {
		if _, err := reconcileSetStopParent(cli); err != nil {
			t.Fatal(err)
		}
	}
	wrapped := cli.(*setStopClient)
	wrapped.delayPodObservation = true
	reconcileStopEntry(t, cli, 2)
	pvc := bindStopPVC(t, cli)
	pod := readStopPod(t, cli)
	pod.Finalizers = []string{"test/hold"}
	if err := cli.Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
	if _, err := reconcileSetStopParent(cli); err != nil {
		t.Fatal(err)
	}
	reconcileStopEntry(t, cli, 1)
	stopped := readStopInstance(t, cli)
	if !ptr.Deref(stopped.Spec.Stop, false) || stopped.Generation != stopped.Status.ObservedGeneration || stopped.Status.CurrentState != workloads.InstanceCurrentStateAbsent {
		t.Fatalf("delayed Pod observation did not produce stopped Absent status: %#v", stopped)
	}
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Stop = nil
	})
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Template.Spec.Containers[0].Image = "db:v2"
		its.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
	})
	wrapped.delayPodObservation = false
	wrapped.beforePodDelete = func() error { _, err := reconcileSetStopParent(cli); return err }
	controller := &InstanceReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(1000)}
	conflictResult, err := controller.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(stopped)})
	if err != nil || !conflictResult.Requeue || conflictResult.RequeueAfter != 0 || wrapped.beforePodDelete != nil {
		t.Fatalf("parent resume did not requeue conflict after Stop deletion: %#v %v", conflictResult, err)
	}
	if ptr.Deref(readStopInstance(t, cli).Spec.Stop, false) || readStopPod(t, cli).DeletionTimestamp.IsZero() {
		t.Fatal("actual parent/child commits did not produce resumed Instance with terminating Pod")
	}
	reconcileStopEntry(t, cli, 1)
	if !reflect.DeepEqual(readStopPVC(t, cli), pvc) {
		t.Fatal("resume changed PVCs before Stop deletion completed")
	}
	held := readStopPod(t, cli)
	held.Finalizers = nil
	if err := cli.Update(context.Background(), held); err != nil {
		t.Fatal(err)
	}
	reconcileStopEntry(t, cli, 3)
	if readStopPod(t, cli).Spec.Containers[0].Image != "db:v2" || readStopPVC(t, cli).Spec.Resources.Requests.Storage().Cmp(resource.MustParse("2Gi")) != 0 {
		t.Fatal("resume did not converge after deletion")
	}
}

func TestInstanceSetStopSynchronizesClonedAssistantsBeforeNameAlignment(t *testing.T) {
	its := setStopFixture()
	its.Annotations = map[string]string{constant.KBAppMultiClusterPlacementKey: "local"}
	its.Spec.InstanceAssistantObjects = []corev1.ObjectReference{{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "demo-config"}}
	cli := newSetStopClient(t, its)
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "demo-config", Namespace: "default"}, Data: map[string]string{"version": "v1"}}
	if err := cli.Create(context.Background(), cm); err != nil {
		t.Fatal(err)
	}
	reconcileSetStop(t, cli, 5)
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(3)) })
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
	reconcileSetStop(t, cli, 8)
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(cm), cm); err != nil {
		t.Fatal(err)
	}
	cm.Data["version"] = "v2"
	if err := cli.Update(context.Background(), cm); err != nil {
		t.Fatal(err)
	}
	reconcileSetStop(t, cli, 4)
	children := listSetStopChildren(t, cli)
	if len(children) != 2 || len(listSetStopPods(t, cli)) != 0 {
		t.Fatal("Stop did not defer topology and runtime creation")
	}
	for _, child := range children {
		if !ptr.Deref(child.Spec.Stop, false) || len(child.Spec.InstanceAssistantObjects) != 1 || child.Spec.InstanceAssistantObjects[0].ConfigMap.Data["version"] != "v2" {
			t.Fatal("stopped clone did not receive desired assistant before name alignment")
		}
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(cm), cm); err != nil {
		t.Fatal(err)
	}
	if cm.Data["version"] != "v2" {
		t.Fatal("stopped child assistant reconciliation overwrote current desired content")
	}
}

func TestInstanceSetStopClearsAppliedStatus(t *testing.T) {
	its := setStopFixture()
	its.Spec.MinReadySeconds = 3600
	cli := newSetStopClient(t, its)
	reconcileSetStop(t, cli, 5)
	bindSetStopClaims(t, cli)
	for _, pod := range listSetStopPods(t, cli) {
		pod.Status.Phase = corev1.PodRunning
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "db", Image: pod.Spec.Containers[0].Image, Ready: true}}
		if err := cli.Status().Update(context.Background(), &pod); err != nil {
			t.Fatal(err)
		}
	}
	reconcileSetStop(t, cli, 5)
	before := readSetStop(t, cli).Status
	if before.ReadyReplicas != 2 || before.AvailableReplicas != 0 || before.UpdatedReplicas != 2 {
		t.Fatal("fixture did not publish applied runtime before Stop")
	}
	for _, status := range readSetStop(t, cli).Status.InstanceStatus {
		if !status.UpToDate {
			t.Fatal("fixture did not publish detailed applied status")
		}
	}
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Stop = ptr.To(true)
		its.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
	})
	for i := 0; i < 2; i++ {
		if _, err := reconcileSetStopParent(cli); err != nil {
			t.Fatal(err)
		}
		stopped := readSetStop(t, cli)
		if stopped.Status.UpdatedReplicas != 0 {
			t.Errorf("Stop pass %d retained UpdatedReplicas=%d", i, stopped.Status.UpdatedReplicas)
		}
		for _, status := range stopped.Status.InstanceStatus {
			if status.UpToDate {
				t.Errorf("Stop pass %d retained UpToDate for %s", i, status.PodName)
			}
		}
	}
	reconcileSetStop(t, cli, 10)
	if len(listSetStopPods(t, cli)) != 0 || readSetStop(t, cli).Status.Replicas != 0 {
		t.Fatal("MinReadySeconds prevented Stop from completing")
	}
}

func reconcileSetStopParent(cli client.Client) (ctrl.Result, error) {
	ctx := context.Background()
	req := ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "demo"}}
	recorder := record.NewFakeRecorder(1000)
	return (&InstanceSetReconciler2{Client: cli, Scheme: cli.Scheme(), Recorder: recorder}).Reconcile(ctx, req)
}

func readSetStop(t *testing.T, cli client.Client) *workloads.InstanceSet {
	t.Helper()
	its := &workloads.InstanceSet{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "demo"}, its); err != nil {
		t.Fatal(err)
	}
	return its
}

func mutateSetStop(t *testing.T, cli client.Client, change func(*workloads.InstanceSet)) {
	t.Helper()
	its := readSetStop(t, cli)
	change(its)
	its.Generation++
	if err := cli.Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
}

func listSetStopPods(t *testing.T, cli client.Client) []corev1.Pod {
	t.Helper()
	list := &corev1.PodList{}
	if err := cli.List(context.Background(), list); err != nil {
		t.Fatal(err)
	}
	return list.Items
}

func bindSetStopClaims(t *testing.T, cli client.Client) []corev1.PersistentVolumeClaim {
	t.Helper()
	claims := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(context.Background(), claims); err != nil {
		t.Fatal(err)
	}
	for _, claim := range claims.Items {
		claim.UID = types.UID(claim.Name + "-uid")
		if err := cli.Update(context.Background(), &claim); err != nil {
			t.Fatal(err)
		}
		claim.Status.Phase = corev1.ClaimBound
		claim.Status.Capacity = claim.Spec.Resources.Requests.DeepCopy()
		if err := cli.Status().Update(context.Background(), &claim); err != nil {
			t.Fatal(err)
		}
	}
	if err := cli.List(context.Background(), claims); err != nil {
		t.Fatal(err)
	}
	return claims.Items
}

func markSetStopPodsReady(t *testing.T, cli client.Client) {
	t.Helper()
	for _, pod := range listSetStopPods(t, cli) {
		pod.Status.Phase = corev1.PodRunning
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(time.Now().Add(-2 * time.Hour))}}
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "db", Image: pod.Spec.Containers[0].Image, Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}
		if err := cli.Status().Update(context.Background(), &pod); err != nil {
			t.Fatal(err)
		}
	}
}

func listSetStopChildren(t *testing.T, cli client.Client) []workloads.Instance {
	t.Helper()
	list := &workloads.InstanceList{}
	if err := cli.List(context.Background(), list); err != nil {
		t.Fatal(err)
	}
	return list.Items
}

func TestInstanceSetStopResumesLatestPayload(t *testing.T) {
	cli := newSetStopClient(t, setStopFixture())
	reconcileSetStop(t, cli, 5)
	bindSetStopClaims(t, cli)
	claims := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(context.Background(), claims); err != nil {
		t.Fatal(err)
	}
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Template.Spec.Containers[0].Image = "db:v2"
		its.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
	})
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
	reconcileSetStop(t, cli, 8)
	for _, child := range listSetStopChildren(t, cli) {
		if !ptr.Deref(child.Spec.Stop, false) || child.Spec.Template.Spec.Containers[0].Image != "db:v2" || len(child.Spec.VolumeClaimTemplates) != 1 || child.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().Cmp(resource.MustParse("2Gi")) != 0 {
			t.Fatal("stopped Instance did not receive latest payload")
		}
	}
	frozen := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(context.Background(), frozen); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(claims.Items, frozen.Items) || len(listSetStopPods(t, cli)) != 0 {
		t.Fatal("stopped runtime or PVCs changed")
	}
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = nil })
	reconcileSetStop(t, cli, 6)
	pods := listSetStopPods(t, cli)
	if len(pods) != 2 {
		t.Fatalf("resume created %d Pods", len(pods))
	}
	for _, pod := range pods {
		if pod.Spec.Containers[0].Image != "db:v2" {
			t.Fatal("resume used obsolete image")
		}
	}
	if err := cli.List(context.Background(), claims); err != nil {
		t.Fatal(err)
	}
	if len(claims.Items) != 2 {
		t.Fatal("resume replaced existing claims")
	}
	for _, claim := range claims.Items {
		if claim.Spec.Resources.Requests.Storage().Cmp(resource.MustParse("2Gi")) != 0 {
			t.Fatal("resume ignored current storage target")
		}
	}
}

func TestInstanceSetStopFlatTemplateChange(t *testing.T) {
	for _, retargetFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("retargetFirst=%t", retargetFirst), func(t *testing.T) {
			its := setStopFixture()
			its.Spec.FlatInstanceOrdinal = true
			cli := newSetStopClient(t, its)
			reconcileSetStop(t, cli, 5)
			changeTemplate := func(its *workloads.InstanceSet) {
				its.Spec.Instances = []workloads.InstanceTemplate{{Name: "named", Replicas: ptr.To(int32(1)), Ordinals: workloads.Ordinals{Discrete: []int32{0}}, Labels: map[string]string{"selected-template": "named"}}}
			}
			mutateSetStop(t, cli, changeTemplate)
			if retargetFirst {
				mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
					its.Spec.InstanceUpdateStrategy = &workloads.InstanceUpdateStrategy{Type: kbappsv1.OnDeleteStrategyType}
				})
				reconcileSetStop(t, cli, 3)
			}
			claims := &corev1.PersistentVolumeClaimList{}
			if err := cli.List(context.Background(), claims); err != nil {
				t.Fatal(err)
			}
			mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.Stop = ptr.To(true)
				its.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
			})
			reconcileSetStop(t, cli, 10)
			stopped := readSetStop(t, cli)
			status := stopped.FindInstanceStatus("demo-0")
			historicalTemplate := ""
			if retargetFirst {
				historicalTemplate = "named"
			}
			if len(listSetStopPods(t, cli)) != 0 || stopped.Status.Replicas != 0 || status == nil || status.TemplateName == nil || *status.TemplateName != historicalTemplate || status.DesiredState != workloads.InstanceDesiredStateOffline || status.CurrentState != workloads.InstanceCurrentStateAbsent {
				t.Fatalf("Stop did not drain and retain assigned template identity: %#v", status)
			}
			currentClaims := &corev1.PersistentVolumeClaimList{}
			if err := cli.List(context.Background(), currentClaims); err != nil || !reflect.DeepEqual(claims.Items, currentClaims.Items) {
				t.Fatalf("Stop changed retained claims: %v", err)
			}
			mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.Stop = ptr.To(false)
				its.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
			})
			reconcileSetStop(t, cli, 8)
			status = readSetStop(t, cli).FindInstanceStatus("demo-0")
			if len(listSetStopPods(t, cli)) != 2 || status == nil || status.TemplateName == nil || *status.TemplateName != "named" || status.DesiredState != workloads.InstanceDesiredStateActive {
				t.Fatal("Resume did not use the latest template assignment")
			}
			for _, pod := range listSetStopPods(t, cli) {
				if pod.Name == "demo-0" && pod.Labels["selected-template"] != "named" {
					t.Fatal("resumed Pod did not use the latest template")
				}
			}
		})
	}
}

func TestInstanceSetStopScaleRetirement(t *testing.T) {
	t.Run("Offline/WhenScaled=Retain", func(t *testing.T) {
		its := setStopFixture()
		its.Spec.Replicas = ptr.To(int32(3))
		its.Spec.PersistentVolumeClaimRetentionPolicy = &kbappsv1.PersistentVolumeClaimRetentionPolicy{WhenScaled: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType, WhenDeleted: kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType}
		cli := newSetStopClient(t, its)
		reconcileSetStop(t, cli, 5)
		bindSetStopClaims(t, cli)
		offlineKey := client.ObjectKey{Namespace: "default", Name: "data-demo-0"}
		claim := &corev1.PersistentVolumeClaim{}
		if err := cli.Get(context.Background(), offlineKey, claim); err != nil {
			t.Fatal(err)
		}
		before := claim.DeepCopy()
		mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
		reconcileSetStop(t, cli, 10)
		if len(listSetStopPods(t, cli)) != 0 {
			t.Fatal("Offline fixture did not finish Stop")
		}
		mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(false) })
		mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
			its.Spec.OfflineInstances = []string{"demo-0"}
			its.Spec.Replicas = ptr.To(int32(1))
			its.Spec.Template.Spec.Containers[0].Image = "db:latest"
		})
		wrapped := cli.(*setStopClient)
		wrapped.childWrites = 0
		wrapped.failChildWriteAt = 3
		if _, err := reconcileSetStopParent(cli); !errors.Is(err, errSetStopChildWrite) {
			t.Fatalf("expected partial Instance update failure, got %v", err)
		}
		committed := 0
		for _, child := range listSetStopChildren(t, cli) {
			if ptr.Deref(child.Spec.ScaledDown, false) {
				committed++
			} else if !ptr.Deref(child.Spec.Stop, false) {
				committed++
			}
		}
		if committed != 2 {
			t.Fatalf("partial failure persisted %d Instance updates, want 2", committed)
		}
		wrapped.failChildWriteAt = 0
		if _, err := reconcileSetStopParent(cli); err != nil {
			t.Fatal(err)
		}
		reconcileSetStop(t, cli, 8)
		pods := listSetStopPods(t, cli)
		if len(pods) != 1 || pods[0].Name != "demo-1" || pods[0].Spec.Containers[0].Image != "db:latest" {
			t.Fatalf("resume did not replace Offline runtime with current desired Pod: %#v", pods)
		}
		children := listSetStopChildren(t, cli)
		if len(children) != 1 || children[0].Name != "demo-1" {
			t.Fatal("resume did not delete excluded Instances")
		}
		if err := cli.Get(context.Background(), offlineKey, claim); err != nil {
			t.Fatalf("resume removed Offline storage: %v", err)
		}
		if claim.UID != before.UID || !reflect.DeepEqual(claim.Spec, before.Spec) {
			t.Fatal("WhenScaled=Retain did not preserve the excluded Instance storage")
		}
		claims := &corev1.PersistentVolumeClaimList{}
		if err := cli.List(context.Background(), claims); err != nil || len(claims.Items) != 3 {
			t.Fatalf("WhenScaled=Retain did not preserve all three claims: %v", err)
		}
	})
	for _, policy := range []kbappsv1.PersistentVolumeClaimRetentionPolicyType{kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType, kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType} {
		t.Run(fmt.Sprintf("policy=%s", policy), func(t *testing.T) {
			its := setStopFixture()
			its.Spec.PersistentVolumeClaimRetentionPolicy = &kbappsv1.PersistentVolumeClaimRetentionPolicy{WhenScaled: policy, WhenDeleted: kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType}
			cli := newSetStopClient(t, its)
			reconcileSetStop(t, cli, 5)
			mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(0)) })
			mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
			reconcileSetStop(t, cli, 12)
			if len(listSetStopPods(t, cli)) != 0 {
				t.Fatal("Stop with a pending scale-to-zero target did not drain")
			}
			claims := &corev1.PersistentVolumeClaimList{}
			if err := cli.List(context.Background(), claims); err != nil {
				t.Fatal(err)
			}
			if len(claims.Items) != 2 {
				t.Fatal("Stop retired storage")
			}
			mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(false) })
			reconcileSetStop(t, cli, 8)
			if len(listSetStopChildren(t, cli)) != 0 {
				t.Fatal("absent retirement was blocked by availability")
			}
			claims = &corev1.PersistentVolumeClaimList{}
			if err := cli.List(context.Background(), claims); err != nil {
				t.Fatal(err)
			}
			expected := 0
			if policy == kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType {
				expected = 2
			}
			if len(claims.Items) != expected {
				t.Fatalf("retired claim count %d, want %d", len(claims.Items), expected)
			}
		})
	}
}

func TestInstanceSetStopOrderedWaitsForHeldRuntime(t *testing.T) {
	supportResize := intctrlutil.SupportResizeSubResource
	intctrlutil.SupportResizeSubResource = func() (bool, error) { return false, nil }
	t.Cleanup(func() { intctrlutil.SupportResizeSubResource = supportResize })
	its := setStopFixture()
	cli := newSetStopClient(t, its)
	reconcileSetStop(t, cli, 5)
	claims := bindSetStopClaims(t, cli)
	held := &corev1.Pod{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "demo-1"}, held); err != nil {
		t.Fatal(err)
	}
	held.Finalizers = []string{"test/hold"}
	if err := cli.Update(context.Background(), held); err != nil {
		t.Fatal(err)
	}
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
		its.Spec.Stop = ptr.To(true)
	})
	reconcileSetStop(t, cli, 5)
	for _, child := range listSetStopChildren(t, cli) {
		if ptr.Deref(child.Spec.Stop, false) != (child.Name == "demo-1") {
			t.Fatal("ordered Stop passed a held higher ordinal")
		}
	}
	result, err := reconcileSetStopParent(cli)
	if err != nil {
		t.Fatal(err)
	}
	if result.RequeueAfter != time.Second {
		t.Fatalf("pending held Stop has no bounded retry: %#v", result)
	}
	if readSetStop(t, cli).Status.Replicas != 2 {
		t.Fatal("terminating runtime was lost from replica count")
	}
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Stop = ptr.To(false)
	})
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Template.Spec.Containers[0].Image = "db:v3"
	})
	reconcileSetStop(t, cli, 3)
	for _, child := range listSetStopChildren(t, cli) {
		if child.Name == "demo-1" && !ptr.Deref(child.Spec.Stop, false) {
			t.Fatal("early resume released a terminating child")
		}
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(held), held); err != nil {
		t.Fatal(err)
	}
	held.Finalizers = nil
	if err := cli.Update(context.Background(), held); err != nil {
		t.Fatal(err)
	}
	reconcileSetStop(t, cli, 4)
	for _, child := range listSetStopChildren(t, cli) {
		if child.Name == "demo-1" && !ptr.Deref(child.Spec.Stop, false) {
			t.Fatal("ordered resume passed unavailable predecessor")
		}
	}
	for i := 0; i < 8; i++ {
		markSetStopPodsReady(t, cli)
		reconcileSetStop(t, cli, 1)
	}
	reconcileSetStop(t, cli, 2)
	pods := listSetStopPods(t, cli)
	status := readSetStop(t, cli).Status
	if len(pods) != 2 || status.Replicas != 2 || status.ReadyReplicas != 2 || status.AvailableReplicas != 2 {
		t.Fatalf("ordered resume did not finish after predecessor availability: %#v", status)
	}
	for _, child := range listSetStopChildren(t, cli) {
		if ptr.Deref(child.Spec.Stop, false) || child.Spec.Template.Spec.Containers[0].Image != "db:v3" {
			t.Fatal("ordered resume left a stopped or obsolete child")
		}
	}
	for _, pod := range pods {
		if pod.Spec.Containers[0].Image != "db:v3" {
			t.Fatal("ordered resume created an obsolete runtime")
		}
	}
	currentClaims := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(context.Background(), currentClaims); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(currentClaims.Items, claims) {
		t.Fatal("held Stop and ordered resume changed storage")
	}
}

func TestInstanceSetStopParallelPartialWriteKeepsBudget(t *testing.T) {
	its := setStopFixture()
	its.Spec.Replicas = ptr.To(int32(3))
	cli := newSetStopClient(t, its)
	reconcileSetStop(t, cli, 5)
	claims := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(context.Background(), claims); err != nil {
		t.Fatal(err)
	}
	assertClaimsUnchanged := func() {
		t.Helper()
		current := &corev1.PersistentVolumeClaimList{}
		if err := cli.List(context.Background(), current); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(current.Items, claims.Items) {
			t.Fatal("partial Stop/resume changed storage")
		}
	}
	countHandoffs := func(stopping bool) int {
		t.Helper()
		count := 0
		for _, child := range listSetStopChildren(t, cli) {
			if ptr.Deref(child.Spec.Stop, false) != stopping {
				continue
			}
			count++
			if child.Generation == child.Status.ObservedGeneration || (!stopping && child.Spec.Template.Spec.Containers[0].Image != "db:latest") {
				t.Fatal("handoff did not invalidate the prior observation or deliver the current payload")
			}
		}
		return count
	}
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Stop = ptr.To(true)
		its.Spec.ParallelPodManagementConcurrency = ptr.To(intstr.FromInt(2))
	})
	wrapped := cli.(*setStopClient)
	wrapped.childWrites = 0
	wrapped.failChildWriteAt = 2
	if _, err := reconcileSetStopParent(cli); !errors.Is(err, errSetStopChildWrite) || wrapped.childWrites != 2 {
		t.Fatalf("expected second child write failure, got %v", err)
	}
	committedStops := countHandoffs(true)
	if committedStops != 1 {
		t.Fatalf("failed Stop invocation persisted %d handoffs, want 1", committedStops)
	}
	assertClaimsUnchanged()
	wrapped.failChildWriteAt = 0
	if _, err := reconcileSetStopParent(cli); err != nil {
		t.Fatal(err)
	}
	stopped := countHandoffs(true)
	if stopped != 2 {
		t.Fatalf("partial Stop handoff admitted %d children into a 2-slot budget", stopped)
	}
	if _, err := reconcileSetStopParent(cli); err != nil {
		t.Fatal(err)
	}
	stopped = countHandoffs(true)
	if stopped != 2 {
		t.Fatal("unobserved Stop requests released slots")
	}
	reconcileSetStop(t, cli, 8)
	if len(listSetStopPods(t, cli)) != 0 {
		t.Fatal("partial Stop did not recover")
	}
	assertClaimsUnchanged()
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Stop = ptr.To(false)
	})
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Template.Spec.Containers[0].Image = "db:latest"
	})
	wrapped.resumeWrites = 0
	wrapped.failResumeWriteAt = 2
	if _, err := reconcileSetStopParent(cli); !errors.Is(err, errSetStopResumeWrite) || wrapped.resumeWrites != 2 {
		t.Fatalf("expected second resume write failure, got %v", err)
	}
	committedResumes := countHandoffs(false)
	if committedResumes != 1 {
		t.Fatalf("failed resume invocation persisted %d handoffs, want 1", committedResumes)
	}
	assertClaimsUnchanged()
	wrapped.failResumeWriteAt = 0
	if _, err := reconcileSetStopParent(cli); err != nil {
		t.Fatal(err)
	}
	resumed := countHandoffs(false)
	if resumed < 1 || resumed > 2 {
		t.Fatalf("partial resume admitted %d children into a 2-slot budget", resumed)
	}
	for _, child := range listSetStopChildren(t, cli) {
		if !ptr.Deref(child.Spec.Stop, false) {
			continue
		}
		for i := 0; i < 2; i++ {
			_, err := (&InstanceReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(1000)}).Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&child)})
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := reconcileSetStopParent(cli); err != nil {
		t.Fatal(err)
	}
	resumed = countHandoffs(false)
	if resumed != 2 {
		t.Fatalf("resume admitted %d children after fresh dormant observations, want 2", resumed)
	}
	if _, err := reconcileSetStopParent(cli); err != nil {
		t.Fatal(err)
	}
	resumed = countHandoffs(false)
	if resumed != 2 {
		t.Fatal("stale resumed availability released slots")
	}
	reconcileSetStop(t, cli, 3)
	for _, pod := range listSetStopPods(t, cli) {
		pod.Status.Phase = corev1.PodRunning
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "db", Image: "db:latest", Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}
		if err := cli.Status().Update(context.Background(), &pod); err != nil {
			t.Fatal(err)
		}
	}
	reconcileSetStop(t, cli, 5)
	pods := listSetStopPods(t, cli)
	if len(pods) != 3 || readSetStop(t, cli).Status.Replicas != 3 {
		t.Fatal("partial resume did not converge after admitted runtimes became available")
	}
	for _, child := range listSetStopChildren(t, cli) {
		if ptr.Deref(child.Spec.Stop, false) || child.Spec.Template.Spec.Containers[0].Image != "db:latest" {
			t.Fatal("partial resume left a stopped or obsolete child")
		}
	}
	for _, pod := range pods {
		if pod.Spec.Containers[0].Image != "db:latest" {
			t.Fatal("partial resume recreated an obsolete runtime")
		}
	}
	assertClaimsUnchanged()
}

func newSetStopClient(t *testing.T, its *workloads.InstanceSet) client.Client {
	t.Helper()
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := workloads.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	model.AddScheme(workloads.AddToScheme)
	return &setStopClient{Client: fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&workloads.InstanceSet{}, &workloads.Instance{}, &corev1.Pod{}, &corev1.PersistentVolumeClaim{}).WithObjects(its).Build()}
}

func setStopFixture() *workloads.InstanceSet {
	inst := stopTestInstance()
	return &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "set-uid", Generation: 1}, Spec: workloads.InstanceSetSpec{
		Replicas: ptr.To(int32(2)), Selector: inst.Spec.Selector, Template: inst.Spec.Template, VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: inst.Spec.VolumeClaimTemplates[0].ObjectMeta, Spec: inst.Spec.VolumeClaimTemplates[0].Spec}},
		PodManagementPolicy: appsv1.ParallelPodManagement, EnableInstanceAPI: ptr.To(true),
	}}
}

func reconcileSetStop(t *testing.T, cli client.Client, count int) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < count; i++ {
		_, err := reconcileSetStopParent(cli)
		if err != nil {
			t.Fatal(err)
		}
		list := &workloads.InstanceList{}
		if err := cli.List(ctx, list); err != nil {
			t.Fatal(err)
		}
		for _, inst := range list.Items {
			_, err = (&InstanceReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(1000)}).Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&inst)})
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestInstanceSetStopScaleReversalKeepsDeletionPolicy(t *testing.T) {
	for _, stopping := range []bool{true, false} {
		t.Run(fmt.Sprintf("Stop=%t", stopping), func(t *testing.T) {
			its := setStopFixture()
			its.Spec.PersistentVolumeClaimRetentionPolicy = &kbappsv1.PersistentVolumeClaimRetentionPolicy{
				WhenScaled: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType, WhenDeleted: kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType,
			}
			cli := newSetStopClient(t, its)
			reconcileSetStop(t, cli, 5)
			if !stopping {
				mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
				reconcileSetStop(t, cli, 8)
			}
			mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.Stop = ptr.To(false)
				its.Spec.Replicas = ptr.To(int32(1))
				its.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
			})
			if _, err := reconcileSetStopParent(cli); err != nil {
				t.Fatal(err)
			}
			inst := &workloads.Instance{}
			key := client.ObjectKey{Namespace: "default", Name: "demo-1"}
			if err := cli.Get(context.Background(), key, inst); err != nil || !ptr.Deref(inst.Spec.ScaledDown, false) || !inst.DeletionTimestamp.IsZero() {
				t.Fatalf("scale-down did not produce the intermediate marker: %v", err)
			}
			if !stopping {
				if _, err := (&InstanceReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(1000)}).Reconcile(context.Background(), ctrl.Request{NamespacedName: key}); err != nil {
					t.Fatal(err)
				}
			}
			mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.Stop = ptr.To(stopping)
				its.Spec.Replicas = ptr.To(int32(2))
				its.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
			})
			if _, err := reconcileSetStopParent(cli); err != nil {
				t.Fatal(err)
			}
			if err := cli.Get(context.Background(), key, inst); err != nil || ptr.Deref(inst.Spec.Stop, false) != stopping {
				t.Fatalf("reversal did not hand off Stop/resume: %v", err)
			}
			if err := cli.Delete(context.Background(), readSetStop(t, cli)); err != nil {
				t.Fatal(err)
			}
			reconcileSetStop(t, cli, 12)
			claims := &corev1.PersistentVolumeClaimList{}
			if err := cli.List(context.Background(), claims); err != nil || len(claims.Items) != 0 || len(listSetStopChildren(t, cli)) != 0 || len(listSetStopPods(t, cli)) != 0 {
				t.Fatalf("reversed scale-down selected WhenScaled instead of WhenDeleted: claims=%d err=%v", len(claims.Items), err)
			}
		})
	}
}

func TestInstanceSetStopInitiallyStopped(t *testing.T) {
	its := setStopFixture()
	its.Spec.Stop = ptr.To(true)
	cli := newSetStopClient(t, its)
	reconcileSetStop(t, cli, 5)
	pods := &corev1.PodList{}
	pvcs := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(context.Background(), pods); err != nil {
		t.Fatal(err)
	}
	if err := cli.List(context.Background(), pvcs); err != nil {
		t.Fatal(err)
	}
	if len(listSetStopChildren(t, cli)) != 0 || len(pods.Items) != 0 || len(pvcs.Items) != 0 {
		t.Fatalf("initial Stop provisioned %d Pods and %d PVCs", len(pods.Items), len(pvcs.Items))
	}
}

var _ = Describe("InstanceSet Stop path parity", func() {
	for _, enableInstanceAPI := range []bool{false, true} {
		It(fmt.Sprintf("preserves data and resumes the latest template with enableInstanceAPI=%t", enableInstanceAPI), func() {
			factory := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace, "stop-parity", "stop-parity", "db").
				WithRandomName().
				AddContainer(corev1.Container{Name: "db", Image: "db:v1"}).
				SetReplicas(1).
				SetEnableInstanceAPI(ptr.To(enableInstanceAPI)).
				SetPodManagementPolicy(appsv1.ParallelPodManagement).
				AddVolumeClaimTemplate(corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{Name: "data"},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1Gi"),
						}},
					},
				})
			factory.Get().Spec.MinReadySeconds = 3600
			factory.Get().Spec.PodUpdatePolicy = kbappsv1.StrictInPlacePodUpdatePolicyType
			factory.Get().Spec.InstanceUpdateStrategy = &workloads.InstanceUpdateStrategy{Type: kbappsv1.OnDeleteStrategyType}
			its := factory.Create(&testCtx).GetObject()
			itsKey := client.ObjectKeyFromObject(its)
			podKey := client.ObjectKey{Namespace: its.Namespace, Name: its.Name + "-0"}
			pvcKey := client.ObjectKey{Namespace: its.Namespace, Name: "data-" + podKey.Name}
			DeferCleanup(func() {
				Eventually(func() error {
					return client.IgnoreNotFound(testapps.GetAndChangeObj(&testCtx, podKey, func(p *corev1.Pod) { p.Finalizers = nil })())
				}).Should(Succeed())
				inNS := client.InNamespace(its.Namespace)
				ml := client.MatchingLabels{constant.AppInstanceLabelKey: "stop-parity"}
				testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.InstanceSetSignature, true, inNS, ml)
				testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.InstanceSignature, true, inNS, ml)
				testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PodSignature, true, inNS, ml)
				testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS, ml)
				testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.ServiceSignature, true, inNS, ml)
			})

			pod := &corev1.Pod{}
			claim := &corev1.PersistentVolumeClaim{}
			Eventually(func() error { return k8sClient.Get(ctx, podKey, pod) }).Should(Succeed())
			Eventually(func() error { return k8sClient.Get(ctx, pvcKey, claim) }).Should(Succeed())
			oldPodUID, pvcUID := pod.UID, claim.UID
			Eventually(testapps.GetAndChangeObj(&testCtx, podKey, func(p *corev1.Pod) {
				p.Finalizers = append(p.Finalizers, "test.kubeblocks.io/stop-parity")
			})).Should(Succeed())
			Eventually(func() error { return k8sClient.Get(ctx, pvcKey, claim) }).Should(Succeed())
			pvcVersion, pvcSpec, pvcOwners := claim.ResourceVersion, claim.Spec.DeepCopy(), claim.OwnerReferences

			Eventually(testapps.GetAndChangeObj(&testCtx, itsKey, func(set *workloads.InstanceSet) {
				set.Spec.Stop = ptr.To(true)
			})).Should(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, podKey, pod)).Should(Succeed())
				g.Expect(pod.DeletionTimestamp.IsZero()).Should(BeFalse())
				g.Expect(k8sClient.Get(ctx, itsKey, its)).Should(Succeed())
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(its.Status.InstanceStatus[0].CurrentState).Should(Equal(workloads.InstanceCurrentStateTerminating))
				g.Expect(its.Status.InstanceStatus[0].UpToDate).Should(BeFalse())
				g.Expect(its.Status.Replicas).Should(Equal(int32(1)))
			}).Should(Succeed())

			Consistently(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, pvcKey, claim)).Should(Succeed())
				g.Expect(claim.ResourceVersion).Should(Equal(pvcVersion))
				g.Expect(claim.Spec).Should(Equal(*pvcSpec))
				g.Expect(claim.OwnerReferences).Should(Equal(pvcOwners))
			}, time.Second).Should(Succeed())
			Eventually(testapps.GetAndChangeObj(&testCtx, itsKey, func(set *workloads.InstanceSet) {
				set.Spec.Stop = nil
			})).Should(Succeed())
			Eventually(testapps.GetAndChangeObj(&testCtx, itsKey, func(set *workloads.InstanceSet) {
				set.Spec.Template.Spec.Containers[0].Image = "db:v2"
				set.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{{Name: "db", ContainerPort: 5432, Protocol: corev1.ProtocolTCP}}
				set.Spec.VolumeClaimTemplates[0].Annotations = map[string]string{"test.kubeblocks.io/resume": "latest"}
			})).Should(Succeed())
			Eventually(func(g Gomega) {
				service := &corev1.Service{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: its.Namespace, Name: its.Name + "-headless"}, service)).Should(Succeed())
				g.Expect(service.Spec.Ports).Should(HaveLen(1))
				g.Expect(service.Spec.Ports[0].Port).Should(Equal(int32(5432)))
			}).Should(Succeed())
			Consistently(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, podKey, pod)).Should(Succeed())
				g.Expect(pod.UID).Should(Equal(oldPodUID))
				g.Expect(pod.Spec.Containers[0].Image).Should(Equal("db:v1"))
				g.Expect(k8sClient.Get(ctx, pvcKey, claim)).Should(Succeed())
				g.Expect(claim.ResourceVersion).Should(Equal(pvcVersion))
			}, time.Second).Should(Succeed())
			Eventually(testapps.GetAndChangeObj(&testCtx, podKey, func(p *corev1.Pod) { p.Finalizers = nil })).Should(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, podKey, pod)).Should(Succeed())
				g.Expect(pod.UID).ShouldNot(Equal(oldPodUID))
				g.Expect(pod.Spec.Containers[0].Image).Should(Equal("db:v2"))
				g.Expect(k8sClient.Get(ctx, pvcKey, claim)).Should(Succeed())
				g.Expect(claim.UID).Should(Equal(pvcUID))
				g.Expect(claim.Spec).Should(Equal(*pvcSpec))
				g.Expect(claim.OwnerReferences).Should(Equal(pvcOwners))
				g.Expect(claim.Annotations["test.kubeblocks.io/resume"]).Should(Equal("latest"))
			}).Should(Succeed())

			Eventually(testapps.GetAndChangeObj(&testCtx, itsKey, func(set *workloads.InstanceSet) { set.Spec.Stop = ptr.To(true) })).Should(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, podKey, &corev1.Pod{}))).Should(BeTrue())
				g.Expect(k8sClient.Get(ctx, itsKey, its)).Should(Succeed())
				g.Expect(its.Status.Replicas).Should(BeZero())
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(its.Status.InstanceStatus[0].CurrentState).Should(Equal(workloads.InstanceCurrentStateAbsent))
				g.Expect(k8sClient.Get(ctx, pvcKey, claim)).Should(Succeed())
				g.Expect(claim.UID).Should(Equal(pvcUID))
			}).Should(Succeed())
		})
	}
})

func TestInstanceSetStopWaitsForDeletionObservation(t *testing.T) {
	cli := newSetStopClient(t, setStopFixture())
	ctx := context.Background()
	reconcileParent := func() {
		t.Helper()
		if _, err := reconcileSetStopParent(cli); err != nil {
			t.Fatal(err)
		}
	}
	reconcileInstance := func(name string) {
		t.Helper()
		_, err := (&InstanceReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(1000)}).Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: name}})
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 3; i++ {
		reconcileParent()
	}
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Replicas = ptr.To(int32(1))
		its.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
	})
	reconcileParent()
	for _, name := range []string{"demo-0", "demo-1"} {
		reconcileInstance(name)
		reconcileInstance(name)
	}
	markSetStopPodsReady(t, cli)
	inst := &workloads.Instance{}
	key := client.ObjectKey{Namespace: "default", Name: "demo-1"}
	if err := cli.Get(ctx, key, inst); err != nil {
		t.Fatal(err)
	}
	if !ptr.Deref(inst.Spec.ScaledDown, false) || inst.Status.CurrentState != workloads.InstanceCurrentStateAbsent || inst.Status.ObservedGeneration != inst.Generation {
		t.Fatalf("fixture did not produce current-generation Absent before deletion: %#v", inst)
	}
	generation := inst.Generation
	pod := &corev1.Pod{}
	if err := cli.Get(ctx, key, pod); err != nil {
		t.Fatal(err)
	}
	pod.Finalizers = []string{"test/hold"}
	if err := cli.Update(ctx, pod); err != nil {
		t.Fatal(err)
	}
	reconcileParent()
	if err := cli.Get(ctx, key, inst); err != nil || inst.DeletionTimestamp.IsZero() || inst.Generation != generation {
		t.Fatalf("scale-down did not delete the Instance without changing generation: %v", err)
	}
	if err := cli.Get(ctx, key, pod); err != nil || pod.Status.Phase != corev1.PodRunning || !pod.DeletionTimestamp.IsZero() {
		t.Fatalf("fixture did not keep the Pod Running before deletion was observed: %v", err)
	}
	mutateSetStop(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
	reconcileParent()
	if err := cli.Get(ctx, key, inst); err != nil || !ptr.Deref(inst.Spec.Stop, false) || inst.Generation <= generation || inst.Status.ObservedGeneration == inst.Generation {
		t.Fatalf("deleting Instance Stop handoff did not invalidate the pre-deletion observation: %v", err)
	}
	remaining := &workloads.Instance{}
	if err := cli.Get(ctx, client.ObjectKey{Namespace: "default", Name: "demo-0"}, remaining); err != nil || ptr.Deref(remaining.Spec.Stop, false) {
		t.Fatalf("pre-deletion Absent released the Stop slot while the Pod was Running: %v", err)
	}
	if readSetStop(t, cli).Status.Replicas != 2 {
		t.Fatal("Stop excluded the deleting Instance before observing its runtime deletion")
	}
	reconcileInstance("demo-1")
	reconcileInstance("demo-1")
	reconcileParent()
	if err := cli.Get(ctx, client.ObjectKeyFromObject(remaining), remaining); err != nil || ptr.Deref(remaining.Spec.Stop, false) {
		t.Fatalf("terminating Pod released the Stop slot: %v", err)
	}
	if err := cli.Get(ctx, key, pod); err != nil {
		t.Fatal(err)
	}
	pod.Finalizers = nil
	if err := cli.Update(ctx, pod); err != nil {
		t.Fatal(err)
	}
	reconcileSetStop(t, cli, 8)
	if len(listSetStopPods(t, cli)) != 0 || readSetStop(t, cli).Status.Replicas != 0 {
		t.Fatal("Stop did not converge after the deleting Pod disappeared")
	}
}

func TestInstanceSetStopWaitsForRetiringRuntime(t *testing.T) {
	for _, policy := range []appsv1.PodManagementPolicyType{appsv1.OrderedReadyPodManagement, appsv1.ParallelPodManagement} {
		t.Run(string(policy), func(t *testing.T) {
			deletingName, remainingName := "demo-1", "demo-0"
			if policy == appsv1.ParallelPodManagement {
				deletingName, remainingName = "demo-0", "demo-1"
			}
			its := setStopFixture()
			its.Spec.PersistentVolumeClaimRetentionPolicy = &kbappsv1.PersistentVolumeClaimRetentionPolicy{WhenScaled: kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType}
			cli := newSetStopClient(t, its)
			reconcileSetStop(t, cli, 5)
			held := &corev1.Pod{}
			if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: deletingName}, held); err != nil {
				t.Fatal(err)
			}
			held.Finalizers = append(held.Finalizers, "test/hold")
			if err := cli.Update(context.Background(), held); err != nil {
				t.Fatal(err)
			}
			pvc := &corev1.PersistentVolumeClaim{}
			pvcKey := client.ObjectKey{Namespace: "default", Name: "data-" + deletingName}
			if err := cli.Get(context.Background(), pvcKey, pvc); err != nil {
				t.Fatal(err)
			}
			pvc.Finalizers = []string{"test/hold-pvc"}
			if err := cli.Update(context.Background(), pvc); err != nil {
				t.Fatal(err)
			}
			mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.Replicas = ptr.To(int32(1))
				its.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
				if policy == appsv1.ParallelPodManagement {
					its.Spec.OfflineInstances = []string{deletingName}
				}
			})
			reconcileSetStop(t, cli, 4)
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(held), held); err != nil {
				t.Fatal(err)
			}
			if held.DeletionTimestamp.IsZero() {
				t.Fatal("fixture did not hold a terminating runtime")
			}
			child := &workloads.Instance{}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(held), child); err != nil {
				t.Fatal(err)
			}
			if child.DeletionTimestamp.IsZero() {
				t.Fatal("fixture child was not deleting")
			}
			mutateSetStop(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.Stop = ptr.To(true)
				its.Spec.PodManagementPolicy = policy
				its.Spec.ParallelPodManagementConcurrency = ptr.To(intstr.FromInt(1))
			})
			reconcileSetStop(t, cli, 4)
			result, err := reconcileSetStopParent(cli)
			if err != nil || result.RequeueAfter != time.Second {
				t.Fatalf("held retirement has no bounded Stop retry: result=%#v err=%v", result, err)
			}
			remainingPod := &corev1.Pod{}
			err = cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: remainingName}, remainingPod)
			if err != nil || !remainingPod.DeletionTimestamp.IsZero() {
				t.Fatalf("Stop passed held deletion: remaining Pod err=%v, deletion=%v", err, remainingPod.DeletionTimestamp)
			}
			held.Finalizers = nil
			if err := cli.Update(context.Background(), held); err != nil {
				t.Fatal(err)
			}
			faulty := &failStopStatusClient{Client: cli, remaining: 1}
			_, err = (&InstanceReconciler{Client: faulty, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(1000)}).Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(child)})
			if err == nil || faulty.remaining != 0 {
				t.Fatalf("expected deleting Instance status failure, got %v", err)
			}
			if _, err = reconcileSetStopParent(cli); err != nil {
				t.Fatal(err)
			}
			remaining := &workloads.Instance{}
			if err = cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: remainingName}, remaining); err != nil || ptr.Deref(remaining.Spec.Stop, false) {
				t.Fatalf("uncommitted absence released Stop budget: %v", err)
			}
			reconcileSetStop(t, cli, 8)
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(held), &corev1.Pod{}); !apierrors.IsNotFound(err) {
				t.Fatalf("scale-down Pod did not disappear: %v", err)
			}
			if err := cli.Get(context.Background(), pvcKey, pvc); err != nil || pvc.DeletionTimestamp.IsZero() {
				t.Fatalf("fixture did not hold PVC deletion: %v", err)
			}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(held), child); err != nil || child.DeletionTimestamp.IsZero() || !ptr.Deref(child.Spec.Stop, false) {
				t.Fatalf("scale-down Instance should still be deleting with Stop: %v", err)
			}
			if len(listSetStopPods(t, cli)) != 0 {
				t.Fatal("PVC cleanup blocked Stop after the scale-down Pod disappeared")
			}
			if child.Status.CurrentState != workloads.InstanceCurrentStateAbsent || child.Status.ObservedGeneration != child.Generation {
				t.Fatalf("deleting Instance did not report current Absent state: %#v", child.Status)
			}
			result, err = reconcileSetStopParent(cli)
			if err != nil || result.Requeue || readSetStop(t, cli).Status.Replicas != 0 {
				t.Fatalf("PVC cleanup prevented Stop completion: result=%#v err=%v", result, err)
			}
			pvc.Finalizers = nil
			if err := cli.Update(context.Background(), pvc); err != nil {
				t.Fatal(err)
			}
			reconcileSetStop(t, cli, 4)
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(held), &workloads.Instance{}); !apierrors.IsNotFound(err) {
				t.Fatalf("scale-down Instance cleanup did not complete: %v", err)
			}
			claims := &corev1.PersistentVolumeClaimList{}
			if err := cli.List(context.Background(), claims); err != nil || len(claims.Items) != 1 || claims.Items[0].Name != "data-"+remainingName {
				t.Fatalf("Stop changed the remaining Instance claim: %v", err)
			}
		})
	}

}

func retentionFixture(t *testing.T, replicas int32, configure ...func(*workloads.InstanceSet)) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{clientgoscheme.AddToScheme, workloads.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "set-uid", Generation: 1}, Spec: workloads.InstanceSetSpec{
		EnableInstanceAPI: ptr.To(true), Replicas: ptr.To(replicas), PodManagementPolicy: appsv1.ParallelPodManagement,
		Selector:                             &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}},
		Template:                             corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "demo"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}}},
		VolumeClaimTemplates:                 []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}, Spec: corev1.PersistentVolumeClaimSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}}}}},
		PersistentVolumeClaimRetentionPolicy: &kbappsv1.PersistentVolumeClaimRetentionPolicy{WhenScaled: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType, WhenDeleted: kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType},
	}}
	for _, change := range configure {
		change(its)
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&workloads.InstanceSet{}, &workloads.Instance{}, &corev1.Pod{}, &corev1.PersistentVolumeClaim{}).WithObjects(its).Build()
	runRetentionControllers(t, cli, 5)
	pods := &corev1.PodList{}
	if err := cli.List(context.Background(), pods); err != nil {
		t.Fatal(err)
	}
	if len(pods.Items) != int(replicas) {
		t.Fatalf("fixture has %d Pods, want %d", len(pods.Items), replicas)
	}
	for _, pod := range pods.Items {
		pod.Status.Phase = corev1.PodRunning
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Hour))}}
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "db", Image: "db:v1", Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}
		if err := cli.Status().Update(context.Background(), &pod); err != nil {
			t.Fatal(err)
		}
	}
	claims := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(context.Background(), claims); err != nil {
		t.Fatal(err)
	}
	if len(claims.Items) != int(replicas) {
		t.Fatalf("fixture has %d PVCs, want %d", len(claims.Items), replicas)
	}
	for _, claim := range claims.Items {
		claim.Status.Phase = corev1.ClaimBound
		claim.Status.Capacity = claim.Spec.Resources.Requests.DeepCopy()
		if err := cli.Status().Update(context.Background(), &claim); err != nil {
			t.Fatal(err)
		}
	}
	runRetentionControllers(t, cli, 3)
	return cli
}

func reconcileRetentionSet(cli client.Client) (ctrl.Result, error) {
	return (&InstanceSetReconciler2{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(1000)}).Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "demo"}})
}

func retentionSet(t *testing.T, cli client.Client) *workloads.InstanceSet {
	t.Helper()
	its := &workloads.InstanceSet{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "demo"}, its); err != nil {
		t.Fatal(err)
	}
	return its
}

func changeRetentionSet(t *testing.T, cli client.Client, change func(*workloads.InstanceSet)) {
	t.Helper()
	its := retentionSet(t, cli)
	change(its)
	its.Generation++
	if err := cli.Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
}

func retentionInstances(t *testing.T, cli client.Client) []workloads.Instance {
	t.Helper()
	list := &workloads.InstanceList{}
	if err := cli.List(context.Background(), list); err != nil {
		t.Fatal(err)
	}
	return list.Items
}

func runRetentionControllers(t *testing.T, cli client.Client, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		if _, err := reconcileRetentionSet(cli); err != nil {
			t.Fatal(err)
		}
		for _, inst := range retentionInstances(t, cli) {
			if _, err := (&InstanceReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(1000)}).Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&inst)}); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func checkRetentionDeleted(t *testing.T, cli client.Client, wantClaims int) {
	t.Helper()
	runRetentionControllers(t, cli, 12)
	pods := &corev1.PodList{}
	claims := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(context.Background(), pods); err != nil {
		t.Fatal(err)
	}
	if len(pods.Items) != 0 || len(retentionInstances(t, cli)) != 0 {
		t.Fatal("Instance and Pod deletion did not converge")
	}
	if err := cli.List(context.Background(), claims); err != nil {
		t.Fatal(err)
	}
	if len(claims.Items) != wantClaims {
		t.Fatalf("PVCs after deletion = %d, want %d", len(claims.Items), wantClaims)
	}
}

func TestInstanceRetentionPolicyChange(t *testing.T) {
	for _, deleting := range []bool{false, true} {
		for _, retain := range []bool{false, true} {
			t.Run(fmt.Sprintf("InstanceSetDeletion=%t/Retain=%t", deleting, retain), func(t *testing.T) {
				cli := retentionFixture(t, 2)
				old, current := kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType, kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType
				want := 0
				if retain {
					old, current, want = current, old, 2
				}
				changeRetentionSet(t, cli, func(its *workloads.InstanceSet) {
					its.Spec.PersistentVolumeClaimRetentionPolicy = &kbappsv1.PersistentVolumeClaimRetentionPolicy{WhenScaled: old, WhenDeleted: old}
				})
				runRetentionControllers(t, cli, 4)
				changeRetentionSet(t, cli, func(its *workloads.InstanceSet) {
					if deleting {
						its.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted = current
					} else {
						its.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled = current
						its.Spec.Replicas = ptr.To(int32(0))
					}
				})
				if deleting {
					if err := cli.Delete(context.Background(), retentionSet(t, cli)); err != nil {
						t.Fatal(err)
					}
				}
				checkRetentionDeleted(t, cli, want)
			})
		}
	}
}

func TestInstanceRetentionScaleReversal(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		policy   appsv1.PodManagementPolicyType
		creating bool
	}{
		{name: "Aligned", policy: appsv1.ParallelPodManagement},
		{name: "OrderedReadyCreation", policy: appsv1.OrderedReadyPodManagement, creating: true},
		{name: "ParallelCreation", policy: appsv1.ParallelPodManagement, creating: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			cli := retentionFixture(t, 2, func(its *workloads.InstanceSet) {
				if scenario.creating {
					its.Spec.Ordinals.Discrete = []int32{2, 3}
				}
			})
			changeRetentionSet(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.Replicas = ptr.To(int32(1))
				its.Spec.PodManagementPolicy = scenario.policy
				its.Spec.ParallelPodManagementConcurrency = ptr.To(intstr.FromInt32(1))
			})
			if _, err := reconcileRetentionSet(cli); err != nil {
				t.Fatal(err)
			}
			inst := &workloads.Instance{}
			key := client.ObjectKey{Namespace: "default", Name: "demo-1"}
			if scenario.creating {
				key.Name = "demo-3"
			}
			if err := cli.Get(context.Background(), key, inst); err != nil {
				t.Fatal(err)
			}
			if !ptr.Deref(inst.Spec.ScaledDown, false) || !inst.DeletionTimestamp.IsZero() {
				t.Fatal("scale-down did not persist the intermediate spec.scaledDown state")
			}
			changeRetentionSet(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.Replicas = ptr.To(int32(2))
				if scenario.creating {
					its.Spec.Replicas = ptr.To(int32(4))
					its.Spec.Ordinals.Discrete = []int32{0, 1, 2, 3}
				}
			})
			runRetentionControllers(t, cli, 3)
			if err := cli.Get(context.Background(), key, inst); err != nil {
				t.Fatal(err)
			}
			if inst.Status.CurrentState != workloads.InstanceCurrentStatePresent {
				t.Fatal("Instance did not return to Present")
			}
			if ptr.Deref(inst.Spec.ScaledDown, false) {
				t.Error("desired Instance still has spec.scaledDown=true")
			}
			if scenario.creating && len(retentionInstances(t, cli)) != 3 {
				t.Fatal("fixture did not block creation of the remaining desired Instance")
			}
			if err := cli.Delete(context.Background(), retentionSet(t, cli)); err != nil {
				t.Fatal(err)
			}
			checkRetentionDeleted(t, cli, 0)
		})
	}
}

type retentionWriteFailureClient struct {
	client.Client
	writes  int
	failure error
}

func (c *retentionWriteFailureClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if inst, ok := obj.(*workloads.Instance); ok {
		old := &workloads.Instance{}
		if err := c.Get(ctx, client.ObjectKeyFromObject(inst), old); err != nil {
			return err
		}
		if !reflect.DeepEqual(old.Spec, inst.Spec) {
			c.writes++
			if c.writes == 2 {
				if apierrors.IsConflict(c.failure) {
					if old.Annotations == nil {
						old.Annotations = map[string]string{}
					}
					old.Annotations["test/concurrent-update"] = "preserved"
					if err := c.Client.Update(ctx, old); err != nil {
						return err
					}
				}
				return c.failure
			}
		}
	}
	return c.Client.Update(ctx, obj, opts...)
}

func TestInstanceRetentionPartialHandoff(t *testing.T) {
	writeFailure := errors.New("injected Instance update failure")
	conflict := apierrors.NewConflict(schema.GroupResource{Group: workloads.GroupVersion.Group, Resource: "instances"}, "demo", errors.New("concurrent update"))
	for _, deleting := range []bool{false, true} {
		for _, failure := range []error{writeFailure, conflict} {
			t.Run(fmt.Sprintf("InstanceSetDeletion=%t/Conflict=%t", deleting, apierrors.IsConflict(failure)), func(t *testing.T) {
				cli := retentionFixture(t, 4)
				changeRetentionSet(t, cli, func(its *workloads.InstanceSet) {
					its.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted = kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType
				})
				runRetentionControllers(t, cli, 4)
				changeRetentionSet(t, cli, func(its *workloads.InstanceSet) {
					its.Spec.PersistentVolumeClaimRetentionPolicy = &kbappsv1.PersistentVolumeClaimRetentionPolicy{WhenScaled: kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType, WhenDeleted: kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType}
					if !deleting {
						its.Spec.Replicas = ptr.To(int32(2))
					}
				})
				if deleting {
					if err := cli.Delete(context.Background(), retentionSet(t, cli)); err != nil {
						t.Fatal(err)
					}
				}
				faulty := &retentionWriteFailureClient{Client: cli, failure: failure}
				result, err := reconcileRetentionSet(faulty)
				if apierrors.IsConflict(failure) {
					if err != nil || !result.Requeue {
						t.Fatalf("conflict must retry, result=%#v err=%v", result, err)
					}
				} else if !errors.Is(err, failure) {
					t.Fatalf("expected injected error, got %v", err)
				}
				updated := 0
				instances := retentionInstances(t, cli)
				if len(instances) != 4 {
					t.Fatal("partial handoff deleted an Instance")
				}
				for _, inst := range instances {
					if !inst.DeletionTimestamp.IsZero() {
						t.Fatal("partial handoff began Instance deletion")
					}
					if inst.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled == kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType {
						updated++
						if !deleting && !ptr.Deref(inst.Spec.ScaledDown, false) {
							t.Fatal("scale-down policy and marker were not persisted together")
						}
					}
				}
				if updated != 1 {
					t.Fatalf("partial handoff persisted %d policies, want 1", updated)
				}
				if deleting {
					result, err = reconcileRetentionSet(cli)
					if err != nil || result.RequeueAfter <= 0 {
						t.Fatalf("policy handoff must schedule another reconcile, result=%#v err=%v", result, err)
					}
					for _, inst := range retentionInstances(t, cli) {
						if !inst.DeletionTimestamp.IsZero() || inst.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted != kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType {
							t.Fatal("Instance deletion preceded persisted current policy")
						}
					}
					if apierrors.IsConflict(failure) {
						preserved := 0
						for _, inst := range retentionInstances(t, cli) {
							if inst.Annotations["test/concurrent-update"] == "preserved" {
								preserved++
							}
						}
						if preserved != 1 {
							t.Fatal("retry overwrote the concurrent Instance metadata update")
						}
					}
				}
				if !deleting {
					runRetentionControllers(t, cli, 8)
					if len(retentionInstances(t, cli)) != 2 {
						t.Fatal("partial scale-down did not converge")
					}
					changeRetentionSet(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(0)) })
				}
				checkRetentionDeleted(t, cli, 0)
			})
		}
	}
}

func TestInstanceRetentionPartialScaleReversal(t *testing.T) {
	cli := retentionFixture(t, 4)
	changeRetentionSet(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(2)) })
	if _, err := reconcileRetentionSet(cli); err != nil {
		t.Fatal(err)
	}
	changeRetentionSet(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(4)) })
	failure := errors.New("injected scale reversal update failure")
	if _, err := reconcileRetentionSet(&retentionWriteFailureClient{Client: cli, failure: failure}); !errors.Is(err, failure) {
		t.Fatalf("expected partial reversal failure, got %v", err)
	}
	marked := 0
	for _, inst := range retentionInstances(t, cli) {
		if !inst.DeletionTimestamp.IsZero() {
			t.Fatal("scale reversal started Instance deletion")
		}
		if ptr.Deref(inst.Spec.ScaledDown, false) {
			marked++
		}
	}
	if marked != 1 {
		t.Fatalf("partial scale reversal has %d marked Instances, want 1", marked)
	}
	runRetentionControllers(t, cli, 3)
	for _, inst := range retentionInstances(t, cli) {
		if ptr.Deref(inst.Spec.ScaledDown, false) {
			t.Fatal("scale reversal retry left spec.scaledDown=true")
		}
	}
	if err := cli.Delete(context.Background(), retentionSet(t, cli)); err != nil {
		t.Fatal(err)
	}
	checkRetentionDeleted(t, cli, 0)
}

func TestInstanceRetentionDeletingInstanceKeepsPolicy(t *testing.T) {
	cli := retentionFixture(t, 2)
	pod := &corev1.Pod{}
	key := client.ObjectKey{Namespace: "default", Name: "demo-1"}
	if err := cli.Get(context.Background(), key, pod); err != nil {
		t.Fatal(err)
	}
	pod.Finalizers = append(pod.Finalizers, "test/hold")
	if err := cli.Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	changeRetentionSet(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(1)) })
	runRetentionControllers(t, cli, 3)
	inst := &workloads.Instance{}
	if err := cli.Get(context.Background(), key, inst); err != nil {
		t.Fatal(err)
	}
	if inst.DeletionTimestamp.IsZero() || !ptr.Deref(inst.Spec.ScaledDown, false) {
		t.Fatal("fixture did not begin scale-down deletion")
	}
	changeRetentionSet(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Replicas = ptr.To(int32(2))
		its.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled = kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType
	})
	if _, err := reconcileRetentionSet(cli); err != nil {
		t.Fatal(err)
	}
	if err := cli.Delete(context.Background(), retentionSet(t, cli)); err != nil {
		t.Fatal(err)
	}
	if _, err := reconcileRetentionSet(cli); err != nil {
		t.Fatal(err)
	}
	if err := cli.Get(context.Background(), key, inst); err != nil {
		t.Fatal(err)
	}
	if !ptr.Deref(inst.Spec.ScaledDown, false) || inst.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled != kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType {
		t.Fatal("changed the policy of an Instance whose deletion already began")
	}
	if err := cli.Get(context.Background(), key, pod); err != nil {
		t.Fatal(err)
	}
	pod.Finalizers = nil
	if err := cli.Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	checkRetentionDeleted(t, cli, 1)
}

func TestInstanceRetentionScaleDownAdmission(t *testing.T) {
	for _, policy := range []appsv1.PodManagementPolicyType{appsv1.OrderedReadyPodManagement, appsv1.ParallelPodManagement} {
		t.Run(string(policy), func(t *testing.T) {
			cli := retentionFixture(t, 2)
			changeRetentionSet(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.PodManagementPolicy = policy
				its.Spec.ParallelPodManagementConcurrency = ptr.To(intstr.FromInt(1))
				its.Spec.Replicas = ptr.To(int32(0))
			})
			if _, err := reconcileRetentionSet(cli); err != nil {
				t.Fatal(err)
			}
			for _, inst := range retentionInstances(t, cli) {
				if ptr.Deref(inst.Spec.ScaledDown, false) != (inst.Name == "demo-1") || !inst.DeletionTimestamp.IsZero() {
					t.Fatalf("unexpected scale-down admission for %s", inst.Name)
				}
			}
			changeRetentionSet(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled = kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType
			})
			if _, err := reconcileRetentionSet(cli); err != nil {
				t.Fatal(err)
			}
			for _, inst := range retentionInstances(t, cli) {
				if !inst.DeletionTimestamp.IsZero() {
					t.Fatal("marked Instance deleted before persisting the changed policy")
				}
				if inst.Name == "demo-1" && inst.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled != kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType {
					t.Fatal("marked Instance kept the old policy")
				}
				if inst.Name == "demo-0" && ptr.Deref(inst.Spec.ScaledDown, false) {
					t.Fatal("policy handoff exceeded the scale-down budget")
				}
			}
			checkRetentionDeleted(t, cli, 0)
		})
	}
}

type its2LifecycleClient struct {
	client.Client
	failStatus         bool
	rejectDataResult   bool
	dataResultFailures int
}

type its2LifecycleStatus struct {
	client.SubResourceWriter
	owner *its2LifecycleClient
}

func (c *its2LifecycleClient) Status() client.SubResourceWriter {
	return &its2LifecycleStatus{SubResourceWriter: c.Client.Status(), owner: c}
}

func (s *its2LifecycleStatus) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	if s.owner.rejectDataResult {
		completed := false
		switch value := obj.(type) {
		case *workloads.Instance:
			completed = ptr.Deref(value.Status.DataLoaded, false)
		case *workloads.InstanceSet:
			for _, status := range value.Status.InstanceStatus {
				completed = completed || ptr.Deref(status.DataLoaded, false)
			}
		}
		if completed {
			s.owner.dataResultFailures++
			return apierrors.NewConflict(schema.GroupResource{Group: workloads.GroupVersion.Group, Resource: "instances"}, obj.GetName(), fmt.Errorf("injected data result conflict"))
		}
	}
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

func its2LifecycleFixture(t *testing.T, replicas int32, provided ...client.Client) (*workloads.InstanceSet, *its2LifecycleClient, *InstanceSetReconciler2) {
	t.Helper()
	action := &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"true"}}}
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "lifecycle", Namespace: "default", UID: "its-uid", Generation: 1, Finalizers: []string{"instanceset.workloads.kubeblocks.io/finalizer"}}, Spec: workloads.InstanceSetSpec{EnableInstanceAPI: ptr.To(true), Replicas: ptr.To(replicas), PodManagementPolicy: appsv1.ParallelPodManagement, Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "lifecycle"}}, Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "lifecycle"}}, Spec: corev1.PodSpec{ServiceAccountName: "member-agent", Containers: []corev1.Container{{Name: "database", Image: "database:v1"}, {Name: kbagent.ContainerName, Image: "agent:v1", Ports: []corev1.ContainerPort{{Name: kbagent.DefaultHTTPPortName, ContainerPort: 3501}, {Name: kbagent.DefaultStreamingPortName, ContainerPort: 3502}}}}}}, LifecycleActions: &workloads.LifecycleActions{MemberJoin: action, MemberLeave: action}}}

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
	its, cli, r := its2LifecycleFixture(t, 2, storage)
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
		if status.MemberJoined != nil {
			t.Fatalf("registration satisfied membership before observing runtime: %#v", status)
		}
	}
	if err := cli.List(context.Background(), list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 0 {
		t.Fatal("registration round created Instance")
	}
	stored.Spec.Replicas = ptr.To[int32](3)
	stored.Generation = 2
	if err := cli.Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	r = &InstanceSetReconciler2{Client: cli, Scheme: model.GetScheme(), Recorder: record.NewFakeRecorder(100)}
	its2LifecycleRound(t, r, its)
	stored = its2LifecycleRead(t, cli, its)
	status := stored.Status.InstanceStatus[2]
	if status.MemberJoined == nil || *status.MemberJoined {
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
	its, cli, r := its2LifecycleFixture(t, 1, storage)
	its2LifecycleRound(t, r, its)
	its2LifecycleRound(t, r, its)
	its2LifecycleRound(t, r, its)
	inst := its2LifecycleInstance(t, cli, "lifecycle-0")
	inst.Status.CurrentState = workloads.InstanceCurrentStatePresent
	inst.Status.Ready = true
	inst.Status.Available = true
	if err := cli.Status().Update(context.Background(), inst); err != nil {
		t.Fatal(err)
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: inst.Name, Namespace: inst.Namespace}, Spec: inst.Spec.Template.Spec}
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
	its2LifecycleRound(t, r, its)
	its2LifecycleRound(t, r, its)
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
	its2LifecycleRound(t, r, its)
	its2LifecycleRound(t, r, its)
	stored = its2LifecycleRead(t, cli, its)
	inst = its2LifecycleInstance(t, cli, "lifecycle-0")
	if !ptr.Deref(stored.Status.InstanceStatus[0].MemberJoined, false) || ptr.Deref(inst.Spec.ScaledDown, false) {
		t.Fatalf("spec reversal did not rejoin retained member: %#v %#v", stored.Status, inst.Spec)
	}
	if len(agent.calls) != 2 || agent.calls[1] != "memberJoin" {
		t.Fatalf("unexpected member calls %v", agent.calls)
	}
}

func TestITS2LifecycleAPIServer(t *testing.T) {
	scenarios := []struct {
		name  string
		check func(*testing.T, client.Client)
	}{
		{"registration", checkITS2LifecycleRegistrationCommit},
		{"leave-reversal", checkITS2LifecycleLeaveCommitAndSpecReversal},
		{"child-result-conflict", checkITS2MembershipResultRetriesInstanceStatusConflict},
		{"data-result-conflict", checkITS2DataResultStatusConflict},
		{"data-recovery", checkITS2LifecycleDataCleanupAndPodRecreation},
		{"initial-child-result-conflict", checkITS2InitialMembershipRetriesInstanceStatusConflict},
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
			storage, err := client.NewWithWatch(cfg, client.Options{Scheme: model.GetScheme()})
			if err != nil {
				t.Fatal(err)
			}
			scenario.check(t, storage)
		})
	}
}

func TestITS2LifecycleOfflineDoesNotLeaveMember(t *testing.T) {
	its, cli, r := its2LifecycleFixture(t, 1)
	its2LifecycleRound(t, r, its)
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
	for i := 0; i < 6; i++ {
		its2LifecycleRound(t, r, its)
		stored = its2LifecycleRead(t, cli, its)
		if status := stored.FindInstanceStatus("lifecycle-0"); status != nil && status.EffectiveDesiredState() == workloads.InstanceDesiredStateOffline {
			break
		}
	}
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
	if cond := meta.FindStatusCondition(stored.Status.Conditions, "InstanceLifecycle"); cond != nil && cond.Message != "waiting for runtime of member lifecycle-1" {
		t.Fatalf("offline member was blocked on leave: %#v", cond)
	}
}

func TestITS2LifecycleAbsentBootstrapMemberStillNeedsLeave(t *testing.T) {
	its, cli, r := its2LifecycleFixture(t, 1)
	its2LifecycleRound(t, r, its)
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

func TestITS2LifecycleBootstrapInstanceStatusPrecedesLeave(t *testing.T) {
	its, cli, r := its2LifecycleFixture(t, 1)
	its2LifecycleRound(t, r, its)
	its2LifecycleRound(t, r, its)
	its2LifecycleRound(t, r, its)
	inst := its2LifecycleInstance(t, cli, "lifecycle-0")
	leaf := &InstanceReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(100)}
	for i := 0; i < 4; i++ {
		if _, err := leaf.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(inst)}); err != nil {
			t.Fatal(err)
		}
	}

	pod := &corev1.Pod{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(inst), pod); err != nil {
		t.Fatal(err)
	}
	pod.Status.PodIP = "10.0.0.1"
	if err := cli.Status().Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 4; i++ {
		if _, err := leaf.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(inst)}); err != nil {
			t.Fatal(err)
		}
	}
	if !its2LifecycleInstance(t, cli, inst.Name).Status.Provisioned {
		t.Fatal("Instance did not record its runtime")
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
	if len(agent.calls) != 0 || !stored.FindInstanceStatus(inst.Name).Provisioned || ptr.Deref(its2LifecycleInstance(t, cli, inst.Name).Spec.ScaledDown, false) {
		t.Fatalf("runtime observation was not committed before membership: calls=%v status=%#v", agent.calls, stored.Status.InstanceStatus)
	}
	its2LifecycleRound(t, r, its)
	its2LifecycleRound(t, r, its)
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

func TestITS2LifecycleAggregatesResultsByInstancePlacement(t *testing.T) {
	makeClient := func() client.Client {
		return fake.NewClientBuilder().WithScheme(model.GetScheme()).WithStatusSubresource(&workloads.InstanceSet{}, &workloads.Instance{}, &corev1.Pod{}).Build()
	}
	control, workerA, workerB := makeClient(), makeClient(), makeClient()
	multi := multicluster.NewClient(control, map[string]client.Client{"worker-a": workerA, "worker-b": workerB})
	its, cli, r := its2LifecycleFixture(t, 2, multi)
	stored := its2LifecycleRead(t, cli, its)
	stored.Annotations = map[string]string{constant.KBAppMultiClusterPlacementKey: "worker-a,worker-b"}
	if err := cli.Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		its2LifecycleRound(t, r, its)
	}
	for i, worker := range []client.Client{workerA, workerB} {
		inst := its2LifecycleInstance(t, worker, fmt.Sprintf("lifecycle-%d", i))
		inst.Status.Provisioned = true
		inst.Status.MemberJoined = ptr.To(i == 0)
		if err := worker.Status().Update(context.Background(), inst); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		its2LifecycleRound(t, r, its)
	}
	stored = its2LifecycleRead(t, cli, its)
	if !ptr.Deref(stored.FindInstanceStatus("lifecycle-0").MemberJoined, false) || ptr.Deref(stored.FindInstanceStatus("lifecycle-1").MemberJoined, false) {
		t.Fatalf("wrong aggregated results: %+v", stored.Status.InstanceStatus)
	}
}

func its2DataFixture(t *testing.T, replicas int32, provided ...client.Client) (*workloads.InstanceSet, *its2LifecycleClient, *InstanceSetReconciler2) {
	t.Helper()
	action := &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"true"}}}
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "lifecycle", Namespace: "default", UID: "its-uid", Generation: 1, Finalizers: []string{"instanceset.workloads.kubeblocks.io/finalizer"}}, Spec: workloads.InstanceSetSpec{EnableInstanceAPI: ptr.To(true), Replicas: ptr.To(replicas), PodManagementPolicy: appsv1.ParallelPodManagement, Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "lifecycle"}}, Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "lifecycle"}}, Spec: corev1.PodSpec{ServiceAccountName: "data-worker", Containers: []corev1.Container{{Name: "database", Image: "database:v1"}, {Name: kbagent.ContainerName, Image: "agent:v1", Ports: []corev1.ContainerPort{{Name: kbagent.DefaultHTTPPortName, ContainerPort: 3501}, {Name: kbagent.DefaultStreamingPortName, ContainerPort: 3502}}}}, InitContainers: []corev1.Container{{Name: kbagent.ContainerName4Worker, Image: "agent:v1", Env: []corev1.EnvVar{{Name: "KB_AGENT_POD_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.name"}}}}, Command: []string{kbagent.BinaryPath}, Args: []string{"--server=false"}}}}}, LifecycleActions: &workloads.LifecycleActions{MemberJoin: action, MemberLeave: action}}}
	its.Spec.LifecycleActions.DataDump = action
	its.Spec.LifecycleActions.DataLoad = action
	its.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}, Spec: corev1.PersistentVolumeClaimSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}}}}}
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

func its2TaskInput(inst *workloads.Instance) string {
	for _, c := range inst.Spec.Template.Spec.InitContainers {
		for _, e := range c.Env {
			if e.Name == "KB_AGENT_TASK" {
				return e.Value
			}
		}
	}
	return ""
}

func TestITS2LifecycleDataCleanupAndPodRecreation(t *testing.T) {
	checkITS2LifecycleDataCleanupAndPodRecreation(t, nil)
}

func TestITS2LifecycleOnDeleteDataCleanupAndPodRecreation(t *testing.T) {
	checkITS2LifecycleDataRecovery(t, nil, true)
}

func checkITS2LifecycleDataCleanupAndPodRecreation(t *testing.T, storage client.Client) {
	checkITS2LifecycleDataRecovery(t, storage, false)
}

func checkITS2LifecycleDataRecovery(t *testing.T, storage client.Client, onDelete bool) {
	ctx := context.Background()
	its, cli, _ := its2DataFixture(t, 1, storage)
	f := &lifecycleEntry{cli: cli, its: its, instanceAPI: true}
	f.bootstrap(t, 1)
	f.its.Spec.Replicas = ptr.To[int32](2)
	f.its.Generation++
	if err := cli.Update(ctx, f.its); err != nil {
		t.Fatal(err)
	}
	key := client.ObjectKey{Namespace: its.Namespace, Name: "lifecycle-1"}
	for i := 0; i < 12; i++ {
		f.stepWithChildren(t)
		if err := cli.Get(ctx, key, &corev1.Pod{}); err == nil {
			break
		}
	}
	target := its2LifecycleInstance(t, cli, key.Name)
	if its2TaskInput(target) == "" {
		t.Fatal("expansion has no data task")
	}
	if onDelete {
		target.Spec.InstanceUpdateStrategyType = ptr.To(kbappsv1.OnDeleteStrategyType)
		if err := cli.Update(ctx, target); err != nil {
			t.Fatal(err)
		}
	}
	before, err := instance.BuildPodRevision(target)
	if err != nil {
		t.Fatal(err)
	}
	markDataWorkerComplete(t, cli, key)
	leaf := &InstanceReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(100)}
	for i := 0; i < 4; i++ {
		if _, err := leaf.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
			t.Fatal(err)
		}
	}
	completed := its2LifecycleInstance(t, cli, key.Name)
	if !ptr.Deref(completed.Status.DataLoaded, false) {
		t.Fatal("Instance did not record completion")
	}
	if ptr.Deref(f.its.FindInstanceStatus(key.Name).DataLoaded, false) {
		t.Fatal("fixture has no aggregation delay")
	}
	cli.failStatus = true
	f.step(t)
	for i := 0; i < 4; i++ {
		f.stepWithChildren(t)
	}
	target = its2LifecycleInstance(t, cli, key.Name)
	if !ptr.Deref(f.its.FindInstanceStatus(key.Name).DataLoaded, false) || its2TaskInput(target) != "" {
		t.Fatal("child result did not aggregate and clean tasks")
	}
	after, err := instance.BuildPodRevision(target)
	if err != nil || before != after {
		t.Fatalf("cleanup changed revision: %s %s %v", before, after, err)
	}
	pod := &corev1.Pod{}
	if err := cli.Get(ctx, key, pod); err != nil {
		t.Fatal(err)
	}
	pod.Finalizers = nil
	if err := cli.Update(ctx, pod); err != nil {
		t.Fatal(err)
	}
	if err := cli.Delete(ctx, pod); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		if _, err := leaf.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
			t.Fatal(err)
		}
	}
	if err := cli.Get(ctx, key, pod); err != nil {
		t.Fatal(err)
	}
	if task, err := generatedDataTask(&pod.Spec); err != nil || task != "" {
		t.Fatalf("completed Pod regained task: %s %v", task, err)
	}
}

func TestITS2LifecycleStopDoesNotWaitForDataSource(t *testing.T) {
	its, cli, r := its2DataFixture(t, 1)
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

func TestITS2LifecycleResumesPendingExpansion(t *testing.T) {
	for _, tc := range []struct {
		name   string
		loaded bool
	}{
		{name: "data loading"},
		{name: "loaded data", loaded: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			its, cli, parent := its2DataFixture(t, 1)
			agent := &its2LifecycleAgent{}
			kbacli.SetMockClient(agent, nil)
			defer kbacli.UnsetMockClient()
			step := func() {
				t.Helper()
				its2LifecycleRound(t, parent, its)
				children := &workloads.InstanceList{}
				if err := cli.List(ctx, children); err != nil {
					t.Fatal(err)
				}
				for i := range children.Items {
					child := &children.Items[i]
					leaf := &InstanceReconciler{Client: cli, Scheme: model.GetScheme(), Recorder: record.NewFakeRecorder(100)}
					if _, err := leaf.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(child)}); err != nil {
						t.Fatal(err)
					}
				}
			}
			ready := func(name string) {
				t.Helper()
				pod := &corev1.Pod{}
				if err := cli.Get(ctx, client.ObjectKey{Namespace: its.Namespace, Name: name}, pod); err != nil {
					t.Fatal(err)
				}
				pod.Status.Phase = corev1.PodRunning
				pod.Status.PodIP = "10.0.0.1"
				pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
				for _, container := range pod.Spec.Containers {
					pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, corev1.ContainerStatus{Name: container.Name, Image: container.Image, Ready: true})
				}
				if err := cli.Status().Update(ctx, pod); err != nil {
					t.Fatal(err)
				}
			}
			for i := 0; i < 4; i++ {
				step()
			}
			ready("lifecycle-0")
			step()
			stored := its2LifecycleRead(t, cli, its)
			stored.Spec.Replicas = ptr.To[int32](2)
			stored.Generation++
			if err := cli.Update(ctx, stored); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 4; i++ {
				step()
			}
			var retainedPVC *corev1.PersistentVolumeClaim
			if tc.loaded {
				for round := 0; round < 12; round++ {
					step()
					if err := cli.Get(ctx, client.ObjectKey{Namespace: its.Namespace, Name: "lifecycle-1"}, &corev1.Pod{}); err == nil {
						break
					}
				}
				retainedPVC = &corev1.PersistentVolumeClaim{}
				if err := cli.Get(ctx, client.ObjectKey{Namespace: its.Namespace, Name: "data-lifecycle-1"}, retainedPVC); err != nil {
					t.Fatal(err)
				}
				markDataWorkerComplete(t, cli, client.ObjectKey{Namespace: its.Namespace, Name: "lifecycle-1"})
				for i := 0; i < 3; i++ {
					step()
				}
			}
			stored = its2LifecycleRead(t, cli, its)
			pending := stored.FindInstanceStatus("lifecycle-1")
			if pending == nil || pending.MemberJoined == nil || *pending.MemberJoined {
				t.Fatalf("expansion was not waiting for member join: %#v", stored.Status.InstanceStatus)
			}
			stored.Spec.Stop = ptr.To(true)
			stored.Generation++
			if err := cli.Update(ctx, stored); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 5; i++ {
				step()
			}
			children := &workloads.InstanceList{}
			pods := &corev1.PodList{}
			if err := cli.List(ctx, children); err != nil {
				t.Fatal(err)
			}
			if err := cli.List(ctx, pods); err != nil {
				t.Fatal(err)
			}
			if len(children.Items) != 2 || len(pods.Items) != 0 || len(agent.calls) != 0 {
				t.Fatalf("stop did not retain instances without member actions: %#v %#v %v", children.Items, pods.Items, agent.calls)
			}
			for _, child := range children.Items {
				if !ptr.Deref(child.Spec.Stop, false) || child.Status.CurrentState != workloads.InstanceCurrentStateAbsent || child.Status.ObservedGeneration != child.Generation {
					t.Fatalf("child did not acknowledge stop: %#v", child)
				}
			}
			stored = its2LifecycleRead(t, cli, its)
			stored.Spec.Stop = ptr.To(false)
			stored.Generation++
			if err := cli.Update(ctx, stored); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 4; i++ {
				step()
			}
			source := its2LifecycleInstance(t, cli, "lifecycle-0")
			if ptr.Deref(source.Spec.Stop, false) {
				t.Fatal("pending lifecycle action blocked source resume")
			}
			ready(source.Name)
			for i := 0; i < 4; i++ {
				step()
			}
			target := its2LifecycleInstance(t, cli, "lifecycle-1")
			if ptr.Deref(target.Spec.Stop, false) {
				t.Fatal("pending lifecycle action blocked target resume")
			}
			task := its2TaskInput(target)
			if (!tc.loaded && task == "") || (tc.loaded && task != "") {
				t.Fatalf("resume lost the current data inputs: task=%s", task)
			}
			if retainedPVC != nil {
				current := &corev1.PersistentVolumeClaim{}
				if err := cli.Get(ctx, client.ObjectKeyFromObject(retainedPVC), current); err != nil || current.UID != retainedPVC.UID {
					t.Fatalf("resume changed completed storage: %#v %v", current, err)
				}
			}
			if tc.loaded {
				ready(target.Name)
				for i := 0; i < 3; i++ {
					step()
				}
				stored = its2LifecycleRead(t, cli, its)
				status := stored.FindInstanceStatus(target.Name)
				if status == nil || !ptr.Deref(status.MemberJoined, false) || len(agent.calls) != 1 || agent.calls[0] != "memberJoin" {
					t.Fatalf("resumed expansion did not complete join: %#v %v", status, agent.calls)
				}
			} else if len(agent.calls) != 0 {
				t.Fatalf("incomplete loading allowed member actions: %v", agent.calls)
			}
		})
	}
}

func TestITS2DataResultStatusConflict(t *testing.T) {
	checkITS2DataResultStatusConflict(t, nil)
}

func checkITS2DataResultStatusConflict(t *testing.T, storage client.Client) {
	ctx := context.Background()
	its, cli, _ := its2DataFixture(t, 1, storage)
	f := &lifecycleEntry{cli: cli, its: its, instanceAPI: true}
	f.bootstrap(t, 1)
	f.its.Spec.Replicas = ptr.To[int32](2)
	f.its.Generation++
	if err := cli.Update(ctx, f.its); err != nil {
		t.Fatal(err)
	}
	key := client.ObjectKey{Namespace: its.Namespace, Name: its.Name + "-1"}
	for i := 0; i < 12; i++ {
		f.stepWithChildren(t)
		if err := cli.Get(ctx, key, &corev1.Pod{}); err == nil {
			break
		}
	}
	childInput := its2LifecycleInstance(t, cli, key.Name)
	generated, err := workloadlifecycle.DataTask(&childInput.Spec.Template.Spec, childInput.Name)
	if err != nil || generated == nil {
		t.Fatalf("missing applied task: %v", err)
	}
	markDataWorkerComplete(t, cli, key)
	conflicted := false
	faulty := &dataCommitClient{Client: cli, update: func(ctx context.Context, current client.Client, sub string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
		if inst, ok := obj.(*workloads.Instance); ok && sub == "status" && ptr.Deref(inst.Status.DataLoaded, false) && !conflicted {
			fresh := &workloads.Instance{}
			if err := current.Get(ctx, key, fresh); err != nil {
				return err
			}
			if fresh.Annotations == nil {
				fresh.Annotations = map[string]string{}
			}
			fresh.Annotations["concurrent-writer"] = "retained"
			if err := current.Update(ctx, fresh); err != nil {
				return err
			}
			err := current.SubResource(sub).Update(ctx, obj, opts...)
			if !apierrors.IsConflict(err) {
				t.Fatalf("expected real resource-version conflict, got %v", err)
			}
			conflicted = true
			return err
		}
		return current.SubResource(sub).Update(ctx, obj, opts...)
	}}
	leaf := &InstanceReconciler{Client: faulty, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(100)}
	result, err := leaf.Reconcile(ctx, ctrl.Request{NamespacedName: key})
	if err != nil || !result.Requeue || !conflicted {
		t.Fatalf("conflict result=%#v err=%v", result, err)
	}
	child := its2LifecycleInstance(t, cli, key.Name)
	if ptr.Deref(child.Status.DataLoaded, false) || its2TaskInput(child) == "" {
		t.Fatal("failed result commit cleaned applied input")
	}
	pod := &corev1.Pod{}
	if err := cli.Get(ctx, key, pod); err != nil {
		t.Fatal(err)
	}
	if task, err := generatedDataTask(&pod.Spec); err != nil || task == "" {
		t.Fatalf("failed result commit lost actual evidence: %s %v", task, err)
	}
	if _, err := leaf.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
		t.Fatal(err)
	}
	child = its2LifecycleInstance(t, cli, key.Name)
	if !ptr.Deref(child.Status.DataLoaded, false) || child.Annotations["concurrent-writer"] != "retained" {
		t.Fatal("retry lost result or concurrent metadata")
	}
	blocked := &dataCommitClient{Client: cli, update: func(ctx context.Context, current client.Client, sub string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
		if _, ok := obj.(*workloads.InstanceSet); ok && sub == "status" {
			return apierrors.NewConflict(workloads.Resource("instancesets"), obj.GetName(), fmt.Errorf("aggregation unavailable"))
		}
		return current.SubResource(sub).Update(ctx, obj, opts...)
	}}
	parent := &InstanceSetReconciler2{Client: blocked, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(100)}
	if _, err := parent.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(its)}); err != nil {
		t.Fatal(err)
	}
	if ptr.Deref(its2LifecycleRead(t, cli, its).FindInstanceStatus(key.Name).DataLoaded, false) {
		t.Fatal("fixture did not retain aggregation lag")
	}
	child.Spec.Stop = ptr.To(true)
	if err := cli.Update(ctx, child); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		if _, err := leaf.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
			t.Fatal(err)
		}
	}
	if err := cli.Get(ctx, key, pod); !apierrors.IsNotFound(err) {
		t.Fatalf("Stop did not remove runtime: %v", err)
	}
	child = its2LifecycleInstance(t, cli, key.Name)
	if its2TaskInput(child) == "" {
		t.Fatal("fixture lost stale parent input")
	}
	child.Spec.Stop = ptr.To(false)
	if err := cli.Update(ctx, child); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		if _, err := leaf.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
			t.Fatal(err)
		}
	}
	if err := cli.Get(ctx, key, pod); err != nil {
		t.Fatal(err)
	}
	if task, err := generatedDataTask(&pod.Spec); err != nil || task != "" {
		t.Fatalf("committed child reloaded stale parent input: %s %v", task, err)
	}
	if !ptr.Deref(its2LifecycleInstance(t, cli, key.Name).Status.DataLoaded, false) {
		t.Fatal("rebuild erased committed result")
	}
	for i := 0; i < 5; i++ {
		f.stepWithChildren(t)
	}
	if !ptr.Deref(f.its.FindInstanceStatus(key.Name).DataLoaded, false) || its2TaskInput(its2LifecycleInstance(t, cli, key.Name)) != "" {
		t.Fatal("parent retry did not aggregate and clean completed input")
	}
}

type dataCommitClient struct {
	client.Client
	update func(context.Context, client.Client, string, client.Object, ...client.SubResourceUpdateOption) error
}

func (c *dataCommitClient) Status() client.SubResourceWriter {
	return &dataCommitWriter{SubResourceWriter: c.Client.Status(), owner: c}
}

type dataCommitWriter struct {
	client.SubResourceWriter
	owner *dataCommitClient
}

func (w *dataCommitWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	return w.owner.update(ctx, w.owner.Client, "status", obj, opts...)
}

func TestITS2LifecycleStopPublishesTargetBeforeChildObservation(t *testing.T) {
	f := newMemberLifecycleEntry(t, true, 1, appsv1.ParallelPodManagement)
	for round := 0; round < 12 && len(lifecycleEntryPods(t, f.cli)) == 0; round++ {
		f.stepWithChildren(t)
	}
	pods := lifecycleEntryPods(t, f.cli)
	if len(pods) != 1 {
		t.Fatal("initial runtime was not created")
	}
	child := its2LifecycleInstance(t, f.cli, pods[0].Name)
	if child.Status.CurrentState != workloads.InstanceCurrentStateAbsent {
		t.Fatalf("fixture already observed runtime: %#v", child.Status)
	}
	f.its.Spec.Stop = ptr.To(true)
	f.its.Generation++
	if err := f.cli.Update(context.Background(), f.its); err != nil {
		t.Fatal(err)
	}
	for round := 0; round < 6; round++ {
		f.step(t)
		status := f.its.FindInstanceStatus(child.Name)
		if status != nil && status.EffectiveDesiredState() == workloads.InstanceDesiredStateOffline {
			if ptr.Deref(its2LifecycleInstance(t, f.cli, child.Name).Spec.Stop, false) {
				t.Fatal("child stopped in target commit round")
			}
			if len(lifecycleEntryPods(t, f.cli)) != 1 {
				t.Fatal("target commit removed runtime")
			}
			return
		}
	}
	t.Fatalf("unobserved child prevented Stop target publication: %#v", f.its.Status)
}

func TestITS2LifecycleRecoversWithoutChildDataResultNotification(t *testing.T) {
	for _, retain := range []bool{false, true} {
		t.Run(fmt.Sprintf("retain=%t", retain), func(t *testing.T) {
			ctx := context.Background()
			f := newDataLifecycleEntry(t, true, 2, appsv1.ParallelPodManagement)
			if retain {
				f.its.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{WhenScaled: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType}
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
			}
			f.bootstrap(t, 2)
			name := f.its.Name + "-0"
			key := client.ObjectKey{Namespace: f.its.Namespace, Name: name}
			oldClaims := &corev1.PersistentVolumeClaimList{}
			if err := f.cli.List(ctx, oldClaims, client.MatchingLabels{constant.KBAppPodNameLabelKey: name}); err != nil {
				t.Fatal(err)
			}
			if len(oldClaims.Items) != 1 || len(oldClaims.Items[0].OwnerReferences) == 0 {
				t.Fatal("fixture has no child-owned claim")
			}
			f.its.Spec.Replicas = ptr.To[int32](1)
			f.its.Spec.OfflineInstances = []string{name}
			f.its.Generation++
			if err := f.cli.Update(ctx, f.its); err != nil {
				t.Fatal(err)
			}
			for round := 0; round < 12; round++ {
				f.step(t)
				if !its2LifecycleInstance(t, f.cli, name).DeletionTimestamp.IsZero() {
					break
				}
			}
			if its2LifecycleInstance(t, f.cli, name).DeletionTimestamp.IsZero() {
				t.Fatal("Offline did not request child deletion")
			}
			leaf := &InstanceReconciler{Client: f.cli, Scheme: f.cli.Scheme(), Recorder: record.NewFakeRecorder(100)}
			for round := 0; round < 16; round++ {
				if _, err := leaf.Reconcile(ctx, ctrl.Request{NamespacedName: key}); err != nil {
					t.Fatal(err)
				}
				if apierrors.IsNotFound(f.cli.Get(ctx, key, &workloads.Instance{})) {
					break
				}
			}
			if !apierrors.IsNotFound(f.cli.Get(ctx, key, &workloads.Instance{})) || !ptr.Deref(f.its.FindInstanceStatus(name).DataLoaded, false) {
				t.Fatal("fixture did not skip child result notification")
			}
			storage := f.cli.(*its2LifecycleClient).Client.(client.WithWatch)
			stale := true
			f.cli = interceptor.NewClient(storage, interceptor.Funcs{List: func(ctx context.Context, delegate client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				if err := delegate.List(ctx, list, opts...); err != nil {
					return err
				}
				if claims, ok := list.(*corev1.PersistentVolumeClaimList); ok && stale {
					options := (&client.ListOptions{}).ApplyOptions(opts)
					if options.LabelSelector != nil && options.LabelSelector.Matches(labels.Set(oldClaims.Items[0].Labels)) {
						claims.Items = append([]corev1.PersistentVolumeClaim(nil), oldClaims.Items...)
					}
				}
				return nil
			}})
			f.its.Spec.Replicas = ptr.To[int32](2)
			f.its.Spec.OfflineInstances = nil
			f.its.Generation++
			if err := f.cli.Update(ctx, f.its); err != nil {
				t.Fatal(err)
			}
			for round := 0; round < 6; round++ {
				f.step(t)
			}
			status := f.its.FindInstanceStatus(name)
			if status == nil || status.EffectiveDesiredState() != workloads.InstanceDesiredStateActive || !apierrors.IsNotFound(f.cli.Get(ctx, key, &workloads.Instance{})) {
				t.Fatalf("old owner observation was reused or blocked target: %#v", status)
			}
			stale = false
			for round := 0; round < 16; round++ {
				f.stepWithChildren(t)
				if err := f.cli.Get(ctx, key, &corev1.Pod{}); err == nil {
					break
				}
			}
			pod := &corev1.Pod{}
			if err := f.cli.Get(ctx, key, pod); err != nil {
				t.Fatal(err)
			}
			task, err := workloadlifecycle.DataTask(&pod.Spec, name)
			status = f.its.FindInstanceStatus(name)
			if err != nil || status == nil || ptr.Deref(status.DataLoaded, false) != retain || (task == nil) != retain {
				t.Fatalf("skipped owner notification changed data reuse: status=%#v task=%#v err=%v", status, task, err)
			}
		})
	}
}
