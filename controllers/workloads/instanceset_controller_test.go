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
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
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
					Generation: int64(1),
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
							Generation: int64(1),
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
						Generation: int64(1),
					},
					{
						Name:       "logging",
						Generation: int64(2),
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
							Generation: int64(1),
						},
						{
							Name:       "logging",
							Generation: int64(2),
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
				its.Spec.Configs[1].Generation = 128
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
							Generation: int64(1),
						},
						{
							Name:       "logging",
							Generation: int64(128),
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
						Generation: int64(1),
					},
					{
						Name:       "logging",
						Generation: int64(2),
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
							Generation: int64(1),
						},
						{
							Name:       "logging",
							Generation: int64(2),
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
				its.Spec.Configs[1].Generation = 128
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
							Generation: int64(1),
						},
						{
							Name:       "logging",
							Generation: int64(128),
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
})
