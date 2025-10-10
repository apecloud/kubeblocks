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

package workloads

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
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
			AddAnnotations(constant.CRDAPIVersionAnnotationKey, workloads.GroupVersion.String()).
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
						Replicas: &intstr.IntOrString{
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
