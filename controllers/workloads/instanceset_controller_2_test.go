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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	kbagentproto "github.com/apecloud/kubeblocks/pkg/kbagent/proto"
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
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.IsInstanceSetReady()).Should(BeTrue())
			})).Should(Succeed())

			beforeUpdate := time.Now()
			time.Sleep(1 * time.Second)
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
			})()).ShouldNot(HaveOccurred())

			updatedInstanceKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      podName(replicas - 1),
			}
			Eventually(testapps.CheckObj(&testCtx, updatedInstanceKey, func(g Gomega, inst *workloads.Instance) {
				g.Expect(inst.Spec.Template.Spec.DNSPolicy).Should(Equal(corev1.DNSClusterFirstWithHostNet))
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, updatedInstanceKey, func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.CreationTimestamp.After(beforeUpdate)).Should(BeTrue())
			})).Should(Succeed())
			mockPodReady(itsObj.Namespace, updatedInstanceKey.Name)
			Eventually(testapps.CheckObj(&testCtx, updatedInstanceKey, func(g Gomega, inst *workloads.Instance) {
				g.Expect(intctrlutil.IsInstanceReady(inst)).Should(BeTrue())
			})).Should(Succeed())

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
					{Name: "follower", UpdatePriority: 1, ParticipatesInQuorum: true},
					{Name: "leader", UpdatePriority: 2, ParticipatesInQuorum: true},
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
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.IsInstanceSetReady()).Should(BeTrue())
			})).Should(Succeed())

			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
			})()).ShouldNot(HaveOccurred())

			followerKey := types.NamespacedName{Namespace: itsObj.Namespace, Name: podName(0)}
			Eventually(testapps.CheckObj(&testCtx, followerKey, func(g Gomega, inst *workloads.Instance) {
				g.Expect(inst.Spec.Template.Spec.DNSPolicy).Should(Equal(corev1.DNSClusterFirstWithHostNet))
			})).Should(Succeed())

			Consistently(func(g Gomega) {
				for i := int32(1); i < replicas; i++ {
					inst := &workloads.Instance{}
					key := types.NamespacedName{Namespace: itsObj.Namespace, Name: podName(i)}
					g.Expect(testCtx.Cli.Get(testCtx.Ctx, key, inst)).Should(Succeed())
					g.Expect(inst.Spec.Template.Spec.DNSPolicy).ShouldNot(Equal(corev1.DNSClusterFirstWithHostNet))
				}
			}).Should(Succeed())
		})

		It("reconfigure", func() {
			var (
				reconfigure string
				parameters  map[string]string
			)

			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					if strings.ToLower(req.Action) == "reconfigure" {
						reconfigure = req.Action
						parameters = req.Parameters
					}
					return kbagentproto.ActionResponse{}, nil
				}).AnyTimes()
			})
			defer kbacli.UnsetMockClient()

			oldReplicas := replicas
			replicas = 1
			defer func() {
				replicas = oldReplicas
			}()

			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.SetInstanceUpdateStrategy(&workloads.InstanceUpdateStrategy{
					Type: kbappsv1.RollingUpdateStrategyType,
				}).AddConfigs(
					workloads.ConfigTemplate{
						Name:       "server",
						Generation: int64(1),
					},
					workloads.ConfigTemplate{
						Name:       "logging",
						Generation: int64(2),
					},
				)
			})

			mockPodsReady()

			By("check the init instance status")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(its.Status.InstanceStatus[0].Configs).Should(Equal([]workloads.InstanceConfigStatus{
					{Name: "server", Generation: int64(1)},
					{Name: "logging", Generation: int64(2)},
				}))
			})).Should(Succeed())

			instKey := types.NamespacedName{Namespace: itsObj.Namespace, Name: podName(0)}
			By("check the instance spec synced")
			Eventually(testapps.CheckObj(&testCtx, instKey, func(g Gomega, inst *workloads.Instance) {
				g.Expect(inst.Spec.Configs).Should(HaveLen(2))
				g.Expect(inst.Spec.Configs[1].Generation).Should(BeEquivalentTo(2))
			})).Should(Succeed())

			By("update configs")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Configs[1].Generation = 128
				its.Spec.Configs[1].Reconfigure = testapps.NewLifecycleAction("reconfigure")
				its.Spec.Configs[1].ReconfigureActionName = ""
				its.Spec.Configs[1].Parameters = map[string]string{"foo": "bar"}
			})()).ShouldNot(HaveOccurred())

			By("check the instance spec updated")
			Eventually(testapps.CheckObj(&testCtx, instKey, func(g Gomega, inst *workloads.Instance) {
				g.Expect(inst.Spec.Configs).Should(HaveLen(2))
				g.Expect(inst.Spec.Configs[1].Generation).Should(BeEquivalentTo(128))
				g.Expect(inst.Status.Configs).Should(Equal([]workloads.InstanceConfigStatus{
					{Name: "server", Generation: int64(1)},
					{Name: "logging", Generation: int64(128)},
				}))
			})).Should(Succeed())

			By("check the its status updated")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(its.Status.InstanceStatus[0].Configs).Should(Equal([]workloads.InstanceConfigStatus{
					{Name: "server", Generation: int64(1)},
					{Name: "logging", Generation: int64(128)},
				}))
			})).Should(Succeed())

			By("check the reconfigure action call")
			Eventually(func(g Gomega) {
				g.Expect(strings.ToLower(reconfigure)).Should(Equal("reconfigure"))
				g.Expect(parameters).Should(HaveKeyWithValue("foo", "bar"))
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

func retentionFixture(t *testing.T, replicas int32, configure ...func(*workloads.InstanceSet)) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{clientgoscheme.AddToScheme, workloads.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "set-uid", Generation: 1,
		Annotations: map[string]string{constant.CRDAPIVersionAnnotationKey: workloads.GroupVersion.String()}}, Spec: workloads.InstanceSetSpec{
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
	writeFailure := errors.New("injected scale reversal update failure")
	conflict := apierrors.NewConflict(schema.GroupResource{Group: workloads.GroupVersion.Group, Resource: "instances"}, "demo", errors.New("concurrent update"))
	for _, scenario := range []struct {
		name              string
		retryBeforeDelete bool
		retainOnDelete    bool
		flatOrdinal       bool
		failure           error
	}{
		{name: "RetryBeforeDeletion", retryBeforeDelete: true, failure: writeFailure},
		{name: "DeleteAfterFailure", failure: writeFailure},
		{name: "RetainAfterFailure", retainOnDelete: true, failure: writeFailure},
		{name: "DeleteAfterConflict", failure: conflict},
		{name: "RetainAfterConflict", retainOnDelete: true, failure: conflict},
		{name: "FlatOrdinalRetainAfterFailure", retainOnDelete: true, flatOrdinal: true, failure: writeFailure},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			cli := retentionFixture(t, 4, func(its *workloads.InstanceSet) {
				its.Spec.FlatInstanceOrdinal = scenario.flatOrdinal
				if scenario.retainOnDelete {
					its.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled = kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType
					its.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted = kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType
				}
			})
			changeRetentionSet(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(2)) })
			if _, err := reconcileRetentionSet(cli); err != nil {
				t.Fatal(err)
			}
			changeRetentionSet(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(4)) })
			result, err := reconcileRetentionSet(&retentionWriteFailureClient{Client: cli, failure: scenario.failure})
			if apierrors.IsConflict(scenario.failure) {
				if err != nil || !result.Requeue {
					t.Fatalf("conflict must retry, result=%#v err=%v", result, err)
				}
			} else if !errors.Is(err, scenario.failure) {
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
			if scenario.retryBeforeDelete {
				runRetentionControllers(t, cli, 3)
				for _, inst := range retentionInstances(t, cli) {
					if ptr.Deref(inst.Spec.ScaledDown, false) {
						t.Fatal("scale reversal retry left spec.scaledDown=true")
					}
				}
			}
			if err := cli.Delete(context.Background(), retentionSet(t, cli)); err != nil {
				t.Fatal(err)
			}
			wantClaims := 0
			if scenario.retainOnDelete {
				wantClaims = 4
			}
			checkRetentionDeleted(t, cli, wantClaims)
		})
	}
}

func TestInstanceRetentionParentDeletionDuringScaleDown(t *testing.T) {
	for _, retainOnDelete := range []bool{false, true} {
		t.Run(fmt.Sprintf("RetainOnDelete=%t", retainOnDelete), func(t *testing.T) {
			cli := retentionFixture(t, 4, func(its *workloads.InstanceSet) {
				if retainOnDelete {
					its.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled = kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType
					its.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted = kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType
				}
			})
			changeRetentionSet(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(2)) })
			if _, err := reconcileRetentionSet(cli); err != nil {
				t.Fatal(err)
			}
			if err := cli.Delete(context.Background(), retentionSet(t, cli)); err != nil {
				t.Fatal(err)
			}
			checkRetentionDeleted(t, cli, 2)
		})
	}
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
