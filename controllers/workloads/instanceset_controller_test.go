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
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/generics"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	kbaproto "github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("InstanceSet Controller", func() {
	var (
		itsName = "test-instance-set"
		itsObj  *workloads.InstanceSet
		itsKey  client.ObjectKey
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
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PodSignature, true, inNS, ml)
		testapps.ClearResourcesWithRemoveFinalizerOption(&testCtx, generics.PersistentVolumeClaimSignature, true, inNS, ml)
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
			SetReplicas(1)
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

	mockPodReady := func(podNames ...string) {
		By("mock pods ready")
		for _, podName := range podNames {
			podKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      podName,
			}
			Eventually(testapps.GetAndChangeObjStatus(&testCtx, podKey, func(pod *corev1.Pod) {
				pod.Status.Phase = corev1.PodRunning
				pod.Status.Conditions = []corev1.PodCondition{
					{
						Type:               corev1.PodReady,
						Status:             corev1.ConditionTrue,
						LastTransitionTime: metav1.Now(),
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
	}

	Context("reconciliation", func() {
		It("should reconcile well", func() {
			name := "test-instance-set"
			port := int32(12345)
			commonLabels := map[string]string{
				constant.AppManagedByLabelKey:   constant.AppName,
				constant.AppNameLabelKey:        "ClusterDefName",
				constant.AppComponentLabelKey:   "CompDefName",
				constant.AppInstanceLabelKey:    "clusterName",
				constant.KBAppComponentLabelKey: "componentName",
			}
			pod := builder.NewPodBuilder(testCtx.DefaultNamespace, "foo").
				AddLabelsInMap(commonLabels).
				AddContainer(corev1.Container{
					Name:  "foo",
					Image: "bar",
					Ports: []corev1.ContainerPort{
						{
							Name:          "foo",
							Protocol:      corev1.ProtocolTCP,
							ContainerPort: port,
						},
					},
				}).GetObject()
			template := corev1.PodTemplateSpec{
				ObjectMeta: pod.ObjectMeta,
				Spec:       pod.Spec,
			}
			its := builder.NewInstanceSetBuilder(testCtx.DefaultNamespace, name).
				SetSelectorMatchLabel(commonLabels).
				AddAnnotations(constant.CRDAPIVersionAnnotationKey, workloads.GroupVersion.String()).
				SetTemplate(template).
				GetObject()
			viper.Set(constant.KBToolsImage, "kb-tool-image")
			Expect(k8sClient.Create(ctx, its)).Should(Succeed())
			Eventually(testapps.CheckObj(&testCtx, client.ObjectKeyFromObject(its),
				func(g Gomega, set *workloads.InstanceSet) {
					g.Expect(set.Status.ObservedGeneration).Should(BeEquivalentTo(1))
				}),
			).Should(Succeed())
			Expect(k8sClient.Delete(ctx, its)).Should(Succeed())
			Eventually(testapps.CheckObjExists(&testCtx, client.ObjectKeyFromObject(its), &workloads.InstanceSet{}, false)).
				Should(Succeed())
		})

		It("rolling", func() {
			replicas := int32(3)
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.SetReplicas(replicas).
					SetInstanceUpdateStrategy(&workloads.InstanceUpdateStrategy{
						Type: kbappsv1.RollingUpdateStrategyType,
						RollingUpdate: &workloads.RollingUpdate{
							Replicas: &intstr.IntOrString{
								Type:   intstr.Int,
								IntVal: 1, // one instance at a time
							},
						},
					}).SetPodManagementPolicy(appsv1.ParallelPodManagement)
			})

			podsKey := []types.NamespacedName{
				{
					Namespace: itsObj.Namespace,
					Name:      fmt.Sprintf("%s-0", itsObj.Name),
				},
				{
					Namespace: itsObj.Namespace,
					Name:      fmt.Sprintf("%s-1", itsObj.Name),
				},
				{
					Namespace: itsObj.Namespace,
					Name:      fmt.Sprintf("%s-2", itsObj.Name),
				},
			}
			mockPodReady(podsKey[0].Name, podsKey[1].Name, podsKey[2].Name)

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
				By("wait new pod created")
				podKey := podsKey[i-1]
				Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
					g.Expect(pod.CreationTimestamp.After(beforeUpdate)).Should(BeTrue())
				})).Should(Succeed())

				// mock new pod ready
				mockPodReady(podKey.Name)

				By(fmt.Sprintf("check its status updated: %s", podKey.Name))
				Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
					g.Expect(its.Status.UpdatedReplicas).Should(Equal(replicas - i + 1))
				})).Should(Succeed())
			}

			By("check its ready")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.IsInstanceSetReady()).Should(BeTrue())
			})).Should(Succeed())
		})
	})

	Context("PVC retention policy", func() {
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

		It("provision", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.AddVolumeClaimTemplate(pvc)
			})

			By("check pods created")
			podKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-0", itsObj.Name),
			}
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, true)).Should(Succeed())

			By("check PVCs created")
			pvcKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s-0", pvc.Name, itsObj.Name),
			}
			Eventually(testapps.CheckObjExists(&testCtx, pvcKey, &corev1.PersistentVolumeClaim{}, true)).Should(Succeed())
		})

		It("when deleted - delete", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.AddVolumeClaimTemplate(pvc).
					SetPVCRetentionPolicy(&workloads.PersistentVolumeClaimRetentionPolicy{
						WhenDeleted: kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType,
					})
			})

			By("delete the ITS object")
			Expect(k8sClient.Delete(ctx, itsObj)).Should(Succeed())

			By("check its object NOT deleted")
			Consistently(testapps.CheckObjExists(&testCtx, itsKey, &workloads.InstanceSet{}, true)).Should(Succeed())

			By("check pods deleted")
			podKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-0", itsObj.Name),
			}
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())

			By("check PVCs deleted, but the pvc-protection finalizer prevent the pvc to be deleted physically")
			pvcKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s-0", pvc.Name, itsObj.Name),
			}
			Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, pvc *corev1.PersistentVolumeClaim) {
				g.Expect(pvc.DeletionTimestamp).ShouldNot(BeNil())
				g.Expect(pvc.Finalizers).To(HaveLen(1))
				g.Expect(pvc.Finalizers[0]).To(Equal("kubernetes.io/pvc-protection"))
			})).Should(Succeed())
		})

		It("when deleted - retain", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.AddVolumeClaimTemplate(pvc).
					SetPVCRetentionPolicy(&workloads.PersistentVolumeClaimRetentionPolicy{
						WhenDeleted: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType,
					})
			})

			By("delete the ITS object")
			Expect(k8sClient.Delete(ctx, itsObj)).Should(Succeed())
			Eventually(testapps.CheckObjExists(&testCtx, itsKey, &workloads.InstanceSet{}, false)).Should(Succeed())

			By("check pods deleted")
			podKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-0", itsObj.Name),
			}
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())

			By("check PVCs retained and not deleted")
			pvcKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s-0", pvc.Name, itsObj.Name),
			}
			Consistently(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, pvc *corev1.PersistentVolumeClaim) {
				g.Expect(pvc.DeletionTimestamp).Should(BeNil())
			})).Should(Succeed())
		})

		It("when scaled - delete", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.AddVolumeClaimTemplate(pvc).
					SetPVCRetentionPolicy(&workloads.PersistentVolumeClaimRetentionPolicy{
						WhenScaled: kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType,
					})
			})

			By("scale-in")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Replicas = ptr.To(int32(0))
			})()).ShouldNot(HaveOccurred())

			By("check pods deleted")
			podKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-0", itsObj.Name),
			}
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())

			By("check PVCs deleted, but the pvc-protection finalizer prevent the pvc to be deleted physically")
			pvcKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s-0", pvc.Name, itsObj.Name),
			}
			Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, pvc *corev1.PersistentVolumeClaim) {
				g.Expect(pvc.DeletionTimestamp).ShouldNot(BeNil())
				g.Expect(pvc.Finalizers).To(HaveLen(1))
				g.Expect(pvc.Finalizers[0]).To(Equal("kubernetes.io/pvc-protection"))
			})).Should(Succeed())
		})

		It("when scaled - retain", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.AddVolumeClaimTemplate(pvc).
					SetPVCRetentionPolicy(&workloads.PersistentVolumeClaimRetentionPolicy{
						WhenScaled: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType,
					})
			})

			By("scale-in")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Replicas = ptr.To(int32(0))
			})()).ShouldNot(HaveOccurred())

			By("check pods deleted")
			podKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-0", itsObj.Name),
			}
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())

			By("check PVCs retained and not deleted")
			pvcKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s-0", pvc.Name, itsObj.Name),
			}
			Consistently(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, pvc *corev1.PersistentVolumeClaim) {
				g.Expect(pvc.DeletionTimestamp).Should(BeNil())
			})).Should(Succeed())
		})
	})

	Context("reconfigure", func() {
		It("instance status", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.AddConfigs(workloads.ConfigTemplate{
					Name:       "server",
					ConfigHash: ptr.To("123456"),
				})
			})

			By("check instance status")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(its.Status.InstanceStatus[0]).Should(Equal(workloads.InstanceStatus{
					PodName: fmt.Sprintf("%s-0", itsObj.Name),
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "server",
							ConfigHash: ptr.To("123456"),
						},
					},
				}))
			})).Should(Succeed())
		})

		It("reconfigure", func() {
			By("mock reconfigure action calls")
			var (
				reconfigure string
				parameters  map[string]string
			)
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req kbaproto.ActionRequest) (kbaproto.ActionResponse, error) {
					if req.Action == "reconfigure" || strings.HasPrefix(req.Action, "udf-reconfigure") {
						reconfigure = req.Action
						parameters = req.Parameters
					}
					return kbaproto.ActionResponse{}, nil
				}).AnyTimes()
			})

			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.SetInstanceUpdateStrategy(&workloads.InstanceUpdateStrategy{
					Type: kbappsv1.RollingUpdateStrategyType,
				}).AddConfigs([]workloads.ConfigTemplate{
					{
						Name:       "server",
						ConfigHash: ptr.To("123456"),
					},
					{
						Name:       "logging",
						ConfigHash: ptr.To("654321"),
					},
				}...)
			})

			By("mock pods running and available")
			podKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-0", itsObj.Name),
			}
			Expect(testapps.GetAndChangeObjStatus(&testCtx, podKey, func(pod *corev1.Pod) {
				pod.Status.Phase = corev1.PodRunning
				pod.Status.Conditions = []corev1.PodCondition{
					{
						Type:               corev1.PodReady,
						Status:             corev1.ConditionTrue,
						LastTransitionTime: metav1.Now(),
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
			})()).ShouldNot(HaveOccurred())

			By("check the init instance status")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(its.Status.InstanceStatus[0]).Should(Equal(workloads.InstanceStatus{
					PodName: fmt.Sprintf("%s-0", itsObj.Name),
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "server",
							ConfigHash: ptr.To("123456"),
						},
						{
							Name:       "logging",
							ConfigHash: ptr.To("654321"),
						},
					},
				}))
			})).Should(Succeed())

			By("check the reconfigure action not called")
			Eventually(func(g Gomega) {
				g.Expect(reconfigure).Should(BeEmpty())
				g.Expect(parameters).Should(BeNil())
			}).Should(Succeed())

			By("update configs")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Configs[1].ConfigHash = ptr.To("abcdef")
				its.Spec.Configs[1].Reconfigure = testapps.NewLifecycleAction("reconfigure")
				its.Spec.Configs[1].ReconfigureActionName = ""
				its.Spec.Configs[1].Parameters = map[string]string{"foo": "bar"}
			})()).ShouldNot(HaveOccurred())

			By("check the instance status updated")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(its.Status.InstanceStatus[0]).Should(Equal(workloads.InstanceStatus{
					PodName: fmt.Sprintf("%s-0", itsObj.Name),
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "server",
							ConfigHash: ptr.To("123456"),
						},
						{
							Name:       "logging",
							ConfigHash: ptr.To("abcdef"),
						},
					},
				}))
			})).Should(Succeed())

			By("check the reconfigure action call")
			Eventually(func(g Gomega) {
				g.Expect(reconfigure).Should(Equal("reconfigure"))
				g.Expect(parameters).ShouldNot(BeNil())
				g.Expect(parameters).Should(HaveKeyWithValue("foo", "bar"))
			}).Should(Succeed())
		})

		It("reconfigure - udf", func() {
			By("mock reconfigure action calls")
			var (
				reconfigure string
				parameters  map[string]string
			)
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req kbaproto.ActionRequest) (kbaproto.ActionResponse, error) {
					if req.Action == "reconfigure" || strings.HasPrefix(req.Action, "udf-reconfigure") {
						reconfigure = req.Action
						parameters = req.Parameters
					}
					return kbaproto.ActionResponse{}, nil
				}).AnyTimes()
			})

			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.SetInstanceUpdateStrategy(&workloads.InstanceUpdateStrategy{
					Type: kbappsv1.RollingUpdateStrategyType,
				}).AddConfigs([]workloads.ConfigTemplate{
					{
						Name:       "server",
						ConfigHash: ptr.To("123456"),
					},
					{
						Name:       "logging",
						ConfigHash: ptr.To("654321"),
					},
				}...)
			})

			mockPodReady(fmt.Sprintf("%s-0", itsObj.Name))

			By("check the init instance status")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(its.Status.InstanceStatus[0]).Should(Equal(workloads.InstanceStatus{
					PodName: fmt.Sprintf("%s-0", itsObj.Name),
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "server",
							ConfigHash: ptr.To("123456"),
						},
						{
							Name:       "logging",
							ConfigHash: ptr.To("654321"),
						},
					},
				}))
			})).Should(Succeed())

			By("check the reconfigure action not called")
			Eventually(func(g Gomega) {
				g.Expect(reconfigure).Should(BeEmpty())
				g.Expect(parameters).Should(BeNil())
			}).Should(Succeed())

			By("update configs")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Configs[1].ConfigHash = ptr.To("abcdef")
				its.Spec.Configs[1].Reconfigure = testapps.NewLifecycleAction("reconfigure")
				its.Spec.Configs[1].ReconfigureActionName = "reconfigure-server"
				its.Spec.Configs[1].Parameters = map[string]string{"foo": "bar"}
			})()).ShouldNot(HaveOccurred())

			By("check the instance status updated")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(its.Status.InstanceStatus[0]).Should(Equal(workloads.InstanceStatus{
					PodName: fmt.Sprintf("%s-0", itsObj.Name),
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "server",
							ConfigHash: ptr.To("123456"),
						},
						{
							Name:       "logging",
							ConfigHash: ptr.To("abcdef"),
						},
					},
				}))
			})).Should(Succeed())

			By("check the reconfigure action call")
			Eventually(func(g Gomega) {
				g.Expect(reconfigure).Should(ContainSubstring("reconfigure-server"))
				g.Expect(parameters).ShouldNot(BeNil())
				g.Expect(parameters).Should(HaveKeyWithValue("foo", "bar"))
			}).Should(Succeed())
		})
	})

	Context("pod naming rule", func() {
		checkPodOrdinal := func(ordinals []int, checkFunc func(podKey types.NamespacedName)) {
			for _, ordinal := range ordinals {
				podKey := types.NamespacedName{
					Namespace: itsObj.Namespace,
					Name:      fmt.Sprintf("%v-%v", itsObj.Name, ordinal),
				}
				checkFunc(podKey)
			}
		}

		eventuallyExist := func(podKey types.NamespacedName) {
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, true)).Should(Succeed())
		}

		eventuallyNotExist := func(podKey types.NamespacedName) {
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())
		}

		consistentlyExist := func(podKey types.NamespacedName) {
			Consistently(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, true)).Should(Succeed())
		}

		consistentlyNotExist := func(podKey types.NamespacedName) {
			Consistently(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())
		}

		It("works with FlatInstanceOrdinal", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.SetFlatInstanceOrdinal(true)
				f.SetPodManagementPolicy(appsv1.ParallelPodManagement)
				f.SetReplicas(3)
			})

			checkPodOrdinal([]int{0, 1, 2}, eventuallyExist)
			mockPodReady(itsObj.Name+"-0", itsObj.Name+"-1", itsObj.Name+"-2")
			By("check its status")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.AssignedOrdinals).Should(HaveKey(instancetemplate.DefaultTemplateName))
				g.Expect(its.Status.AssignedOrdinals[instancetemplate.DefaultTemplateName].Discrete).Should(HaveExactElements(int32(0), int32(1), int32(2)))
			})).Should(Succeed())

			// offline one instance
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Replicas = ptr.To[int32](2)
				its.Spec.OfflineInstances = []string{itsObj.Name + "-1"}
			})()).Should(Succeed())
			checkPodOrdinal([]int{1}, eventuallyNotExist)
			By("check its status")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.AssignedOrdinals).Should(HaveKey(instancetemplate.DefaultTemplateName))
				g.Expect(its.Status.AssignedOrdinals[instancetemplate.DefaultTemplateName].Discrete).Should(HaveExactElements(int32(0), int32(2)))
			})).Should(Succeed())

			// scale up
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Replicas = ptr.To[int32](4)
			})()).Should(Succeed())
			checkPodOrdinal([]int{0, 2, 3, 4}, eventuallyExist)
			mockPodReady(itsObj.Name+"-3", itsObj.Name+"-4")
			By("check its status")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.AssignedOrdinals).Should(HaveKey(instancetemplate.DefaultTemplateName))
				g.Expect(its.Status.AssignedOrdinals[instancetemplate.DefaultTemplateName].Discrete).Should(HaveExactElements(int32(0), int32(2), int32(3), int32(4)))
			})).Should(Succeed())

			// delete OfflineInstances will not affect running instances
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.OfflineInstances = []string{}
			})()).Should(Succeed())
			checkPodOrdinal([]int{0, 2, 3, 4}, consistentlyExist)
			checkPodOrdinal([]int{1}, consistentlyNotExist)
			By("check its status")
			Consistently(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.AssignedOrdinals).Should(HaveKey(instancetemplate.DefaultTemplateName))
				g.Expect(its.Status.AssignedOrdinals[instancetemplate.DefaultTemplateName].Discrete).Should(HaveExactElements(int32(0), int32(2), int32(3), int32(4)))
			})).Should(Succeed())
		})
	})

	Context("start & stop", func() {
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

		BeforeEach(func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.SetFlatInstanceOrdinal(true).
					AddVolumeClaimTemplate(pvc)
			})

			By("check pods created")
			podKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-0", itsObj.Name),
			}
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, true)).Should(Succeed())

			By("check PVCs created")
			pvcKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s-0", pvc.Name, itsObj.Name),
			}
			Eventually(testapps.CheckObjExists(&testCtx, pvcKey, &corev1.PersistentVolumeClaim{}, true)).Should(Succeed())
		})

		It("stop", func() {
			By("stop the its")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Stop = ptr.To(true)
			})()).Should(Succeed())

			By("check pods deleted")
			podKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-0", itsObj.Name),
			}
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())

			By("check PVCs still exist")
			pvcKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-%s-0", pvc.Name, itsObj.Name),
			}
			Consistently(testapps.CheckObjExists(&testCtx, pvcKey, &corev1.PersistentVolumeClaim{}, true)).Should(Succeed())
		})

		It("start", func() {
			By("stop the its first")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Stop = ptr.To(true)
			})()).Should(Succeed())

			By("stop the its")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Stop = ptr.To(true)
			})()).Should(Succeed())

			By("check pods deleted")
			podKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-0", itsObj.Name),
			}
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())

			By("start it")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Stop = nil
			})()).Should(Succeed())

			By("check pods created")
			Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, true)).Should(Succeed())
		})

		It("stop & start - discrete ordinals", func() {
			By("scale up to 3 replicas")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
				its.Spec.Replicas = ptr.To(int32(3))
			})()).Should(Succeed())

			By("check pods created and mock them ready")
			for i := 0; i < 3; i++ {
				podKey := types.NamespacedName{
					Namespace: itsObj.Namespace,
					Name:      fmt.Sprintf("%s-%d", itsObj.Name, i),
				}
				Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, true)).Should(Succeed())

				mockPodReady(podKey.Name)
			}

			By("offline instance 1")
			offlineOrdinal := 1
			offlinePodKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-%d", itsObj.Name, offlineOrdinal),
			}
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Replicas = ptr.To(int32(2))
				its.Spec.OfflineInstances = []string{offlinePodKey.Name}
			})()).Should(Succeed())

			By("check instance 1 offline")
			Eventually(testapps.CheckObjExists(&testCtx, offlinePodKey, &corev1.Pod{}, false)).Should(Succeed())

			By("cleanup offline instances & stop the its")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.OfflineInstances = nil
				its.Spec.Stop = ptr.To(true)
			})()).Should(Succeed())

			By("check pods deleted")
			for i := 0; i < 3; i++ {
				podKey := types.NamespacedName{
					Namespace: itsObj.Namespace,
					Name:      fmt.Sprintf("%s-%d", itsObj.Name, i),
				}
				Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())
			}

			By("start it")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Stop = nil
			})()).Should(Succeed())

			By("check pods created")
			for i := 0; i < 3; i++ {
				podKey := types.NamespacedName{
					Namespace: itsObj.Namespace,
					Name:      fmt.Sprintf("%s-%d", itsObj.Name, i),
				}
				if i == offlineOrdinal {
					Consistently(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, false)).Should(Succeed())
				} else {
					Eventually(testapps.CheckObjExists(&testCtx, podKey, &corev1.Pod{}, true)).Should(Succeed())
				}
			}
		})
	})
})
