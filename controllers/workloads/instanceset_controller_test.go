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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
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

	mockPodReady := func(podNames ...string) []*corev1.Pod {
		By("mock pods ready")
		pods := make([]*corev1.Pod, 0)
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
				pods = append(pods, pod)
			})()).Should(Succeed())
		}
		return pods
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

		It("instance status", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.AddConfigs([]workloads.ConfigTemplate{
					{
						Name:       "log",
						ConfigHash: ptr.To("123456"),
					},
					{
						Name:       "server",
						ConfigHash: ptr.To("123456"),
					},
				}...)
			})

			By("check pod config annotation")
			configMap := map[string]string{
				"log":    "123456",
				"server": "123456",
			}
			configVal, err := json.Marshal(configMap)
			Expect(err).NotTo(HaveOccurred())
			podKey := types.NamespacedName{
				Namespace: itsObj.Namespace,
				Name:      fmt.Sprintf("%s-0", itsObj.Name),
			}
			Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.Annotations).Should(HaveKeyWithValue(constant.CMInsConfigurationHashLabelKey, string(configVal)))
			})).Should(Succeed())

			By("check instance status")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(instanceStatusWithoutRevisionAndHealth(its.Status.InstanceStatus[0])).Should(Equal(workloads.InstanceStatus{
					PodName:      fmt.Sprintf("%s-0", itsObj.Name),
					TemplateName: ptr.To(""),
					DesiredState: workloads.InstanceDesiredStateActive,
					CurrentState: workloads.InstanceCurrentStatePresent,
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "log",
							ConfigHash: ptr.To("123456"),
						},
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
						Name:       "log",
						ConfigHash: ptr.To("123456"),
					},
					{
						Name:       "server",
						ConfigHash: ptr.To("123456"),
					},
				}...)
			})

			mockPodReady(fmt.Sprintf("%s-0", itsObj.Name))

			By("check the reconfigure action NOT called")
			Consistently(func(g Gomega) {
				g.Expect(reconfigure).Should(BeEmpty())
				g.Expect(parameters).Should(BeNil())
			}).Should(Succeed())

			By("check the init instance status")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(instanceStatusWithoutRevisionAndHealth(its.Status.InstanceStatus[0])).Should(Equal(workloads.InstanceStatus{
					PodName:      fmt.Sprintf("%s-0", itsObj.Name),
					TemplateName: ptr.To(""),
					DesiredState: workloads.InstanceDesiredStateActive,
					CurrentState: workloads.InstanceCurrentStatePresent,
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "log",
							ConfigHash: ptr.To("123456"),
						},
						{
							Name:       "server",
							ConfigHash: ptr.To("123456"),
						},
					},
				}))
			})).Should(Succeed())

			By("update configs to reconfigure")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Configs[0].ConfigHash = ptr.To("abcdef")
				its.Spec.Configs[0].Reconfigure = testapps.NewLifecycleAction("reconfigure")
				its.Spec.Configs[0].ReconfigureActionName = ""
				its.Spec.Configs[0].Parameters = map[string]string{"foo": "bar"}
			})()).ShouldNot(HaveOccurred())

			By("check the reconfigure action call")
			Eventually(func(g Gomega) {
				g.Expect(reconfigure).Should(Equal("reconfigure"))
				g.Expect(parameters).ShouldNot(BeNil())
				g.Expect(parameters).Should(HaveKeyWithValue("foo", "bar"))
			}).Should(Succeed())

			By("check the instance status updated")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(instanceStatusWithoutRevisionAndHealth(its.Status.InstanceStatus[0])).Should(Equal(workloads.InstanceStatus{
					PodName:      fmt.Sprintf("%s-0", itsObj.Name),
					TemplateName: ptr.To(""),
					DesiredState: workloads.InstanceDesiredStateActive,
					CurrentState: workloads.InstanceCurrentStatePresent,
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "log",
							ConfigHash: ptr.To("abcdef"),
						},
						{
							Name:       "server",
							ConfigHash: ptr.To("123456"),
						},
					},
				}))
			})).Should(Succeed())
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
						Name:       "log",
						ConfigHash: ptr.To("123456"),
					},
					{
						Name:       "server",
						ConfigHash: ptr.To("123456"),
					},
				}...)
			})

			mockPodReady(fmt.Sprintf("%s-0", itsObj.Name))

			By("check the reconfigure action NOT called")
			Consistently(func(g Gomega) {
				g.Expect(reconfigure).Should(BeEmpty())
				g.Expect(parameters).Should(BeNil())
			}).Should(Succeed())

			By("check the init instance status")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(instanceStatusWithoutRevisionAndHealth(its.Status.InstanceStatus[0])).Should(Equal(workloads.InstanceStatus{
					PodName:      fmt.Sprintf("%s-0", itsObj.Name),
					TemplateName: ptr.To(""),
					DesiredState: workloads.InstanceDesiredStateActive,
					CurrentState: workloads.InstanceCurrentStatePresent,
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "log",
							ConfigHash: ptr.To("123456"),
						},
						{
							Name:       "server",
							ConfigHash: ptr.To("123456"),
						},
					},
				}))
			})).Should(Succeed())

			By("update configs to reconfigure")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Configs[0].ConfigHash = ptr.To("abcdef")
				its.Spec.Configs[0].Reconfigure = testapps.NewLifecycleAction("reconfigure")
				its.Spec.Configs[0].ReconfigureActionName = "reconfigure-server"
				its.Spec.Configs[0].Parameters = map[string]string{"foo": "bar"}
			})()).ShouldNot(HaveOccurred())

			By("check the reconfigure action call")
			Eventually(func(g Gomega) {
				g.Expect(reconfigure).Should(ContainSubstring("reconfigure-server"))
				g.Expect(parameters).ShouldNot(BeNil())
				g.Expect(parameters).Should(HaveKeyWithValue("foo", "bar"))
			}).Should(Succeed())

			By("check the instance status updated")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(instanceStatusWithoutRevisionAndHealth(its.Status.InstanceStatus[0])).Should(Equal(workloads.InstanceStatus{
					PodName:      fmt.Sprintf("%s-0", itsObj.Name),
					TemplateName: ptr.To(""),
					DesiredState: workloads.InstanceDesiredStateActive,
					CurrentState: workloads.InstanceCurrentStatePresent,
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "log",
							ConfigHash: ptr.To("abcdef"),
						},
						{
							Name:       "server",
							ConfigHash: ptr.To("123456"),
						},
					},
				}))
			})).Should(Succeed())
		})

		It("restart", func() {
			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.SetInstanceUpdateStrategy(&workloads.InstanceUpdateStrategy{
					Type: kbappsv1.RollingUpdateStrategyType,
				}).AddConfigs([]workloads.ConfigTemplate{
					{
						Name:       "log",
						ConfigHash: ptr.To("123456"),
					},
					{
						Name:       "server",
						ConfigHash: ptr.To("123456"),
					},
				}...)
			})

			pods := mockPodReady(fmt.Sprintf("%s-0", itsObj.Name))
			Expect(pods).Should(HaveLen(1))

			By("check the init instance status")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(instanceStatusWithoutRevisionAndHealth(its.Status.InstanceStatus[0])).Should(Equal(workloads.InstanceStatus{
					PodName:      fmt.Sprintf("%s-0", itsObj.Name),
					TemplateName: ptr.To(""),
					DesiredState: workloads.InstanceDesiredStateActive,
					CurrentState: workloads.InstanceCurrentStatePresent,
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "log",
							ConfigHash: ptr.To("123456"),
						},
						{
							Name:       "server",
							ConfigHash: ptr.To("123456"),
						},
					},
				}))
			})).Should(Succeed())

			By("update configs to restart")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Configs[0].ConfigHash = ptr.To("abcdef")
				its.Spec.Configs[0].Restart = ptr.To(true)
			})()).ShouldNot(HaveOccurred())

			By("check the instance restarted")
			podKey := client.ObjectKeyFromObject(pods[0])
			Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.UID).ShouldNot(Equal(pods[0].UID))
			})).Should(Succeed())

			By("check the instance status updated")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(instanceStatusWithoutRevisionAndHealth(its.Status.InstanceStatus[0])).Should(Equal(workloads.InstanceStatus{
					PodName:      fmt.Sprintf("%s-0", itsObj.Name),
					TemplateName: ptr.To(""),
					DesiredState: workloads.InstanceDesiredStateActive,
					CurrentState: workloads.InstanceCurrentStatePresent,
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "log",
							ConfigHash: ptr.To("abcdef"),
						},
						{
							Name:       "server",
							ConfigHash: ptr.To("123456"),
						},
					},
				}))
			})).Should(Succeed())
		})

		It("reconfigure & restart", func() {
			By("mock reconfigure action calls")
			var (
				reconfigure          string
				parameters           map[string]string
				reconfigureTimestamp time.Time
			)
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req kbaproto.ActionRequest) (kbaproto.ActionResponse, error) {
					if req.Action == "reconfigure" || strings.HasPrefix(req.Action, "udf-reconfigure") {
						reconfigure = req.Action
						parameters = req.Parameters
						reconfigureTimestamp = time.Now()
						time.Sleep(1200 * time.Millisecond) // The precision of pod creationTimestamp is only in seconds.
					}
					return kbaproto.ActionResponse{}, nil
				}).AnyTimes()
			})

			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.SetInstanceUpdateStrategy(&workloads.InstanceUpdateStrategy{
					Type: kbappsv1.RollingUpdateStrategyType,
				}).AddConfigs([]workloads.ConfigTemplate{
					{
						Name:       "log",
						ConfigHash: ptr.To("123456"),
					},
					{
						Name:       "server",
						ConfigHash: ptr.To("123456"),
					},
				}...)
			})

			pods := mockPodReady(fmt.Sprintf("%s-0", itsObj.Name))
			Expect(pods).Should(HaveLen(1))

			By("check the reconfigure action NOT called")
			Consistently(func(g Gomega) {
				g.Expect(reconfigure).Should(BeEmpty())
				g.Expect(parameters).Should(BeNil())
			}).Should(Succeed())

			By("check the init instance status")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(instanceStatusWithoutRevisionAndHealth(its.Status.InstanceStatus[0])).Should(Equal(workloads.InstanceStatus{
					PodName:      fmt.Sprintf("%s-0", itsObj.Name),
					TemplateName: ptr.To(""),
					DesiredState: workloads.InstanceDesiredStateActive,
					CurrentState: workloads.InstanceCurrentStatePresent,
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "log",
							ConfigHash: ptr.To("123456"),
						},
						{
							Name:       "server",
							ConfigHash: ptr.To("123456"),
						},
					},
				}))
			})).Should(Succeed())

			By("update configs to reconfigure and restart")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Configs[0].ConfigHash = ptr.To("abcdef")
				its.Spec.Configs[0].Restart = ptr.To(true)
				its.Spec.Configs[0].Reconfigure = testapps.NewLifecycleAction("reconfigure")
				its.Spec.Configs[0].ReconfigureActionName = ""
				its.Spec.Configs[0].Parameters = map[string]string{"foo": "bar"}
			})()).ShouldNot(HaveOccurred())

			By("check the reconfigure action call")
			Eventually(func(g Gomega) {
				g.Expect(reconfigure).Should(Equal("reconfigure"))
				g.Expect(parameters).ShouldNot(BeNil())
				g.Expect(parameters).Should(HaveKeyWithValue("foo", "bar"))
			}).Should(Succeed())

			By("check the instance restarted")
			podKey := client.ObjectKeyFromObject(pods[0])
			Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.UID).ShouldNot(Equal(pods[0].UID))
				// Check that the new pod is created after reconfigure action called.
				g.Expect(pod.CreationTimestamp.Time.After(reconfigureTimestamp)).Should(BeTrue())
			})).Should(Succeed())

			By("check the instance status updated")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(instanceStatusWithoutRevisionAndHealth(its.Status.InstanceStatus[0])).Should(Equal(workloads.InstanceStatus{
					PodName:      fmt.Sprintf("%s-0", itsObj.Name),
					TemplateName: ptr.To(""),
					DesiredState: workloads.InstanceDesiredStateActive,
					CurrentState: workloads.InstanceCurrentStatePresent,
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "log",
							ConfigHash: ptr.To("abcdef"),
						},
						{
							Name:       "server",
							ConfigHash: ptr.To("123456"),
						},
					},
				}))
			})).Should(Succeed())
		})

		It("reconfigure and restart", func() {
			By("mock reconfigure action calls")
			var (
				cnt                  int32
				reconfigure          string
				parameters           map[string]string
				reconfigureTimestamp time.Time
			)
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, req kbaproto.ActionRequest) (kbaproto.ActionResponse, error) {
					if req.Action == "reconfigure" || strings.HasPrefix(req.Action, "udf-reconfigure") {
						cnt += 1
						reconfigure = req.Action
						parameters = req.Parameters
						reconfigureTimestamp = time.Now()
						time.Sleep(1200 * time.Millisecond) // The precision of pod creationTimestamp is only in seconds.
					}
					return kbaproto.ActionResponse{}, nil
				}).AnyTimes()
			})

			createITSObj(itsName, func(f *testapps.MockInstanceSetFactory) {
				f.SetInstanceUpdateStrategy(&workloads.InstanceUpdateStrategy{
					Type: kbappsv1.RollingUpdateStrategyType,
				}).AddConfigs([]workloads.ConfigTemplate{
					{
						Name:       "client",
						ConfigHash: ptr.To("123456"),
					},
					{
						Name:       "log",
						ConfigHash: ptr.To("123456"),
					},
					{
						Name:       "server",
						ConfigHash: ptr.To("123456"),
					},
				}...)
			})

			pods := mockPodReady(fmt.Sprintf("%s-0", itsObj.Name))
			Expect(pods).Should(HaveLen(1))

			By("check the reconfigure action NOT called")
			Consistently(func(g Gomega) {
				g.Expect(cnt).Should(Equal(int32(0)))
				g.Expect(reconfigure).Should(BeEmpty())
				g.Expect(parameters).Should(BeNil())
			}).Should(Succeed())

			By("check the init instance status")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(instanceStatusWithoutRevisionAndHealth(its.Status.InstanceStatus[0])).Should(Equal(workloads.InstanceStatus{
					PodName:      fmt.Sprintf("%s-0", itsObj.Name),
					TemplateName: ptr.To(""),
					DesiredState: workloads.InstanceDesiredStateActive,
					CurrentState: workloads.InstanceCurrentStatePresent,
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "client",
							ConfigHash: ptr.To("123456"),
						},
						{
							Name:       "log",
							ConfigHash: ptr.To("123456"),
						},
						{
							Name:       "server",
							ConfigHash: ptr.To("123456"),
						},
					},
				}))
			})).Should(Succeed())

			By("update configs to reconfigure and restart")
			Expect(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Configs[0].ConfigHash = ptr.To("abcdef")
				its.Spec.Configs[0].Reconfigure = testapps.NewLifecycleAction("reconfigure")
				its.Spec.Configs[0].ReconfigureActionName = ""
				its.Spec.Configs[0].Parameters = map[string]string{"foo": "bar"}
				its.Spec.Configs[1].ConfigHash = ptr.To("abcdef")
				its.Spec.Configs[1].Restart = ptr.To(true)
				its.Spec.Configs[2].ConfigHash = ptr.To("abcdef")
				its.Spec.Configs[2].Restart = ptr.To(true)
				its.Spec.Configs[2].Reconfigure = testapps.NewLifecycleAction("reconfigure")
				its.Spec.Configs[2].ReconfigureActionName = ""
				its.Spec.Configs[2].Parameters = map[string]string{"foo": "bar"}
			})()).ShouldNot(HaveOccurred())

			By("check the reconfigure action call")
			Eventually(func(g Gomega) {
				g.Expect(cnt).Should(Equal(int32(2)))
				g.Expect(reconfigure).Should(Equal("reconfigure"))
				g.Expect(parameters).ShouldNot(BeNil())
				g.Expect(parameters).Should(HaveKeyWithValue("foo", "bar"))
			}).Should(Succeed())

			By("check the instance restarted")
			podKey := client.ObjectKeyFromObject(pods[0])
			Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.UID).ShouldNot(Equal(pods[0].UID))
				// Check that the new pod is created after reconfigure action called.
				g.Expect(pod.CreationTimestamp.Time.After(reconfigureTimestamp)).Should(BeTrue())
			})).Should(Succeed())

			By("check the instance status updated")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.Status.InstanceStatus).Should(HaveLen(1))
				g.Expect(instanceStatusWithoutRevisionAndHealth(its.Status.InstanceStatus[0])).Should(Equal(workloads.InstanceStatus{
					PodName:      fmt.Sprintf("%s-0", itsObj.Name),
					TemplateName: ptr.To(""),
					DesiredState: workloads.InstanceDesiredStateActive,
					CurrentState: workloads.InstanceCurrentStatePresent,
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       "client",
							ConfigHash: ptr.To("abcdef"),
						},
						{
							Name:       "log",
							ConfigHash: ptr.To("abcdef"),
						},
						{
							Name:       "server",
							ConfigHash: ptr.To("abcdef"),
						},
					},
				}))
			})).Should(Succeed())
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

	Context("deferred update", func() {
		It("handles serviceaccount update", func() {
			createITSObj(itsName, func(factory *testapps.MockInstanceSetFactory) {
				factory.SetInstanceUpdateStrategy(&workloads.InstanceUpdateStrategy{
					Type: kbappsv1.RollingUpdateStrategyType,
				})
			})
			By("update proposed sa name")
			newSAName := "new-sa"
			Eventually(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				if its.Annotations == nil {
					its.Annotations = map[string]string{}
				}
				its.Annotations[constant.ProposedServiceAccountNameAnnotationKey] = newSAName
			})).Should(Succeed())
			podsKey := []types.NamespacedName{
				{
					Namespace: itsObj.Namespace,
					Name:      fmt.Sprintf("%s-0", itsObj.Name),
				},
				{
					Namespace: itsObj.Namespace,
					Name:      fmt.Sprintf("%s-1", itsObj.Name),
				},
			}
			mockPodReady(podsKey[0].Name)
			Consistently(testapps.CheckObj(&testCtx, podsKey[0], func(g Gomega, pod *corev1.Pod) {
				// default sa name is empty
				g.Expect(pod.Spec.ServiceAccountName).Should(BeEmpty())
			})).Should(Succeed())
			By("check scale out does not affect current pod")
			Eventually(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Replicas = ptr.To[int32](2)
			})).Should(Succeed())
			Consistently(testapps.CheckObj(&testCtx, podsKey[0], func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.Spec.ServiceAccountName).Should(BeEmpty())
			})).Should(Succeed())
			By("check new pod uses new sa name")
			mockPodReady(podsKey[1].Name)
			Eventually(testapps.CheckObj(&testCtx, podsKey[1], func(g Gomega, pod *corev1.Pod) {
				g.Expect(pod.Spec.ServiceAccountName).Should(Equal(newSAName))
			})).Should(Succeed())

			beforeUpdate := time.Now()
			time.Sleep(time.Second)
			By("update its spec")
			Eventually(testapps.GetAndChangeObj(&testCtx, itsKey, func(its *workloads.InstanceSet) {
				its.Spec.Template.Spec.Containers[0].Command = []string{"new-command"}
			})).Should(Succeed())

			replicas := 2
			for i := replicas; i > 0; i-- {
				By("wait new pod created")
				podKey := podsKey[i-1]
				Eventually(testapps.CheckObj(&testCtx, podKey, func(g Gomega, pod *corev1.Pod) {
					g.Expect(pod.CreationTimestamp.After(beforeUpdate)).Should(BeTrue())
					g.Expect(pod.Spec.ServiceAccountName).Should(Equal(newSAName))
				})).Should(Succeed())

				mockPodReady(podKey.Name)
			}

			By("check its ready")
			Eventually(testapps.CheckObj(&testCtx, itsKey, func(g Gomega, its *workloads.InstanceSet) {
				g.Expect(its.IsInstanceSetReady()).Should(BeTrue())
				g.Expect(its.Annotations).Should(HaveKeyWithValue(constant.ServiceAccountInUseAnnotationKey, newSAName))
			})).Should(Succeed())
		})
	})
})

func instanceStatusWithoutRevisionAndHealth(status workloads.InstanceStatus) workloads.InstanceStatus {
	status.CurrentRevision = ""
	status.UpdateRevision = ""
	status.UpToDate = false
	status.Ready = false
	status.Available = false
	status.Failed = false
	return status
}

func TestInstanceSetFlatOrdinalTransfer(t *testing.T) {
	for _, swap := range []bool{false, true} {
		for _, failStatus := range []bool{false, true} {
			t.Run(fmt.Sprintf("swap=%t/statusFailure=%t", swap, failStatus), func(t *testing.T) {
				scheme := runtime.NewScheme()
				for _, add := range []func(*runtime.Scheme) error{clientgoscheme.AddToScheme, workloads.AddToScheme} {
					if err := add(scheme); err != nil {
						t.Fatal(err)
					}
				}
				model.AddScheme(workloads.AddToScheme)
				its := &workloads.InstanceSet{
					ObjectMeta: metav1.ObjectMeta{Name: "flat-transfer", Namespace: "default", UID: "its-uid", Generation: 1},
					Spec: workloads.InstanceSetSpec{
						Replicas: ptr.To(int32(1)), FlatInstanceOrdinal: true,
						PodManagementPolicy:    appsv1.ParallelPodManagement,
						InstanceUpdateStrategy: &workloads.InstanceUpdateStrategy{Type: kbappsv1.OnDeleteStrategyType},
						Selector:               &metav1.LabelSelector{MatchLabels: map[string]string{"app": "flat-transfer"}},
						Template:               corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "flat-transfer"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}}},
						Instances:              []workloads.InstanceTemplate{{Name: "a", Replicas: ptr.To(int32(1)), Ordinals: workloads.Ordinals{Discrete: []int32{0}}}},
						VolumeClaimTemplates:   []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}, Spec: corev1.PersistentVolumeClaimSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}}}}},
					},
				}
				if swap {
					its.Spec.Replicas = ptr.To(int32(2))
					its.Spec.Instances = append(its.Spec.Instances, workloads.InstanceTemplate{Name: "b", Replicas: ptr.To(int32(1)), Ordinals: workloads.Ordinals{Discrete: []int32{1}}})
					its.Spec.InstanceUpdateStrategy = nil
				}
				cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&workloads.InstanceSet{}, &corev1.Pod{}, &corev1.PersistentVolumeClaim{}).WithObjects(its).Build()
				key := client.ObjectKeyFromObject(its)
				reconcile := func(c client.Client) error {
					_, err := (&InstanceSetReconciler{Client: c, Scheme: scheme, Recorder: record.NewFakeRecorder(100)}).Reconcile(context.Background(), ctrl.Request{NamespacedName: key})
					return err
				}
				run := func() {
					t.Helper()
					if err := reconcile(cli); err != nil {
						t.Fatal(err)
					}
				}
				for i := 0; i < 3; i++ {
					run()
				}
				pod := &corev1.Pod{}
				if err := cli.Get(context.Background(), client.ObjectKey{Namespace: key.Namespace, Name: its.Name + "-0"}, pod); err != nil {
					t.Fatal(err)
				}
				pod.Status.Phase = corev1.PodRunning
				pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}
				if err := cli.Status().Update(context.Background(), pod); err != nil {
					t.Fatal(err)
				}
				pod.Finalizers = []string{"test/hold"}
				if err := cli.Update(context.Background(), pod); err != nil {
					t.Fatal(err)
				}
				if err := cli.Delete(context.Background(), pod); err != nil {
					t.Fatal(err)
				}
				claim := &corev1.PersistentVolumeClaim{}
				if err := cli.Get(context.Background(), client.ObjectKey{Namespace: key.Namespace, Name: "data-" + pod.Name}, claim); err != nil {
					t.Fatal(err)
				}
				claim.UID = "claim-uid"
				claim.Spec.VolumeName = "bound-volume"
				if err := cli.Update(context.Background(), claim); err != nil {
					t.Fatal(err)
				}
				if err := cli.Get(context.Background(), key, its); err != nil {
					t.Fatal(err)
				}
				if swap {
					its.Spec.Instances[0].Ordinals.Discrete = []int32{1}
					its.Spec.Instances[1].Ordinals.Discrete = []int32{0}
				} else {
					its.Spec.Instances[0].Name = "b"
				}
				its.Generation++
				if err := cli.Update(context.Background(), its); err != nil {
					t.Fatal(err)
				}
				expectedTemplate := "b"
				if failStatus {
					injected := errors.New("injected status failure")
					faulty := interceptor.NewClient(cli, interceptor.Funcs{SubResourceUpdate: func(ctx context.Context, c client.Client, sub string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
						if _, ok := obj.(*workloads.InstanceSet); ok && sub == "status" {
							return injected
						}
						return c.SubResource(sub).Update(ctx, obj, opts...)
					}})
					if err := reconcile(faulty); !errors.Is(err, injected) {
						t.Fatalf("expected status failure, got %v", err)
					}
					persisted := &workloads.InstanceSet{}
					if err := cli.Get(context.Background(), key, persisted); err != nil {
						t.Fatal(err)
					}
					if persisted.Status.ObservedGeneration == its.Generation {
						t.Fatal("status failure was not persisted as an incomplete commit")
					}
					persistedClaim := &corev1.PersistentVolumeClaim{}
					if err := cli.Get(context.Background(), client.ObjectKeyFromObject(claim), persistedClaim); err != nil {
						t.Fatal(err)
					}
					if persistedClaim.Labels[constant.KBAppInstanceTemplateLabelKey] != "b" {
						t.Fatal("fixture did not commit PVC transfer before status failure")
					}
					if !swap {
						its.Spec.Instances[0].Name = "c"
						its.Generation++
						if err := cli.Update(context.Background(), its); err != nil {
							t.Fatal(err)
						}
						expectedTemplate = "c"
					}
				}
				for i := 0; i < 3; i++ {
					run()
				}
				current := &corev1.PersistentVolumeClaim{}
				if err := cli.Get(context.Background(), client.ObjectKeyFromObject(claim), current); err != nil {
					t.Fatalf("allocated claim was deleted: %v", err)
				}
				if current.UID != claim.UID || current.Spec.VolumeName != claim.Spec.VolumeName || !reflect.DeepEqual(current.OwnerReferences, claim.OwnerReferences) || !current.DeletionTimestamp.IsZero() {
					t.Fatal("allocated storage identity changed")
				}
				if err := cli.Get(context.Background(), key, its); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(its.Status.AssignedOrdinals[expectedTemplate].Discrete, []int32{0}) || len(its.Status.AssignedOrdinals) != len(its.Spec.Instances) {
					t.Fatalf("ordinal did not transfer: %#v", its.Status.AssignedOrdinals)
				}
				if err := cli.Get(context.Background(), client.ObjectKeyFromObject(pod), pod); err != nil {
					t.Fatal(err)
				}
				if pod.DeletionTimestamp.IsZero() {
					t.Fatal("fixture did not retain terminating Pod")
				}
				pod.Finalizers = nil
				if err := cli.Update(context.Background(), pod); err != nil {
					t.Fatal(err)
				}
				for i := 0; i < 3; i++ {
					run()
				}
				if err := cli.Get(context.Background(), client.ObjectKeyFromObject(pod), pod); err != nil {
					t.Fatalf("Pod did not recover: %v", err)
				}
				if !pod.DeletionTimestamp.IsZero() || pod.Labels[instancetemplate.TemplateNameLabelKey] != expectedTemplate {
					t.Fatal("replacement Pod did not use desired template")
				}
				if err := cli.Get(context.Background(), client.ObjectKeyFromObject(claim), current); err != nil {
					t.Fatal(err)
				}
				if current.UID != claim.UID || current.Spec.VolumeName != claim.Spec.VolumeName || !reflect.DeepEqual(current.OwnerReferences, claim.OwnerReferences) {
					t.Fatal("replacement lost retained storage")
				}
			})
		}
	}
}

func TestInstanceSetStopDuringFlatOrdinalTransfer(t *testing.T) {
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{clientgoscheme.AddToScheme, workloads.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "stop-transfer", Namespace: "default", UID: "its-uid", Generation: 1}, Spec: workloads.InstanceSetSpec{
		Replicas: ptr.To(int32(2)), FlatInstanceOrdinal: true, PodManagementPolicy: appsv1.ParallelPodManagement,
		InstanceUpdateStrategy: &workloads.InstanceUpdateStrategy{Type: kbappsv1.OnDeleteStrategyType},
		Selector:               &metav1.LabelSelector{MatchLabels: map[string]string{"app": "stop-transfer"}},
		Template:               corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "stop-transfer"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}}},
		Instances:              []workloads.InstanceTemplate{{Name: "a", Replicas: ptr.To(int32(2)), Ordinals: workloads.Ordinals{Discrete: []int32{0, 1}}}},
		VolumeClaimTemplates:   []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}, Spec: corev1.PersistentVolumeClaimSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}}}}},
	}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&workloads.InstanceSet{}, &corev1.Pod{}, &corev1.PersistentVolumeClaim{}).WithObjects(its).Build()
	key := client.ObjectKeyFromObject(its)
	reconcile := func() error {
		_, err := (&InstanceSetReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(100)}).Reconcile(context.Background(), ctrl.Request{NamespacedName: key})
		return err
	}
	for i := 0; i < 3; i++ {
		if err := reconcile(); err != nil {
			t.Fatal(err)
		}
	}
	claimsBefore := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(context.Background(), claimsBefore); err != nil {
		t.Fatal(err)
	}
	if err := cli.Get(context.Background(), key, its); err != nil {
		t.Fatal(err)
	}
	its.Spec.Instances[0].Name = "b"
	its.Spec.Stop = ptr.To(true)
	its.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
	its.Generation++
	if err := cli.Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := reconcile(); err != nil {
			pod := &corev1.Pod{}
			podErr := cli.Get(context.Background(), client.ObjectKey{Namespace: key.Namespace, Name: "stop-transfer-0"}, pod)
			t.Fatalf("Stop round %d failed; remaining Pod get=%v deletion=%v: %v", i, podErr, pod.DeletionTimestamp, err)
		}
	}
	pods := &corev1.PodList{}
	if err := cli.List(context.Background(), pods); err != nil {
		t.Fatal(err)
	}
	if len(pods.Items) != 0 {
		t.Fatalf("Stop did not drain all Pods: %#v", pods.Items)
	}
	claimsAfter := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(context.Background(), claimsAfter); err != nil {
		t.Fatal(err)
	}
	if len(claimsBefore.Items) != 2 || !reflect.DeepEqual(claimsBefore.Items, claimsAfter.Items) {
		t.Fatal("Stop changed retained storage")
	}
	if err := cli.Get(context.Background(), key, its); err != nil {
		t.Fatal(err)
	}
	if len(its.Status.InstanceStatus) != 2 {
		t.Fatalf("incomplete stopped identities: %#v", its.Status)
	}
	for _, status := range its.Status.InstanceStatus {
		if status.TemplateName == nil || *status.TemplateName != "a" || status.DesiredState != workloads.InstanceDesiredStateOffline || status.CurrentState != workloads.InstanceCurrentStateAbsent {
			t.Fatalf("Stop lost historical template identity: %#v", status)
		}
	}
	its.Spec.Stop = ptr.To(false)
	its.Spec.PodManagementPolicy = appsv1.ParallelPodManagement
	its.Generation++
	if err := cli.Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := reconcile(); err != nil {
			t.Fatal(err)
		}
	}
	if err := cli.List(context.Background(), pods); err != nil {
		t.Fatal(err)
	}
	if len(pods.Items) != 2 {
		t.Fatal("resume did not recreate both transferred identities")
	}
	for _, pod := range pods.Items {
		if pod.Labels[instancetemplate.TemplateNameLabelKey] != "b" {
			t.Fatal("resume used obsolete template")
		}
	}
}

func TestInstanceAPIStopRetainsTransferredIdentity(t *testing.T) {
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{clientgoscheme.AddToScheme, workloads.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	model.AddScheme(workloads.AddToScheme)
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "instance-stop-transfer", Namespace: "default", UID: "its-uid", Generation: 1},
		Spec: workloads.InstanceSetSpec{
			EnableInstanceAPI: ptr.To(true), Replicas: ptr.To(int32(1)), FlatInstanceOrdinal: true,
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Selector:            &metav1.LabelSelector{MatchLabels: map[string]string{"app": "instance-stop-transfer"}},
			Template:            corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "instance-stop-transfer"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}}},
			Instances:           []workloads.InstanceTemplate{{Name: "a", Replicas: ptr.To(int32(1)), Ordinals: workloads.Ordinals{Discrete: []int32{0}}}},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&workloads.InstanceSet{}, &workloads.Instance{}).WithObjects(its).Build()
	key := client.ObjectKeyFromObject(its)
	run := func() {
		t.Helper()
		_, err := (&InstanceSetReconciler2{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(100)}).Reconcile(context.Background(), ctrl.Request{NamespacedName: key})
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 3; i++ {
		run()
	}
	inst := &workloads.Instance{}
	instKey := client.ObjectKey{Namespace: key.Namespace, Name: its.Name + "-0"}
	if err := cli.Get(context.Background(), instKey, inst); err != nil {
		t.Fatal(err)
	}
	inst.UID = "retained-instance-uid"
	if err := cli.Update(context.Background(), inst); err != nil {
		t.Fatal(err)
	}
	inst.Status.ObservedGeneration = inst.Generation
	inst.Status.CurrentState = workloads.InstanceCurrentStatePresent
	inst.Status.Ready, inst.Status.Available = true, true
	inst.Status.Conditions = []metav1.Condition{
		{Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue},
		{Type: string(workloads.InstanceAvailable), Status: metav1.ConditionTrue},
	}
	if err := cli.Status().Update(context.Background(), inst); err != nil {
		t.Fatal(err)
	}
	run()
	if err := cli.Get(context.Background(), key, its); err != nil {
		t.Fatal(err)
	}
	its.Spec.Instances[0].Name = "b"
	its.Spec.Stop = ptr.To(true)
	its.Generation++
	if err := cli.Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		run()
	}
	if err := cli.Get(context.Background(), key, its); err != nil {
		t.Fatal(err)
	}
	status := its.FindInstanceStatus(instKey.Name)
	if status == nil || status.TemplateName == nil || *status.TemplateName != "a" || status.DesiredState != workloads.InstanceDesiredStateOffline || status.CurrentState != workloads.InstanceCurrentStatePresent || status.UpToDate {
		t.Fatalf("Stop lost retained parent status: %#v", status)
	}
	if err := cli.Get(context.Background(), instKey, inst); err != nil {
		t.Fatal(err)
	}
	if inst.UID != "retained-instance-uid" || !inst.DeletionTimestamp.IsZero() || ptr.Deref(inst.Spec.ScaledDown, false) {
		t.Fatal("Stop replaced or retired allocated Instance")
	}
	its.Spec.Stop = ptr.To(false)
	its.Generation++
	if err := cli.Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		run()
	}
	if err := cli.Get(context.Background(), key, its); err != nil {
		t.Fatal(err)
	}
	status = its.FindInstanceStatus(instKey.Name)
	if status == nil || status.TemplateName == nil || *status.TemplateName != "b" || status.DesiredState != workloads.InstanceDesiredStateActive || status.CurrentState != workloads.InstanceCurrentStatePresent {
		t.Fatalf("resume lost desired parent status: %#v", status)
	}
	if err := cli.Get(context.Background(), instKey, inst); err != nil {
		t.Fatal(err)
	}
	if inst.UID != "retained-instance-uid" || inst.Spec.InstanceTemplateName != "b" {
		t.Fatal("resume replaced identity or missed desired template")
	}
}
