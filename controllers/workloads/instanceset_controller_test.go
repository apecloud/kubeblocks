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
	"net"
	"os"
	"path/filepath"
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
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	workloadlifecycle "github.com/apecloud/kubeblocks/pkg/controller/workloads/lifecycle"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
	"github.com/apecloud/kubeblocks/pkg/kbagent"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	kbaproto "github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	kbaserver "github.com/apecloud/kubeblocks/pkg/kbagent/server"
	kbaservice "github.com/apecloud/kubeblocks/pkg/kbagent/service"
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
					Provisioned:  true,
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
					Provisioned:  true,
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
					Provisioned:  true,
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
					Provisioned:  true,
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
					Provisioned:  true,
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
					Provisioned:  true,
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
					Provisioned:  true,
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
					Provisioned:  true,
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
					Provisioned:  true,
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
					Provisioned:  true,
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
					Provisioned:  true,
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

func TestInstanceSetReconcilePersistsTrackedFactsAfterRuntimeRemoval(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		t.Run(map[bool]string{false: "Pod runtime", true: "Instance runtime"}[instanceAPI], func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, workloads.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "tracked", Namespace: "default", UID: "its", Generation: 1,
				Finalizers: []string{"instanceset.workloads.kubeblocks.io/finalizer"}},
				Spec: workloads.InstanceSetSpec{Replicas: ptr.To[int32](0), EnableInstanceAPI: ptr.To(instanceAPI),
					Selector:         &metav1.LabelSelector{MatchLabels: map[string]string{"app": "tracked"}},
					Template:         corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "mysql"}}}},
					OfflineInstances: []string{"tracked-0", "tracked-1", "tracked-2"}}}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&workloads.InstanceSet{}, &workloads.Instance{}, &corev1.Pod{}).WithObjects(its).Build()
			key := client.ObjectKeyFromObject(its)
			if err := cli.Get(ctx, key, its); err != nil {
				t.Fatal(err)
			}
			facts := []*bool{nil, ptr.To(false), ptr.To(true)}
			its.Status.ObservedGeneration = its.Generation
			for i, name := range its.Spec.OfflineInstances {
				its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{PodName: name, DataLoaded: facts[i], MemberJoined: facts[i],
					CurrentState: workloads.InstanceCurrentStatePresent, CurrentRevision: "stale", Ready: true, Available: true, Failed: true, Role: "stale"})
			}
			if err := cli.Status().Update(ctx, its); err != nil {
				t.Fatal(err)
			}
			owner := []metav1.OwnerReference{{APIVersion: workloads.GroupVersion.String(), Kind: workloads.InstanceSetKind, Name: its.Name, UID: its.UID, Controller: ptr.To(true)}}
			meta := metav1.ObjectMeta{Name: "tracked-0", Namespace: its.Namespace, Labels: instanceset.GetMatchLabels(its.Name), OwnerReferences: owner}
			var observed client.Object
			if instanceAPI {
				observed = &workloads.Instance{ObjectMeta: meta}
			} else {
				meta.Labels[appsv1.ControllerRevisionHashLabelKey] = "observed"
				observed = &corev1.Pod{ObjectMeta: meta, Spec: its.Spec.Template.Spec}
			}
			if err := cli.Create(ctx, observed); err != nil {
				t.Fatal(err)
			}
			if instanceAPI {
				inst := observed.(*workloads.Instance)
				inst.Status = workloads.InstanceStatus2{CurrentState: workloads.InstanceCurrentStatePresent, CurrentRevision: "observed", Ready: true, Available: true}
				if err := cli.Status().Update(ctx, inst); err != nil {
					t.Fatal(err)
				}
			}
			reconcile := func() {
				t.Helper()
				recorder := record.NewFakeRecorder(100)
				var err error
				if instanceAPI {
					_, err = (&InstanceSetReconciler2{Client: cli, Scheme: scheme, Recorder: recorder}).Reconcile(ctx, ctrl.Request{NamespacedName: key})
				} else {
					_, err = (&InstanceSetReconciler{Client: cli, Scheme: scheme, Recorder: recorder}).Reconcile(ctx, ctrl.Request{NamespacedName: key})
				}
				if err != nil {
					t.Fatal(err)
				}
				if err := cli.Get(ctx, key, its); err != nil {
					t.Fatal(err)
				}
			}
			reconcile()
			assertPersistedTrackedFacts(t, its, facts)
			status := its.FindInstanceStatus("tracked-0")
			if status == nil || !status.Provisioned || status.CurrentState != workloads.InstanceCurrentStatePresent || status.CurrentRevision != "observed" {
				t.Fatalf("Reconcile did not commit the observed runtime fact: %#v", status)
			}
			if err := client.IgnoreNotFound(cli.Delete(ctx, observed)); err != nil {
				t.Fatal(err)
			}
			reconcile()
			assertPersistedTrackedFacts(t, its, facts)
			status = its.FindInstanceStatus("tracked-0")
			if status == nil || !status.Provisioned || status.CurrentState != workloads.InstanceCurrentStateAbsent || status.CurrentRevision != "" || status.Ready || status.Available || status.Failed || status.Role != "" {
				t.Fatalf("fresh reconciler did not retain facts and clear runtime fields: %#v", status)
			}
			its.Spec.OfflineInstances = nil
			its.Generation++
			if err := cli.Update(ctx, its); err != nil {
				t.Fatal(err)
			}
			reconcile()
			reconcile()
			if len(its.Status.InstanceStatus) != 1 {
				t.Fatalf("released nil/false membership retained history: %#v", its.Status.InstanceStatus)
			}
			joined := its.FindInstanceStatus("tracked-2")
			if joined == nil || joined.DesiredState != workloads.InstanceDesiredStateReleased || joined.CurrentState != workloads.InstanceCurrentStateAbsent || !ptr.Deref(joined.DataLoaded, false) || !ptr.Deref(joined.MemberJoined, false) {
				t.Fatalf("recorded membership was lost after allocation removal: %#v", joined)
			}
			joined.MemberJoined = ptr.To(false)
			if err := cli.Status().Update(ctx, its); err != nil {
				t.Fatal(err)
			}
			reconcile()
			if len(its.Status.InstanceStatus) != 0 {
				t.Fatalf("recorded leave did not release persisted history: %#v", its.Status.InstanceStatus)
			}
		})
	}
}

func newDataLifecycleITS(replicas int32) *workloads.InstanceSet {
	return &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "data-lifecycle", Namespace: "default", UID: "data-lifecycle", Generation: 1,
			Finalizers: []string{"instanceset.workloads.kubeblocks.io/finalizer"}},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To(replicas), Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "data-lifecycle"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "data-lifecycle"}}, Spec: corev1.PodSpec{
				ServiceAccountName: "data-loader", Containers: []corev1.Container{{Name: "db", Image: "database", VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}}}},
			}},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}, Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
			}}},
			LifecycleActions: &workloads.LifecycleActions{
				DataDump: &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"dump"}}},
				DataLoad: &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"load"}}}, DataVolume: "data",
				Worker: &corev1.Container{Name: "worker-context", Image: "tools", VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}}},
			},
		},
	}
}

func newLifecycleEntryClient(t *testing.T, its *workloads.InstanceSet, funcs interceptor.Funcs) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, workloads.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&workloads.InstanceSet{}, &corev1.Pod{}).WithObjects(its).WithInterceptorFuncs(funcs).Build()
}

func reconcileLifecycleITS(t *testing.T, cli client.Client, key client.ObjectKey) (*workloads.InstanceSet, ctrl.Result, error) {
	t.Helper()
	// Construct a new reconciler on every invocation to exercise restart recovery.
	r := &InstanceSetReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(100)}
	result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: key})
	stored := &workloads.InstanceSet{}
	if getErr := cli.Get(context.Background(), key, stored); getErr != nil {
		t.Fatal(getErr)
	}
	return stored, result, err
}

func lifecycleEntryPods(t *testing.T, cli client.Client) []corev1.Pod {
	t.Helper()
	list := &corev1.PodList{}
	if err := cli.List(context.Background(), list); err != nil {
		t.Fatal(err)
	}
	return list.Items
}

func TestInstanceSetLifecycleRegistrationPrecedesRuntimeCreation(t *testing.T) {
	its := newDataLifecycleITS(2)
	rejectStatus := true
	cli := newLifecycleEntryClient(t, its, interceptor.Funcs{SubResourceUpdate: func(ctx context.Context, cli client.Client, subresource string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
		if subresource == "status" && rejectStatus {
			return apierrors.NewConflict(schema.GroupResource{Group: workloads.GroupVersion.Group, Resource: "instancesets"}, obj.GetName(), fmt.Errorf("injected status conflict"))
		}
		return cli.SubResource(subresource).Update(ctx, obj, opts...)
	}})
	key := client.ObjectKeyFromObject(its)
	stored, result, err := reconcileLifecycleITS(t, cli, key)
	if err != nil || !result.Requeue || len(stored.Status.InstanceStatus) != 0 || len(lifecycleEntryPods(t, cli)) != 0 {
		t.Fatalf("failed registration created runtime or committed partial facts: status=%+v pods=%+v err=%v", stored.Status, lifecycleEntryPods(t, cli), err)
	}
	rejectStatus = false
	stored, _, err = reconcileLifecycleITS(t, cli, key)
	if err != nil || len(stored.Status.InstanceStatus) != 2 || len(lifecycleEntryPods(t, cli)) != 0 {
		t.Fatalf("complete bootstrap allocation was not committed alone: status=%+v err=%v", stored.Status, err)
	}
	for _, status := range stored.Status.InstanceStatus {
		if status.DataLoaded != nil || status.MemberJoined != nil || status.Provisioned {
			t.Fatalf("registration invented an execution result: %+v", status)
		}
	}
	for i := 0; i < 3; i++ {
		stored, _, err = reconcileLifecycleITS(t, cli, key)
		if err != nil {
			t.Fatal(err)
		}
	}
	pods := lifecycleEntryPods(t, cli)
	if len(pods) != 1 || pods[0].Name != "data-lifecycle-0" || stored.FindInstanceStatus("data-lifecycle-1").DataLoaded != nil {
		t.Fatalf("OrderedReady pending initial identity was classified as expansion: pods=%+v status=%+v", pods, stored.Status.InstanceStatus)
	}
	for _, c := range pods[0].Spec.InitContainers {
		for _, env := range c.Env {
			if env.Name == "KB_AGENT_TASK" || env.Name == "KB_AGENT_DATA_LOAD_RESULT" {
				t.Fatalf("bootstrap Pod unexpectedly requires a data task: %+v", c)
			}
		}
	}
	stored.Spec.Replicas = ptr.To[int32](3)
	stored.Generation++
	if err := cli.Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	stored, _, err = reconcileLifecycleITS(t, cli, key)
	if err != nil || stored.FindInstanceStatus("data-lifecycle-2") == nil || stored.FindInstanceStatus("data-lifecycle-2").DataLoaded == nil || *stored.FindInstanceStatus("data-lifecycle-2").DataLoaded || stored.FindInstanceStatus("data-lifecycle-1").DataLoaded != nil {
		t.Fatalf("new allocation while bootstrap remains pending was not distinguished: %+v err=%v", stored.Status.InstanceStatus, err)
	}
}

func TestInstanceSetLifecycleZeroToOneWaitsForDataSource(t *testing.T) {
	its := newDataLifecycleITS(0)
	cli := newLifecycleEntryClient(t, its, interceptor.Funcs{})
	key := client.ObjectKeyFromObject(its)
	stored, _, err := reconcileLifecycleITS(t, cli, key)
	if err != nil || stored.Status.ObservedGeneration != 1 {
		t.Fatalf("zero bootstrap was not committed: %+v err=%v", stored.Status, err)
	}
	stored.Spec.Replicas = ptr.To[int32](1)
	stored.Generation++
	if err := cli.Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		var result ctrl.Result
		stored, result, err = reconcileLifecycleITS(t, cli, key)
		if err != nil || result.RequeueAfter == 0 {
			t.Fatalf("waiting expansion has no retry: result=%+v err=%v", result, err)
		}
	}
	status := stored.FindInstanceStatus("data-lifecycle-0")
	if status == nil || status.DataLoaded == nil || *status.DataLoaded || len(lifecycleEntryPods(t, cli)) != 0 {
		t.Fatalf("zero-to-one bypassed source requirement: status=%+v pods=%+v", status, lifecycleEntryPods(t, cli))
	}
}

func TestInstanceSetLifecycleRecoversDataResultAcrossStatusFailureAndPodRecreation(t *testing.T) {
	ctx := context.Background()
	its := newDataLifecycleITS(1)
	rejectCompletion := false
	cli := newLifecycleEntryClient(t, its, interceptor.Funcs{SubResourceUpdate: func(ctx context.Context, cli client.Client, subresource string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
		if stored, ok := obj.(*workloads.InstanceSet); ok && subresource == "status" && rejectCompletion {
			if status := stored.FindInstanceStatus("data-lifecycle-1"); status != nil && ptr.Deref(status.DataLoaded, false) {
				return apierrors.NewConflict(schema.GroupResource{Group: workloads.GroupVersion.Group, Resource: "instancesets"}, obj.GetName(), fmt.Errorf("injected completion status conflict"))
			}
		}
		return cli.SubResource(subresource).Update(ctx, obj, opts...)
	}})
	key := client.ObjectKeyFromObject(its)
	step := func() *workloads.InstanceSet {
		t.Helper()
		stored, _, err := reconcileLifecycleITS(t, cli, key)
		if err != nil {
			t.Fatal(err)
		}
		return stored
	}
	for i := 0; i < 4; i++ {
		its = step()
	}
	source := &corev1.Pod{}
	if err := cli.Get(ctx, client.ObjectKey{Namespace: its.Namespace, Name: "data-lifecycle-0"}, source); err != nil {
		t.Fatal(err)
	}
	source.Status.PodIP = "10.0.0.1"
	source.Status.Phase = corev1.PodRunning
	source.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}
	if err := cli.Status().Update(ctx, source); err != nil {
		t.Fatal(err)
	}
	its = step()
	its.Spec.Replicas = ptr.To[int32](2)
	its.Generation++
	if err := cli.Update(ctx, its); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		its = step()
	}
	targetKey := client.ObjectKey{Namespace: its.Namespace, Name: "data-lifecycle-1"}
	target := &corev1.Pod{}
	if err := cli.Get(ctx, targetKey, target); err != nil {
		t.Fatal(err)
	}
	assertTask := func(pod *corev1.Pod, wantTask bool) {
		t.Helper()
		var task, guard bool
		for _, c := range pod.Spec.InitContainers {
			for _, env := range c.Env {
				task = task || env.Name == "KB_AGENT_TASK"
				guard = guard || env.Name == "KB_AGENT_DATA_LOAD_RESULT"
			}
		}
		if task != wantTask || !guard {
			t.Fatalf("target task=%v guard=%v, want task=%v: %+v", task, guard, wantTask, pod.Spec.InitContainers)
		}
	}
	assertTask(target, true)
	pvcKey := client.ObjectKey{Namespace: its.Namespace, Name: "data-data-lifecycle-1"}
	pvc := &corev1.PersistentVolumeClaim{}
	if err := cli.Get(ctx, pvcKey, pvc); err != nil {
		t.Fatal(err)
	}
	pvc.Annotations = map[string]string{kbaproto.DataLoadedAnnotationKey: "true"}
	if err := cli.Update(ctx, pvc); err != nil {
		t.Fatal(err)
	}
	rejectCompletion = true
	stored, result, err := reconcileLifecycleITS(t, cli, key)
	if err != nil || !result.Requeue || ptr.Deref(stored.FindInstanceStatus(targetKey.Name).DataLoaded, false) {
		t.Fatalf("completion conflict was not exercised: status=%+v err=%v", stored.Status.InstanceStatus, err)
	}
	if err := cli.Get(ctx, targetKey, target); err != nil {
		t.Fatal(err)
	}
	assertTask(target, true)
	rejectCompletion = false
	its = step()
	if !ptr.Deref(its.FindInstanceStatus(targetKey.Name).DataLoaded, false) {
		t.Fatal("fresh reconciler did not recover result from actual PVC")
	}
	// Simulate runtime loss after successful loading; the retained PVC is the durable result.
	target.Finalizers = nil
	if err := cli.Update(ctx, target); err != nil {
		t.Fatal(err)
	}
	if err := cli.Delete(ctx, target); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		its = step()
	}
	if err := cli.Get(ctx, targetKey, target); err != nil {
		t.Fatal(err)
	}
	assertTask(target, false)
	target.Status.Phase = corev1.PodPending
	if err := cli.Status().Update(ctx, target); err != nil {
		t.Fatal(err)
	}
	if !ptr.Deref(its.FindInstanceStatus(targetKey.Name).DataLoaded, false) {
		t.Fatal("runtime reconstruction lost completed data result")
	}
	// A replacement PVC with the same name must not inherit the old completion.
	pvc.Finalizers = nil
	if err := cli.Update(ctx, pvc); err != nil {
		t.Fatal(err)
	}
	if err := cli.Delete(ctx, pvc); err != nil {
		t.Fatal(err)
	}
	pvc.ResourceVersion = ""
	pvc.UID = ""
	pvc.Annotations = nil
	if err := cli.Create(ctx, pvc); err != nil {
		t.Fatal(err)
	}
	its = step()
	if ptr.Deref(its.FindInstanceStatus(targetKey.Name).DataLoaded, false) {
		t.Fatal("replacement data PVC inherited historical result")
	}
	for i := 0; i < 4; i++ {
		step()
	}
	if err := cli.Get(ctx, targetKey, target); err != nil {
		t.Fatal(err)
	}
	assertTask(target, true)
}

func TestInstanceSetLifecycleMemberActionsRespectStatusFailureAndLatestSpec(t *testing.T) {
	ctx := context.Background()
	its := newDataLifecycleITS(1)
	its.Spec.LifecycleActions.DataDump = nil
	its.Spec.LifecycleActions.DataLoad = nil
	its.Spec.LifecycleActions.MemberJoin = &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"join"}}}
	its.Spec.LifecycleActions.MemberLeave = &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"leave"}}}
	var joins, leaves int
	agent := kbacli.NewMockClient(gomock.NewController(t))
	agent.EXPECT().Action(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req kbaproto.ActionRequest) (kbaproto.ActionResponse, error) {
		switch req.Action {
		case "memberJoin":
			joins++
		case "memberLeave":
			leaves++
		default:
			t.Fatalf("unexpected lifecycle action: %+v", req)
		}
		return kbaproto.ActionResponse{}, nil
	}).AnyTimes()
	kbacli.SetMockClient(agent, nil)
	t.Cleanup(kbacli.UnsetMockClient)
	rejectLeave := false
	cli := newLifecycleEntryClient(t, its, interceptor.Funcs{SubResourceUpdate: func(ctx context.Context, cli client.Client, subresource string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
		if stored, ok := obj.(*workloads.InstanceSet); ok && subresource == "status" && rejectLeave {
			if status := stored.FindInstanceStatus("data-lifecycle-1"); status != nil && status.MemberJoined != nil && !*status.MemberJoined {
				return apierrors.NewConflict(schema.GroupResource{Group: workloads.GroupVersion.Group, Resource: "instancesets"}, obj.GetName(), fmt.Errorf("injected leave status conflict"))
			}
		}
		return cli.SubResource(subresource).Update(ctx, obj, opts...)
	}})
	key := client.ObjectKeyFromObject(its)
	step := func() *workloads.InstanceSet {
		t.Helper()
		stored, _, err := reconcileLifecycleITS(t, cli, key)
		if err != nil {
			t.Fatal(err)
		}
		return stored
	}
	markReady := func(name string) {
		t.Helper()
		pod := &corev1.Pod{}
		if err := cli.Get(ctx, client.ObjectKey{Namespace: its.Namespace, Name: name}, pod); err != nil {
			t.Fatal(err)
		}
		pod.Status.PodIP = "10.0.0.2"
		pod.Status.Phase = corev1.PodRunning
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}
		if err := cli.Status().Update(ctx, pod); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 3; i++ {
		its = step()
	}
	markReady("data-lifecycle-0")
	its = step()
	if joins != 0 || its.FindInstanceStatus("data-lifecycle-0").MemberJoined != nil {
		t.Fatal("bootstrap was recorded as a successful membership action")
	}
	setReplicas := func(replicas int32) {
		t.Helper()
		its.Spec.Replicas = ptr.To(replicas)
		its.Generation++
		if err := cli.Update(ctx, its); err != nil {
			t.Fatal(err)
		}
	}
	setReplicas(2)
	for i := 0; i < 3; i++ {
		its = step()
	}
	markReady("data-lifecycle-1")
	its = step()
	if joins != 1 || !ptr.Deref(its.FindInstanceStatus("data-lifecycle-1").MemberJoined, false) {
		t.Fatalf("member join did not execute and persist: calls=%d status=%+v", joins, its.Status.InstanceStatus)
	}
	setReplicas(1)
	rejectLeave = true
	stored, result, err := reconcileLifecycleITS(t, cli, key)
	if err != nil || !result.Requeue || leaves != 1 || !ptr.Deref(stored.FindInstanceStatus("data-lifecycle-1").MemberJoined, false) || len(lifecycleEntryPods(t, cli)) != 2 {
		t.Fatalf("leave status failure deleted runtime or lost result: calls=%d pods=%+v status=%+v err=%v", leaves, lifecycleEntryPods(t, cli), stored.Status.InstanceStatus, err)
	}
	its = stored
	setReplicas(2)
	rejectLeave = false
	its = step()
	if leaves != 1 || joins != 1 || len(lifecycleEntryPods(t, cli)) != 2 {
		t.Fatalf("retry acted on obsolete shrink: join=%d leave=%d pods=%+v", joins, leaves, lifecycleEntryPods(t, cli))
	}
	setReplicas(1)
	its = step()
	if leaves != 2 || ptr.Deref(its.FindInstanceStatus("data-lifecycle-1").MemberJoined, true) || len(lifecycleEntryPods(t, cli)) != 2 {
		t.Fatalf("leave completion was not committed before removal: leave=%d status=%+v pods=%+v", leaves, its.Status.InstanceStatus, lifecycleEntryPods(t, cli))
	}
	its = step()
	if len(lifecycleEntryPods(t, cli)) != 1 {
		t.Fatal("committed leave did not permit runtime removal")
	}
}

func TestInstanceSetLifecycleMemberOnlyExecutesGeneratedAgentActions(t *testing.T) {
	ctx := context.Background()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(t.TempDir(), "membership")
	its := newDataLifecycleITS(1)
	its.Spec.VolumeClaimTemplates = nil
	its.Spec.Template.Spec.Containers[0].VolumeMounts = nil
	its.Spec.LifecycleActions = &workloads.LifecycleActions{
		MemberJoin:  &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"sh", "-c", "printf 'join\\n' >> \"$1\"", "sh", logPath}}},
		MemberLeave: &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"sh", "-c", "printf 'leave\\n' >> \"$1\"", "sh", logPath}}},
	}
	existingEnv, err := kbagent.BuildEnv4Server([]kbaproto.Action{{Name: "reconfigure", Exec: &kbaproto.ExecAction{Commands: []string{"true"}}}}, []kbaproto.Probe{{Instance: "existing", Action: "reconfigure"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	its.Spec.Template.Spec.Containers = append(its.Spec.Template.Spec.Containers, corev1.Container{
		Name: kbagent.ContainerName, Image: "tools", Env: existingEnv,
		Ports: []corev1.ContainerPort{{Name: kbagent.DefaultHTTPPortName, ContainerPort: int32(port)}},
	})
	cli := newLifecycleEntryClient(t, its, interceptor.Funcs{})
	key := client.ObjectKeyFromObject(its)
	step := func() {
		t.Helper()
		var err error
		its, _, err = reconcileLifecycleITS(t, cli, key)
		if err != nil {
			t.Fatal(err)
		}
	}
	markReady := func(name string) {
		t.Helper()
		pod := &corev1.Pod{}
		if err := cli.Get(ctx, client.ObjectKey{Namespace: its.Namespace, Name: name}, pod); err != nil {
			t.Fatal(err)
		}
		pod.Status.PodIP = "127.0.0.1"
		pod.Status.Phase = corev1.PodRunning
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}
		if err := cli.Status().Update(ctx, pod); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 3; i++ {
		step()
	}
	markReady("data-lifecycle-0")
	step()
	its.Spec.Replicas = ptr.To(int32(2))
	its.Generation++
	if err := cli.Update(ctx, its); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		step()
	}
	target := &corev1.Pod{}
	if err := cli.Get(ctx, client.ObjectKey{Namespace: its.Namespace, Name: "data-lifecycle-1"}, target); err != nil {
		t.Fatal(err)
	}
	var actions []kbaproto.Action
	var probes []kbaproto.Probe
	for _, container := range target.Spec.Containers {
		if container.Name != kbagent.ContainerName {
			continue
		}
		for _, env := range container.Env {
			switch env.Name {
			case "KB_AGENT_ACTION":
				if err := json.Unmarshal([]byte(env.Value), &actions); err != nil {
					t.Fatal(err)
				}
			case "KB_AGENT_PROBE":
				if err := json.Unmarshal([]byte(env.Value), &probes); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if len(actions) != 3 || len(probes) != 1 || probes[0].Action != "reconfigure" || len(target.Spec.InitContainers) != 0 {
		t.Fatalf("member-only setup discarded existing agent configuration or added a data worker: actions=%+v probes=%+v init=%+v", actions, probes, target.Spec.InitContainers)
	}
	services, err := kbaservice.New(ctrl.Log, actions, probes, nil)
	if err != nil {
		t.Fatal(err)
	}
	server := kbaserver.NewHTTPServer(ctrl.Log, kbaserver.Config{Address: "127.0.0.1", Port: port}, services)
	if err := server.StartNonBlocking(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	agentClient, err := kbacli.NewClient(func() (string, int32, error) { return "127.0.0.1", int32(port), nil })
	if err != nil {
		t.Fatal(err)
	}
	// Route the out-of-cluster controller to the real HTTP agent instead of Kubernetes port forwarding.
	kbacli.SetMockClient(agentClient, nil)
	t.Cleanup(kbacli.UnsetMockClient)
	markReady(target.Name)
	step()
	if !ptr.Deref(its.FindInstanceStatus(target.Name).MemberJoined, false) {
		t.Fatalf("real agent join completion was not persisted: status=%+v conditions=%+v", its.Status.InstanceStatus, its.Status.Conditions)
	}
	its.Spec.Replicas = ptr.To(int32(1))
	its.Generation++
	if err := cli.Update(ctx, its); err != nil {
		t.Fatal(err)
	}
	step()
	if ptr.Deref(its.FindInstanceStatus(target.Name).MemberJoined, true) || len(lifecycleEntryPods(t, cli)) != 2 {
		t.Fatal("real agent leave completion was not committed before runtime removal")
	}
	output, err := os.ReadFile(logPath)
	if err != nil || string(output) != "join\nleave\n" {
		t.Fatalf("generated member commands did not execute: output=%q err=%v", output, err)
	}
}

func TestInstanceSetLifecycleRetainsUnknownMembershipWithoutRuntime(t *testing.T) {
	its := newDataLifecycleITS(0)
	its.Spec.LifecycleActions.DataDump = nil
	its.Spec.LifecycleActions.DataLoad = nil
	its.Spec.LifecycleActions.MemberLeave = &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"leave"}}}
	its.Status.ObservedGeneration = its.Generation
	its.Status.InstanceStatus = []workloads.InstanceStatus{{PodName: "data-lifecycle-0", Provisioned: true, DesiredState: workloads.InstanceDesiredStateReleased, CurrentState: workloads.InstanceCurrentStateAbsent}}
	cli := newLifecycleEntryClient(t, its, interceptor.Funcs{})
	for i := 0; i < 3; i++ {
		stored, result, err := reconcileLifecycleITS(t, cli, client.ObjectKeyFromObject(its))
		status := stored.FindInstanceStatus("data-lifecycle-0")
		if err != nil || result.RequeueAfter == 0 || status == nil || !status.Provisioned || status.MemberJoined != nil {
			t.Fatalf("unresolved bootstrap membership was dropped or invented: status=%+v result=%+v err=%v", status, result, err)
		}
	}
}

func assertPersistedTrackedFacts(t *testing.T, its *workloads.InstanceSet, facts []*bool) {
	t.Helper()
	for i, name := range []string{"tracked-0", "tracked-1", "tracked-2"} {
		status := its.FindInstanceStatus(name)
		if status == nil || status.DesiredState != workloads.InstanceDesiredStateOffline || !reflect.DeepEqual(status.DataLoaded, facts[i]) || !reflect.DeepEqual(status.MemberJoined, facts[i]) || status.Provisioned != (i == 0) {
			t.Fatalf("committed nullable facts or provisioning changed for %s: %#v", name, status)
		}
	}
}

var _ = Describe("InstanceSet tracked status API persistence", func() {
	It("round trips omitted, false, and true tracked facts through the status subresource", func() {
		its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "tracked-facts-api", Namespace: testCtx.DefaultNamespace},
			Spec: workloads.InstanceSetSpec{Replicas: ptr.To[int32](0),
				Selector:         &metav1.LabelSelector{MatchLabels: map[string]string{"app": "tracked-facts-api"}},
				Template:         corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "mysql"}}}},
				OfflineInstances: []string{"tracked-facts-api-0", "tracked-facts-api-1", "tracked-facts-api-2"}}}
		Expect(k8sClient.Create(ctx, its)).To(Succeed())
		key := client.ObjectKeyFromObject(its)
		DeferCleanup(func() {
			Eventually(func() error {
				current := &workloads.InstanceSet{}
				if err := k8sClient.Get(ctx, key, current); err != nil {
					return client.IgnoreNotFound(err)
				}
				current.Finalizers = nil
				if err := k8sClient.Update(ctx, current); err != nil {
					return err
				}
				return client.IgnoreNotFound(k8sClient.Delete(ctx, current))
			}).Should(Succeed())
		})
		facts := []*bool{nil, ptr.To(false), ptr.To(true)}
		for _, values := range [][]*bool{facts, {ptr.To(true), ptr.To(false), nil}} {
			Eventually(func() error {
				if err := k8sClient.Get(ctx, key, its); err != nil {
					return err
				}
				its.Status.InstanceStatus = nil
				for i, name := range its.Spec.OfflineInstances {
					its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{
						PodName: name, DesiredState: workloads.InstanceDesiredStateOffline, CurrentState: workloads.InstanceCurrentStateAbsent,
						Provisioned: i != 0, DataLoaded: values[i], MemberJoined: values[2-i],
					})
				}
				return k8sClient.Status().Update(ctx, its)
			}).Should(Succeed())
			Eventually(func(g Gomega) {
				stored := &workloads.InstanceSet{}
				g.Expect(k8sClient.Get(ctx, key, stored)).To(Succeed())
				for i, name := range its.Spec.OfflineInstances {
					status := stored.FindInstanceStatus(name)
					g.Expect(status).NotTo(BeNil())
					g.Expect(status.Provisioned).To(Equal(i != 0))
					g.Expect(reflect.DeepEqual(status.DataLoaded, values[i])).To(BeTrue())
					g.Expect(reflect.DeepEqual(status.MemberJoined, values[2-i])).To(BeTrue())
				}
			}).Should(Succeed())
		}
	})
})

func TestInstanceSetLifecycleStopAndOfflineDoNotRequireLeave(t *testing.T) {
	for _, stop := range []bool{false, true} {
		t.Run(fmt.Sprintf("stop=%v", stop), func(t *testing.T) {
			ctx := context.Background()
			its := newDataLifecycleITS(1)
			its.Spec.LifecycleActions.DataDump = nil
			its.Spec.LifecycleActions.DataLoad = nil
			its.Spec.LifecycleActions.MemberLeave = &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"leave"}}}
			cli := newLifecycleEntryClient(t, its, interceptor.Funcs{})
			key := client.ObjectKeyFromObject(its)
			for i := 0; i < 4; i++ {
				var err error
				its, _, err = reconcileLifecycleITS(t, cli, key)
				if err != nil {
					t.Fatal(err)
				}
			}
			status := its.FindInstanceStatus("data-lifecycle-0")
			if status == nil {
				t.Fatal("missing bootstrap status")
			}
			status.Provisioned = true
			status.MemberJoined = ptr.To(true)
			if err := cli.Status().Update(ctx, its); err != nil {
				t.Fatal(err)
			}
			if stop {
				its.Spec.Stop = ptr.To(true)
			} else {
				its.Spec.OfflineInstances = []string{"data-lifecycle-0"}
				its.Spec.Replicas = ptr.To[int32](0)
			}
			its.Generation++
			if err := cli.Update(ctx, its); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 4; i++ {
				var err error
				its, _, err = reconcileLifecycleITS(t, cli, key)
				if err != nil {
					t.Fatal(err)
				}
			}
			if pods := lifecycleEntryPods(t, cli); len(pods) != 0 {
				t.Fatalf("stop/offline runtime is blocked by retained membership: %+v", pods)
			}
			status = its.FindInstanceStatus("data-lifecycle-0")
			if status == nil || !ptr.Deref(status.MemberJoined, false) {
				t.Fatalf("stop/offline invented leave completion: %+v", status)
			}
		})
	}
}

func TestInstanceSetLifecycleRefreshesPendingWorkerAfterDonorAddressChanges(t *testing.T) {
	ctx := context.Background()
	its := newDataLifecycleITS(1)
	its.Spec.DisableDefaultHeadlessService = true
	cli := newLifecycleEntryClient(t, its, interceptor.Funcs{})
	key := client.ObjectKeyFromObject(its)
	step := func() {
		t.Helper()
		var err error
		its, _, err = reconcileLifecycleITS(t, cli, key)
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 4; i++ {
		step()
	}
	source := &corev1.Pod{}
	sourceKey := client.ObjectKey{Namespace: its.Namespace, Name: "data-lifecycle-0"}
	if err := cli.Get(ctx, sourceKey, source); err != nil {
		t.Fatal(err)
	}
	source.Status = corev1.PodStatus{PodIP: "10.0.0.1", Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}}
	if err := cli.Status().Update(ctx, source); err != nil {
		t.Fatal(err)
	}
	step()
	its.Spec.Replicas = ptr.To[int32](2)
	its.Generation++
	if err := cli.Update(ctx, its); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		step()
	}
	target := &corev1.Pod{}
	targetKey := client.ObjectKey{Namespace: its.Namespace, Name: "data-lifecycle-1"}
	if err := cli.Get(ctx, targetKey, target); err != nil {
		t.Fatal(err)
	}
	target.Status.Phase = corev1.PodPending
	if err := cli.Status().Update(ctx, target); err != nil {
		t.Fatal(err)
	}
	oldTask, err := workloadlifecycle.GeneratedTask(&target.Spec)
	if err != nil || oldTask == "" {
		t.Fatalf("missing generated worker %s %v", oldTask, err)
	}
	source.Status.PodIP = "10.0.0.99"
	if err := cli.Status().Update(ctx, source); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		step()
	}
	if err := cli.Get(ctx, targetKey, target); err != nil {
		t.Fatal(err)
	}
	current, err := workloadlifecycle.GeneratedTask(&target.Spec)
	if err != nil || current == oldTask {
		t.Fatalf("pending worker kept stale donor: %s %v", current, err)
	}
	var task kbaproto.Task
	if err := json.Unmarshal([]byte(current), &task); err != nil {
		t.Fatal(err)
	}
	if task.NewReplica.Remote != "10.0.0.99" {
		t.Fatalf("pending worker did not use current donor address: %+v", task)
	}
	target.Status.Phase = corev1.PodRunning
	if err := cli.Status().Update(ctx, target); err != nil {
		t.Fatal(err)
	}
	source.Status.PodIP = "10.0.0.100"
	if err := cli.Status().Update(ctx, source); err != nil {
		t.Fatal(err)
	}
	step()
	if err := cli.Get(ctx, targetKey, target); err != nil {
		t.Fatal("running database was deleted for stale completed init instructions", err)
	}
}

func TestInstanceSetLifecycleReexpandRetainedDataRestoresResultAndJoins(t *testing.T) {
	ctx := context.Background()
	its := newDataLifecycleITS(1)
	its.Spec.LifecycleActions.MemberJoin = &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"join"}}}
	its.Spec.LifecycleActions.MemberLeave = &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"leave"}}}
	its.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{WhenScaled: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType}
	var joins, leaves int
	agent := kbacli.NewMockClient(gomock.NewController(t))
	agent.EXPECT().Action(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req kbaproto.ActionRequest) (kbaproto.ActionResponse, error) {
		switch req.Action {
		case "memberJoin":
			joins++
		case "memberLeave":
			leaves++
		}
		return kbaproto.ActionResponse{}, nil
	}).AnyTimes()
	kbacli.SetMockClient(agent, nil)
	t.Cleanup(kbacli.UnsetMockClient)
	cli := newLifecycleEntryClient(t, its, interceptor.Funcs{})
	key := client.ObjectKeyFromObject(its)
	step := func() {
		t.Helper()
		var err error
		its, _, err = reconcileLifecycleITS(t, cli, key)
		if err != nil {
			t.Fatal(err)
		}
	}
	ready := func(name string) {
		t.Helper()
		pod := &corev1.Pod{}
		if err := cli.Get(ctx, client.ObjectKey{Namespace: its.Namespace, Name: name}, pod); err != nil {
			t.Fatal(err)
		}
		pod.Status = corev1.PodStatus{Phase: corev1.PodRunning, PodIP: "10.0.0.1", Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}}
		if err := cli.Status().Update(ctx, pod); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 4; i++ {
		step()
	}
	ready("data-lifecycle-0")
	step()
	its.Spec.Replicas = ptr.To[int32](2)
	its.Generation++
	if err := cli.Update(ctx, its); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		step()
	}
	pvcKey := client.ObjectKey{Namespace: its.Namespace, Name: "data-data-lifecycle-1"}
	pvc := &corev1.PersistentVolumeClaim{}
	if err := cli.Get(ctx, pvcKey, pvc); err != nil {
		t.Fatal(err)
	}
	pvc.Annotations = map[string]string{kbaproto.DataLoadedAnnotationKey: "true"}
	if err := cli.Update(ctx, pvc); err != nil {
		t.Fatal(err)
	}
	ready("data-lifecycle-1")
	for i := 0; i < 3; i++ {
		step()
	}
	if joins != 1 {
		t.Fatalf("first expansion did not join: %d", joins)
	}
	its.Spec.Replicas = ptr.To[int32](1)
	its.Generation++
	if err := cli.Update(ctx, its); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		step()
	}
	if leaves != 1 || its.FindInstanceStatus("data-lifecycle-1") != nil {
		t.Fatalf("retained shrink did not release identity: leaves=%d status=%+v", leaves, its.Status.InstanceStatus)
	}
	if err := cli.Get(ctx, pvcKey, pvc); err != nil {
		t.Fatal("retained data PVC was deleted", err)
	}
	its.Spec.Replicas = ptr.To[int32](2)
	its.Generation++
	if err := cli.Update(ctx, its); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		step()
	}
	status := its.FindInstanceStatus("data-lifecycle-1")
	if status == nil || !ptr.Deref(status.DataLoaded, false) || status.MemberJoined == nil || *status.MemberJoined {
		t.Fatalf("retained expansion did not restore data and track membership separately: %+v", status)
	}
	target := &corev1.Pod{}
	if err := cli.Get(ctx, client.ObjectKey{Namespace: its.Namespace, Name: "data-lifecycle-1"}, target); err != nil {
		t.Fatal(err)
	}
	task, err := workloadlifecycle.GeneratedTask(&target.Spec)
	if err != nil || task != "" {
		t.Fatal("retained completed data was reloaded", err)
	}
	ready("data-lifecycle-1")
	step()
	if joins != 2 || !ptr.Deref(its.FindInstanceStatus("data-lifecycle-1").MemberJoined, false) {
		t.Fatalf("retained re-expansion skipped actual join: %d %+v", joins, its.Status.InstanceStatus)
	}
}

func TestInstanceSetLifecycleRegistersCompleteFlatMultitemplateAllocation(t *testing.T) {
	ctx := context.Background()
	its := newDataLifecycleITS(3)
	its.Spec.FlatInstanceOrdinal = true
	its.Spec.Ordinals = workloads.Ordinals{Discrete: []int32{0, 1}}
	its.Spec.Instances = []workloads.InstanceTemplate{{Name: "other", Replicas: ptr.To[int32](1), Ordinals: workloads.Ordinals{Discrete: []int32{2}}}}
	cli := newLifecycleEntryClient(t, its, interceptor.Funcs{})
	key := client.ObjectKeyFromObject(its)
	stored, _, err := reconcileLifecycleITS(t, cli, key)
	if err != nil || len(stored.Status.InstanceStatus) != 3 || len(lifecycleEntryPods(t, cli)) != 0 {
		t.Fatalf("partial flat bootstrap registration: %+v %v", stored.Status.InstanceStatus, err)
	}
	for _, s := range stored.Status.InstanceStatus {
		if s.DataLoaded != nil || s.MemberJoined != nil {
			t.Fatal("flat bootstrap invented execution results")
		}
	}
	stored.Spec.Replicas = ptr.To[int32](4)
	stored.Spec.Instances[0].Replicas = ptr.To[int32](2)
	stored.Spec.Instances[0].Ordinals.Discrete = []int32{2, 3}
	stored.Generation++
	if err := cli.Update(ctx, stored); err != nil {
		t.Fatal(err)
	}
	stored, _, err = reconcileLifecycleITS(t, cli, key)
	status := stored.FindInstanceStatus("data-lifecycle-3")
	if err != nil || len(stored.Status.InstanceStatus) != 4 || status == nil || status.TemplateName == nil || *status.TemplateName != "other" || status.DataLoaded == nil || *status.DataLoaded || len(lifecycleEntryPods(t, cli)) != 0 {
		t.Fatalf("new flat identity while bootstrap is pending was not registered alone: %+v %v", stored.Status.InstanceStatus, err)
	}
}
