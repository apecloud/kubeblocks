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
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
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

		It("status - configs", func() {
			createInstObj(instName, func(f *testapps.MockInstanceFactory) {
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

			podKey := instKey
			Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
				expectConfigHashAnnotation(g, pod, map[string]string{
					"log":    "123456",
					"server": "654321",
				})
			})).Should(Succeed())

			mockPodReady(instObj.Namespace, instObj.Name)

			Eventually(testapps.CheckObj(&testCtx, instKey, func(g Gomega, inst *workloads.Instance) {
				g.Expect(inst.Status.Configs).Should(Equal([]workloads.InstanceConfigStatus{
					{
						Name:       "log",
						ConfigHash: ptr.To("123456"),
					},
					{
						Name:       "server",
						ConfigHash: ptr.To("654321"),
					},
				}))
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
			pod := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, podKey, pod)).Should(Succeed())
			Expect(k8sClient.Delete(ctx, pod)).Should(Succeed())

			By("check the pod recreated with new spec")
			Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, replacement *corev1.Pod) {
				g.Expect(replacement.UID).ShouldNot(Equal(pod.UID))
				g.Expect(replacement.Spec.Containers[0].Image).Should(Equal("bar:v2"))
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

		It("reconfigure", func() {
			var (
				reconfigure string
				parameters  map[string]string
			)
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					if req.Action == "reconfigure" {
						reconfigure = req.Action
						parameters = req.Parameters
					}
					return kbagentproto.ActionResponse{}, nil
				}).AnyTimes()
			})
			defer kbacli.UnsetMockClient()

			createInstObj(instName, func(f *testapps.MockInstanceFactory) {
				f.AddConfigs(
					workloads.ConfigTemplate{
						Name:       "log",
						ConfigHash: ptr.To("123456"),
					},
					workloads.ConfigTemplate{
						Name:       "server",
						ConfigHash: ptr.To("123456"),
					},
				)
			})

			mockPodReady(instObj.Namespace, instObj.Name)

			Expect(testapps.GetAndChangeObj(&testCtx, instKey, func(inst *workloads.Instance) {
				inst.Spec.Configs[0].ConfigHash = ptr.To("abcdef")
				inst.Spec.Configs[0].Reconfigure = testapps.NewLifecycleAction("reconfigure")
				inst.Spec.Configs[0].Parameters = map[string]string{"foo": "bar"}
			})()).Should(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(reconfigure).Should(Equal("reconfigure"))
				g.Expect(parameters).Should(HaveKeyWithValue("foo", "bar"))
			}).Should(Succeed())

			podKey := instKey
			Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
				expectConfigHashAnnotation(g, pod, map[string]string{
					"log":    "abcdef",
					"server": "123456",
				})
			})).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, instKey, func(g Gomega, inst *workloads.Instance) {
				g.Expect(inst.Status.Configs).Should(Equal([]workloads.InstanceConfigStatus{
					{
						Name:       "log",
						ConfigHash: ptr.To("abcdef"),
					},
					{
						Name:       "server",
						ConfigHash: ptr.To("123456"),
					},
				}))
			})).Should(Succeed())
		})

		It("restart on config update", func() {
			createInstObj(instName, func(f *testapps.MockInstanceFactory) {
				f.AddConfigs(
					workloads.ConfigTemplate{
						Name:       "log",
						ConfigHash: ptr.To("123456"),
					},
					workloads.ConfigTemplate{
						Name:       "server",
						ConfigHash: ptr.To("123456"),
					},
				)
			})

			mockPodReady(instObj.Namespace, instObj.Name)
			podKey := instKey
			podObj := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, podKey, podObj)).Should(Succeed())

			Expect(testapps.GetAndChangeObj(&testCtx, instKey, func(inst *workloads.Instance) {
				inst.Spec.Configs[0].ConfigHash = ptr.To("abcdef")
				inst.Spec.Configs[0].Restart = ptr.To(true)
			})()).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.UID).ShouldNot(Equal(podObj.UID))
				expectConfigHashAnnotation(g, pod, map[string]string{
					"log":    "abcdef",
					"server": "123456",
				})
			})).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, instKey, func(g Gomega, inst *workloads.Instance) {
				g.Expect(inst.Status.Configs).Should(Equal([]workloads.InstanceConfigStatus{
					{
						Name:       "log",
						ConfigHash: ptr.To("abcdef"),
					},
					{
						Name:       "server",
						ConfigHash: ptr.To("123456"),
					},
				}))
			})).Should(Succeed())
		})

		It("reconfigure and restart", func() {
			var (
				reconfigure          string
				parameters           map[string]string
				reconfigureTimestamp time.Time
			)
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					if req.Action == "reconfigure" {
						reconfigure = req.Action
						parameters = req.Parameters
						reconfigureTimestamp = time.Now()
						time.Sleep(1200 * time.Millisecond)
					}
					return kbagentproto.ActionResponse{}, nil
				}).AnyTimes()
			})
			defer kbacli.UnsetMockClient()

			createInstObj(instName, func(f *testapps.MockInstanceFactory) {
				f.AddConfigs(
					workloads.ConfigTemplate{
						Name:       "log",
						ConfigHash: ptr.To("123456"),
					},
					workloads.ConfigTemplate{
						Name:       "server",
						ConfigHash: ptr.To("123456"),
					},
				)
			})

			mockPodReady(instObj.Namespace, instObj.Name)
			podKey := instKey
			podObj := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, podKey, podObj)).Should(Succeed())

			Expect(testapps.GetAndChangeObj(&testCtx, instKey, func(inst *workloads.Instance) {
				inst.Spec.Configs[0].ConfigHash = ptr.To("abcdef")
				inst.Spec.Configs[0].Restart = ptr.To(true)
				inst.Spec.Configs[0].Reconfigure = testapps.NewLifecycleAction("reconfigure")
				inst.Spec.Configs[0].Parameters = map[string]string{"foo": "bar"}
			})()).Should(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(reconfigure).Should(Equal("reconfigure"))
				g.Expect(parameters).Should(HaveKeyWithValue("foo", "bar"))
			}).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.UID).ShouldNot(Equal(podObj.UID))
				g.Expect(pod.CreationTimestamp.Time.After(reconfigureTimestamp)).Should(BeTrue())
				expectConfigHashAnnotation(g, pod, map[string]string{
					"log":    "abcdef",
					"server": "123456",
				})
			})).Should(Succeed())

			Eventually(testapps.CheckObj(&testCtx, instKey, func(g Gomega, inst *workloads.Instance) {
				g.Expect(inst.Status.Configs).Should(Equal([]workloads.InstanceConfigStatus{
					{
						Name:       "log",
						ConfigHash: ptr.To("abcdef"),
					},
					{
						Name:       "server",
						ConfigHash: ptr.To("123456"),
					},
				}))
			})).Should(Succeed())
		})

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

func expectConfigHashAnnotation(g Gomega, pod *corev1.Pod, expected map[string]string) {
	g.Expect(pod.Annotations).Should(HaveKey(constant.CMInsConfigurationHashLabelKey))
	actual := make(map[string]string)
	g.Expect(json.Unmarshal([]byte(pod.Annotations[constant.CMInsConfigurationHashLabelKey]), &actual)).Should(Succeed())
	g.Expect(actual).Should(Equal(expected))
}

func stopTestInstance() *workloads.Instance {
	return &workloads.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-0", Namespace: "default", UID: "instance-uid", Generation: 1},
		Spec: workloads.InstanceSpec{
			InstanceSetName: "demo",
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "demo"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}},
			},
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{{
				ObjectMeta: metav1.ObjectMeta{Name: "data"},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
				},
			}},
		},
	}
}

func stopTestClient(t *testing.T, inst *workloads.Instance) client.Client {
	t.Helper()
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := workloads.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	model.AddScheme(workloads.AddToScheme)
	return fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&workloads.Instance{}, &corev1.Pod{}, &corev1.PersistentVolumeClaim{}).WithObjects(inst).Build()
}

func reconcileStopEntry(t *testing.T, cli client.Client, count int) ctrl.Result {
	t.Helper()
	var result ctrl.Result
	for i := 0; i < count; i++ {
		reconciler := &InstanceReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(1000)}
		var err error
		result, err = reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "demo-0"}})
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func readStopInstance(t *testing.T, cli client.Client) *workloads.Instance {
	t.Helper()
	inst := &workloads.Instance{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "demo-0"}, inst); err != nil {
		t.Fatal(err)
	}
	return inst
}

func readStopPod(t *testing.T, cli client.Client) *corev1.Pod {
	t.Helper()
	pod := &corev1.Pod{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "demo-0"}, pod); err != nil {
		t.Fatal(err)
	}
	return pod
}

func readStopPVC(t *testing.T, cli client.Client) *corev1.PersistentVolumeClaim {
	t.Helper()
	pvc := &corev1.PersistentVolumeClaim{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "data-demo-0"}, pvc); err != nil {
		t.Fatal(err)
	}
	return pvc
}

func bindStopPVC(t *testing.T, cli client.Client) *corev1.PersistentVolumeClaim {
	t.Helper()
	pvc := readStopPVC(t, cli)
	pvc.UID = "pvc-uid"
	pvc.Spec.VolumeName = "pv-data"
	if err := cli.Update(context.Background(), pvc); err != nil {
		t.Fatal(err)
	}
	pvc.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}
	if err := cli.Status().Update(context.Background(), pvc); err != nil {
		t.Fatal(err)
	}
	return readStopPVC(t, cli)
}

func assertStopPodAbsent(t *testing.T, cli client.Client) {
	t.Helper()
	err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "demo-0"}, &corev1.Pod{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("Pod should be absent, got %v", err)
	}
}

func TestInstanceStopInitiallyStopped(t *testing.T) {
	inst := stopTestInstance()
	cli := stopTestClient(t, inst)
	inst.Spec.Stop = ptr.To(true)
	inst.Generation++
	if err := cli.Update(context.Background(), inst); err != nil {
		t.Fatal(err)
	}
	reconcileStopEntry(t, cli, 4)
	assertStopPodAbsent(t, cli)
	pvcs := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(context.Background(), pvcs); err != nil {
		t.Fatal(err)
	}
	if len(pvcs.Items) != 0 {
		t.Fatal("initial stop provisioned PVCs")
	}
	got := readStopInstance(t, cli)
	if got.Status.CurrentState != workloads.InstanceCurrentStateAbsent || got.Status.UpToDate || ptr.Deref(got.Spec.ScaledDown, false) {
		t.Fatalf("incorrect stopped status: %#v", got.Status)
	}
	revision := got.Status.UpdateRevision
	got.Spec.Stop = nil
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	reconcileStopEntry(t, cli, 2)
	if revision == "" || readStopInstance(t, cli).Status.UpdateRevision != revision {
		t.Fatal("clearing Stop changed the Pod revision")
	}
	readStopPod(t, cli)
	readStopPVC(t, cli)
}

func TestInstanceStopSuspendsPVCsAndResumesLatestTemplate(t *testing.T) {
	inst := stopTestInstance()
	cli := stopTestClient(t, inst)
	reconcileStopEntry(t, cli, 3)
	pvc := bindStopPVC(t, cli)
	pod := readStopPod(t, cli)
	pod.Finalizers = []string{"test/hold"}
	if err := cli.Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	got := readStopInstance(t, cli)
	got.Spec.Template.Spec.Containers[0].Image = "db:v2"
	got.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
	got.Spec.InstanceAssistantObjects = []workloads.InstanceAssistantObject{{ConfigMap: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "demo-env", Namespace: "default"}, Data: map[string]string{"version": "v2"}}}}
	got.Spec.Stop = ptr.To(true)
	got.Spec.MinReadySeconds = 3600
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	result := reconcileStopEntry(t, cli, 3)
	if result.RequeueAfter != time.Second || readStopPod(t, cli).DeletionTimestamp.IsZero() || readStopInstance(t, cli).Status.CurrentState != workloads.InstanceCurrentStateTerminating {
		t.Fatalf("Stop did not observe the held terminating Pod and retry: %#v", result)
	}
	reconcileStopEntry(t, cli, 3)
	if !reflect.DeepEqual(readStopPVC(t, cli), pvc) {
		t.Fatal("Stop changed existing storage")
	}
	cm := &corev1.ConfigMap{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "demo-env"}, cm); err != nil || cm.Data["version"] != "v2" {
		t.Fatalf("stopped assistant did not receive current content: %v", err)
	}
	held := readStopPod(t, cli)
	held.Finalizers = nil
	if err := cli.Update(context.Background(), held); err != nil {
		t.Fatal(err)
	}
	reconcileStopEntry(t, cli, 1)
	got = readStopInstance(t, cli)
	if got.Status.CurrentState != workloads.InstanceCurrentStateAbsent || got.Status.ObservedGeneration != got.Generation {
		t.Fatal("resume handoff requires current Absent observation")
	}
	got.Spec.Stop = ptr.To(false)
	got.Spec.MinReadySeconds = 0
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	reconcileStopEntry(t, cli, 3)
	if readStopPod(t, cli).Spec.Containers[0].Image != "db:v2" {
		t.Fatal("resume used an obsolete Pod template")
	}
	resumed := readStopPVC(t, cli)
	if resumed.UID != pvc.UID || resumed.Spec.Resources.Requests.Storage().Cmp(resource.MustParse("2Gi")) != 0 {
		t.Fatal("resume replaced storage or ignored the current expansion target")
	}
}

type failStopDeleteClient struct {
	client.Client
	remaining int
	deleted   int
}

func (c *failStopDeleteClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if _, ok := obj.(*corev1.Pod); ok && c.remaining > 0 {
		c.remaining--
		return errors.New("injected Pod delete failure")
	}
	err := c.Client.Delete(ctx, obj, opts...)
	if _, ok := obj.(*corev1.Pod); ok && err == nil {
		c.deleted++
	}
	return err
}
func TestInstanceStopRetriesAfterCommitFailure(t *testing.T) {
	inst := stopTestInstance()
	cli := stopTestClient(t, inst)
	reconcileStopEntry(t, cli, 3)
	bindStopPVC(t, cli)
	got := readStopInstance(t, cli)
	got.Spec.Stop = ptr.To(true)
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	got = readStopInstance(t, cli)
	got.Spec.InstanceAssistantObjects = []workloads.InstanceAssistantObject{{ConfigMap: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "demo-partial", Namespace: "default"}, Data: map[string]string{"committed": "yes"}}}}
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	faulty := &failStopDeleteClient{Client: cli, remaining: 1}
	reconciler := &InstanceReconciler{Client: faulty, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(1000)}
	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "demo-0"}}); err == nil || faulty.remaining != 0 {
		t.Fatalf("expected injected Pod delete failure, got %v", err)
	}
	readStopPod(t, cli)
	assistant := &corev1.ConfigMap{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "demo-partial"}, assistant); err != nil || assistant.Data["committed"] != "yes" {
		t.Fatalf("expected actual partial assistant commit: %v", err)
	}
	reconcileStopEntry(t, faulty, 4)
	assertStopPodAbsent(t, cli)
	if faulty.deleted != 1 || readStopPVC(t, cli).UID != "pvc-uid" || readStopInstance(t, cli).Status.CurrentState != workloads.InstanceCurrentStateAbsent {
		t.Fatal("retry did not converge without repeating side effects")
	}
}

func TestInstanceStopActualDeletionKeepsRetentionPolicy(t *testing.T) {
	for _, retain := range []bool{false, true} {
		t.Run(fmt.Sprintf("retain=%t", retain), func(t *testing.T) {
			inst := stopTestInstance()
			inst.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{WhenDeleted: kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType}
			if retain {
				inst.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted = kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType
			}
			cli := stopTestClient(t, inst)
			reconcileStopEntry(t, cli, 3)
			pvc := bindStopPVC(t, cli)
			got := readStopInstance(t, cli)
			got.Spec.Stop = ptr.To(true)
			got.Generation++
			if err := cli.Update(context.Background(), got); err != nil {
				t.Fatal(err)
			}
			reconcileStopEntry(t, cli, 3)
			if err := cli.Delete(context.Background(), readStopInstance(t, cli)); err != nil {
				t.Fatal(err)
			}
			reconcileStopEntry(t, cli, 4)
			err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "demo-0"}, &workloads.Instance{})
			if !apierrors.IsNotFound(err) {
				t.Fatalf("Instance deletion incomplete: %v", err)
			}
			existing := &corev1.PersistentVolumeClaim{}
			err = cli.Get(context.Background(), client.ObjectKeyFromObject(pvc), existing)
			if retain {
				if err != nil || existing.UID != pvc.UID || len(existing.OwnerReferences) != 0 {
					t.Fatalf("retention broken: %v %#v", err, existing)
				}
			} else if !apierrors.IsNotFound(err) {
				t.Fatalf("delete policy ignored: %v", err)
			}
		})
	}
}

func TestInstanceStopSkipsUpdateActionsAndPreservesRevision(t *testing.T) {
	inst := stopTestInstance()
	inst.Spec.LifecycleActions = &workloads.LifecycleActions{Switchover: &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"switchover"}}}}
	inst.Spec.Configs = []workloads.ConfigTemplate{{Name: "config", ConfigHash: ptr.To("old")}}
	cli := stopTestClient(t, inst)
	reconcileStopEntry(t, cli, 3)
	bindStopPVC(t, cli)
	revision := readStopInstance(t, cli).Status.UpdateRevision
	got := readStopInstance(t, cli)
	got.Spec.Stop = ptr.To(false)
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	reconcileStopEntry(t, cli, 1)
	if readStopInstance(t, cli).Status.UpdateRevision != revision {
		t.Fatal("explicit Stop=false changed the Pod revision")
	}
	pod := readStopPod(t, cli)
	pod.Status.Phase = corev1.PodRunning
	pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(time.Now().Add(-time.Hour))}}
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "db", Image: "db:v1", Ready: true}}
	if err := cli.Status().Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	got = readStopInstance(t, cli)
	got.Spec.Stop = ptr.To(true)
	got.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
	got.Spec.Configs[0].ConfigHash = ptr.To("new")
	got.Spec.Configs[0].Reconfigure = &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"reload-config"}}}
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	mock := kbacli.NewMockClient(gomock.NewController(t))
	kbacli.SetMockClient(mock, nil)
	t.Cleanup(kbacli.UnsetMockClient)
	reconcileStopEntry(t, cli, 1)
	assertStopPodAbsent(t, cli)
	observed := readStopInstance(t, cli).Status
	if observed.CurrentState != workloads.InstanceCurrentStatePresent || !observed.Ready || !observed.Available || observed.UpToDate {
		t.Fatalf("stop did not preserve the actual ready Pod observation: %#v", observed)
	}
	reconcileStopEntry(t, cli, 1)
	got = readStopInstance(t, cli)
	if got.Status.CurrentState != workloads.InstanceCurrentStateAbsent || got.Status.ObservedGeneration != got.Generation {
		t.Fatal("resume handoff requires current Absent observation")
	}
	changedRevision := got.Status.UpdateRevision
	got.Spec.Stop = nil
	got.Spec.MinReadySeconds = 3600
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	reconcileStopEntry(t, cli, 2)
	if readStopInstance(t, cli).Status.UpdateRevision != changedRevision {
		t.Fatal("clearing Stop changed the Pod revision")
	}
}

type failStopStatusClient struct {
	client.Client
	remaining int
}

func (c *failStopStatusClient) Status() client.SubResourceWriter {
	return &failStopStatusWriter{SubResourceWriter: c.Client.Status(), owner: c}
}

type failStopStatusWriter struct {
	client.SubResourceWriter
	owner *failStopStatusClient
}

func (w *failStopStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	if _, ok := obj.(*workloads.Instance); ok && w.owner.remaining > 0 {
		w.owner.remaining--
		return errors.New("injected Instance status failure")
	}
	return w.SubResourceWriter.Update(ctx, obj, opts...)
}

func TestInstanceStopRetriesAfterPodDeletedBeforeStatusCommit(t *testing.T) {
	inst := stopTestInstance()
	cli := stopTestClient(t, inst)
	reconcileStopEntry(t, cli, 3)
	pvc := bindStopPVC(t, cli)
	got := readStopInstance(t, cli)
	got.Spec.Stop = ptr.To(true)
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	faulty := &failStopStatusClient{Client: cli, remaining: 1}
	controller := &InstanceReconciler{Client: faulty, Recorder: record.NewFakeRecorder(1000)}
	if _, err := controller.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(inst)}); err == nil || faulty.remaining != 0 {
		t.Fatalf("expected injected status failure after Pod deletion, got %v", err)
	}
	assertStopPodAbsent(t, cli)
	reconcileStopEntry(t, cli, 3)
	if readStopInstance(t, cli).Status.CurrentState != workloads.InstanceCurrentStateAbsent || !reflect.DeepEqual(readStopPVC(t, cli), pvc) {
		t.Fatal("restart did not recover the observed state after partial commit")
	}
}

type failResumePodClient struct {
	client.Client
	remaining int
	created   int
}

func (c *failResumePodClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if _, ok := obj.(*corev1.Pod); ok && c.remaining > 0 {
		c.remaining--
		return errors.New("injected resume Pod create failure")
	}
	err := c.Client.Create(ctx, obj, opts...)
	if _, ok := obj.(*corev1.Pod); ok && err == nil {
		c.created++
	}
	return err
}

func TestInstanceStopResumeRetriesAfterPVCCommit(t *testing.T) {
	inst := stopTestInstance()
	cli := stopTestClient(t, inst)
	reconcileStopEntry(t, cli, 3)
	pvc := bindStopPVC(t, cli)
	got := readStopInstance(t, cli)
	got.Spec.Stop = ptr.To(true)
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	reconcileStopEntry(t, cli, 2)
	got = readStopInstance(t, cli)
	got.Spec.Stop = nil
	got.Spec.Template.Spec.Containers[0].Image = "db:v2"
	got.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
	got.Generation++
	if err := cli.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	faulty := &failResumePodClient{Client: cli, remaining: 1}
	controller := &InstanceReconciler{Client: faulty, Recorder: record.NewFakeRecorder(1000)}
	if _, err := controller.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(inst)}); err == nil || faulty.remaining != 0 {
		t.Fatalf("expected injected resume Pod create failure, got %v", err)
	}
	assertStopPodAbsent(t, cli)
	committed := readStopPVC(t, cli)
	if committed.UID != pvc.UID || committed.Spec.Resources.Requests.Storage().Cmp(resource.MustParse("2Gi")) != 0 {
		t.Fatal("expected storage update to commit before Pod creation failed")
	}
	reconcileStopEntry(t, cli, 3)
	if readStopPod(t, cli).Spec.Containers[0].Image != "db:v2" || readStopPVC(t, cli).UID != pvc.UID {
		t.Fatal("resume retry did not converge with the original PVC")
	}
}

func TestInstanceStopResumeRetriesAfterPodCreatedBeforeStatusCommit(t *testing.T) {
	for _, minReadySeconds := range []int32{0, 3600} {
		t.Run(fmt.Sprintf("minReadySeconds=%d", minReadySeconds), func(t *testing.T) {
			inst := stopTestInstance()
			cli := stopTestClient(t, inst)
			reconcileStopEntry(t, cli, 3)
			pvc := bindStopPVC(t, cli)
			got := readStopInstance(t, cli)
			got.Spec.Stop = ptr.To(true)
			got.Generation++
			if err := cli.Update(context.Background(), got); err != nil {
				t.Fatal(err)
			}
			reconcileStopEntry(t, cli, 2)
			got = readStopInstance(t, cli)
			got.Spec.Stop = ptr.To(false)
			got.Spec.MinReadySeconds = minReadySeconds
			got.Spec.Template.Spec.Containers[0].Image = "db:v2"
			got.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
			got.Generation++
			if err := cli.Update(context.Background(), got); err != nil {
				t.Fatal(err)
			}
			counting := &failResumePodClient{Client: cli}
			faulty := &failStopStatusClient{Client: counting, remaining: 1}
			controller := &InstanceReconciler{Client: faulty, Recorder: record.NewFakeRecorder(1000)}
			if _, err := controller.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(inst)}); err == nil || faulty.remaining != 0 {
				t.Fatalf("expected injected status failure after the resumed Pod was created, got %v", err)
			}
			pod := readStopPod(t, cli)
			if counting.created != 1 || pod.Spec.Containers[0].Image != "db:v2" || readStopInstance(t, cli).Status.CurrentState != workloads.InstanceCurrentStateAbsent {
				t.Fatal("expected the new Pod to commit while Instance status remained stale")
			}
			committedClaim := readStopPVC(t, cli)
			if committedClaim.UID != pvc.UID || !reflect.DeepEqual(committedClaim.OwnerReferences, pvc.OwnerReferences) || committedClaim.Spec.Resources.Requests.Storage().Cmp(resource.MustParse("2Gi")) != 0 {
				t.Fatal("expected the PVC expansion to commit with its original identity and owner")
			}
			pod.Status.Phase = corev1.PodPending
			if err := cli.Status().Update(context.Background(), pod); err != nil {
				t.Fatal(err)
			}
			pod = readStopPod(t, cli)
			result := reconcileStopEntry(t, counting, 3)
			status := readStopInstance(t, cli).Status
			if status.CurrentState != workloads.InstanceCurrentStatePresent || status.ObservedGeneration != got.Generation || status.Ready || status.Available {
				t.Fatalf("restart did not observe the resumed unready Pod: %#v", status)
			}
			if counting.created != 1 || !reflect.DeepEqual(readStopPod(t, cli), pod) {
				t.Fatal("retry changed or recreated the already committed Pod")
			}
			claim := readStopPVC(t, cli)
			if !reflect.DeepEqual(claim, committedClaim) {
				t.Fatal("retry changed the already committed PVC")
			}
			if result.RequeueAfter != 0 {
				t.Fatalf("unready runtime should wait for a Pod event: %#v", result)
			}
		})
	}
}

var _ = Describe("Instance availability scheduling", func() {
	It("allows storage alignment and Stop while ready runtime waits for availability", func() {
		scheme := runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(scheme)).Should(Succeed())
		Expect(workloads.AddToScheme(scheme)).Should(Succeed())
		inst := stopTestInstance()
		inst.Spec.MinReadySeconds = 3600
		claims := inst.Spec.VolumeClaimTemplates
		inst.Spec.VolumeClaimTemplates = nil
		cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&workloads.Instance{}, &corev1.Pod{}, &corev1.PersistentVolumeClaim{}).WithObjects(inst).Build()
		key := client.ObjectKeyFromObject(inst)
		run := func() ctrl.Result {
			result, err := (&InstanceReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(1000)}).Reconcile(context.Background(), ctrl.Request{NamespacedName: key})
			Expect(err).ShouldNot(HaveOccurred())
			return result
		}
		for i := 0; i < 3; i++ {
			run()
		}
		pod := &corev1.Pod{}
		Expect(cli.Get(context.Background(), key, pod)).Should(Succeed())
		pod.Status.Phase = corev1.PodRunning
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "db", Image: "db:v1", Ready: true}}
		Expect(cli.Status().Update(context.Background(), pod)).Should(Succeed())
		Expect(cli.Get(context.Background(), key, inst)).Should(Succeed())
		inst.Spec.VolumeClaimTemplates = claims
		inst.Generation++
		Expect(cli.Update(context.Background(), inst)).Should(Succeed())
		Expect(run().RequeueAfter).Should(BeNumerically(">", 0))
		pvcs := &corev1.PersistentVolumeClaimList{}
		Expect(cli.List(context.Background(), pvcs)).Should(Succeed())
		Expect(pvcs.Items).Should(HaveLen(1))
		Expect(cli.Get(context.Background(), key, inst)).Should(Succeed())
		Expect(inst.Status.Ready).Should(BeTrue())
		Expect(inst.Status.Available).Should(BeFalse())

		inst.Spec.Stop = ptr.To(true)
		inst.Generation++
		Expect(cli.Update(context.Background(), inst)).Should(Succeed())
		Expect(run().RequeueAfter).Should(Equal(time.Second))
		Expect(apierrors.IsNotFound(cli.Get(context.Background(), key, pod))).Should(BeTrue())
	})
})
