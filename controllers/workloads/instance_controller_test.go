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
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	kbagentproto "github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("Instance Controller", func() {
	var (
		clusterName     = "test-cluster"
		itsName         = "test-cluster-inst"
		instName        = "test-cluster-inst-0"
		instObj         *workloads.Instance
		instKey         client.ObjectKey
		minReadySeconds int32 = 15
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
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.InstanceSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PodSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS, ml)
	}

	BeforeEach(func() {
		cleanEnv()
	})

	AfterEach(func() {
		cleanEnv()
	})

	createInstObj := func(name string, processors ...func(factory *testapps.MockInstanceFactory)) {
		By("create the instance object")
		f := testapps.NewInstanceFactory(testCtx.DefaultNamespace, name).
			WithRandomName().
			AddLabelsInMap(map[string]string{
				constant.AppManagedByLabelKey: constant.AppName,
				constant.AppInstanceLabelKey:  clusterName,
			}).
			AddContainer(corev1.Container{
				Name:  "foo",
				Image: "bar:v1",
			}).
			SetInstanceSetName(itsName)
		for _, processor := range processors {
			if processor != nil {
				processor(f)
			}
		}
		instObj = f.Create(&testCtx).GetObject()
		instKey = client.ObjectKeyFromObject(instObj)

		Eventually(testapps.CheckObj(&testCtx, instKey, func(g Gomega, inst *workloads.Instance) {
			g.Expect(inst.Status.ObservedGeneration).Should(BeEquivalentTo(1))
		})).Should(Succeed())
	}

	Context("provision", func() {
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

		It("create & delete", func() {
			createInstObj(instName, nil)

			Expect(k8sClient.Delete(ctx, instObj)).Should(Succeed())
			Eventually(testapps.CheckObjExists(&testCtx, instKey, &workloads.Instance{}, false)).Should(Succeed())
		})

		It("status", func() {
			createInstObj(instName, nil)

			mockPodReady(instObj.Namespace, instObj.Name)

			Eventually(testapps.CheckObj(&testCtx, instKey, func(g Gomega, inst *workloads.Instance) {
				g.Expect(inst.Status.UpToDate).Should(BeTrue())
				g.Expect(inst.Status.Ready).Should(BeTrue())
				g.Expect(inst.Status.Available).Should(BeTrue())
			})).Should(Succeed())
		})

		It("status - available & role", func() {
			createInstObj(instName, func(f *testapps.MockInstanceFactory) {
				f.SetMinReadySeconds(minReadySeconds).
					SetRoles([]workloads.ReplicaRole{
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

			mockPodReady(instObj.Namespace, instObj.Name)

			Eventually(testapps.CheckObj(&testCtx, instKey, func(g Gomega, inst *workloads.Instance) {
				g.Expect(inst.Status.UpToDate).Should(BeTrue())
				g.Expect(inst.Status.Ready).Should(BeTrue())
				g.Expect(inst.Status.Available).Should(BeFalse())
				g.Expect(inst.Status.Role).Should(BeEmpty())
			})).Should(Succeed())

			mockPodReadyNAvailable(instObj.Namespace, instObj.Name, minReadySeconds)

			Eventually(testapps.CheckObj(&testCtx, instKey, func(g Gomega, inst *workloads.Instance) {
				g.Expect(inst.Status.UpToDate).Should(BeTrue())
				g.Expect(inst.Status.Ready).Should(BeTrue())
				g.Expect(inst.Status.Available).Should(BeTrue())
				g.Expect(inst.Status.Role).Should(BeEmpty())
			})).Should(Succeed())

			mockPodReadyNAvailableWithRole(instObj.Namespace, instObj.Name, "leader", minReadySeconds)

			Eventually(testapps.CheckObj(&testCtx, instKey, func(g Gomega, inst *workloads.Instance) {
				g.Expect(inst.Status.UpToDate).Should(BeTrue())
				g.Expect(inst.Status.Ready).Should(BeTrue())
				g.Expect(inst.Status.Available).Should(BeTrue())
				g.Expect(inst.Status.Role).Should(Equal("leader"))
			})).Should(Succeed())
		})

		It("delete - delete pvc", func() {
			createInstObj(instName, func(f *testapps.MockInstanceFactory) {
				f.AddVolumeClaimTemplate(pvc).
					SetPVCRetentionPolicy(&workloads.PersistentVolumeClaimRetentionPolicy{
						WhenDeleted: kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType,
					})
			})

			By("delete the instance object")
			Expect(k8sClient.Delete(ctx, instObj)).Should(Succeed())

			By("check the instance object NOT deleted")
			Consistently(testapps.CheckObjExists(&testCtx, instKey, &workloads.Instance{}, true)).Should(Succeed())

			By("check pods deleted")
			podKey := types.NamespacedName{
				Namespace: instObj.Namespace,
				Name:      instObj.Name,
			}
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())

			By("check PVCs deleted, but the pvc-protection finalizer prevent the pvc to be deleted physically")
			pvcKey := types.NamespacedName{
				Namespace: instObj.Namespace,
				Name:      fmt.Sprintf("%s-%s", pvc.Name, instObj.Name),
			}
			Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, pvc *corev1.PersistentVolumeClaim) {
				g.Expect(pvc.DeletionTimestamp).ShouldNot(BeNil())
				g.Expect(pvc.Finalizers).To(HaveLen(1))
				g.Expect(pvc.Finalizers[0]).To(Equal("kubernetes.io/pvc-protection"))
			})).Should(Succeed())
		})

		It("delete - retain pvc", func() {
			createInstObj(instName, func(f *testapps.MockInstanceFactory) {
				f.AddVolumeClaimTemplate(pvc).
					SetPVCRetentionPolicy(&workloads.PersistentVolumeClaimRetentionPolicy{
						WhenDeleted: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType,
					})
			})

			By("delete the instance object")
			Expect(k8sClient.Delete(ctx, instObj)).Should(Succeed())
			Eventually(testapps.CheckObjExists(&testCtx, instKey, &workloads.Instance{}, false)).Should(Succeed())

			By("check pods deleted")
			podKey := types.NamespacedName{
				Namespace: instObj.Namespace,
				Name:      instObj.Name,
			}
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())

			By("check PVCs retained and not deleted")
			pvcKey := types.NamespacedName{
				Namespace: instObj.Namespace,
				Name:      fmt.Sprintf("%s-%s", pvc.Name, instObj.Name),
			}
			Consistently(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, pvc *corev1.PersistentVolumeClaim) {
				g.Expect(pvc.DeletionTimestamp).Should(BeNil())
			})).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, pvc *corev1.PersistentVolumeClaim) {
				// verify owner references are cleared to prevent garbage collection
				ownerRefs := pvc.GetOwnerReferences()
				g.Expect(ownerRefs).Should(HaveLen(0), "Owner references should be cleared when retention policy is Retain")
			})).Should(Succeed())
		})
	})

	Context("update", func() {
		var (
			supportResizeSubResource func() (bool, error)
		)

		BeforeEach(func() {
			supportResizeSubResource = intctrlutil.SupportResizeSubResource
			intctrlutil.SupportResizeSubResource = func() (bool, error) { return true, nil }
		})

		AfterEach(func() {
			intctrlutil.SupportResizeSubResource = supportResizeSubResource
		})

		It("update", func() {
			createInstObj(instName, nil)

			mockPodReady(instObj.Namespace, instObj.Name)

			By("update the pod spec")
			Expect(testapps.GetAndChangeObj(&testCtx, instKey, func(inst *workloads.Instance) {
				inst.Spec.Template.Spec.Containers[0].Image = "bar:v2"
			})()).Should(Succeed())

			By("check the pod is updated")
			podKey := instKey
			Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.Spec.Containers[0].Image).Should(Equal("bar:v2"))
			})).Should(Succeed())
		})

		It("update strategy type - on-delete", func() {
			createInstObj(instName, func(f *testapps.MockInstanceFactory) {
				f.SetInstanceUpdateStrategyType(ptr.To(kbappsv1.OnDeleteStrategyType))
			})

			mockPodReady(instObj.Namespace, instObj.Name)

			By("update the pod spec")
			Expect(testapps.GetAndChangeObj(&testCtx, instKey, func(inst *workloads.Instance) {
				inst.Spec.Template.Spec.Containers[0].Image = "bar:v2"
			})()).Should(Succeed())

			By("check the pod is not updated")
			podKey := instKey
			Consistently(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.Spec.Containers[0].Image).Should(Equal("bar:v1"))
			})).Should(Succeed())

			By("delete pod")
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: podKey.Namespace,
					Name:      podKey.Name,
				},
			}
			Expect(k8sClient.Delete(ctx, pod)).Should(Succeed())
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())

			By("check the pod recreated with new spec")
			Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.Spec.Containers[0].Image).Should(Equal("bar:v2"))
			})).Should(Succeed())
		})

		It("update pending pod", func() {
			createInstObj(instName, func(f *testapps.MockInstanceFactory) {
				f.AddVolume(corev1.Volume{
					Name: "vol-not-exist",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "vol-not-exist",
						},
					},
				})
			})

			By("check the pod is pending")
			podKey := instKey
			Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.Status.Phase).Should(Equal(corev1.PodPending))
			})).Should(Succeed())
			podObj := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, podKey, podObj)).Should(Succeed())

			By("update the pod spec")
			Expect(testapps.GetAndChangeObj(&testCtx, instKey, func(inst *workloads.Instance) {
				inst.Spec.Template.Spec.Containers[0].Image = "bar:v2"
			})()).Should(Succeed())

			By("check the pod is recreated")
			Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.UID).ShouldNot(Equal(podObj.UID)) // recreated
				g.Expect(pod.Spec.Containers[0].Image).Should(Equal("bar:v2"))
			})).Should(Succeed())
		})

		It("can be updated", func() {
			createInstObj(instName, func(f *testapps.MockInstanceFactory) {
				f.SetMinReadySeconds(minReadySeconds).
					SetRoles([]workloads.ReplicaRole{
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

			mockPodReadyNAvailable(instObj.Namespace, instObj.Name, minReadySeconds)

			By("update the pod spec")
			Expect(testapps.GetAndChangeObj(&testCtx, instKey, func(inst *workloads.Instance) {
				inst.Spec.Template.Spec.Containers[0].Image = "bar:v2"
			})()).Should(Succeed())

			By("check the pod is not updated") // blocked by pod which is not have role label
			podKey := instKey
			Consistently(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.Spec.Containers[0].Image).Should(Equal("bar:v1"))
			})).Should(Succeed())

			mockPodReadyNAvailableWithRole(instObj.Namespace, instObj.Name, "leader", minReadySeconds)

			By("check the pod is updated")
			Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.Spec.Containers[0].Image).Should(Equal("bar:v2"))
			})).Should(Succeed())
		})

		It("blocked - strict in-place vs recreate", func() {
			createInstObj(instName, func(f *testapps.MockInstanceFactory) {
				f.SetPodUpdatePolicy(kbappsv1.StrictInPlacePodUpdatePolicyType)
			})

			mockPodReady(instObj.Namespace, instObj.Name)

			By("update the pod spec")
			Expect(testapps.GetAndChangeObj(&testCtx, instKey, func(inst *workloads.Instance) {
				inst.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet // re-create
			})()).Should(Succeed())

			By("check the pod is not updated")
			podKey := instKey
			Consistently(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.Spec.DNSPolicy).Should(Equal(corev1.DNSClusterFirst))
			})).Should(Succeed())

			By("check the instance status")
			Eventually(testapps.CheckObj(&testCtx, instKey, func(g Gomega, inst *workloads.Instance) {
				g.Expect(inst.Status.Conditions).To(HaveLen(1))
				g.Expect(inst.Status.Conditions[0].Type).Should(Equal(workloads.InstanceUpdateRestricted))
				g.Expect(inst.Status.Conditions[0].Status).Should(Equal(corev1.ConditionTrue))
			}))
		})

		It("in-place", func() {
			createInstObj(instName, nil)

			mockPodReady(instObj.Namespace, instObj.Name)
			podKey := instKey
			podObj := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, podKey, podObj)).Should(Succeed())

			By("update the pod spec")
			Expect(testapps.GetAndChangeObj(&testCtx, instKey, func(inst *workloads.Instance) {
				inst.Spec.Template.Spec.Containers[0].Image = "bar:v2"
			})()).Should(Succeed())

			By("check the pod is updated")
			Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.UID).Should(Equal(podObj.UID)) // in-place update
				g.Expect(pod.Spec.Containers[0].Image).Should(Equal("bar:v2"))
			})).Should(Succeed())
		})

		It("recreate", func() {
			createInstObj(instName, nil)

			mockPodReady(instObj.Namespace, instObj.Name)
			podKey := instKey
			podObj := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, podKey, podObj)).Should(Succeed())

			By("update the pod spec")
			Expect(testapps.GetAndChangeObj(&testCtx, instKey, func(inst *workloads.Instance) {
				inst.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet // re-create
			})()).Should(Succeed())

			By("check the pod is updated")
			Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.UID).ShouldNot(Equal(podObj.UID)) // recreated
				g.Expect(pod.Spec.DNSPolicy).Should(Equal(corev1.DNSClusterFirstWithHostNet))
			})).Should(Succeed())
		})

		It("switchover", func() {
			var (
				switchover = false
			)

			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					if strings.ToLower(req.Action) == "switchover" {
						switchover = true
					}
					return kbagentproto.ActionResponse{}, nil
				}).AnyTimes()
			})
			defer kbacli.UnsetMockClient()

			createInstObj(instName, func(f *testapps.MockInstanceFactory) {
				f.SetLifecycleActions(&workloads.LifecycleActions{
					Switchover: &kbappsv1.Action{
						Exec: &kbappsv1.ExecAction{},
					},
				})
			})

			mockPodReady(instObj.Namespace, instObj.Name)
			podKey := instKey
			podObj := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, podKey, podObj)).Should(Succeed())

			By("update the pod spec")
			Expect(testapps.GetAndChangeObj(&testCtx, instKey, func(inst *workloads.Instance) {
				inst.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet // re-create
			})()).Should(Succeed())

			By("check the pod is updated")
			Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.UID).ShouldNot(Equal(podObj.UID)) // recreated
				g.Expect(pod.Spec.DNSPolicy).Should(Equal(corev1.DNSClusterFirstWithHostNet))
			})).Should(Succeed())

			By("check the switchover action is triggered")
			Expect(switchover).Should(BeTrue())
		})

		// It("reconfigure", func() {
		//	// TODO
		// })
		//
		// It("member join", func() {
		//	// TODO
		// })
		//
		// It("member leave", func() {
		//	// TODO
		// })
		//
		// It("data load (source ref)", func() {
		//	// TODO
		// })
	})
})

func mockPodStatusReady(namespace, podName string, readyTime metav1.Time) {
	podKey := types.NamespacedName{
		Namespace: namespace,
		Name:      podName,
	}
	Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, true)).Should(Succeed())
	Eventually(testapps.GetAndChangeObjStatus(&testCtx, podKey, func(pod *corev1.Pod) {
		pod.Status.Phase = corev1.PodRunning
		pod.Status.Conditions = []corev1.PodCondition{
			{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: readyTime,
			},
		}
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{
			{
				Name: pod.Spec.Containers[0].Name,
				State: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{},
				},
				Image: pod.Spec.Containers[0].Image,
			},
		}
	})()).Should(Succeed())
}

func mockPodReady(namespace, podName string) {
	By(fmt.Sprintf("mock pod ready: %s", podName))
	mockPodStatusReady(namespace, podName, metav1.Now())
}

func mockPodReadyNAvailable(namespace, podName string, minReadySeconds int32) {
	By(fmt.Sprintf("mock pod ready & available: %s", podName))
	mockPodStatusReady(namespace, podName, metav1.NewTime(time.Now().Add(time.Duration(-1*(minReadySeconds+1))*time.Second)))
}

func mockPodReadyNAvailableWithRole(namespace, podName, role string, minReadySeconds int32) {
	By(fmt.Sprintf("mock pod ready & available with role: %s, %s", podName, role))
	mockPodStatusReady(namespace, podName, metav1.NewTime(time.Now().Add(time.Duration(-1*(minReadySeconds+1))*time.Second)))
	podKey := types.NamespacedName{
		Namespace: namespace,
		Name:      podName,
	}
	Eventually(testapps.GetAndChangeObj(&testCtx, podKey, func(pod *corev1.Pod) {
		pod.Labels[constant.RoleLabelKey] = role
	})()).Should(Succeed())
}
