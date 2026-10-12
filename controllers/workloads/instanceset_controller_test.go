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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
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
							MaxUnavailable: &intstr.IntOrString{
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
			Eventually(testapps.CheckObj(&testCtx, pvcKey, func(g Gomega, pvc *corev1.PersistentVolumeClaim) {
				// verify owner references are cleared to prevent garbage collection
				ownerRefs := pvc.GetOwnerReferences()
				g.Expect(ownerRefs).Should(HaveLen(0), "Owner references should be cleared when retention policy is Retain")
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
				// verify owner references still exist since InstanceSet is not deleted
				ownerRefs := pvc.GetOwnerReferences()
				g.Expect(ownerRefs).Should(HaveLen(1), "Owner references should still exist when scaling down and retention policy is Retain")
				g.Expect(ownerRefs[0].Kind).Should(Equal("InstanceSet"))
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

func pvcLifecycleFixture(t *testing.T, replicas int32) client.WithWatch {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := workloads.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	model.AddScheme(workloads.AddToScheme)
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "pvc-lifecycle", Namespace: "default", UID: "its-uid", Generation: 1,
			Annotations: map[string]string{constant.CRDAPIVersionAnnotationKey: workloads.GroupVersion.String()}},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To(replicas), PodManagementPolicy: appsv1.ParallelPodManagement,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "pvc-lifecycle"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "pvc-lifecycle"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}}},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}, Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
			}}},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&workloads.InstanceSet{}, &corev1.Pod{}, &corev1.PersistentVolumeClaim{}).WithObjects(its).Build()
	pvcLifecycleReconcile(t, cli, 4)
	if len(pvcLifecyclePods(t, cli)) != int(replicas) {
		t.Fatal("fixture did not provision running identities")
	}
	for _, claim := range pvcLifecycleClaims(t, cli) {
		claim.UID = types.UID(claim.Name + "-uid")
		claim.Spec.VolumeName = claim.Name + "-volume"
		if err := cli.Update(context.Background(), &claim); err != nil {
			t.Fatal(err)
		}
	}
	return cli
}

func pvcLifecycleReconcile(t *testing.T, cli client.Client, count int) ctrl.Result {
	t.Helper()
	var result ctrl.Result
	for i := 0; i < count; i++ {
		reconciler := &InstanceSetReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(1000)}
		var err error
		result, err = reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "pvc-lifecycle"}})
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func pvcLifecycleSet(t *testing.T, cli client.Client) *workloads.InstanceSet {
	t.Helper()
	its := &workloads.InstanceSet{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "pvc-lifecycle"}, its); err != nil {
		t.Fatal(err)
	}
	return its
}

func pvcLifecycleChange(t *testing.T, cli client.Client, change func(*workloads.InstanceSet)) {
	t.Helper()
	its := pvcLifecycleSet(t, cli)
	change(its)
	its.Generation++
	if err := cli.Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
}

func pvcLifecyclePods(t *testing.T, cli client.Client) []corev1.Pod {
	t.Helper()
	pods := &corev1.PodList{}
	if err := cli.List(context.Background(), pods); err != nil {
		t.Fatal(err)
	}
	return pods.Items
}

func pvcLifecycleClaims(t *testing.T, cli client.Client) []corev1.PersistentVolumeClaim {
	t.Helper()
	claims := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(context.Background(), claims); err != nil {
		t.Fatal(err)
	}
	return claims.Items
}

func pvcLifecycleReady(t *testing.T, cli client.Client) {
	t.Helper()
	for _, claim := range pvcLifecycleClaims(t, cli) {
		claim.Status.Capacity = claim.Spec.Resources.Requests.DeepCopy()
		if err := cli.Status().Update(context.Background(), &claim); err != nil {
			t.Fatal(err)
		}
	}
	for _, pod := range pvcLifecyclePods(t, cli) {
		pod.Status.Phase = corev1.PodRunning
		pod.Status.PodIP = "127.0.0.1"
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "db", Image: "db:v1", Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}
		if err := cli.Status().Update(context.Background(), &pod); err != nil {
			t.Fatal(err)
		}
	}
	pvcLifecycleReconcile(t, cli, 2)
}

func TestInstanceSetRetainedClaimsAfterRecreation(t *testing.T) {
	cli := pvcLifecycleFixture(t, 2)
	pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{
			WhenDeleted: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType,
			WhenScaled:  kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType,
		}
	})
	extra := pvcLifecycleClaims(t, cli)[0].DeepCopy()
	extra.Name = "retained-" + extra.Name
	extra.UID = types.UID(extra.Name + "-uid")
	extra.ResourceVersion = ""
	if err := cli.Create(context.Background(), extra); err != nil {
		t.Fatal(err)
	}
	original := pvcLifecycleSet(t, cli)
	if err := cli.Delete(context.Background(), original); err != nil {
		t.Fatal(err)
	}
	pvcLifecycleReconcile(t, cli, 6)
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(original), &workloads.InstanceSet{}); !apierrors.IsNotFound(err) {
		t.Fatalf("original InstanceSet was not deleted: %v", err)
	}
	claims := pvcLifecycleClaims(t, cli)
	if len(claims) != 3 {
		t.Fatal("root deletion did not retain all claims")
	}
	for _, claim := range claims {
		if len(claim.OwnerReferences) != 0 {
			t.Fatal("retained claim was not detached from the deleted root")
		}
	}
	replacement := original.DeepCopy()
	replacement.ResourceVersion = ""
	replacement.UID = "replacement-root"
	replacement.Generation = 1
	replacement.Finalizers = nil
	replacement.Status = workloads.InstanceSetStatus{}
	replacement.Spec.Replicas = ptr.To(int32(0))
	if err := cli.Create(context.Background(), replacement); err != nil {
		t.Fatal(err)
	}
	pvcLifecycleReconcile(t, cli, 4)
	if !reflect.DeepEqual(claims, pvcLifecycleClaims(t, cli)) {
		t.Fatal("same-name recreation changed unadopted retained claims")
	}
	pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(1)) })
	pvcLifecycleReconcile(t, cli, 4)
	for _, claim := range pvcLifecycleClaims(t, cli) {
		switch claim.Name {
		case "data-pvc-lifecycle-0":
			if !metav1.IsControlledBy(&claim, replacement) || !reflect.DeepEqual(claim.Spec, claims[0].Spec) || claim.UID != claims[0].UID {
				t.Fatal("active claim was not adopted with its storage preserved")
			}
		case extra.Name:
			if !reflect.DeepEqual(claim, claims[2]) {
				t.Fatal("adoption changed the detached sibling claim")
			}
		default:
			if !reflect.DeepEqual(claim, claims[1]) {
				t.Fatal("adoption changed the unused retained claim")
			}
		}
	}
	pods := pvcLifecyclePods(t, cli)
	if len(pods) != 1 {
		t.Fatal("replacement did not create its active Pod")
	}
	if err := cli.Delete(context.Background(), &pods[0]); err != nil {
		t.Fatal(err)
	}
	pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(0)) })
	pvcLifecycleReconcile(t, cli, 4)
	if remaining := pvcLifecycleClaims(t, cli); !reflect.DeepEqual(remaining, claims[1:]) {
		t.Fatal("retirement must remove only the adopted claim and preserve unowned storage")
	}
}

func TestInstanceSetPVCLiveOffline(t *testing.T) {
	cli := pvcLifecycleFixture(t, 1)
	pvcLifecycleReady(t, cli)
	offline := pvcLifecycleClaims(t, cli)[0]
	ready := pvcLifecyclePods(t, cli)[0].Status
	pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.OfflineInstances = []string{"pvc-lifecycle-0"}
	})
	pvcLifecycleReconcile(t, cli, 1)
	active := &corev1.Pod{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "pvc-lifecycle-1"}, active); err != nil {
		t.Fatal(err)
	}
	active.Status = ready
	if err := cli.Status().Update(context.Background(), active); err != nil {
		t.Fatal(err)
	}
	pvcLifecycleReconcile(t, cli, 2)
	if pods := pvcLifecyclePods(t, cli); len(pods) != 1 || pods[0].Name != "pvc-lifecycle-1" {
		t.Fatalf("live Offline transition did not replace the Pod: %#v", pods)
	}
	current := &corev1.PersistentVolumeClaim{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(&offline), current); err != nil || !reflect.DeepEqual(offline, *current) {
		t.Fatalf("live Offline retirement changed allocated storage: %v", err)
	}
}

func TestInstanceSetPVCAbsentAllocation(t *testing.T) {
	for _, policy := range []kbappsv1.PersistentVolumeClaimRetentionPolicyType{
		kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType,
		kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType,
	} {
		t.Run(string(policy), func(t *testing.T) {
			cli := pvcLifecycleFixture(t, 2)
			pvcLifecycleReady(t, cli)
			claims := pvcLifecycleClaims(t, cli)
			pods := pvcLifecyclePods(t, cli)
			if err := cli.Delete(context.Background(), &pods[0]); err != nil {
				t.Fatal(err)
			}
			pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.Replicas = ptr.To(int32(1))
				its.Spec.OfflineInstances = []string{"pvc-lifecycle-0"}
				its.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{WhenScaled: policy}
			})
			pvcLifecycleReconcile(t, cli, 4)
			if !reflect.DeepEqual(claims, pvcLifecycleClaims(t, cli)) {
				t.Fatal("absent Offline allocation changed storage")
			}
			if pods := pvcLifecyclePods(t, cli); len(pods) != 1 || pods[0].Name != "pvc-lifecycle-1" {
				t.Fatalf("Offline identity was recreated: %#v", pods)
			}
			pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.Replicas = ptr.To(int32(0))
				its.Spec.OfflineInstances = nil
			})
			pvcLifecycleReconcile(t, cli, 4)
			if len(pvcLifecyclePods(t, cli)) != 0 {
				t.Fatal("retired Pods survived scale-down")
			}
			remaining := pvcLifecycleClaims(t, cli)
			if policy == kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType {
				if !reflect.DeepEqual(claims, remaining) {
					t.Fatal("WhenScaled=Retain changed storage")
				}
			} else if len(remaining) != 0 {
				t.Fatal("absent unallocated claim survived retirement")
			}
		})
	}
}

func TestInstanceSetPVCRetirementAfterPartialCommit(t *testing.T) {
	cli := pvcLifecycleFixture(t, 4)
	for _, pod := range pvcLifecyclePods(t, cli) {
		if err := cli.Delete(context.Background(), &pod); err != nil {
			t.Fatal(err)
		}
	}
	allocated := []corev1.PersistentVolumeClaim{}
	for _, name := range []string{"data-pvc-lifecycle-0", "data-pvc-lifecycle-3"} {
		claim := &corev1.PersistentVolumeClaim{}
		if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, claim); err != nil {
			t.Fatal(err)
		}
		allocated = append(allocated, *claim)
	}
	pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.OfflineInstances = []string{"pvc-lifecycle-3"}
		its.Spec.Replicas = ptr.To(int32(1))
	})
	failure := errors.New("injected retired PVC delete failure")
	fired, attempts, deleted := false, 0, 0
	faulty := interceptor.NewClient(cli, interceptor.Funcs{
		Delete: func(ctx context.Context, delegate client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			if _, ok := obj.(*corev1.PersistentVolumeClaim); ok {
				attempts++
				if !fired && attempts == 2 {
					fired = true
					return failure
				}
				err := delegate.Delete(ctx, obj, opts...)
				if err == nil {
					deleted++
				}
				return err
			}
			return delegate.Delete(ctx, obj, opts...)
		},
	})
	controller := &InstanceSetReconciler{Client: faulty, Recorder: record.NewFakeRecorder(1000)}
	_, err := controller.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "pvc-lifecycle"}})
	if !errors.Is(err, failure) || !fired || attempts != 2 || deleted != 1 || len(pvcLifecycleClaims(t, cli)) != 3 || len(pvcLifecyclePods(t, cli)) != 0 {
		t.Fatalf("expected one persisted retired-claim deletion before failure, got %v", err)
	}
	for _, claim := range allocated {
		current := &corev1.PersistentVolumeClaim{}
		if err := cli.Get(context.Background(), client.ObjectKeyFromObject(&claim), current); err != nil || !reflect.DeepEqual(claim, *current) {
			t.Fatal("partial retirement changed Active or Offline storage")
		}
	}
	pvcLifecycleReconcile(t, faulty, 5)
	if deleted != 2 || len(pvcLifecycleClaims(t, cli)) != 2 || len(pvcLifecyclePods(t, cli)) != 1 {
		t.Fatal("fresh retry did not complete retirement and resume")
	}
	for _, claim := range allocated {
		current := &corev1.PersistentVolumeClaim{}
		if err := cli.Get(context.Background(), client.ObjectKeyFromObject(&claim), current); err != nil || !reflect.DeepEqual(claim, *current) {
			t.Fatal("retirement retry changed Active or Offline storage")
		}
	}
}

func TestInstanceSetPVCMixedRetirement(t *testing.T) {
	for _, tc := range []struct {
		management appsv1.PodManagementPolicyType
		policy     kbappsv1.PersistentVolumeClaimRetentionPolicyType
	}{
		{appsv1.OrderedReadyPodManagement, kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType},
		{appsv1.ParallelPodManagement, kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType},
		{appsv1.OrderedReadyPodManagement, kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType},
	} {
		t.Run(string(tc.management)+"/"+string(tc.policy), func(t *testing.T) {
			management, policy := tc.management, tc.policy
			cli := pvcLifecycleFixture(t, 4)
			pvcLifecycleReady(t, cli)
			orphan := &corev1.Pod{}
			if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "pvc-lifecycle-3"}, orphan); err != nil {
				t.Fatal(err)
			}
			if err := cli.Delete(context.Background(), orphan); err != nil {
				t.Fatal(err)
			}
			claims := pvcLifecycleClaims(t, cli)
			unknown := claims[0].DeepCopy()
			unknown.Name = "unassigned-claim"
			unknown.ResourceVersion = ""
			unknown.UID = "unassigned-claim-uid"
			unknown.Labels[constant.KBAppPodNameLabelKey] = ""
			if err := cli.Create(context.Background(), unknown); err != nil {
				t.Fatal(err)
			}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(unknown), unknown); err != nil {
				t.Fatal(err)
			}
			pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.Replicas = ptr.To(int32(1))
				its.Spec.PodManagementPolicy = management
				its.Spec.ParallelPodManagementConcurrency = ptr.To(intstr.FromInt(1))
				its.Spec.PersistentVolumeClaimRetentionPolicy = &kbappsv1.PersistentVolumeClaimRetentionPolicy{WhenScaled: policy}
			})
			pvcLifecycleReconcile(t, cli, 1)
			pods := pvcLifecyclePods(t, cli)
			if len(pods) != 2 {
				t.Fatalf("expected exactly one admitted Pod retirement, got %d", len(pods))
			}
			for _, pod := range pods {
				if pod.Name == "pvc-lifecycle-2" {
					t.Fatal("highest live ordinal was not retired")
				}
			}
			actual := map[string]corev1.PersistentVolumeClaim{}
			for _, claim := range pvcLifecycleClaims(t, cli) {
				actual[claim.Name] = claim
			}
			expectedClaims := 3
			if policy == kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType {
				expectedClaims = 5
			}
			if len(actual) != expectedClaims {
				t.Fatalf("expected %d claims after admitted retirement, got %d", expectedClaims, len(actual))
			}
			for _, claim := range claims {
				current, found := actual[claim.Name]
				retired := policy != kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType && (claim.Name == "data-pvc-lifecycle-2" || claim.Name == "data-pvc-lifecycle-3")
				if retired {
					if found {
						t.Fatalf("retired claim survived: %s", claim.Name)
					}
				} else if !found || !reflect.DeepEqual(claim, current) {
					t.Fatalf("non-admitted or retained claim changed: %s", claim.Name)
				}
			}
			if current, found := actual[unknown.Name]; !found || !reflect.DeepEqual(*unknown, current) {
				t.Fatal("blank identity claim changed")
			}
		})
	}
}

func TestInstanceSetPVCAbsentOwnership(t *testing.T) {
	cli := pvcLifecycleFixture(t, 1)
	owned := pvcLifecycleClaims(t, cli)[0]
	claims := map[string]corev1.PersistentVolumeClaim{}
	for _, ownership := range []string{"detached", "previous-root", "noncontroller"} {
		claim := owned.DeepCopy()
		claim.Name = "foreign-" + ownership
		claim.ResourceVersion = ""
		switch ownership {
		case "detached":
			claim.OwnerReferences = nil
		case "previous-root":
			claim.OwnerReferences[0].UID = "previous-root"
		case "noncontroller":
			claim.OwnerReferences[0].Controller = ptr.To(false)
		}
		if err := cli.Create(context.Background(), claim); err != nil {
			t.Fatal(err)
		}
		if err := cli.Get(context.Background(), client.ObjectKeyFromObject(claim), claim); err != nil {
			t.Fatal(err)
		}
		claims[claim.Name] = *claim
	}
	pods := pvcLifecyclePods(t, cli)
	if err := cli.Delete(context.Background(), &pods[0]); err != nil {
		t.Fatal(err)
	}
	pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Replicas = ptr.To(int32(0)) })
	pvcLifecycleReconcile(t, cli, 4)
	actual := pvcLifecycleClaims(t, cli)
	if len(actual) != len(claims) {
		t.Fatalf("absent cleanup must remove only the current-root controlled claim, got %d claims", len(actual))
	}
	for _, claim := range actual {
		if !reflect.DeepEqual(claim, claims[claim.Name]) {
			t.Fatalf("absent cleanup changed foreign sibling claim %s", claim.Name)
		}
	}
}

func TestInstanceSetPVCReusedClaimAfterIdentityChange(t *testing.T) {
	cli := pvcLifecycleFixture(t, 1)
	pvcLifecycleReady(t, cli)
	for _, initialPod := range pvcLifecyclePods(t, cli) {
		if err := cli.Delete(context.Background(), &initialPod); err != nil {
			t.Fatal(err)
		}
	}
	for _, initialClaim := range pvcLifecycleClaims(t, cli) {
		if err := cli.Delete(context.Background(), &initialClaim); err != nil {
			t.Fatal(err)
		}
	}
	pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Instances = []workloads.InstanceTemplate{{Name: "a-pvc-lifecycle", Replicas: ptr.To(int32(1))}}
	})
	pvcLifecycleReconcile(t, cli, 1)
	pvcLifecycleReady(t, cli)
	pvcLifecycleReconcile(t, cli, 4)
	claims := pvcLifecycleClaims(t, cli)
	pods := pvcLifecyclePods(t, cli)
	if len(claims) != 1 || len(pods) != 1 || pods[0].Name != "pvc-lifecycle-a-pvc-lifecycle-0" {
		t.Fatalf("invalid fixture claims=%#v pods=%#v", claims, pods)
	}
	claim := claims[0]
	claim.UID = "transfer-original-uid"
	claim.Spec.VolumeName = "transfer-original-volume"
	if err := cli.Update(context.Background(), &claim); err != nil {
		t.Fatal(err)
	}
	if err := cli.Delete(context.Background(), &pods[0]); err != nil {
		t.Fatal(err)
	}
	pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Instances = nil
		its.Spec.VolumeClaimTemplates[0].Name = "data-pvc-lifecycle-a"
	})
	pvcLifecycleReconcile(t, cli, 1)
	current := &corev1.PersistentVolumeClaim{}
	err := cli.Get(context.Background(), client.ObjectKeyFromObject(&claim), current)
	if err != nil {
		t.Fatalf("Active claim %s deleted after successful identity adoption: %v", claim.Name, err)
	}
	if current.UID != claim.UID || !reflect.DeepEqual(current.Spec, claim.Spec) || current.Labels[constant.KBAppPodNameLabelKey] != "pvc-lifecycle-0" {
		t.Fatalf("Active claim lost storage or identity: %#v", current)
	}
}
