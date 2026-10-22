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
	"k8s.io/apimachinery/pkg/api/meta"
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
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
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
					if !reflect.DeepEqual(persistedClaim, claim) {
						t.Fatal("status failure changed a terminating Pod's PVC")
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
				if !reflect.DeepEqual(current, claim) {
					t.Fatal("transfer changed storage while the old Pod was terminating")
				}
				pod.Finalizers = nil
				if err := cli.Update(context.Background(), pod); err != nil {
					t.Fatal(err)
				}
				if failStatus {
					injected := errors.New("injected replacement Pod create failure")
					faulty := interceptor.NewClient(cli, interceptor.Funcs{Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
						if replacement, ok := obj.(*corev1.Pod); ok && replacement.Name == pod.Name {
							return injected
						}
						return c.Create(ctx, obj, opts...)
					}})
					if err := reconcile(faulty); !errors.Is(err, injected) {
						t.Fatalf("expected replacement create failure, got %v", err)
					}
					if err := cli.Get(context.Background(), client.ObjectKeyFromObject(claim), current); err != nil {
						t.Fatal(err)
					}
					if current.Labels[constant.KBAppInstanceTemplateLabelKey] != expectedTemplate || current.UID != claim.UID || current.Spec.VolumeName != claim.Spec.VolumeName || !reflect.DeepEqual(current.OwnerReferences, claim.OwnerReferences) {
						t.Fatal("fixture did not commit the retained PVC transfer before Pod create failure")
					}
					if err := cli.Get(context.Background(), client.ObjectKeyFromObject(pod), &corev1.Pod{}); !apierrors.IsNotFound(err) {
						t.Fatalf("failed replacement unexpectedly exists: %v", err)
					}
					if !swap {
						if err := cli.Get(context.Background(), key, its); err != nil {
							t.Fatal(err)
						}
						its.Spec.Instances[0].Name = "d"
						its.Generation++
						if err := cli.Update(context.Background(), its); err != nil {
							t.Fatal(err)
						}
						expectedTemplate = "d"
					}
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
				if current.UID != claim.UID || current.Spec.VolumeName != claim.Spec.VolumeName || !reflect.DeepEqual(current.OwnerReferences, claim.OwnerReferences) || current.Labels[constant.KBAppInstanceTemplateLabelKey] != expectedTemplate {
					t.Fatal("replacement lost retained storage or its latest template assignment")
				}
				if err := cli.Get(context.Background(), key, its); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(its.Status.AssignedOrdinals[expectedTemplate].Discrete, []int32{0}) {
					t.Fatal("recovery did not publish the latest ordinal assignment")
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

func TestInstanceSetTransferredIdentityCleanup(t *testing.T) {
	for _, transfer := range []string{"none", "named", "default"} {
		for _, offline := range []bool{false, true} {
			for _, retain := range []bool{false, true} {
				t.Run(fmt.Sprintf("transfer=%s/offline=%t/retain=%t", transfer, offline, retain), func(t *testing.T) {
					scheme := runtime.NewScheme()
					for _, add := range []func(*runtime.Scheme) error{clientgoscheme.AddToScheme, workloads.AddToScheme} {
						if err := add(scheme); err != nil {
							t.Fatal(err)
						}
					}
					its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "cleanup-transfer", Namespace: "default", UID: "its-uid", Generation: 1}, Spec: workloads.InstanceSetSpec{
						Replicas: ptr.To(int32(2)), FlatInstanceOrdinal: true, PodManagementPolicy: appsv1.ParallelPodManagement,
						InstanceUpdateStrategy: &workloads.InstanceUpdateStrategy{Type: kbappsv1.OnDeleteStrategyType},
						Selector:               &metav1.LabelSelector{MatchLabels: map[string]string{"app": "cleanup-transfer"}},
						Template:               corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "cleanup-transfer"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}}},
						Instances:              []workloads.InstanceTemplate{{Name: "a", Replicas: ptr.To(int32(2)), Ordinals: workloads.Ordinals{Discrete: []int32{0, 1}}}},
						VolumeClaimTemplates:   []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}, Spec: corev1.PersistentVolumeClaimSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}}}}},
					}}
					if retain {
						its.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{WhenScaled: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType}
					}
					cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&workloads.InstanceSet{}, &corev1.Pod{}, &corev1.PersistentVolumeClaim{}).WithObjects(its).Build()
					key := client.ObjectKeyFromObject(its)
					run := func() {
						t.Helper()
						_, err := (&InstanceSetReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(100)}).Reconcile(context.Background(), ctrl.Request{NamespacedName: key})
						if err != nil {
							t.Fatal(err)
						}
					}
					for i := 0; i < 3; i++ {
						run()
					}
					pod := &corev1.Pod{}
					podKey := client.ObjectKey{Namespace: key.Namespace, Name: its.Name + "-0"}
					if err := cli.Get(context.Background(), podKey, pod); err != nil {
						t.Fatal(err)
					}
					pod.Finalizers = []string{"test/hold"}
					if err := cli.Update(context.Background(), pod); err != nil {
						t.Fatal(err)
					}
					claims := &corev1.PersistentVolumeClaimList{}
					if err := cli.List(context.Background(), claims); err != nil {
						t.Fatal(err)
					}
					if len(claims.Items) != 2 {
						t.Fatalf("initial PVC count: %d", len(claims.Items))
					}
					originalClaims := make(map[string]*corev1.PersistentVolumeClaim, len(claims.Items))
					for i := range claims.Items {
						claim := &claims.Items[i]
						claim.UID = types.UID(claim.Name)
						claim.Spec.VolumeName = "bound-" + claim.Name
						if err := cli.Update(context.Background(), claim); err != nil {
							t.Fatal(err)
						}
						originalClaims[claim.Name] = claim.DeepCopy()
					}
					if err := cli.Get(context.Background(), key, its); err != nil {
						t.Fatal(err)
					}
					expectedTemplate := "a"
					switch transfer {
					case "named":
						its.Spec.Instances[0].Name = "b"
						expectedTemplate = "b"
					case "default":
						its.Spec.Instances = nil
						its.Spec.Ordinals = workloads.Ordinals{Discrete: []int32{0, 1}}
						expectedTemplate = ""
					}
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
					if status := its.FindInstanceStatus(podKey.Name); status == nil || status.TemplateName == nil || *status.TemplateName != expectedTemplate || status.DesiredState != workloads.InstanceDesiredStateActive {
						t.Fatalf("transfer did not publish target identity: %#v", status)
					}
					if err := cli.Get(context.Background(), podKey, pod); err != nil {
						t.Fatal(err)
					}
					if template, ok := instancetemplate.TemplateNameFromLabels(pod.Labels); !ok || template != "a" || !pod.DeletionTimestamp.IsZero() {
						t.Fatal("OnDelete fixture did not preserve the old Pod")
					}
					its.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
					wantDesired := workloads.InstanceDesiredStateReleased
					wantClaims := 0
					if offline {
						its.Spec.Replicas = ptr.To(int32(1))
						its.Spec.OfflineInstances = []string{podKey.Name}
						if len(its.Spec.Instances) != 0 {
							its.Spec.Instances[0].Replicas = ptr.To(int32(1))
						}
						wantDesired = workloads.InstanceDesiredStateOffline
						wantClaims = 1
					} else {
						its.Spec.Replicas = ptr.To(int32(0))
						its.Spec.Instances = nil
						its.Spec.Ordinals = workloads.Ordinals{}
					}
					if retain {
						wantClaims = 2
					}
					its.Generation++
					if err := cli.Update(context.Background(), its); err != nil {
						t.Fatal(err)
					}
					for i := 0; i < 4; i++ {
						run()
					}
					if err := cli.Get(context.Background(), podKey, pod); err != nil {
						t.Fatal(err)
					}
					if pod.DeletionTimestamp.IsZero() {
						t.Fatal("cleanup never reached the remaining Pod")
					}
					if err := cli.Get(context.Background(), key, its); err != nil {
						t.Fatal(err)
					}
					if status := its.FindInstanceStatus(podKey.Name); status == nil || status.TemplateName == nil || *status.TemplateName != expectedTemplate || status.DesiredState != wantDesired || status.CurrentState != workloads.InstanceCurrentStateTerminating || status.UpToDate {
						t.Fatalf("incorrect retained cleanup status: %#v", status)
					}
					if err := cli.List(context.Background(), claims); err != nil {
						t.Fatal(err)
					}
					if len(claims.Items) != wantClaims {
						t.Fatalf("PVC cleanup policy: got %d claims, want %d", len(claims.Items), wantClaims)
					}
					for _, claim := range claims.Items {
						original := originalClaims[claim.Name]
						if original == nil || claim.UID != original.UID || claim.Spec.VolumeName != original.Spec.VolumeName || !reflect.DeepEqual(claim.OwnerReferences, original.OwnerReferences) {
							t.Fatalf("retained PVC identity changed: %#v", claim)
						}
					}
					pod.Finalizers = nil
					if err := cli.Update(context.Background(), pod); err != nil {
						t.Fatal(err)
					}
					for i := 0; i < 2; i++ {
						run()
					}
					if err := cli.Get(context.Background(), key, its); err != nil {
						t.Fatal(err)
					}
					status := its.FindInstanceStatus(podKey.Name)
					if offline {
						if status == nil || status.TemplateName == nil || *status.TemplateName != expectedTemplate || status.DesiredState != workloads.InstanceDesiredStateOffline || status.CurrentState != workloads.InstanceCurrentStateAbsent {
							t.Fatalf("offline identity was lost after deletion: %#v", status)
						}
					} else if status != nil {
						t.Fatalf("released identity survived deletion: %#v", status)
					}
				})
			}
		}
	}
}

func TestInstanceAPITransferredIdentityCleanup(t *testing.T) {
	for _, offline := range []bool{false, true} {
		t.Run(fmt.Sprintf("offline=%t", offline), func(t *testing.T) {
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{clientgoscheme.AddToScheme, workloads.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			model.AddScheme(workloads.AddToScheme)
			its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "instance-cleanup", Namespace: "default", UID: "its-uid", Generation: 1}, Spec: workloads.InstanceSetSpec{
				EnableInstanceAPI: ptr.To(true), Replicas: ptr.To(int32(1)), FlatInstanceOrdinal: true, PodManagementPolicy: appsv1.ParallelPodManagement,
				Selector:  &metav1.LabelSelector{MatchLabels: map[string]string{"app": "instance-cleanup"}},
				Template:  corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "instance-cleanup"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}}},
				Instances: []workloads.InstanceTemplate{{Name: "a", Replicas: ptr.To(int32(1)), Ordinals: workloads.Ordinals{Discrete: []int32{0}}}},
			}}
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
			inst.UID = "retained-instance"
			inst.Finalizers = []string{"test/hold"}
			if err := cli.Update(context.Background(), inst); err != nil {
				t.Fatal(err)
			}
			inst.Status.CurrentState = workloads.InstanceCurrentStatePresent
			if err := cli.Status().Update(context.Background(), inst); err != nil {
				t.Fatal(err)
			}
			if err := cli.Get(context.Background(), key, its); err != nil {
				t.Fatal(err)
			}
			its.Spec.Instances[0].Name = "b"
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
			if status := its.FindInstanceStatus(instKey.Name); status == nil || status.TemplateName == nil || *status.TemplateName != "b" || status.DesiredState != workloads.InstanceDesiredStateActive {
				t.Fatalf("transfer did not publish desired identity: %#v", status)
			}
			if err := cli.Get(context.Background(), instKey, inst); err != nil {
				t.Fatal(err)
			}
			if inst.Spec.InstanceTemplateName != "a" {
				t.Fatal("fixture did not retain the unready Instance's old spec")
			}
			its.Spec.Replicas = ptr.To(int32(0))
			its.Spec.Instances = nil
			its.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
			wantDesired := workloads.InstanceDesiredStateReleased
			if offline {
				its.Spec.OfflineInstances = []string{instKey.Name}
				wantDesired = workloads.InstanceDesiredStateOffline
			}
			its.Generation++
			if err := cli.Update(context.Background(), its); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 3; i++ {
				run()
			}
			if err := cli.Get(context.Background(), instKey, inst); err != nil {
				t.Fatal(err)
			}
			if inst.UID != "retained-instance" || !ptr.Deref(inst.Spec.ScaledDown, false) || inst.DeletionTimestamp.IsZero() {
				t.Fatal("cleanup did not retire the retained Instance")
			}
			if err := cli.Get(context.Background(), key, its); err != nil {
				t.Fatal(err)
			}
			if status := its.FindInstanceStatus(instKey.Name); status == nil || status.TemplateName == nil || *status.TemplateName != "b" || status.DesiredState != wantDesired || status.UpToDate {
				t.Fatalf("cleanup lost the published identity: %#v", status)
			}
			inst.Finalizers = nil
			if err := cli.Update(context.Background(), inst); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				run()
			}
			if err := cli.Get(context.Background(), key, its); err != nil {
				t.Fatal(err)
			}
			status := its.FindInstanceStatus(instKey.Name)
			if offline {
				if status == nil || status.TemplateName == nil || *status.TemplateName != "b" || status.DesiredState != workloads.InstanceDesiredStateOffline || status.CurrentState != workloads.InstanceCurrentStateAbsent {
					t.Fatalf("offline identity was lost after deletion: %#v", status)
				}
			} else if status != nil {
				t.Fatalf("released identity survived deletion: %#v", status)
			}
		})
	}
}

func legacyStopRecoveryFixture(t *testing.T, replicas int32, minReadySeconds int32) client.WithWatch {
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
		ObjectMeta: metav1.ObjectMeta{Name: "legacy-stop", Namespace: "default", UID: "its-uid", Generation: 1},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To(replicas), MinReadySeconds: minReadySeconds, PodManagementPolicy: appsv1.ParallelPodManagement,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "legacy-stop"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "legacy-stop"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db:v1"}}}},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}, Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
			}}},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&workloads.InstanceSet{}, &corev1.Pod{}, &corev1.PersistentVolumeClaim{}).WithObjects(its).Build()
	legacyStopRecoveryReconcile(t, cli, 4)
	if len(legacyStopRecoveryPods(t, cli)) != int(replicas) {
		t.Fatal("fixture did not provision running identities")
	}
	return cli
}

func legacyStopRecoveryReconcile(t *testing.T, cli client.Client, count int) ctrl.Result {
	t.Helper()
	var result ctrl.Result
	for i := 0; i < count; i++ {
		reconciler := &InstanceSetReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(1000)}
		var err error
		result, err = reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "legacy-stop"}})
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func legacyStopRecoverySet(t *testing.T, cli client.Client) *workloads.InstanceSet {
	t.Helper()
	its := &workloads.InstanceSet{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "legacy-stop"}, its); err != nil {
		t.Fatal(err)
	}
	return its
}

func legacyStopRecoveryChange(t *testing.T, cli client.Client, change func(*workloads.InstanceSet)) {
	t.Helper()
	its := legacyStopRecoverySet(t, cli)
	change(its)
	its.Generation++
	if err := cli.Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
}

func legacyStopRecoveryPods(t *testing.T, cli client.Client) []corev1.Pod {
	t.Helper()
	pods := &corev1.PodList{}
	if err := cli.List(context.Background(), pods); err != nil {
		t.Fatal(err)
	}
	return pods.Items
}

func legacyStopRecoveryClaims(t *testing.T, cli client.Client) []corev1.PersistentVolumeClaim {
	t.Helper()
	claims := &corev1.PersistentVolumeClaimList{}
	if err := cli.List(context.Background(), claims); err != nil {
		t.Fatal(err)
	}
	return claims.Items
}

func legacyStopRecoveryReady(t *testing.T, cli client.Client) {
	t.Helper()
	for _, claim := range legacyStopRecoveryClaims(t, cli) {
		claim.Status.Capacity = claim.Spec.Resources.Requests.DeepCopy()
		if err := cli.Status().Update(context.Background(), &claim); err != nil {
			t.Fatal(err)
		}
	}
	for _, pod := range legacyStopRecoveryPods(t, cli) {
		pod.Status.Phase = corev1.PodRunning
		pod.Status.PodIP = "127.0.0.1"
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "db", Image: "db:v1", Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}
		if err := cli.Status().Update(context.Background(), &pod); err != nil {
			t.Fatal(err)
		}
	}
	legacyStopRecoveryReconcile(t, cli, 2)
}

func legacyStopRecoveryHold(t *testing.T, cli client.Client) {
	t.Helper()
	pod := &corev1.Pod{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "legacy-stop-0"}, pod); err != nil {
		t.Fatal(err)
	}
	pod.Finalizers = append(pod.Finalizers, "test/hold")
	if err := cli.Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyInstanceSetStopRecoveryMinReadySeconds(t *testing.T) {
	cli := legacyStopRecoveryFixture(t, 2, 3600)
	legacyStopRecoveryReady(t, cli)
	status := legacyStopRecoverySet(t, cli).Status
	if status.ReadyReplicas != 2 || status.AvailableReplicas != 0 {
		t.Fatalf("fixture did not exercise MinReadySeconds: %#v", status)
	}
	if result := legacyStopRecoveryReconcile(t, cli, 1); result.RequeueAfter != time.Second {
		t.Fatalf("fixture did not reach status cutoff: %#v", result)
	}
	claims := legacyStopRecoveryClaims(t, cli)
	legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Stop = ptr.To(true)
		its.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
	})
	legacyStopRecoveryReconcile(t, cli, 6)
	if len(legacyStopRecoveryPods(t, cli)) != 0 || legacyStopRecoverySet(t, cli).Status.Replicas != 0 {
		t.Fatal("MinReadySeconds blocked ordered Stop after the first Pod disappeared")
	}
	if !reflect.DeepEqual(claims, legacyStopRecoveryClaims(t, cli)) {
		t.Fatal("Stop changed storage")
	}
}

func TestLegacyInstanceSetStopRecoveryFlatTemplateChange(t *testing.T) {
	for _, retargetFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("retargetFirst=%t", retargetFirst), func(t *testing.T) {
			cli := legacyStopRecoveryFixture(t, 2, 0)
			legacyStopRecoveryReady(t, cli)
			legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.FlatInstanceOrdinal = true })
			legacyStopRecoveryReconcile(t, cli, 3)
			changeTemplate := func(its *workloads.InstanceSet) {
				its.Spec.Instances = []workloads.InstanceTemplate{{Name: "named", Replicas: ptr.To(int32(1)), Ordinals: workloads.Ordinals{Discrete: []int32{0}}}}
			}
			if retargetFirst {
				legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) {
					changeTemplate(its)
					its.Spec.InstanceUpdateStrategy = &workloads.InstanceUpdateStrategy{Type: kbappsv1.OnDeleteStrategyType}
				})
				legacyStopRecoveryReconcile(t, cli, 2)
			}
			claims := legacyStopRecoveryClaims(t, cli)
			legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.Stop = ptr.To(true)
				its.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
				changeTemplate(its)
			})
			legacyStopRecoveryReconcile(t, cli, 6)
			its := legacyStopRecoverySet(t, cli)
			if len(legacyStopRecoveryPods(t, cli)) != 0 || its.Status.Replicas != 0 || its.Status.ObservedGeneration != its.Generation {
				t.Fatal("Stop with a template change did not finish draining Pods")
			}
			historicalTemplate := ""
			if retargetFirst {
				historicalTemplate = "named"
			}
			status := its.FindInstanceStatus("legacy-stop-0")
			if status == nil || status.TemplateName == nil || *status.TemplateName != historicalTemplate || status.DesiredState != workloads.InstanceDesiredStateOffline || status.CurrentState != workloads.InstanceCurrentStateAbsent {
				t.Fatalf("Stop did not retain the retained template identity after Pod deletion: %#v", status)
			}
			if !reflect.DeepEqual(claims, legacyStopRecoveryClaims(t, cli)) {
				t.Fatal("Stop with a template change modified retained PVCs")
			}
			legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(false) })
			legacyStopRecoveryReconcile(t, cli, 2)
			legacyStopRecoveryReady(t, cli)
			legacyStopRecoveryReconcile(t, cli, 3)
			status = legacyStopRecoverySet(t, cli).FindInstanceStatus("legacy-stop-0")
			if len(legacyStopRecoveryPods(t, cli)) != 2 || status == nil || status.TemplateName == nil || *status.TemplateName != "named" || status.DesiredState != workloads.InstanceDesiredStateActive {
				t.Fatal("resume did not apply the desired template assignment")
			}
			for _, pod := range legacyStopRecoveryPods(t, cli) {
				if pod.Name == "legacy-stop-0" && pod.Labels[instancetemplate.TemplateNameLabelKey] != "named" {
					t.Fatal("resumed Pod did not use the desired template")
				}
			}
		})
	}
}

func TestLegacyInstanceSetStopRecoveryMalformedObservedConfig(t *testing.T) {
	cli := legacyStopRecoveryFixture(t, 2, 0)
	legacyStopRecoveryReady(t, cli)
	for _, status := range legacyStopRecoverySet(t, cli).Status.InstanceStatus {
		if !status.UpToDate || status.CurrentState != workloads.InstanceCurrentStatePresent {
			t.Fatal("fixture did not publish applied runtime before Stop")
		}
	}
	claims := legacyStopRecoveryClaims(t, cli)
	for _, pod := range legacyStopRecoveryPods(t, cli) {
		if pod.Annotations == nil {
			pod.Annotations = map[string]string{}
		}
		pod.Annotations[constant.CMInsConfigurationHashLabelKey] = "invalid-json"
		if err := cli.Update(context.Background(), &pod); err != nil {
			t.Fatal(err)
		}
	}
	legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Stop = ptr.To(true)
		its.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
	})
	legacyStopRecoveryReconcile(t, cli, 6)
	if len(legacyStopRecoveryPods(t, cli)) != 0 || legacyStopRecoverySet(t, cli).Status.Replicas != 0 {
		t.Fatal("malformed observed config blocked Stop")
	}
	if !reflect.DeepEqual(claims, legacyStopRecoveryClaims(t, cli)) {
		t.Fatal("Stop with malformed observed config changed storage")
	}
}

func TestLegacyInstanceSetStopRecoveryTerminatingSummary(t *testing.T) {
	cli := legacyStopRecoveryFixture(t, 1, 0)
	legacyStopRecoveryReady(t, cli)
	legacyStopRecoveryHold(t, cli)
	legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
	legacyStopRecoveryReconcile(t, cli, 2)
	assertTerminating := func() {
		t.Helper()
		its := legacyStopRecoverySet(t, cli)
		if its.Status.Replicas != 1 || its.Status.ReadyReplicas != 0 || its.Status.AvailableReplicas != 0 || its.Status.CurrentReplicas != 0 || its.Status.UpdatedReplicas != 0 {
			t.Fatalf("terminating runtime counted as usable: %#v", its.Status)
		}
		instance := its.FindInstanceStatus("legacy-stop-0")
		if instance == nil || instance.CurrentState != workloads.InstanceCurrentStateTerminating || instance.Ready || instance.Available || instance.UpToDate {
			t.Fatalf("detailed runtime contradicts terminating summary: %#v", instance)
		}
	}
	assertTerminating()
	legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(false) })
	legacyStopRecoveryReconcile(t, cli, 1)
	assertTerminating()
}

func TestLegacyInstanceSetStopRecoveryWaitsForTermination(t *testing.T) {
	cli := legacyStopRecoveryFixture(t, 2, 0)
	legacyStopRecoveryReady(t, cli)
	legacyStopRecoveryHold(t, cli)
	heldClaim := &corev1.PersistentVolumeClaim{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "data-legacy-stop-0"}, heldClaim); err != nil {
		t.Fatal(err)
	}
	legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
	failure := errors.New("injected Stop status failure before early resume")
	fired := false
	faulty := interceptor.NewClient(cli, interceptor.Funcs{
		SubResourceUpdate: func(ctx context.Context, delegate client.Client, name string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			if _, ok := obj.(*workloads.InstanceSet); ok && name == "status" && !fired {
				fired = true
				return failure
			}
			return delegate.SubResource(name).Update(ctx, obj, opts...)
		},
	})
	controller := &InstanceSetReconciler{Client: faulty, Recorder: record.NewFakeRecorder(1000)}
	if _, err := controller.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "legacy-stop"}}); !errors.Is(err, failure) || !fired {
		t.Fatalf("expected injected Stop status failure, got %v", err)
	}
	its := legacyStopRecoverySet(t, cli)
	status := its.FindInstanceStatus("legacy-stop-0")
	if its.Status.ObservedGeneration == its.Generation || status == nil || !status.UpToDate || status.CurrentState != workloads.InstanceCurrentStatePresent {
		t.Fatal("fixture did not retain stale applied status after persisted Stop deletion")
	}
	if pods := legacyStopRecoveryPods(t, cli); len(pods) != 1 || pods[0].DeletionTimestamp.IsZero() {
		t.Fatal("Stop status failure did not follow actual Pod deletions")
	}
	pod := legacyStopRecoveryPods(t, cli)[0]
	pod.Annotations = map[string]string{constant.CMInsConfigurationHashLabelKey: "invalid-json"}
	if err := cli.Update(context.Background(), &pod); err != nil {
		t.Fatal(err)
	}
	mock := kbacli.NewMockClient(gomock.NewController(t))
	kbacli.SetMockClient(mock, nil)
	t.Cleanup(kbacli.UnsetMockClient)
	legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Stop = ptr.To(false)
		its.Spec.Template.Spec.Containers[0].Image = "db:v2"
		its.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{{Name: "db", ContainerPort: 5432, Protocol: corev1.ProtocolTCP}}
		its.Spec.VolumeClaimTemplates[0].Annotations = map[string]string{"desired": "latest"}
		its.Spec.InstanceUpdateStrategy = &workloads.InstanceUpdateStrategy{Type: kbappsv1.RollingUpdateStrategyType, RollingUpdate: &workloads.RollingUpdate{MaxUnavailable: ptr.To(intstr.FromInt(2))}}
		its.Spec.Configs = []workloads.ConfigTemplate{{Name: "server", ConfigHash: ptr.To("new"), Reconfigure: &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"true"}}}}}
	})
	before := legacyStopRecoveryPods(t, cli)[0]
	if result := legacyStopRecoveryReconcile(t, cli, 1); result.RequeueAfter != time.Second {
		t.Fatalf("early resume did not wait for termination: %#v", result)
	}
	status = legacyStopRecoverySet(t, cli).FindInstanceStatus(before.Name)
	if status == nil || status.UpToDate {
		t.Fatal("resume target publication trusted stale applied status for a terminating Pod")
	}
	pods := legacyStopRecoveryPods(t, cli)
	if len(pods) != 2 {
		t.Fatal("termination wait blocked independent Parallel creation")
	}
	for _, pod := range pods {
		if pod.Name == before.Name {
			if !reflect.DeepEqual(before, pod) {
				t.Fatal("resume modified the terminating Pod")
			}
		} else {
			if pod.Spec.Containers[0].Image != "db:v2" || !pod.DeletionTimestamp.IsZero() {
				t.Fatal("independent Pod did not receive the latest resume payload")
			}
			pod.Status.Phase = corev1.PodPending
			if err := cli.Status().Update(context.Background(), &pod); err != nil {
				t.Fatal(err)
			}
		}
	}
	if result := legacyStopRecoveryReconcile(t, cli, 1); result.RequeueAfter != time.Second {
		t.Fatalf("termination wait did not reenter: %#v", result)
	}
	currentClaim := &corev1.PersistentVolumeClaim{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(heldClaim), currentClaim); err != nil || !reflect.DeepEqual(heldClaim, currentClaim) {
		t.Fatalf("termination wait changed held Pod storage: %v", err)
	}
	for _, claim := range legacyStopRecoveryClaims(t, cli) {
		if claim.Name != heldClaim.Name && claim.Annotations["desired"] != "latest" {
			t.Fatal("independent Pod PVC did not commit during termination wait")
		}
	}
	services := &corev1.ServiceList{}
	if err := cli.List(context.Background(), services); err != nil || len(services.Items) != 1 || len(services.Items[0].Spec.Ports) != 1 || services.Items[0].Spec.Ports[0].Port != 5432 {
		t.Fatalf("termination wait did not commit assistant changes: %v", err)
	}
	its = legacyStopRecoverySet(t, cli)
	if its.Status.ObservedGeneration != its.Generation || its.Status.Replicas != 2 || its.Status.UpdatedReplicas != 1 {
		t.Fatalf("termination wait did not commit current generation and runtime counts: generation=%d status=%#v", its.Generation, its.Status)
	}
}

func TestLegacyInstanceSetStopRecoveryFreezesPVCOnEarlyResume(t *testing.T) {
	cli := legacyStopRecoveryFixture(t, 1, 0)
	legacyStopRecoveryHold(t, cli)
	legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
	legacyStopRecoveryReconcile(t, cli, 2)
	claim := legacyStopRecoveryClaims(t, cli)[0]
	claim.UID = "claim-uid"
	claim.Spec.VolumeName = "retained-volume"
	if err := cli.Update(context.Background(), &claim); err != nil {
		t.Fatal(err)
	}
	before := legacyStopRecoveryClaims(t, cli)[0]
	legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Stop = ptr.To(false)
		its.Spec.InstanceUpdateStrategy = &workloads.InstanceUpdateStrategy{Type: kbappsv1.OnDeleteStrategyType}
		its.Spec.Template.Spec.Containers[0].Image = "db:v2"
		its.Spec.VolumeClaimTemplates[0].Annotations = map[string]string{"desired": "latest"}
		its.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
	})
	legacyStopRecoveryReconcile(t, cli, 3)
	if after := legacyStopRecoveryClaims(t, cli)[0]; !reflect.DeepEqual(before, after) {
		t.Fatalf("early resume mutated PVC before terminating Pod disappeared: before=%#v after=%#v", before, after)
	}
	pod := legacyStopRecoveryPods(t, cli)[0]
	if pod.DeletionTimestamp.IsZero() || pod.Spec.Containers[0].Image != "db:v1" {
		t.Fatal("early resume replaced or changed terminating runtime")
	}
	pod.Finalizers = nil
	if err := cli.Update(context.Background(), &pod); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("injected legacy resume Pod create failure")
	fired, created := false, 0
	faulty := interceptor.NewClient(cli, interceptor.Funcs{
		Create: func(ctx context.Context, delegate client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if _, ok := obj.(*corev1.Pod); ok {
				if !fired {
					fired = true
					return failure
				}
				err := delegate.Create(ctx, obj, opts...)
				if err == nil {
					created++
				}
				return err
			}
			return delegate.Create(ctx, obj, opts...)
		},
	})
	controller := &InstanceSetReconciler{Client: faulty, Recorder: record.NewFakeRecorder(1000)}
	_, err := controller.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "legacy-stop"}})
	if !errors.Is(err, failure) || !fired {
		t.Fatalf("expected injected resume failure, got %v", err)
	}
	committed := legacyStopRecoveryClaims(t, cli)[0]
	if len(legacyStopRecoveryPods(t, cli)) != 0 || created != 0 || committed.UID != before.UID || !reflect.DeepEqual(committed.OwnerReferences, before.OwnerReferences) || committed.Spec.VolumeName != before.Spec.VolumeName || committed.Annotations["desired"] != "latest" || committed.Spec.Resources.Requests.Storage().Cmp(resource.MustParse("2Gi")) != 0 {
		t.Fatal("failure did not follow an actual update of the retained PVC")
	}
	legacyStopRecoveryReconcile(t, faulty, 3)
	pods := legacyStopRecoveryPods(t, cli)
	if created != 1 || len(pods) != 1 || pods[0].Spec.Containers[0].Image != "db:v2" || !pods[0].DeletionTimestamp.IsZero() {
		t.Fatal("resume did not create latest runtime after held Pod disappeared")
	}
	current := legacyStopRecoveryClaims(t, cli)[0]
	if !reflect.DeepEqual(committed, current) {
		t.Fatal("resume did not apply latest target to retained claim")
	}
}

func TestLegacyInstanceSetStopRecoveryPartialCommit(t *testing.T) {
	for _, failStatus := range []bool{false, true} {
		t.Run(fmt.Sprintf("failStatus=%t", failStatus), func(t *testing.T) {
			cli := legacyStopRecoveryFixture(t, 3, 0)
			legacyStopRecoveryReady(t, cli)
			claims := legacyStopRecoveryClaims(t, cli)
			legacyStopRecoveryChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(true) })
			failure := errors.New("injected legacy Stop commit failure")
			fired, attempts, deleted := false, 0, 0
			faulty := interceptor.NewClient(cli, interceptor.Funcs{
				Delete: func(ctx context.Context, delegate client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
					if _, ok := obj.(*corev1.Pod); ok {
						attempts++
						if !failStatus && !fired && attempts == 2 {
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
				SubResourceUpdate: func(ctx context.Context, delegate client.Client, name string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
					if _, ok := obj.(*workloads.InstanceSet); ok && name == "status" && failStatus && !fired {
						fired = true
						return failure
					}
					return delegate.SubResource(name).Update(ctx, obj, opts...)
				},
			})
			controller := &InstanceSetReconciler{Client: faulty, Recorder: record.NewFakeRecorder(1000)}
			_, err := controller.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "legacy-stop"}})
			if !errors.Is(err, failure) || !fired {
				t.Fatalf("expected injected Stop failure, got %v", err)
			}
			wantDeleted := 1
			if failStatus {
				wantDeleted = 3
			}
			if deleted != wantDeleted || len(legacyStopRecoveryPods(t, cli)) != 3-wantDeleted {
				t.Fatalf("failure did not follow persisted Pod removal: deleted=%d", deleted)
			}
			if legacyStopRecoverySet(t, cli).Status.ObservedGeneration == legacyStopRecoverySet(t, cli).Generation {
				t.Fatal("failed commit unexpectedly persisted current root status")
			}
			if !reflect.DeepEqual(claims, legacyStopRecoveryClaims(t, cli)) {
				t.Fatal("partial Stop changed storage")
			}
			legacyStopRecoveryReconcile(t, faulty, 6)
			its := legacyStopRecoverySet(t, cli)
			if deleted != 3 || len(legacyStopRecoveryPods(t, cli)) != 0 || its.Status.Replicas != 0 || its.Status.ObservedGeneration != its.Generation {
				t.Fatal("fresh Stop retries did not converge from persisted resources")
			}
			if !reflect.DeepEqual(claims, legacyStopRecoveryClaims(t, cli)) {
				t.Fatal("Stop retry changed storage")
			}
		})
	}
}

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
		ObjectMeta: metav1.ObjectMeta{Name: "pvc-lifecycle", Namespace: "default", UID: "its-uid", Generation: 1},
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

func pvcLifecycleReconcile(t *testing.T, cli client.Client, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		reconciler := &InstanceSetReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(1000)}
		_, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "default", Name: "pvc-lifecycle"}})
		if err != nil {
			t.Fatal(err)
		}
	}
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
	original := pvcLifecycleSet(t, cli)
	if err := cli.Delete(context.Background(), original); err != nil {
		t.Fatal(err)
	}
	pvcLifecycleReconcile(t, cli, 6)
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(original), &workloads.InstanceSet{}); !apierrors.IsNotFound(err) {
		t.Fatalf("original InstanceSet was not deleted: %v", err)
	}
	claims := pvcLifecycleClaims(t, cli)
	if len(claims) != 2 {
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
			if !metav1.IsControlledBy(&claim, replacement) || !reflect.DeepEqual(claim.Spec, claims[0].Spec) {
				t.Fatal("active claim was not adopted with its storage preserved")
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

func TestInstanceSetPVCAllocationAndRetirement(t *testing.T) {
	cases := []struct {
		policy   kbappsv1.PersistentVolumeClaimRetentionPolicyType
		replicas int32
	}{
		{kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType, 0},
		{kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType, 1},
	}
	for _, tc := range cases {
		policy, replicas := tc.policy, tc.replicas
		t.Run(fmt.Sprintf("%s/replicas=%d", policy, replicas), func(t *testing.T) {
			cli := pvcLifecycleFixture(t, 2)
			pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
				its.Spec.Stop = ptr.To(true)
				its.Spec.Replicas = ptr.To(replicas)
				its.Spec.PersistentVolumeClaimRetentionPolicy = &kbappsv1.PersistentVolumeClaimRetentionPolicy{WhenScaled: policy}
			})
			pvcLifecycleReconcile(t, cli, 6)
			claims := pvcLifecycleClaims(t, cli)
			if len(pvcLifecyclePods(t, cli)) != 0 || len(claims) != 2 {
				t.Fatal("simultaneous Stop and scale-down did not retain both claims while draining")
			}
			pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) { its.Spec.Stop = ptr.To(false) })
			pvcLifecycleReconcile(t, cli, 3)
			if len(pvcLifecyclePods(t, cli)) != int(replicas) {
				t.Fatal("resume did not restore exactly the desired identities")
			}
			if policy == kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType {
				if !reflect.DeepEqual(claims, pvcLifecycleClaims(t, cli)) {
					t.Fatal("WhenScaled=Retain changed stopped claims")
				}
			} else {
				for _, claim := range claims {
					current := &corev1.PersistentVolumeClaim{}
					err := cli.Get(context.Background(), client.ObjectKeyFromObject(&claim), current)
					if !apierrors.IsNotFound(err) {
						t.Fatalf("WhenScaled=Delete retained absent retired identity claim: %v", err)
					}
				}
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
	for _, name := range []string{"data-pvc-lifecycle-0", "data-pvc-lifecycle-1"} {
		claim := &corev1.PersistentVolumeClaim{}
		if err := cli.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, claim); err != nil {
			t.Fatal(err)
		}
		allocated = append(allocated, *claim)
	}
	pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
		its.Spec.Replicas = ptr.To(int32(2))
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
			t.Fatal("partial retirement changed desired storage")
		}
	}
	pvcLifecycleReconcile(t, faulty, 5)
	if deleted != 2 || len(pvcLifecycleClaims(t, cli)) != 2 || len(pvcLifecyclePods(t, cli)) != 2 {
		t.Fatal("fresh retry did not complete retirement and resume")
	}
	for _, claim := range allocated {
		current := &corev1.PersistentVolumeClaim{}
		if err := cli.Get(context.Background(), client.ObjectKeyFromObject(&claim), current); err != nil || !reflect.DeepEqual(claim, *current) {
			t.Fatal("retirement retry changed desired storage")
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

func TestInstanceSetPVCOfflineRetirementPolicy(t *testing.T) {
	for _, policy := range []kbappsv1.PersistentVolumeClaimRetentionPolicyType{kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType, kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType} {
		for _, absent := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/absent=%t", policy, absent), func(t *testing.T) {
				cli := pvcLifecycleFixture(t, 1)
				original := pvcLifecycleClaims(t, cli)[0]
				if absent {
					pods := pvcLifecyclePods(t, cli)
					if err := cli.Delete(context.Background(), &pods[0]); err != nil {
						t.Fatal(err)
					}
				}
				pvcLifecycleChange(t, cli, func(its *workloads.InstanceSet) {
					its.Spec.OfflineInstances = []string{"pvc-lifecycle-0"}
					its.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{WhenScaled: policy}
				})
				pvcLifecycleReconcile(t, cli, 1)
				pvcLifecycleReady(t, cli)
				pvcLifecycleReconcile(t, cli, 4)
				pods := pvcLifecyclePods(t, cli)
				if len(pods) != 1 || pods[0].Name != "pvc-lifecycle-1" {
					t.Fatalf("specified replica was not retired: %#v", pods)
				}
				current := &corev1.PersistentVolumeClaim{}
				err := cli.Get(context.Background(), client.ObjectKeyFromObject(&original), current)
				if policy == kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType {
					if err != nil || current.UID != original.UID || !reflect.DeepEqual(current.Spec, original.Spec) {
						t.Fatalf("retention policy changed original storage: %v", err)
					}
				} else if !apierrors.IsNotFound(err) {
					t.Fatalf("Offline bypassed Delete retention policy: %v", err)
				}
				expected := 1
				if policy == kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType {
					expected++
				}
				if len(pvcLifecycleClaims(t, cli)) != expected {
					t.Fatal("retirement left unexpected claims")
				}
			})
		}
	}
}

func newMemberLifecycleITS(replicas int32) *workloads.InstanceSet {
	return &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "member-lifecycle", Namespace: "default", UID: "member-lifecycle", Generation: 1,
			Finalizers: []string{"instanceset.workloads.kubeblocks.io/finalizer"}},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To(replicas), Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "member-lifecycle"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "member-lifecycle"}}, Spec: corev1.PodSpec{
				ServiceAccountName: "member-agent", Containers: []corev1.Container{{Name: "db", Image: "database", VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}}}, {Name: kbagent.ContainerName, Image: "tools", Ports: []corev1.ContainerPort{{Name: kbagent.DefaultHTTPPortName, ContainerPort: 3501}, {Name: kbagent.DefaultStreamingPortName, ContainerPort: 3502}}}},
			}},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}, Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
			}}},
			LifecycleActions: &workloads.LifecycleActions{
				MemberJoin:  &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"join"}}},
				MemberLeave: &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"leave"}}},
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
	its := newMemberLifecycleITS(2)
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
		if status.MemberJoined != nil || status.Provisioned {
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
	if len(pods) != 1 || pods[0].Name != "member-lifecycle-0" || stored.FindInstanceStatus("member-lifecycle-1").MemberJoined != nil {
		t.Fatalf("OrderedReady pending initial identity was classified as expansion: pods=%+v status=%+v", pods, stored.Status.InstanceStatus)
	}
	for _, c := range pods[0].Spec.InitContainers {
		for _, env := range c.Env {
			if env.Name == "KB_AGENT_TASK" {
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
	if err != nil || stored.FindInstanceStatus("member-lifecycle-2") == nil || stored.FindInstanceStatus("member-lifecycle-2").MemberJoined == nil || *stored.FindInstanceStatus("member-lifecycle-2").MemberJoined || stored.FindInstanceStatus("member-lifecycle-1").MemberJoined != nil {
		t.Fatalf("new allocation while bootstrap remains pending was not distinguished: %+v err=%v", stored.Status.InstanceStatus, err)
	}
}

func TestInstanceSetLifecycleMemberActionsRespectStatusFailureAndLatestSpec(t *testing.T) {
	ctx := context.Background()
	its := newMemberLifecycleITS(1)

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
			if status := stored.FindInstanceStatus("member-lifecycle-1"); status != nil && status.MemberJoined != nil && !*status.MemberJoined {
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
	markReady("member-lifecycle-0")
	its = step()
	if joins != 0 || !ptr.Deref(its.FindInstanceStatus("member-lifecycle-0").MemberJoined, false) {
		t.Fatal("bootstrap membership was not satisfied without a join action")
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
	markReady("member-lifecycle-1")
	its = step()
	if joins != 1 || !ptr.Deref(its.FindInstanceStatus("member-lifecycle-1").MemberJoined, false) {
		t.Fatalf("member join did not execute and persist: calls=%d status=%+v", joins, its.Status.InstanceStatus)
	}
	setReplicas(1)
	rejectLeave = true
	stored, result, err := reconcileLifecycleITS(t, cli, key)
	if err != nil || !result.Requeue || leaves != 1 || !ptr.Deref(stored.FindInstanceStatus("member-lifecycle-1").MemberJoined, false) || len(lifecycleEntryPods(t, cli)) != 2 {
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
	if leaves != 2 || ptr.Deref(its.FindInstanceStatus("member-lifecycle-1").MemberJoined, true) || len(lifecycleEntryPods(t, cli)) != 2 {
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
	its := newMemberLifecycleITS(1)
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
	its.Spec.Template.Spec.InitContainers = nil
	its.Spec.Template.Spec.Containers[1] = corev1.Container{
		Name: kbagent.ContainerName, Image: "tools", Env: existingEnv,
		Ports: []corev1.ContainerPort{{Name: kbagent.DefaultHTTPPortName, ContainerPort: int32(port)}},
	}
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
	markReady("member-lifecycle-0")
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
	if err := cli.Get(ctx, client.ObjectKey{Namespace: its.Namespace, Name: "member-lifecycle-1"}, target); err != nil {
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
	its := newMemberLifecycleITS(0)

	its.Spec.LifecycleActions.MemberLeave = &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"leave"}}}
	its.Status.ObservedGeneration = its.Generation
	its.Status.InstanceStatus = []workloads.InstanceStatus{{PodName: "member-lifecycle-0", Provisioned: true, DesiredState: workloads.InstanceDesiredStateReleased, CurrentState: workloads.InstanceCurrentStateAbsent}}
	cli := newLifecycleEntryClient(t, its, interceptor.Funcs{})
	for i := 0; i < 3; i++ {
		stored, result, err := reconcileLifecycleITS(t, cli, client.ObjectKeyFromObject(its))
		status := stored.FindInstanceStatus("member-lifecycle-0")
		if err != nil || result.RequeueAfter == 0 || status == nil || !status.Provisioned || status.MemberJoined != nil {
			t.Fatalf("unresolved bootstrap membership was dropped or invented: status=%+v result=%+v err=%v", status, result, err)
		}
	}
}

func TestInstanceSetLifecycleStopAndOfflineDoNotRequireLeave(t *testing.T) {
	for _, stop := range []bool{false, true} {
		t.Run(fmt.Sprintf("stop=%v", stop), func(t *testing.T) {
			ctx := context.Background()
			its := newMemberLifecycleITS(1)

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
			status := its.FindInstanceStatus("member-lifecycle-0")
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
				its.Spec.OfflineInstances = []string{"member-lifecycle-0"}
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
			status = its.FindInstanceStatus("member-lifecycle-0")
			if status == nil || !ptr.Deref(status.MemberJoined, false) {
				t.Fatalf("stop/offline invented leave completion: %+v", status)
			}
		})
	}
}

func TestInstanceSetLifecycleRegistersCompleteFlatMultitemplateAllocation(t *testing.T) {
	ctx := context.Background()
	its := newMemberLifecycleITS(3)
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
	status := stored.FindInstanceStatus("member-lifecycle-3")
	if err != nil || len(stored.Status.InstanceStatus) != 4 || status == nil || status.TemplateName == nil || *status.TemplateName != "other" || len(lifecycleEntryPods(t, cli)) != 0 {
		t.Fatalf("new flat identity while bootstrap is pending was not registered alone: %+v %v", stored.Status.InstanceStatus, err)
	}
}

type lifecycleEntry struct {
	cli         client.Client
	its         *workloads.InstanceSet
	instanceAPI bool
}

func newMemberLifecycleEntry(t *testing.T, instanceAPI bool, replicas int32, policy appsv1.PodManagementPolicyType) *lifecycleEntry {
	t.Helper()
	var its *workloads.InstanceSet
	var cli client.Client
	if instanceAPI {
		its, cli, _ = its2LifecycleFixture(t, replicas)
	} else {
		its = newMemberLifecycleITS(replicas)

		its.Spec.LifecycleActions.MemberJoin = &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"join"}}}
		its.Spec.LifecycleActions.MemberLeave = &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"leave"}}}
		cli = newLifecycleEntryClient(t, its, interceptor.Funcs{})
	}
	its.Spec.PodManagementPolicy = policy
	its.Spec.ParallelPodManagementConcurrency = ptr.To(intstr.FromInt(1))
	if err := cli.Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
	return &lifecycleEntry{cli: cli, its: its, instanceAPI: instanceAPI}
}

func (f *lifecycleEntry) step(t *testing.T) ctrl.Result {
	t.Helper()
	req := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(f.its)}
	var result ctrl.Result
	var err error
	if f.instanceAPI {
		r := &InstanceSetReconciler2{Client: f.cli, Scheme: f.cli.Scheme(), Recorder: record.NewFakeRecorder(100)}
		result, err = r.Reconcile(context.Background(), req)
	} else {
		r := &InstanceSetReconciler{Client: f.cli, Scheme: f.cli.Scheme(), Recorder: record.NewFakeRecorder(100)}
		result, err = r.Reconcile(context.Background(), req)
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := f.cli.Get(context.Background(), req.NamespacedName, f.its); err != nil {
		t.Fatal(err)
	}
	return result
}

func (f *lifecycleEntry) stepWithChildren(t *testing.T) ctrl.Result {
	t.Helper()
	result := f.step(t)
	if f.instanceAPI {
		children := &workloads.InstanceList{}
		if err := f.cli.List(context.Background(), children); err != nil {
			t.Fatal(err)
		}
		for i := range children.Items {
			leaf := &InstanceReconciler{Client: f.cli, Scheme: f.cli.Scheme(), Recorder: record.NewFakeRecorder(100)}
			if _, err := leaf.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&children.Items[i])}); err != nil {
				t.Fatal(err)
			}
		}
	}
	return result
}

func (f *lifecycleEntry) setReady(t *testing.T, name string, ready bool) {
	t.Helper()
	ctx := context.Background()
	pod := &corev1.Pod{}
	if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: name}, pod); err != nil {
		t.Fatal(err)
	}
	pod.Status = corev1.PodStatus{InitContainerStatuses: pod.Status.InitContainerStatuses, Phase: corev1.PodRunning, PodIP: "10.0.0.1", Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse, LastTransitionTime: metav1.Now()}}}
	if ready {
		pod.Status.Conditions[0].Status = corev1.ConditionTrue
	}
	for _, c := range pod.Spec.Containers {
		pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, corev1.ContainerStatus{Name: c.Name, Image: c.Image, Ready: ready})
	}
	if err := f.cli.Status().Update(ctx, pod); err != nil {
		t.Fatal(err)
	}
	if f.instanceAPI {
		leaf := &InstanceReconciler{Client: f.cli, Scheme: f.cli.Scheme(), Recorder: record.NewFakeRecorder(100)}
		if _, err := leaf.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(pod)}); err != nil {
			t.Fatal(err)
		}
	}
}

func (f *lifecycleEntry) bootstrap(t *testing.T, replicas int) {
	t.Helper()
	for round := 0; round < 24; round++ {
		result := f.stepWithChildren(t)
		pods := lifecycleEntryPods(t, f.cli)
		for _, pod := range pods {
			f.setReady(t, pod.Name, true)
		}
		if len(pods) == replicas && result.RequeueAfter == 5*time.Second && f.its.Status.ReadyReplicas == int32(replicas) {
			return
		}
	}
	t.Fatalf("bootstrap did not converge: %#v", f.its.Status)
}

func TestInstanceSetLifecycleInitialMembershipSatisfiedOnObservation(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, configured := range []bool{false, true} {
			t.Run(fmt.Sprintf("instanceAPI=%t/configured=%t", instanceAPI, configured), func(t *testing.T) {
				f := newMemberLifecycleEntry(t, instanceAPI, 1, appsv1.ParallelPodManagement)
				if !configured {
					f.its.Spec.LifecycleActions = nil
					if err := f.cli.Update(context.Background(), f.its); err != nil {
						t.Fatal(err)
					}
				}
				agent := &its2LifecycleAgent{}
				kbacli.SetMockClient(agent, nil)
				defer kbacli.UnsetMockClient()
				f.step(t)
				if configured && (len(lifecycleEntryPods(t, f.cli)) != 0 || f.its.FindInstanceStatus(f.its.Name+"-0").MemberJoined != nil) {
					t.Fatal("registration satisfied membership before observing runtime")
				}
				for round := 0; round < 12; round++ {
					f.stepWithChildren(t)
					if status := f.its.FindInstanceStatus(f.its.Name + "-0"); status != nil && status.Provisioned {
						break
					}
				}
				status := f.its.FindInstanceStatus(f.its.Name + "-0")
				if status == nil || !status.Provisioned || (configured && !ptr.Deref(status.MemberJoined, false)) || (!configured && status.MemberJoined != nil) {
					t.Fatalf("observed initial membership not satisfied according to configured step: %#v", status)
				}
				if instanceAPI {
					child := its2LifecycleInstance(t, f.cli, status.PodName)
					if !reflect.DeepEqual(child.Status.MemberJoined, status.MemberJoined) {
						t.Fatalf("child and parent membership differ: child=%#v parent=%#v", child.Status, status)
					}
				}
				if len(agent.calls) != 0 {
					t.Fatalf("initial membership satisfaction ran actions: %v", agent.calls)
				}
			})
		}
	}
}

func TestInstanceSetLifecycleReplacementWaitsForJoin(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {

		t.Run(fmt.Sprintf("instanceAPI=%t", instanceAPI), func(t *testing.T) {
			ctx := context.Background()
			f := newMemberLifecycleEntry(t, instanceAPI, 1, appsv1.OrderedReadyPodManagement)

			f.bootstrap(t, 1)
			sourceName, targetName := f.its.Name+"-0", f.its.Name+"-1"
			agent := &its2LifecycleAgent{}
			kbacli.SetMockClient(agent, nil)
			defer kbacli.UnsetMockClient()
			f.its.Spec.Ordinals = workloads.Ordinals{Discrete: []int32{1}}
			f.its.Generation++
			if err := f.cli.Update(ctx, f.its); err != nil {
				t.Fatal(err)
			}
			for round := 0; round < 12; round++ {
				f.stepWithChildren(t)
				if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: targetName}, &corev1.Pod{}); err == nil {
					break
				} else if !apierrors.IsNotFound(err) {
					t.Fatal(err)
				}
			}

			for round := 0; round < 8; round++ {
				f.stepWithChildren(t)
			}
			if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: sourceName}, &corev1.Pod{}); err != nil || len(agent.calls) != 0 {
				t.Fatalf("old member retired before replacement joined: %v calls=%v", err, agent.calls)
			}
			if status := f.its.FindInstanceStatus(targetName); status == nil || status.MemberJoined == nil || *status.MemberJoined {
				t.Fatalf("observed replacement completed a pending join: %#v", status)
			}
			if instanceAPI && its2LifecycleInstance(t, f.cli, targetName).Status.MemberJoined != nil {
				t.Fatal("pending replacement child initialized membership without Join")
			}
			f.setReady(t, targetName, true)
			for round := 0; round < 12; round++ {
				f.stepWithChildren(t)
			}
			if len(agent.calls) != 2 || agent.calls[0] != "memberJoin" || agent.calls[1] != "memberLeave" {
				t.Fatalf("replacement did not join before leave: %v", agent.calls)
			}
			if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: sourceName}, &corev1.Pod{}); !apierrors.IsNotFound(err) {
				t.Fatalf("joined replacement did not release old member: %v", err)
			}
		})

	}
}

func TestInstanceSetLifecycleRetirementRespectsAlignmentAdmission(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, policy := range []appsv1.PodManagementPolicyType{appsv1.OrderedReadyPodManagement, appsv1.ParallelPodManagement} {
			t.Run(fmt.Sprintf("instanceAPI=%t/%s", instanceAPI, policy), func(t *testing.T) {
				f := newMemberLifecycleEntry(t, instanceAPI, 3, policy)
				f.bootstrap(t, 3)
				agent := &its2LifecycleAgent{}
				kbacli.SetMockClient(agent, nil)
				defer kbacli.UnsetMockClient()
				f.its.Spec.Replicas = ptr.To[int32](1)
				f.its.Generation++
				if err := f.cli.Update(context.Background(), f.its); err != nil {
					t.Fatal(err)
				}
				if policy == appsv1.ParallelPodManagement {
					f.setReady(t, f.its.Name+"-0", false)
					f.step(t)
					if len(agent.calls) != 0 {
						t.Fatalf("Leave ran without parallel admission: %v", agent.calls)
					}
					f.setReady(t, f.its.Name+"-0", true)
				}
				for round := 0; round < 6 && len(agent.calls) == 0; round++ {
					f.step(t)
				}
				retired := f.its.FindInstanceStatus(f.its.Name + "-2")
				next := f.its.FindInstanceStatus(f.its.Name + "-1")
				if len(agent.calls) != 1 || agent.calls[0] != "memberLeave" || retired == nil || retired.MemberJoined == nil || *retired.MemberJoined || next == nil || !ptr.Deref(next.MemberJoined, false) {
					t.Fatalf("membership bypassed retirement order: calls=%v status=%#v", agent.calls, f.its.Status.InstanceStatus)
				}
				if len(lifecycleEntryPods(t, f.cli)) != 3 {
					t.Fatal("member result round removed runtime")
				}
				if instanceAPI && ptr.Deref(its2LifecycleInstance(t, f.cli, f.its.Name+"-2").Spec.ScaledDown, false) {
					t.Fatal("member result round marked ScaledDown")
				}
				f.step(t)
				if len(agent.calls) != 1 {
					t.Fatal("another Leave ran while committing the first runtime removal")
				}
				if instanceAPI {
					if !ptr.Deref(its2LifecycleInstance(t, f.cli, f.its.Name+"-2").Spec.ScaledDown, false) {
						t.Fatal("committed Leave did not permit ScaledDown")
					}
				} else if len(lifecycleEntryPods(t, f.cli)) != 2 {
					t.Fatal("committed Leave did not permit deletion")
				}
			})
		}
	}
}

func TestInstanceSetLifecycleReachesUpdateAndPollsSuccessfulExits(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		t.Run(fmt.Sprintf("instanceAPI=%t", instanceAPI), func(t *testing.T) {
			f := newMemberLifecycleEntry(t, instanceAPI, 1, appsv1.OrderedReadyPodManagement)
			f.bootstrap(t, 1)
			f.its.Spec.InstanceUpdateStrategy = &workloads.InstanceUpdateStrategy{Type: kbappsv1.OnDeleteStrategyType}
			f.its.Generation++
			if err := f.cli.Update(context.Background(), f.its); err != nil {
				t.Fatal(err)
			}
			if result := f.step(t); result.RequeueAfter != 5*time.Second {
				t.Fatalf("successful OnDelete exit lost observation poll: %#v", result)
			}
			f.its.Spec.InstanceUpdateStrategy = nil
			f.its.Spec.Template.Spec.Containers[0].Image = "database:v2"
			f.its.Spec.Template.Spec.Containers[0].Command = []string{"database-v2"}
			f.its.Spec.PodUpdatePolicy = kbappsv1.ReCreatePodUpdatePolicyType
			f.its.Generation++
			if err := f.cli.Update(context.Background(), f.its); err != nil {
				t.Fatal(err)
			}
			handedOff := false
			for round := 0; round < 6; round++ {
				result := f.step(t)
				if instanceAPI {
					child := its2LifecycleInstance(t, f.cli, f.its.Name+"-0")
					handedOff = child.Spec.Template.Spec.Containers[0].Image == "database:v2"
				} else {
					pods := lifecycleEntryPods(t, f.cli)
					handedOff = len(pods) == 0 || pods[0].Spec.Containers[0].Image == "database:v2"
				}
				if handedOff {
					if result.RequeueAfter == 0 {
						t.Fatal("successful update lost lifecycle poll")
					}
					break
				}
			}
			if !handedOff {
				t.Fatal("lifecycle alignment prevented actual rollout")
			}
		})
	}
}

func TestInstanceSetLifecyclePollsUnalignedUpdateExit(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		t.Run(fmt.Sprintf("instanceAPI=%t", instanceAPI), func(t *testing.T) {
			f := newMemberLifecycleEntry(t, instanceAPI, 1, appsv1.OrderedReadyPodManagement)
			f.bootstrap(t, 1)
			f.setReady(t, f.its.Name+"-0", false)
			f.its.Spec.Replicas = ptr.To[int32](2)
			f.its.Generation++
			if err := f.cli.Update(context.Background(), f.its); err != nil {
				t.Fatal(err)
			}
			f.step(t)
			if result := f.step(t); result.RequeueAfter != 5*time.Second {
				t.Fatalf("unaligned update exit lost observation poll: %#v", result)
			}
			if len(lifecycleEntryPods(t, f.cli)) != 1 {
				t.Fatal("OrderedReady admitted creation before predecessor became available")
			}
		})
	}
}

func TestITS2LifecycleInstanceWithoutRuntimeIsNotProvisioned(t *testing.T) {
	its, cli, r := its2LifecycleFixture(t, 1)
	its2LifecycleRound(t, r, its)
	its2LifecycleRound(t, r, its)
	child := its2LifecycleInstance(t, cli, "lifecycle-0")
	for round := 0; round < 3; round++ {
		its2LifecycleRound(t, r, its)
	}
	stored := its2LifecycleRead(t, cli, its)
	status := stored.FindInstanceStatus(child.Name)
	if status == nil || status.Provisioned || status.MemberJoined != nil || len(lifecycleEntryPods(t, cli)) != 0 {
		t.Fatalf("Instance spec invented runtime or member result: %#v", stored.Status.InstanceStatus)
	}
	stored.Spec.Replicas = ptr.To[int32](0)
	stored.Generation++
	if err := cli.Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	agent := &its2LifecycleAgent{}
	kbacli.SetMockClient(agent, nil)
	defer kbacli.UnsetMockClient()
	its2LifecycleRound(t, r, its)
	if len(agent.calls) != 0 || !ptr.Deref(its2LifecycleInstance(t, cli, child.Name).Spec.ScaledDown, false) {
		t.Fatalf("unprovisioned identity required an invented Leave: %v", agent.calls)
	}
}

func TestInstanceSetLifecycleAllocationSwitchSeparatesCreationAndMemberResult(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		t.Run(fmt.Sprintf("instanceAPI=%t", instanceAPI), func(t *testing.T) {
			f := newMemberLifecycleEntry(t, instanceAPI, 2, appsv1.ParallelPodManagement)
			f.bootstrap(t, 2)
			agent := &its2LifecycleAgent{}
			kbacli.SetMockClient(agent, nil)
			defer kbacli.UnsetMockClient()
			f.its.Spec.Ordinals = workloads.Ordinals{Discrete: []int32{0, 2}}
			f.its.Generation++
			if err := f.cli.Update(context.Background(), f.its); err != nil {
				t.Fatal(err)
			}
			f.step(t)
			if len(agent.calls) != 0 || len(lifecycleEntryPods(t, f.cli)) != 2 {
				t.Fatal("registration changed runtime or membership")
			}
			f.step(t)
			if len(agent.calls) != 0 {
				t.Fatal("member action shared an uncommitted creation round")
			}
			if instanceAPI {
				leaf := &InstanceReconciler{Client: f.cli, Scheme: f.cli.Scheme(), Recorder: record.NewFakeRecorder(100)}
				for i := 0; i < 2; i++ {
					if _, err := leaf.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-2"}}); err != nil {
						t.Fatal(err)
					}
				}
			}
			f.setReady(t, f.its.Name+"-2", true)
			for round := 0; round < 6; round++ {
				f.step(t)
				if status := f.its.FindInstanceStatus(f.its.Name + "-2"); status != nil && ptr.Deref(status.MemberJoined, false) {
					break
				}
			}
			if len(agent.calls) != 1 || agent.calls[0] != "memberJoin" || len(lifecycleEntryPods(t, f.cli)) != 3 {
				t.Fatalf("new member did not join before retirement: calls=%v status=%#v", agent.calls, f.its.Status.InstanceStatus)
			}
			faulty := &its2LifecycleClient{Client: f.cli, failStatus: true}
			f.cli = faulty
			result := f.step(t)
			old := f.its.FindInstanceStatus(f.its.Name + "-1")
			if !result.Requeue || len(agent.calls) != 2 || agent.calls[1] != "memberLeave" || old == nil || !ptr.Deref(old.MemberJoined, false) || len(lifecycleEntryPods(t, f.cli)) != 3 {
				t.Fatalf("member result conflict leaked runtime mutation: calls=%v status=%#v result=%#v", agent.calls, f.its.Status.InstanceStatus, result)
			}
			if instanceAPI && ptr.Deref(its2LifecycleInstance(t, f.cli, f.its.Name+"-1").Spec.ScaledDown, false) {
				t.Fatal("member result conflict leaked ScaledDown")
			}
			f.step(t)
			old = f.its.FindInstanceStatus(f.its.Name + "-1")
			wantCalls := 3
			if instanceAPI {
				wantCalls = 2
			}
			if len(agent.calls) != wantCalls || old == nil || old.MemberJoined == nil || *old.MemberJoined || len(lifecycleEntryPods(t, f.cli)) != 3 {
				t.Fatalf("member result retry did not commit alone: calls=%v status=%#v", agent.calls, f.its.Status.InstanceStatus)
			}
		})
	}
}

func TestITS2LifecycleZeroRegistrationPrecedesExpansion(t *testing.T) {
	its, cli, r := its2LifecycleFixture(t, 0)
	its2LifecycleRound(t, r, its)
	stored := its2LifecycleRead(t, cli, its)
	if stored.Status.ObservedGeneration != stored.Generation || len(stored.Status.InstanceStatus) != 0 {
		t.Fatalf("zero bootstrap was not registered: %#v", stored.Status)
	}
	stored.Spec.Replicas = ptr.To[int32](1)
	stored.Generation++
	if err := cli.Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	cli.failStatus = true
	its2LifecycleRound(t, r, its)
	stored = its2LifecycleRead(t, cli, its)
	children := &workloads.InstanceList{}
	if err := cli.List(context.Background(), children); err != nil {
		t.Fatal(err)
	}
	if len(children.Items) != 0 || len(stored.Status.InstanceStatus) != 0 || stored.Status.ObservedGeneration != 1 {
		t.Fatalf("new-generation registration conflict created runtime: %#v %#v", children.Items, stored.Status)
	}
	its2LifecycleRound(t, r, its)
	stored = its2LifecycleRead(t, cli, its)
	status := stored.FindInstanceStatus("lifecycle-0")
	if status == nil || status.MemberJoined == nil || *status.MemberJoined {
		t.Fatalf("zero-to-one was not registered as expansion: %#v", stored.Status)
	}
}

func TestInstanceSetLifecycleRegistersExpansionWhileStopped(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {

		t.Run(fmt.Sprintf("instanceAPI=%t", instanceAPI), func(t *testing.T) {
			ctx := context.Background()
			f := newMemberLifecycleEntry(t, instanceAPI, 1, appsv1.OrderedReadyPodManagement)

			f.bootstrap(t, 1)
			agent := &its2LifecycleAgent{}
			kbacli.SetMockClient(agent, nil)
			defer kbacli.UnsetMockClient()
			f.its.Spec.Stop = ptr.To(true)
			f.its.Generation++
			if err := f.cli.Update(ctx, f.its); err != nil {
				t.Fatal(err)
			}
			for round := 0; round < 8; round++ {
				f.stepWithChildren(t)
			}
			f.its.Spec.Replicas = ptr.To[int32](2)
			f.its.Generation++
			if err := f.cli.Update(ctx, f.its); err != nil {
				t.Fatal(err)
			}
			for round := 0; round < 6; round++ {
				f.stepWithChildren(t)
			}
			targetName := f.its.Name + "-1"
			status := f.its.FindInstanceStatus(targetName)
			if status == nil || status.EffectiveDesiredState() != workloads.InstanceDesiredStateOffline || status.MemberJoined == nil || *status.MemberJoined {
				t.Fatalf("stopped expansion lost loading or membership requirements: %#v", f.its.Status.InstanceStatus)
			}
			if len(lifecycleEntryPods(t, f.cli)) != 0 || len(agent.calls) != 0 {
				t.Fatal("stopped allocation created runtime or executed a member action")
			}
			f.its.Spec.Stop = ptr.To(false)
			f.its.Generation++
			if err := f.cli.Update(ctx, f.its); err != nil {
				t.Fatal(err)
			}
			for round := 0; round < 4; round++ {
				f.stepWithChildren(t)
			}
			f.setReady(t, f.its.Name+"-0", true)
			for round := 0; round < 6; round++ {
				f.stepWithChildren(t)
			}
			target := &corev1.Pod{}
			if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: targetName}, target); err != nil {
				t.Fatal(err)
			}

			f.setReady(t, targetName, true)
			for round := 0; round < 8; round++ {
				f.stepWithChildren(t)
			}
			status = f.its.FindInstanceStatus(targetName)
			if status == nil || !ptr.Deref(status.MemberJoined, false) || len(agent.calls) != 1 || agent.calls[0] != "memberJoin" {
				t.Fatalf("resumed allocation did not complete loading and Join: %#v calls=%v", status, agent.calls)
			}
		})

	}
}

func TestInstanceSetLifecycleRetiresMemberWithoutPod(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		t.Run(fmt.Sprintf("instanceAPI=%t", instanceAPI), func(t *testing.T) {
			ctx := context.Background()
			f := newMemberLifecycleEntry(t, instanceAPI, 2, appsv1.ParallelPodManagement)
			f.its.Spec.LifecycleActions.MemberLeave.TargetPodSelector = kbappsv1.AnyReplica
			f.its.Spec.VolumeClaimTemplates = newMemberLifecycleITS(2).Spec.VolumeClaimTemplates
			f.its.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{WhenScaled: kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType}
			if err := f.cli.Update(ctx, f.its); err != nil {
				t.Fatal(err)
			}
			f.bootstrap(t, 2)
			agent := &its2LifecycleAgent{}
			kbacli.SetMockClient(agent, nil)
			defer kbacli.UnsetMockClient()
			targetKey := client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-1"}
			target := &corev1.Pod{}
			if err := f.cli.Get(ctx, targetKey, target); err != nil {
				t.Fatal(err)
			}
			pvcKey := client.ObjectKey{Namespace: f.its.Namespace, Name: "data-" + target.Name}
			if err := f.cli.Get(ctx, pvcKey, &corev1.PersistentVolumeClaim{}); err != nil {
				t.Fatal(err)
			}
			target.Finalizers = nil
			if err := f.cli.Update(ctx, target); err != nil {
				t.Fatal(err)
			}
			if err := f.cli.Delete(ctx, target); err != nil {
				t.Fatal(err)
			}
			if err := f.cli.Get(ctx, targetKey, &corev1.Pod{}); !apierrors.IsNotFound(err) {
				t.Fatalf("Pod deletion did not complete: %v", err)
			}
			f.its.Spec.Replicas = ptr.To[int32](1)
			f.its.Generation++
			if err := f.cli.Update(ctx, f.its); err != nil {
				t.Fatal(err)
			}
			for round := 0; round < 24; round++ {
				f.stepWithChildren(t)
			}
			if len(agent.calls) != 1 || agent.calls[0] != "memberLeave" {
				t.Fatalf("absent member did not leave through its healthy peer: calls=%v status=%#v", agent.calls, f.its.FindInstanceStatus(target.Name))
			}
			if status := f.its.FindInstanceStatus(target.Name); status != nil {
				t.Fatalf("completed Leave retained the released identity: %#v", status)
			}
			if err := f.cli.Get(ctx, pvcKey, &corev1.PersistentVolumeClaim{}); !apierrors.IsNotFound(err) {
				t.Fatalf("completed Leave did not clean the released PVC: %v", err)
			}
		})
	}
}

func TestInstanceSetLifecycleLeaveWaitsWithoutExecutor(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, policy := range []appsv1.PodManagementPolicyType{appsv1.OrderedReadyPodManagement, appsv1.ParallelPodManagement} {
			t.Run(fmt.Sprintf("instanceAPI=%t/%s", instanceAPI, policy), func(t *testing.T) {
				defer func() {
					if got := recover(); got != nil {
						t.Fatalf("missing executor must wait instead of panic: %v", got)
					}
				}()
				ctx := context.Background()
				f := newMemberLifecycleEntry(t, instanceAPI, 1, policy)
				f.its.Spec.LifecycleActions.MemberLeave.TargetPodSelector = kbappsv1.AnyReplica
				f.its.Spec.VolumeClaimTemplates = newMemberLifecycleITS(1).Spec.VolumeClaimTemplates
				f.its.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{WhenScaled: kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType}
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				f.bootstrap(t, 1)
				agent := &its2LifecycleAgent{}
				kbacli.SetMockClient(agent, nil)
				defer kbacli.UnsetMockClient()
				key := client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-0"}
				pod := &corev1.Pod{}
				if err := f.cli.Get(ctx, key, pod); err != nil {
					t.Fatal(err)
				}
				pod.Finalizers = nil
				if err := f.cli.Update(ctx, pod); err != nil {
					t.Fatal(err)
				}
				if err := f.cli.Delete(ctx, pod); err != nil {
					t.Fatal(err)
				}
				f.its.Spec.Replicas = ptr.To[int32](0)
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 6; round++ {
					result := f.step(t)
					status := f.its.FindInstanceStatus(key.Name)
					if !workloadlifecycle.NeedsLeave(f.its, status) || result.RequeueAfter == 0 || len(agent.calls) != 0 {
						t.Fatalf("missing executor lost the pending Leave or retry: status=%#v result=%#v calls=%v", status, result, agent.calls)
					}
				}
				pvcKey := client.ObjectKey{Namespace: key.Namespace, Name: "data-" + key.Name}
				if err := f.cli.Get(ctx, pvcKey, &corev1.PersistentVolumeClaim{}); err != nil {
					t.Fatalf("waiting for Leave removed storage: %v", err)
				}
				f.its.Spec.Replicas = ptr.To[int32](1)
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				f.bootstrap(t, 1)
				f.its.Spec.Replicas = ptr.To[int32](0)
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 16; round++ {
					f.stepWithChildren(t)
				}
				if len(agent.calls) != 1 || agent.calls[0] != "memberLeave" || f.its.FindInstanceStatus(key.Name) != nil {
					t.Fatalf("restored executor did not finish Leave: calls=%v status=%#v", agent.calls, f.its.FindInstanceStatus(key.Name))
				}
				if err := f.cli.Get(ctx, pvcKey, &corev1.PersistentVolumeClaim{}); !apierrors.IsNotFound(err) {
					t.Fatalf("completed Leave did not clean storage: %v", err)
				}
			})
		}
	}
}

func TestInstanceSetLifecycleStopScaleThenResume(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, policy := range []appsv1.PodManagementPolicyType{appsv1.OrderedReadyPodManagement, appsv1.ParallelPodManagement} {
			for _, retention := range []kbappsv1.PersistentVolumeClaimRetentionPolicyType{kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType, kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType} {
				t.Run(fmt.Sprintf("instanceAPI=%t/%s/%s", instanceAPI, policy, retention), func(t *testing.T) {
					ctx := context.Background()
					f := newMemberLifecycleEntry(t, instanceAPI, 2, policy)
					f.its.Spec.LifecycleActions.MemberLeave.TargetPodSelector = kbappsv1.AnyReplica
					f.its.Spec.VolumeClaimTemplates = newMemberLifecycleITS(2).Spec.VolumeClaimTemplates
					f.its.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{WhenScaled: retention}
					if err := f.cli.Update(ctx, f.its); err != nil {
						t.Fatal(err)
					}
					f.bootstrap(t, 2)
					key := client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-1"}
					pod := &corev1.Pod{}
					if err := f.cli.Get(ctx, key, pod); err != nil {
						t.Fatal(err)
					}
					wantDNS := pod.Spec.Hostname + "." + intctrlutil.ServiceFQDN(pod.Namespace, pod.Spec.Subdomain)
					failed := &unavailableMemberAgent{}
					kbacli.SetMockClient(failed, nil)
					defer kbacli.UnsetMockClient()
					f.its.Spec.Stop = ptr.To(true)
					f.its.Generation++
					if err := f.cli.Update(ctx, f.its); err != nil {
						t.Fatal(err)
					}
					for round := 0; round < 24; round++ {
						f.stepWithChildren(t)
					}
					if pods := lifecycleEntryPods(t, f.cli); len(pods) != 0 {
						t.Fatalf("Stop did not remove all Pods: %v", pods)
					}
					f.its.Spec.Replicas = ptr.To[int32](1)
					f.its.Generation++
					if err := f.cli.Update(ctx, f.its); err != nil {
						t.Fatal(err)
					}
					for round := 0; round < 8; round++ {
						f.stepWithChildren(t)
					}
					if failed.calls != 0 {
						t.Fatalf("Stop executed membership actions: %d", failed.calls)
					}
					f.its.Spec.Stop = ptr.To(false)
					f.its.Generation++
					if err := f.cli.Update(ctx, f.its); err != nil {
						t.Fatal(err)
					}
					for round := 0; round < 24; round++ {
						f.stepWithChildren(t)
						for _, current := range lifecycleEntryPods(t, f.cli) {
							if current.Name == f.its.Name+"-0" {
								f.setReady(t, current.Name, true)
							}
						}
					}
					if failed.calls == 0 || !workloadlifecycle.NeedsLeave(f.its, f.its.FindInstanceStatus(key.Name)) {
						t.Fatalf("resumed peer did not retry the failed Leave: calls=%d status=%#v", failed.calls, f.its.FindInstanceStatus(key.Name))
					}
					pvcKey := client.ObjectKey{Namespace: key.Namespace, Name: "data-" + key.Name}
					if err := f.cli.Get(ctx, pvcKey, &corev1.PersistentVolumeClaim{}); err != nil {
						t.Fatalf("failed Leave removed storage: %v", err)
					}
					agent := &memberDNSAgent{}
					kbacli.SetMockClient(agent, nil)
					for round := 0; round < 16; round++ {
						f.stepWithChildren(t)
					}
					assertMemberDNSRequest(t, agent.requests, "memberLeave", "KB_LEAVE_MEMBER_POD_FQDN", key.Name, wantDNS)
					if status := f.its.FindInstanceStatus(key.Name); status != nil {
						t.Fatalf("completed Leave retained the released identity: %#v", status)
					}
					if instanceAPI {
						if err := f.cli.Get(ctx, key, &workloads.Instance{}); !apierrors.IsNotFound(err) {
							t.Fatalf("completed Leave retained the excluded Instance: %v", err)
						}
					}
					if err := f.cli.Get(ctx, key, &corev1.Pod{}); !apierrors.IsNotFound(err) {
						t.Fatalf("completed retirement retained a Pod: %v", err)
					}
					storageErr := f.cli.Get(ctx, pvcKey, &corev1.PersistentVolumeClaim{})
					if retention == kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType && !apierrors.IsNotFound(storageErr) {
						t.Fatalf("WhenScaled=Delete did not remove storage: %v", storageErr)
					}
					if retention == kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType && storageErr != nil {
						t.Fatalf("WhenScaled=Retain did not preserve storage: %v", storageErr)
					}
				})
			}
		}
	}
}

func TestITS2MembershipResultRetriesInstanceStatusConflict(t *testing.T) {
	checkITS2MembershipResultRetriesInstanceStatusConflict(t, nil)
}

func TestITS2InitialMembershipRetriesInstanceStatusConflict(t *testing.T) {
	checkITS2InitialMembershipRetriesInstanceStatusConflict(t, nil)
}

func TestITS2LifecycleRecreatedInstancePreservesSatisfiedMembership(t *testing.T) {
	f := newMemberLifecycleEntry(t, true, 1, appsv1.ParallelPodManagement)
	f.bootstrap(t, 1)
	agent := &its2LifecycleAgent{}
	kbacli.SetMockClient(agent, nil)
	defer kbacli.UnsetMockClient()
	child := its2LifecycleInstance(t, f.cli, f.its.Name+"-0")
	key := client.ObjectKeyFromObject(child)
	if err := f.cli.Delete(context.Background(), child); err != nil {
		t.Fatal(err)
	}
	leaf := &InstanceReconciler{Client: f.cli, Scheme: f.cli.Scheme(), Recorder: record.NewFakeRecorder(100)}
	for round := 0; round < 8; round++ {
		if _, err := leaf.Reconcile(context.Background(), ctrl.Request{NamespacedName: key}); err != nil {
			t.Fatal(err)
		}
		if err := f.cli.Get(context.Background(), key, &workloads.Instance{}); apierrors.IsNotFound(err) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
	}
	if err := f.cli.Get(context.Background(), key, &workloads.Instance{}); !apierrors.IsNotFound(err) {
		t.Fatalf("deleted Instance did not finish cleanup: %v", err)
	}
	f.bootstrap(t, 1)
	child = its2LifecycleInstance(t, f.cli, key.Name)
	if !ptr.Deref(child.Status.MemberJoined, false) || !ptr.Deref(f.its.FindInstanceStatus(key.Name).MemberJoined, false) || len(agent.calls) != 0 {
		t.Fatalf("recreated runtime lost satisfied membership or repeated actions: child=%#v parent=%#v calls=%v", child.Status, f.its.Status.InstanceStatus, agent.calls)
	}
}

func checkITS2InitialMembershipRetriesInstanceStatusConflict(t *testing.T, storage client.Client) {
	its, cli, _ := its2LifecycleFixture(t, 1, storage)
	f := &lifecycleEntry{cli: cli, its: its, instanceAPI: true}
	name := its.Name + "-0"
	for round := 0; round < 12; round++ {
		f.stepWithChildren(t)
		if len(lifecycleEntryPods(t, cli)) == 1 {
			break
		}
	}
	f.setReady(t, name, true)
	conflicted := false
	f.cli = interceptor.NewClient(cli.Client.(client.WithWatch), interceptor.Funcs{SubResourceUpdate: func(ctx context.Context, delegate client.Client, subresource string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
		if proposed, ok := obj.(*workloads.Instance); ok && subresource == "status" && ptr.Deref(proposed.Status.MemberJoined, false) && !conflicted {
			conflicted = true
			f.setReady(t, name, false)
		}
		return delegate.SubResource(subresource).Update(ctx, obj, opts...)
	}})
	agent := &its2LifecycleAgent{}
	kbacli.SetMockClient(agent, nil)
	defer kbacli.UnsetMockClient()
	result := f.step(t)
	child := its2LifecycleInstance(t, cli, name)
	if !conflicted || !result.Requeue || child.Status.Ready || child.Status.MemberJoined != nil || f.its.FindInstanceStatus(name).MemberJoined != nil {
		t.Fatalf("initial satisfaction overwrote concurrent child status or reached parent before child commit: child=%#v parent=%#v result=%#v", child.Status, f.its.Status, result)
	}
	f.step(t)
	child = its2LifecycleInstance(t, cli, name)
	if status := f.its.FindInstanceStatus(name); !ptr.Deref(status.MemberJoined, false) || !ptr.Deref(child.Status.MemberJoined, false) || child.Status.Ready || len(agent.calls) != 0 {
		t.Fatalf("retry did not commit initial membership while preserving child readiness: child=%#v parent=%#v calls=%v", child.Status, status, agent.calls)
	}
}

func checkITS2MembershipResultRetriesInstanceStatusConflict(t *testing.T, storage client.Client) {
	its, cli, _ := its2LifecycleFixture(t, 1, storage)
	f := &lifecycleEntry{cli: cli, its: its, instanceAPI: true}
	f.bootstrap(t, 1)
	f.its.Spec.Replicas = ptr.To[int32](2)
	f.its.Generation++
	if err := cli.Update(context.Background(), f.its); err != nil {
		t.Fatal(err)
	}
	target := client.ObjectKey{Namespace: its.Namespace, Name: its.Name + "-1"}
	for i := 0; i < 12; i++ {
		f.stepWithChildren(t)
		if err := cli.Get(context.Background(), target, &corev1.Pod{}); err == nil {
			break
		}
	}
	f.setReady(t, target.Name, true)
	calls := 0
	agent := &memberResultConflictAgent{dispatch: func() {
		calls++
		if calls == 1 {
			// A concurrent leaf observes readiness loss while the remote Join is in flight.
			// Its status update changes the child's resource version before the member result commits.
			f.setReady(t, target.Name, false)
		}
	}}
	kbacli.SetMockClient(agent, nil)
	defer kbacli.UnsetMockClient()
	for i := 0; i < 6 && calls == 0; i++ {
		f.step(t)
	}
	child := its2LifecycleInstance(t, cli, target.Name)
	if calls != 1 || child.Status.Ready || ptr.Deref(child.Status.MemberJoined, false) || ptr.Deref(f.its.FindInstanceStatus(target.Name).MemberJoined, false) {
		t.Fatalf("stale member result overwrote leaf status or parent success: calls=%d child=%#v parent=%#v", calls, child.Status, f.its.Status)
	}
	f.setReady(t, target.Name, true)
	for i := 0; i < 6; i++ {
		f.stepWithChildren(t)
	}
	if calls != 2 || !ptr.Deref(its2LifecycleInstance(t, cli, target.Name).Status.MemberJoined, false) || !ptr.Deref(f.its.FindInstanceStatus(target.Name).MemberJoined, false) {
		t.Fatalf("member result did not recover through conflict: calls=%d parent=%#v", calls, f.its.Status)
	}
	for i := 0; i < 3; i++ {
		f.stepWithChildren(t)
	}
	if calls != 2 {
		t.Fatal("committed member action repeated")
	}
}

type memberResultConflictAgent struct{ dispatch func() }

func (a *memberResultConflictAgent) Close() error { return nil }
func (a *memberResultConflictAgent) Action(_ context.Context, _ kbaproto.ActionRequest) (kbaproto.ActionResponse, error) {
	a.dispatch()
	return kbaproto.ActionResponse{}, nil
}

type memberDNSAgent struct{ requests []kbaproto.ActionRequest }

type unavailableMemberAgent struct{ calls int }

func TestInstanceSetMemberActionUsesValidatedExecutor(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, legacySelector := range []bool{false, true} {
			t.Run(fmt.Sprintf("instanceAPI=%t/legacySelector=%t", instanceAPI, legacySelector), func(t *testing.T) {
				ctx := context.Background()
				f := newMemberLifecycleEntry(t, instanceAPI, 1, appsv1.ParallelPodManagement)
				f.bootstrap(t, 1)
				action := f.its.Spec.LifecycleActions.MemberJoin
				if legacySelector {
					action.Exec.TargetPodSelector = kbappsv1.AnyReplica
				} else {
					action.TargetPodSelector = kbappsv1.AnyReplica
				}
				f.its.Spec.Replicas = ptr.To[int32](3)
				f.its.Spec.ParallelPodManagementConcurrency = ptr.To(intstr.FromInt(2))
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 12 && len(lifecycleEntryPods(t, f.cli)) < 3; round++ {
					f.stepWithChildren(t)
				}
				if len(lifecycleEntryPods(t, f.cli)) != 3 {
					t.Fatal("parallel expansion did not create both siblings")
				}
				f.setReady(t, f.its.Name+"-1", true)
				pendingName := f.its.Name + "-2"
				agent := &unavailableMemberAgent{}
				kbacli.SetMockClient(agent, nil)
				defer kbacli.UnsetMockClient()
				for round := 0; round < 128; round++ {
					f.step(t)
					condition := meta.FindStatusCondition(f.its.Status.Conditions, "InstanceLifecycle")
					if condition != nil && strings.Contains(condition.Message, "member dispatch failed") && strings.Contains(condition.Message, "at pod "+pendingName) {
						t.Fatalf("action was dispatched to an executor rejected by endpoint validation: %s", condition.Message)
					}
					status := f.its.FindInstanceStatus(f.its.Name + "-1")
					if status == nil || status.MemberJoined == nil || *status.MemberJoined {
						t.Fatalf("failed membership request changed completion: %#v", status)
					}
				}
				if agent.calls == 0 {
					t.Fatal("no membership request reached an available executor")
				}
			})
		}
	}
}

func TestInstanceSetMemberActionsUseRuntimePodDNS(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		t.Run(fmt.Sprintf("instanceAPI=%t", instanceAPI), func(t *testing.T) {
			ctx := context.Background()
			f := newMemberLifecycleEntry(t, instanceAPI, 1, appsv1.OrderedReadyPodManagement)
			f.bootstrap(t, 1)
			agent := &memberDNSAgent{}
			kbacli.SetMockClient(agent, nil)
			defer kbacli.UnsetMockClient()
			f.its.Spec.Replicas = ptr.To[int32](2)
			f.its.Generation++
			if err := f.cli.Update(ctx, f.its); err != nil {
				t.Fatal(err)
			}
			for round := 0; round < 12 && len(agent.requests) == 0; round++ {
				f.stepWithChildren(t)
				for _, pod := range lifecycleEntryPods(t, f.cli) {
					f.setReady(t, pod.Name, true)
				}
			}
			pod := &corev1.Pod{}
			if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-1"}, pod); err != nil {
				t.Fatal(err)
			}
			if pod.Spec.Hostname == "" || pod.Spec.Subdomain == "" {
				t.Fatalf("controller did not establish Pod DNS: %#v", pod.Spec)
			}
			want := pod.Spec.Hostname + "." + intctrlutil.ServiceFQDN(pod.Namespace, pod.Spec.Subdomain)
			assertMemberDNSRequest(t, agent.requests, "memberJoin", "KB_JOIN_MEMBER_POD_FQDN", pod.Name, want)
			agent.requests = nil
			f.its.Spec.Replicas = ptr.To[int32](1)
			f.its.Generation++
			if err := f.cli.Update(ctx, f.its); err != nil {
				t.Fatal(err)
			}
			for round := 0; round < 8 && len(agent.requests) == 0; round++ {
				f.step(t)
			}
			assertMemberDNSRequest(t, agent.requests, "memberLeave", "KB_LEAVE_MEMBER_POD_FQDN", pod.Name, want)
		})
	}
}

func assertMemberDNSRequest(t *testing.T, requests []kbaproto.ActionRequest, action, fqdnKey, podName, want string) {
	t.Helper()
	if len(requests) != 1 || requests[0].Action != action {
		t.Fatalf("expected one %s request, got %#v", action, requests)
	}
	if got := requests[0].Parameters[fqdnKey]; got != want {
		t.Errorf("%s for Pod %s = %q, want runtime Pod DNS %q", fqdnKey, podName, got, want)
	}
}

func (a *memberDNSAgent) Close() error { return nil }

func (a *memberDNSAgent) Action(_ context.Context, req kbaproto.ActionRequest) (kbaproto.ActionResponse, error) {
	a.requests = append(a.requests, req)
	return kbaproto.ActionResponse{}, nil
}

func (a *unavailableMemberAgent) Close() error { return nil }

func (a *unavailableMemberAgent) Action(_ context.Context, _ kbaproto.ActionRequest) (kbaproto.ActionResponse, error) {
	a.calls++
	return kbaproto.ActionResponse{}, fmt.Errorf("member dispatch failed")
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
				its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{PodName: name, MemberJoined: facts[i],
					CurrentState: workloads.InstanceCurrentStatePresent, CurrentRevision: "stale", Ready: true, Available: true, Failed: true, Role: "stale"})
			}
			if err := cli.Status().Update(ctx, its); err != nil {
				t.Fatal(err)
			}
			owner := []metav1.OwnerReference{{APIVersion: workloads.GroupVersion.String(), Kind: workloads.InstanceSetKind, Name: its.Name, UID: its.UID, Controller: ptr.To(true)}}
			meta := metav1.ObjectMeta{Name: "tracked-0", Namespace: its.Namespace, Labels: map[string]string{constant.AppManagedByLabelKey: constant.AppName, "workloads.kubeblocks.io/managed-by": workloads.InstanceSetKind, "workloads.kubeblocks.io/instance": its.Name}, OwnerReferences: owner}
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
			assertPersistedMemberFacts(t, its, facts)
			status := its.FindInstanceStatus("tracked-0")
			if status == nil || !status.Provisioned || status.CurrentState != workloads.InstanceCurrentStatePresent || status.CurrentRevision != "observed" {
				t.Fatalf("Reconcile did not commit the observed runtime fact: %#v", status)
			}
			if err := client.IgnoreNotFound(cli.Delete(ctx, observed)); err != nil {
				t.Fatal(err)
			}
			reconcile()
			assertPersistedMemberFacts(t, its, facts)
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
			if joined == nil || joined.DesiredState != workloads.InstanceDesiredStateReleased || joined.CurrentState != workloads.InstanceCurrentStateAbsent || !ptr.Deref(joined.MemberJoined, false) {
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

func assertPersistedMemberFacts(t *testing.T, its *workloads.InstanceSet, facts []*bool) {
	t.Helper()
	for i, fact := range facts {
		status := its.FindInstanceStatus(fmt.Sprintf("tracked-%d", i))
		if status == nil || !reflect.DeepEqual(status.MemberJoined, fact) {
			t.Fatalf("membership changed across controller entry: %#v", its.Status.InstanceStatus)
		}
	}
}

func TestInstanceSetPendingMembersAllowCancellationAndOffline(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, offline := range []bool{false, true} {
			t.Run(fmt.Sprintf("instanceAPI=%t/offline=%t", instanceAPI, offline), func(t *testing.T) {
				initial := 1
				if offline {
					initial = 2
				}
				f := newMemberLifecycleEntry(t, instanceAPI, int32(initial), appsv1.ParallelPodManagement)
				f.bootstrap(t, initial)
				agent := &its2LifecycleAgent{}
				kbacli.SetMockClient(agent, nil)
				defer kbacli.UnsetMockClient()
				f.its.Spec.Replicas = ptr.To[int32](3)
				f.its.Spec.ParallelPodManagementConcurrency = ptr.To(intstr.FromInt(3))
				f.its.Generation++
				if err := f.cli.Update(context.Background(), f.its); err != nil {
					t.Fatal(err)
				}
				for i := 0; i < 8; i++ {
					f.stepWithChildren(t)
				}
				removed, pending := f.its.Name+"-2", f.its.Name+"-1"
				if offline {
					removed, pending = f.its.Name+"-0", f.its.Name+"-2"
					f.its.Spec.OfflineInstances = []string{removed}
				} else {
					f.its.Spec.Replicas = ptr.To[int32](2)
				}
				f.its.Generation++
				if err := f.cli.Update(context.Background(), f.its); err != nil {
					t.Fatal(err)
				}
				for i := 0; i < 16; i++ {
					f.stepWithChildren(t)
				}
				if err := f.cli.Get(context.Background(), client.ObjectKey{Namespace: f.its.Namespace, Name: removed}, &corev1.Pod{}); !apierrors.IsNotFound(err) {
					t.Fatalf("pending Join blocked independent retirement: %v", err)
				}
				status := f.its.FindInstanceStatus(pending)
				if status == nil || status.MemberJoined == nil || *status.MemberJoined || len(agent.calls) != 0 {
					t.Fatalf("pending membership changed during retirement: %#v calls=%v", status, agent.calls)
				}
			})
		}
	}
}
func TestInstanceSetZeroToOneJoinsMember(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		t.Run(fmt.Sprintf("instanceAPI=%t", instanceAPI), func(t *testing.T) {
			f := newMemberLifecycleEntry(t, instanceAPI, 0, appsv1.OrderedReadyPodManagement)
			f.bootstrap(t, 0)
			if f.its.Status.ObservedGeneration != f.its.Generation {
				t.Fatal("zero allocation was not committed")
			}
			agent := &its2LifecycleAgent{}
			kbacli.SetMockClient(agent, nil)
			defer kbacli.UnsetMockClient()
			f.its.Spec.Replicas = ptr.To[int32](1)
			f.its.Generation++
			if err := f.cli.Update(context.Background(), f.its); err != nil {
				t.Fatal(err)
			}
			f.step(t)
			status := f.its.FindInstanceStatus(f.its.Name + "-0")
			if status == nil || status.MemberJoined == nil || *status.MemberJoined || len(lifecycleEntryPods(t, f.cli)) != 0 {
				t.Fatalf("zero-to-one did not register Join before creation: %#v", f.its.Status)
			}
			for i := 0; i < 12; i++ {
				f.stepWithChildren(t)
				for _, pod := range lifecycleEntryPods(t, f.cli) {
					f.setReady(t, pod.Name, true)
				}
			}
			if len(agent.calls) != 1 || agent.calls[0] != "memberJoin" || !ptr.Deref(f.its.FindInstanceStatus(f.its.Name+"-0").MemberJoined, false) {
				t.Fatalf("first expansion skipped Join: %v %#v", agent.calls, f.its.Status)
			}
		})
	}
}

func TestInstanceSetInitialStepsAreSatisfiedWithoutActions(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		t.Run(fmt.Sprintf("instanceAPI=%t", instanceAPI), func(t *testing.T) {
			f := newDataLifecycleEntry(t, instanceAPI, 2, appsv1.ParallelPodManagement)
			agent := &its2LifecycleAgent{}
			kbacli.SetMockClient(agent, nil)
			defer kbacli.UnsetMockClient()
			f.bootstrap(t, 2)
			for _, status := range f.its.Status.InstanceStatus {
				if !status.Provisioned || !ptr.Deref(status.DataLoaded, false) || !ptr.Deref(status.MemberJoined, false) {
					t.Fatalf("initial steps are not satisfied: provisioned=%t data=%v member=%v", status.Provisioned, status.DataLoaded, status.MemberJoined)
				}
				if instanceAPI {
					child := its2LifecycleInstance(t, f.cli, status.PodName)
					if !ptr.Deref(child.Status.DataLoaded, false) || !ptr.Deref(child.Status.MemberJoined, false) {
						t.Fatalf("initial Instance results differ from parent: %#v", child.Status)
					}
				}
			}
			if len(agent.calls) != 0 {
				t.Fatalf("initialization executed member actions: %v", agent.calls)
			}
			for _, pod := range lifecycleEntryPods(t, f.cli) {
				if task, err := workloadlifecycle.GeneratedTask(&pod.Spec); err != nil || task != "" {
					t.Fatalf("initialization created a data task: %s %v", task, err)
				}
			}
		})
	}
}

func TestInstanceSetInitialDataWithoutHooksIsUntracked(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		t.Run(fmt.Sprintf("instanceAPI=%t", instanceAPI), func(t *testing.T) {
			f := newMemberLifecycleEntry(t, instanceAPI, 1, appsv1.ParallelPodManagement)
			f.bootstrap(t, 1)
			for _, status := range f.its.Status.InstanceStatus {
				if status.DataLoaded != nil {
					t.Fatalf("initial instance without data hooks has a data result: %#v", status)
				}
				if instanceAPI && its2LifecycleInstance(t, f.cli, status.PodName).Status.DataLoaded != nil {
					t.Fatal("child without data hooks has a data result")
				}
			}
		})
	}
}

func TestInstanceSetReexpandsRetainedDataWithoutLoading(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, retainedReplicas := range []int32{0, 1} {
			for _, completedExpansion := range []bool{false, true} {
				t.Run(fmt.Sprintf("instanceAPI=%t/retainedReplicas=%d/completedExpansion=%t", instanceAPI, retainedReplicas, completedExpansion), func(t *testing.T) {
					ctx := context.Background()
					initial := int32(2)
					if completedExpansion {
						initial = 1
					}
					f := newDataLifecycleEntry(t, instanceAPI, initial, appsv1.ParallelPodManagement)
					f.its.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{WhenScaled: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType}
					if err := f.cli.Update(ctx, f.its); err != nil {
						t.Fatal(err)
					}
					agent := &its2LifecycleAgent{}
					kbacli.SetMockClient(agent, nil)
					defer kbacli.UnsetMockClient()
					f.bootstrap(t, int(initial))
					key := client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-1"}
					if completedExpansion {
						f.its.Spec.Replicas = ptr.To[int32](2)
						f.its.Generation++
						if err := f.cli.Update(ctx, f.its); err != nil {
							t.Fatal(err)
						}
						for i := 0; i < 12; i++ {
							f.stepWithChildren(t)
						}
						f.markDataLoaded(t, key.Name)
						f.setReady(t, key.Name, true)
						for i := 0; i < 8; i++ {
							f.stepWithChildren(t)
						}
						status := f.its.FindInstanceStatus(key.Name)
						if status == nil || !ptr.Deref(status.DataLoaded, false) || !ptr.Deref(status.MemberJoined, false) || len(agent.calls) != 1 || agent.calls[0] != "memberJoin" {
							t.Fatalf("expansion did not complete before shrink: %#v calls=%v", status, agent.calls)
						}
						agent.calls = nil
					}
					pod := &corev1.Pod{}
					if err := f.cli.Get(ctx, key, pod); err != nil {
						t.Fatal(err)
					}
					pvcName, err := lifecycleTestDataPVCName(pod)
					if err != nil {
						t.Fatal(err)
					}
					pvc := &corev1.PersistentVolumeClaim{}
					pvcKey := client.ObjectKey{Namespace: key.Namespace, Name: pvcName}
					if err := f.cli.Get(ctx, pvcKey, pvc); err != nil {
						t.Fatal(err)
					}
					originalPVC := pvc.DeepCopy()
					f.its.Spec.Replicas = ptr.To(retainedReplicas)
					f.its.Generation++
					if err := f.cli.Update(ctx, f.its); err != nil {
						t.Fatal(err)
					}
					for i := 0; i < 24; i++ {
						f.stepWithChildren(t)
					}
					if err := f.cli.Get(ctx, key, &corev1.Pod{}); !apierrors.IsNotFound(err) {
						t.Fatalf("scale-in retained runtime: %v", err)
					}
					if err := f.cli.Get(ctx, pvcKey, pvc); err != nil || pvc.UID != originalPVC.UID {
						t.Fatalf("scale-in failed to retain the same PVC: %v", err)
					}
					f.its.Spec.Replicas = ptr.To[int32](2)
					f.its.Generation++
					if err := f.cli.Update(ctx, f.its); err != nil {
						t.Fatal(err)
					}
					for i := 0; i < 16; i++ {
						f.stepWithChildren(t)
						for _, pod := range lifecycleEntryPods(t, f.cli) {
							if task, err := workloadlifecycle.GeneratedTask(&pod.Spec); err != nil || task != "" {
								t.Fatalf("retained data was assigned another copy: %s %v", task, err)
							}
							f.setReady(t, pod.Name, true)
						}
					}
					if err := f.cli.Get(ctx, key, pod); err != nil {
						t.Fatalf("re-expansion runtime missing: %v status=%+v calls=%v pods=%+v", err, f.its.Status, agent.calls, lifecycleEntryPods(t, f.cli))
					}
					if actualPVC, err := lifecycleTestDataPVCName(pod); err != nil || actualPVC != pvcName {
						t.Fatalf("re-expanded instance changed its retained PVC: %s %v", actualPVC, err)
					}
					if err := f.cli.Get(ctx, pvcKey, pvc); err != nil || pvc.UID != originalPVC.UID {
						t.Fatalf("re-expansion replaced the retained PVC: %v", err)
					}
					for i := 0; i < 8; i++ {
						f.stepWithChildren(t)
					}
					status := f.its.FindInstanceStatus(key.Name)
					if status == nil || !ptr.Deref(status.DataLoaded, false) || !ptr.Deref(status.MemberJoined, false) {
						t.Fatalf("re-expanded retained instance did not satisfy data and rejoin: %#v", status)
					}
					changed := int(2 - retainedReplicas)
					if len(agent.calls) != changed*2 {
						t.Fatalf("re-expansion skipped actual membership changes: %v", agent.calls)
					}
					for i, action := range agent.calls {
						want := "memberLeave"
						if i >= changed {
							want = "memberJoin"
						}
						if action != want {
							t.Fatalf("re-expansion membership order: %v", agent.calls)
						}
					}
				})
			}
		}
	}
}

func newDataLifecycleITS(replicas int32) *workloads.InstanceSet {
	return &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "data-lifecycle", Namespace: "default", UID: "data-lifecycle", Generation: 1,
			Finalizers: []string{"instanceset.workloads.kubeblocks.io/finalizer"}},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To(replicas), Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "data-lifecycle"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "data-lifecycle"}}, Spec: corev1.PodSpec{
				ServiceAccountName: "data-loader", Containers: []corev1.Container{{Name: "db", Image: "database", VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}}}, {Name: kbagent.ContainerName, Image: "tools", Ports: []corev1.ContainerPort{{Name: kbagent.DefaultHTTPPortName, ContainerPort: 3501}, {Name: kbagent.DefaultStreamingPortName, ContainerPort: 3502}}}},
				InitContainers: []corev1.Container{{Name: kbagent.ContainerName4Worker, Image: "tools", Env: []corev1.EnvVar{{Name: "KB_AGENT_POD_NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.name"}}}}, Command: []string{kbagent.BinaryPath}, Args: []string{"--server=false"}, VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}}}},
			}},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}, Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}},
			}}},
			LifecycleActions: &workloads.LifecycleActions{
				DataDump: &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"dump"}}},
				DataLoad: &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"load"}}},
			},
		},
	}
}

func newDataLifecycleEntry(t *testing.T, instanceAPI bool, replicas int32, policy appsv1.PodManagementPolicyType) *lifecycleEntry {
	t.Helper()
	var its *workloads.InstanceSet
	var cli client.Client
	if instanceAPI {
		its, cli, _ = its2DataFixture(t, replicas)
	} else {
		its = newDataLifecycleITS(replicas)
		its.Spec.LifecycleActions.MemberJoin = &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"join"}}}
		its.Spec.LifecycleActions.MemberLeave = &kbappsv1.Action{Exec: &kbappsv1.ExecAction{Command: []string{"leave"}}}
		cli = newLifecycleEntryClient(t, its, interceptor.Funcs{})
	}
	its.Spec.PodManagementPolicy = policy
	its.Spec.ParallelPodManagementConcurrency = ptr.To(intstr.FromInt(1))
	if err := cli.Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
	return &lifecycleEntry{cli: cli, its: its, instanceAPI: instanceAPI}
}

func markDataWorkerComplete(t *testing.T, cli client.Client, key client.ObjectKey) {
	t.Helper()
	pod := &corev1.Pod{}
	if err := cli.Get(context.Background(), key, pod); err != nil {
		t.Fatal(err)
	}
	task, err := workloadlifecycle.GeneratedTask(&pod.Spec)
	if err != nil || task == "" {
		t.Fatalf("worker completion requires a data task: %q %v", task, err)
	}
	pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{Name: kbagent.ContainerName4Worker, State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}}}
	if err := cli.Status().Update(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
}

func (f *lifecycleEntry) markDataLoaded(t *testing.T, name string) {
	t.Helper()
	markDataWorkerComplete(t, f.cli, client.ObjectKey{Namespace: f.its.Namespace, Name: name})
}

func lifecycleTestDataPVCName(pod *corev1.Pod) (string, error) {
	for _, volume := range pod.Spec.Volumes {
		if volume.Name == "data" && volume.PersistentVolumeClaim != nil {
			return volume.PersistentVolumeClaim.ClaimName, nil
		}
	}
	return "", fmt.Errorf("fixture has no data PVC")
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
		var task bool
		for _, c := range pod.Spec.InitContainers {
			for _, env := range c.Env {
				task = task || env.Name == "KB_AGENT_TASK"
			}
		}
		if task != wantTask {
			t.Fatalf("target task=%v, want task=%v: %+v", task, wantTask, pod.Spec.InitContainers)
		}
	}
	assertTask(target, true)
	markDataWorkerComplete(t, cli, client.ObjectKey{Namespace: its.Namespace, Name: "data-lifecycle-1"})
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
		t.Fatal("fresh reconciler did not recover worker result")
	}
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
		pod.Status = corev1.PodStatus{InitContainerStatuses: pod.Status.InitContainerStatuses, Phase: corev1.PodRunning, PodIP: "10.0.0.1", Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}}
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
	markDataWorkerComplete(t, cli, client.ObjectKey{Namespace: its.Namespace, Name: "data-lifecycle-1"})
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
	retained := its.FindInstanceStatus("data-lifecycle-1")
	if leaves != 1 || retained == nil || retained.DesiredState != workloads.InstanceDesiredStateReleased || !ptr.Deref(retained.DataLoaded, false) || ptr.Deref(retained.MemberJoined, true) {
		t.Fatalf("retained shrink lost the instance result: leaves=%d status=%+v", leaves, its.Status.InstanceStatus)
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

func TestInstanceSetLifecyclePendingCopiesAllowCancellationAndOffline(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, operation := range []struct{ offline, loaded, noSource bool }{{}, {loaded: true}, {noSource: true}, {offline: true}} {
			t.Run(fmt.Sprintf("instanceAPI=%t/offline=%t/loaded=%t/noSource=%t", instanceAPI, operation.offline, operation.loaded, operation.noSource), func(t *testing.T) {
				ctx := context.Background()
				bootstrap := 1
				if operation.offline {
					bootstrap = 2
				}
				f := newDataLifecycleEntry(t, instanceAPI, int32(bootstrap), appsv1.ParallelPodManagement)
				if operation.offline {
					f.its.Spec.LifecycleActions.DataDump.TargetPodSelector = kbappsv1.OrdinalSelector
					f.its.Spec.LifecycleActions.DataDump.MatchingKey = "0"
					if err := f.cli.Update(ctx, f.its); err != nil {
						t.Fatal(err)
					}
				}
				f.bootstrap(t, bootstrap)
				agent := &its2LifecycleAgent{}
				kbacli.SetMockClient(agent, nil)
				defer kbacli.UnsetMockClient()
				f.its.Spec.Replicas = ptr.To[int32](3)
				f.its.Spec.ParallelPodManagementConcurrency = ptr.To(intstr.FromInt(3))
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 8; round++ {
					f.stepWithChildren(t)
				}
				for _, pod := range lifecycleEntryPods(t, f.cli) {
					if pod.Name == f.its.Name+"-0" || (operation.offline && pod.Name == f.its.Name+"-1") {
						continue
					}
					pod.Status.Phase = corev1.PodPending
					if err := f.cli.Status().Update(ctx, &pod); err != nil {
						t.Fatal(err)
					}
				}
				removeName := f.its.Name + "-2"
				pendingName := f.its.Name + "-1"
				if operation.noSource {
					f.its.Spec.LifecycleActions.DataDump.TargetPodSelector = kbappsv1.OrdinalSelector
					f.its.Spec.LifecycleActions.DataDump.MatchingKey = "99"
				}
				if operation.loaded {
					f.markDataLoaded(t, removeName)
					for round := 0; round < 6; round++ {
						f.stepWithChildren(t)
					}
				}
				if operation.offline {
					removeName = f.its.Name + "-0"
					pendingName = f.its.Name + "-2"
					f.its.Spec.OfflineInstances = []string{removeName}
				} else {
					f.its.Spec.Replicas = ptr.To[int32](2)
				}
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 16; round++ {
					f.stepWithChildren(t)
				}
				if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: removeName}, &corev1.Pod{}); !apierrors.IsNotFound(err) {
					t.Fatalf("another pending copy blocked requested runtime removal: %v", err)
				}
				status := f.its.FindInstanceStatus(pendingName)
				if status == nil || status.DataLoaded == nil || *status.DataLoaded || len(agent.calls) != 0 {
					t.Fatalf("uncompleted copy changed result or membership: %#v calls=%v", status, agent.calls)
				}
			})
		}
	}
}

func TestInstanceSetLifecycleDeletionPreservesExternalDataPVC(t *testing.T) {
	for _, policy := range []kbappsv1.PersistentVolumeClaimRetentionPolicyType{"", kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType, kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType} {
		t.Run(string(policy), func(t *testing.T) {
			ctx := context.Background()
			its := newDataLifecycleITS(1)
			its.Spec.VolumeClaimTemplates = nil
			its.Spec.Template.Spec.Volumes = []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "external-data"}}}}
			if policy != "" {
				its.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{WhenDeleted: policy}
			}
			cli := newLifecycleEntryClient(t, its, interceptor.Funcs{})
			pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "external-data", Namespace: its.Namespace, UID: "external-data", Annotations: map[string]string{"owner": "external"}}}
			if err := cli.Create(ctx, pvc); err != nil {
				t.Fatal(err)
			}
			f := &lifecycleEntry{cli: cli, its: its}
			f.bootstrap(t, 1)
			before := pvc.DeepCopy()
			if err := cli.Delete(ctx, its); err != nil {
				t.Fatal(err)
			}
			deleted := false
			for round := 0; round < 8; round++ {
				r := &InstanceSetReconciler{Client: cli, Scheme: cli.Scheme(), Recorder: record.NewFakeRecorder(100)}
				if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(its)}); err != nil {
					t.Fatal(err)
				}
				if err := cli.Get(ctx, client.ObjectKeyFromObject(its), &workloads.InstanceSet{}); apierrors.IsNotFound(err) {
					deleted = true
					break
				} else if err != nil {
					t.Fatal(err)
				}
			}
			if !deleted {
				t.Fatal("external data PVC prevented InstanceSet deletion")
			}
			if err := cli.Get(ctx, client.ObjectKeyFromObject(pvc), pvc); err != nil {
				t.Fatalf("external data PVC was deleted: %v", err)
			}
			if pvc.ResourceVersion != before.ResourceVersion || pvc.UID != before.UID || len(pvc.OwnerReferences) != 0 || pvc.Annotations["owner"] != "external" {
				t.Fatalf("external data PVC was modified: %#v", pvc)
			}
			if pods := lifecycleEntryPods(t, cli); len(pods) != 0 {
				t.Fatalf("owned Pods survived InstanceSet deletion: %v", pods)
			}
		})
	}
}

func TestInstanceSetLifecycleResumesDataSourceBeforeEarlierNamedTarget(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, policy := range []appsv1.PodManagementPolicyType{appsv1.OrderedReadyPodManagement, appsv1.ParallelPodManagement} {
			t.Run(fmt.Sprintf("instanceAPI=%t/%s", instanceAPI, policy), func(t *testing.T) {
				ctx := context.Background()
				f := newDataLifecycleEntry(t, instanceAPI, 1, policy)
				its, cli := f.its, f.cli
				its.Spec.Instances = []workloads.InstanceTemplate{{Name: "z", Replicas: ptr.To[int32](1)}}
				if err := cli.Update(ctx, its); err != nil {
					t.Fatal(err)
				}
				f.bootstrap(t, 1)
				sourceName, targetName := its.Name+"-z-0", its.Name+"-a-0"
				agent := &its2LifecycleAgent{}
				kbacli.SetMockClient(agent, nil)
				defer kbacli.UnsetMockClient()
				f.its.Spec.Replicas = ptr.To[int32](2)
				f.its.Spec.Instances = append(f.its.Spec.Instances, workloads.InstanceTemplate{Name: "a", Replicas: ptr.To[int32](1)})
				f.its.Generation++
				if err := cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 6; round++ {
					f.stepWithChildren(t)
				}
				target := &corev1.Pod{}
				if err := cli.Get(ctx, client.ObjectKey{Namespace: its.Namespace, Name: targetName}, target); err != nil {
					t.Fatal(err)
				}
				if task, err := workloadlifecycle.GeneratedTask(&target.Spec); err != nil || task == "" || ptr.Deref(f.its.FindInstanceStatus(targetName).DataLoaded, true) {
					t.Fatalf("expansion was not waiting on its data task: task=%s err=%v status=%#v", task, err, f.its.Status.InstanceStatus)
				}
				f.its.Spec.Stop = ptr.To(true)
				f.its.Generation++
				if err := cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 12; round++ {
					f.stepWithChildren(t)
				}
				if len(lifecycleEntryPods(t, cli)) != 0 || len(agent.calls) != 0 {
					t.Fatalf("Stop did not remove runtimes without membership actions: %v", agent.calls)
				}
				if instanceAPI {
					for _, name := range []string{sourceName, targetName} {
						child := its2LifecycleInstance(t, cli, name)
						if !ptr.Deref(child.Spec.Stop, false) || child.Status.CurrentState != workloads.InstanceCurrentStateAbsent || child.Status.ObservedGeneration != child.Generation {
							t.Fatalf("Instance did not acknowledge Stop: %#v", child)
						}
					}
				}
				f.its.Spec.Stop = ptr.To(false)
				f.its.Generation++
				if err := cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				waitForPod := func(name string) *corev1.Pod {
					t.Helper()
					for round := 0; round < 8; round++ {
						f.stepWithChildren(t)
						pod := &corev1.Pod{}
						err := cli.Get(ctx, client.ObjectKey{Namespace: its.Namespace, Name: name}, pod)
						if err == nil {
							return pod
						}
						if !apierrors.IsNotFound(err) {
							t.Fatal(err)
						}
					}
					t.Fatalf("pending data target blocked resume of %s: %#v", name, f.its.Status.Conditions)
					return nil
				}
				waitForPod(sourceName)
				f.setReady(t, sourceName, true)
				target = waitForPod(targetName)
				if task, err := workloadlifecycle.GeneratedTask(&target.Spec); err != nil || task == "" || len(agent.calls) != 0 {
					t.Fatalf("resume lost pending loading or executed membership early: task=%s calls=%v err=%v", task, agent.calls, err)
				}
			})
		}
	}
}

func TestInstanceSetLifecycleKeepsReplacementDataSourceUntilLoadCompletes(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, policy := range []appsv1.PodManagementPolicyType{appsv1.OrderedReadyPodManagement, appsv1.ParallelPodManagement} {
			t.Run(fmt.Sprintf("instanceAPI=%t/%s", instanceAPI, policy), func(t *testing.T) {
				ctx := context.Background()
				f := newDataLifecycleEntry(t, instanceAPI, 1, policy)
				f.bootstrap(t, 1)
				sourceName, targetName := f.its.Name+"-0", f.its.Name+"-1"
				agent := &its2LifecycleAgent{}
				kbacli.SetMockClient(agent, nil)
				defer kbacli.UnsetMockClient()
				f.setReady(t, sourceName, false)
				f.its.Spec.Ordinals = workloads.Ordinals{Discrete: []int32{1}}
				f.its.Spec.ParallelPodManagementConcurrency = ptr.To(intstr.FromInt(2))
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 6; round++ {
					f.stepWithChildren(t)
				}
				if source := f.its.FindInstanceStatus(sourceName); source == nil || source.DesiredState != workloads.InstanceDesiredStateReleased {
					t.Fatalf("replacement did not release the old allocation: %#v", source)
				}
				f.setReady(t, sourceName, true)
				for round := 0; round < 8; round++ {
					f.stepWithChildren(t)
				}
				target := &corev1.Pod{}
				if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: targetName}, target); err != nil {
					t.Fatalf("recovered data source did not unblock replacement: %v", err)
				}
				if task, err := workloadlifecycle.GeneratedTask(&target.Spec); err != nil || task == "" {
					t.Fatalf("replacement has no data task: %s %v", task, err)
				}
				if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: sourceName}, &corev1.Pod{}); err != nil || len(agent.calls) != 0 {
					t.Fatalf("data source retired before loading completed: %v calls=%v", err, agent.calls)
				}
				f.markDataLoaded(t, targetName)
				f.setReady(t, targetName, true)
				for round := 0; round < 12; round++ {
					f.stepWithChildren(t)
				}
				status := f.its.FindInstanceStatus(targetName)
				if status == nil || !ptr.Deref(status.DataLoaded, false) || !ptr.Deref(status.MemberJoined, false) || len(agent.calls) != 2 || agent.calls[0] != "memberJoin" || agent.calls[1] != "memberLeave" {
					t.Fatalf("loaded replacement did not complete membership: status=%#v calls=%v", status, agent.calls)
				}
				if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: sourceName}, &corev1.Pod{}); !apierrors.IsNotFound(err) {
					t.Fatalf("completed replacement retained old runtime: %v", err)
				}
			})
		}
	}
}

func TestInstanceSetLifecyclePendingDataDoesNotBlockStopOrCancellation(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, stop := range []bool{false, true} {
			t.Run(fmt.Sprintf("instanceAPI=%t/stop=%t", instanceAPI, stop), func(t *testing.T) {
				ctx := context.Background()
				f := newDataLifecycleEntry(t, instanceAPI, 1, appsv1.OrderedReadyPodManagement)
				f.bootstrap(t, 1)
				agent := &its2LifecycleAgent{}
				kbacli.SetMockClient(agent, nil)
				defer kbacli.UnsetMockClient()
				f.its.Spec.Replicas = ptr.To[int32](2)
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 8; round++ {
					f.stepWithChildren(t)
				}
				targetName := f.its.Name + "-1"
				if status := f.its.FindInstanceStatus(targetName); status == nil || status.DataLoaded == nil || *status.DataLoaded {
					t.Fatalf("expansion is not waiting for data: %#v", status)
				}
				if stop {
					f.its.Spec.Stop = ptr.To(true)
				} else {
					f.its.Spec.Replicas = ptr.To[int32](1)
				}
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 12; round++ {
					f.stepWithChildren(t)
				}
				wantPods := 1
				if stop {
					wantPods = 0
				}
				if pods := lifecycleEntryPods(t, f.cli); len(pods) != wantPods || len(agent.calls) != 0 {
					t.Fatalf("pending data blocked removal or caused membership calls: pods=%v calls=%v", pods, agent.calls)
				}
				if stop {
					status := f.its.FindInstanceStatus(targetName)
					if status == nil || status.DesiredState != workloads.InstanceDesiredStateOffline || status.DataLoaded == nil || *status.DataLoaded {
						t.Fatalf("Stop changed pending data result: %#v", status)
					}
				} else if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: targetName}, &corev1.Pod{}); !apierrors.IsNotFound(err) {
					t.Fatalf("cancelled data target still exists: %v", err)
				}
			})
		}
	}
}

func TestInstanceSetLifecycleKeepsHealthyDataSource(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		t.Run(fmt.Sprintf("instanceAPI=%t", instanceAPI), func(t *testing.T) {
			ctx := context.Background()
			f := newDataLifecycleEntry(t, instanceAPI, 2, appsv1.ParallelPodManagement)
			f.bootstrap(t, 2)
			source := &corev1.Pod{}
			if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-1"}, source); err != nil {
				t.Fatal(err)
			}
			source.Status.PodIP = "10.0.0.2"
			if err := f.cli.Status().Update(ctx, source); err != nil {
				t.Fatal(err)
			}
			f.its.Spec.Replicas = ptr.To[int32](3)
			f.its.Generation++
			if err := f.cli.Update(ctx, f.its); err != nil {
				t.Fatal(err)
			}
			for round := 0; round < 6; round++ {
				f.stepWithChildren(t)
			}
			target := &corev1.Pod{}
			key := client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-2"}
			if err := f.cli.Get(ctx, key, target); err != nil {
				t.Fatal(err)
			}
			target.Status.Phase = corev1.PodPending
			if err := f.cli.Status().Update(ctx, target); err != nil {
				t.Fatal(err)
			}
			task, err := workloadlifecycle.GeneratedTask(&target.Spec)
			if err != nil || task == "" {
				t.Fatalf("expansion did not create a data task: %s %v", task, err)
			}
			for round := 0; round < 32; round++ {
				f.stepWithChildren(t)
				if err := f.cli.Get(ctx, key, target); err != nil {
					t.Fatalf("unchanged healthy donors caused target recreation: %v", err)
				}
				if current, err := workloadlifecycle.GeneratedTask(&target.Spec); err != nil || current != task {
					t.Fatalf("unchanged healthy donors replaced the running task: before=%s after=%s err=%v", task, current, err)
				}
			}
		})
	}
}

func TestInstanceSetLifecycleRetainedPendingDataRecovery(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, removePVC := range []bool{false, true} {
			t.Run(fmt.Sprintf("instanceAPI=%t/removePVC=%t", instanceAPI, removePVC), func(t *testing.T) {
				ctx := context.Background()
				f := newDataLifecycleEntry(t, instanceAPI, 1, appsv1.ParallelPodManagement)
				f.its.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{WhenScaled: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType}
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				f.bootstrap(t, 1)
				agent := &its2LifecycleAgent{}
				kbacli.SetMockClient(agent, nil)
				defer kbacli.UnsetMockClient()
				f.its.Spec.Replicas = ptr.To[int32](2)
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 12; round++ {
					f.stepWithChildren(t)
				}
				targetKey := client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-1"}
				target := &corev1.Pod{}
				if err := f.cli.Get(ctx, targetKey, target); err != nil {
					t.Fatal(err)
				}
				if task, err := workloadlifecycle.GeneratedTask(&target.Spec); err != nil || task == "" {
					t.Fatalf("initial expansion has no data task: %q %v", task, err)
				}
				if status := f.its.FindInstanceStatus(target.Name); status == nil || status.DataLoaded == nil || *status.DataLoaded {
					t.Fatalf("initial expansion did not record pending data: %#v", status)
				}
				pvcName, err := lifecycleTestDataPVCName(target)
				if err != nil {
					t.Fatal(err)
				}
				target.Status.Phase = corev1.PodPending
				if err := f.cli.Status().Update(ctx, target); err != nil {
					t.Fatal(err)
				}
				f.its.Spec.Replicas = ptr.To[int32](1)
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 24; round++ {
					f.stepWithChildren(t)
				}
				if err := f.cli.Get(ctx, targetKey, &corev1.Pod{}); !apierrors.IsNotFound(err) {
					t.Fatalf("canceled expansion kept its Pod: %v", err)
				}
				pvc := &corev1.PersistentVolumeClaim{}
				if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: pvcName}, pvc); err != nil {
					t.Fatal("cancellation did not retain the incomplete data PVC", err)
				}
				if len(agent.calls) != 0 {
					t.Fatalf("cancellation invented data or membership completion: pvc=%#v calls=%v", pvc.Annotations, agent.calls)
				}
				if status := f.its.FindInstanceStatus(targetKey.Name); status == nil || status.DesiredState != workloads.InstanceDesiredStateReleased || status.DataLoaded == nil || *status.DataLoaded {
					t.Fatalf("retained incomplete PVC lost its loading result: %#v", status)
				}
				if removePVC {
					pvc.Finalizers = nil
					if err := f.cli.Update(ctx, pvc); err != nil {
						t.Fatal(err)
					}
					if err := f.cli.Delete(ctx, pvc); err != nil {
						t.Fatal(err)
					}
					for round := 0; round < 12; round++ {
						f.stepWithChildren(t)
					}
					if status := f.its.FindInstanceStatus(targetKey.Name); status != nil {
						t.Fatalf("removed retained PVC left a released identity: %#v", status)
					}
					return
				}
				f.its.Spec.Replicas = ptr.To[int32](2)
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for round := 0; round < 16; round++ {
					f.stepWithChildren(t)
				}
				status := f.its.FindInstanceStatus(targetKey.Name)
				if status == nil || status.DataLoaded == nil || *status.DataLoaded {
					t.Fatalf("re-expansion lost the incomplete data requirement: %#v", status)
				}
				if err := f.cli.Get(ctx, targetKey, target); err != nil {
					t.Fatal(err)
				}
				task := its2TaskInput(&workloads.Instance{Spec: workloads.InstanceSpec{Template: corev1.PodTemplateSpec{Spec: target.Spec}}})
				if task == "" || len(agent.calls) != 0 {
					t.Fatalf("re-expansion did not restore the task before joining: task=%s calls=%v", task, agent.calls)
				}
				f.markDataLoaded(t, target.Name)
				for round := 0; round < 8; round++ {
					f.stepWithChildren(t)
				}
				f.setReady(t, target.Name, true)
				for round := 0; round < 12; round++ {
					f.stepWithChildren(t)
				}
				status = f.its.FindInstanceStatus(target.Name)
				if status == nil || !ptr.Deref(status.DataLoaded, false) || !ptr.Deref(status.MemberJoined, false) || len(agent.calls) != 1 || agent.calls[0] != "memberJoin" {
					t.Fatalf("re-expanded target did not complete data and join: status=%#v calls=%v", status, agent.calls)
				}
			})
		}
	}
}

func TestInstanceSetLifecycleDataResultRetryAndCleanup(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		t.Run(fmt.Sprintf("instanceAPI=%t", instanceAPI), func(t *testing.T) {
			ctx := context.Background()
			f := newDataLifecycleEntry(t, instanceAPI, 1, appsv1.ParallelPodManagement)
			f.bootstrap(t, 1)
			f.its.Spec.Replicas = ptr.To[int32](2)
			f.its.Generation++
			if err := f.cli.Update(ctx, f.its); err != nil {
				t.Fatal(err)
			}
			key := client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-1"}
			for i := 0; i < 12; i++ {
				f.stepWithChildren(t)
				if err := f.cli.Get(ctx, key, &corev1.Pod{}); err == nil {
					break
				}
			}
			pod := &corev1.Pod{}
			if err := f.cli.Get(ctx, key, pod); err != nil {
				t.Fatal(err)
			}
			before, err := workloadlifecycle.GeneratedTask(&pod.Spec)
			if err != nil || before == "" {
				t.Fatalf("missing pending task: %s %v", before, err)
			}
			f.its.Spec.MinReadySeconds = 5
			if err := f.cli.Update(ctx, f.its); err != nil {
				t.Fatal(err)
			}
			source := &corev1.Pod{}
			if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-0"}, source); err != nil {
				t.Fatal(err)
			}
			for i := range source.Status.Conditions {
				source.Status.Conditions[i].LastTransitionTime = metav1.NewTime(time.Now().Add(-time.Minute))
			}
			if err := f.cli.Status().Update(ctx, source); err != nil {
				t.Fatal(err)
			}
			pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{Name: kbagent.ContainerName4Worker, State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1}}}}
			if err := f.cli.Status().Update(ctx, pod); err != nil {
				t.Fatal(err)
			}
			f.stepWithChildren(t)
			if ptr.Deref(f.its.FindInstanceStatus(key.Name).DataLoaded, false) {
				t.Fatal("failed worker completed loading")
			}
			markDataWorkerComplete(t, f.cli, key)
			faulty := &its2LifecycleClient{Client: f.cli, rejectDataResult: true}
			f.cli = faulty
			for i := 0; i < 3; i++ {
				f.stepWithChildren(t)
			}
			if faulty.dataResultFailures == 0 || ptr.Deref(f.its.FindInstanceStatus(key.Name).DataLoaded, false) {
				t.Fatal("completion failure was not exercised")
			}
			if instanceAPI {
				inst := its2LifecycleInstance(t, f.cli, key.Name)
				if ptr.Deref(inst.Status.DataLoaded, false) || its2TaskInput(inst) == "" {
					t.Fatal("failed status write cleaned Instance task")
				}
			}
			if err := f.cli.Get(ctx, key, pod); err != nil {
				t.Fatal(err)
			}
			if task, err := workloadlifecycle.GeneratedTask(&pod.Spec); err != nil || task != before {
				t.Fatalf("pending task changed: %s %v", task, err)
			}
			faulty.rejectDataResult = false
			for i := 0; i < 8; i++ {
				f.stepWithChildren(t)
			}
			if !ptr.Deref(f.its.FindInstanceStatus(key.Name).DataLoaded, false) {
				t.Fatal("successful worker result not recovered")
			}
			if f.its.FindInstanceStatus(key.Name).Ready {
				t.Fatal("fixture did not exercise completion before Pod readiness")
			}
			pod.Finalizers = nil
			if err := f.cli.Update(ctx, pod); err != nil {
				t.Fatal(err)
			}
			if err := f.cli.Delete(ctx, pod); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 8; i++ {
				f.stepWithChildren(t)
			}
			if err := f.cli.Get(ctx, key, pod); err != nil {
				t.Fatal(err)
			}
			if task, err := workloadlifecycle.GeneratedTask(&pod.Spec); err != nil || task != "" {
				t.Fatalf("completed instance regenerated loading: %s %v", task, err)
			}
		})
	}
}

func TestInstanceSetGeneratedDataTaskExecutesWorker(t *testing.T) {
	ctx := context.Background()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	its := newDataLifecycleITS(1)
	its.Spec.Template.Spec.Containers[1].Ports[1].ContainerPort = int32(listener.Addr().(*net.TCPAddr).Port)
	dataPath := filepath.Join(t.TempDir(), "copied")
	countPath := filepath.Join(t.TempDir(), "attempts")
	its.Spec.LifecycleActions.DataDump.Exec.Command = []string{"sh", "-c", "printf 'workload data\\n'"}
	its.Spec.LifecycleActions.DataLoad.Exec.Command = []string{"sh", "-c", `echo load >> "$COUNT_PATH"; cat > "$DATA_PATH"; exit "$LOAD_EXIT_CODE"`}
	its.Spec.LifecycleActions.TemplateVars = map[string]string{"DATA_PATH": dataPath, "COUNT_PATH": countPath}
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
	if err := cli.Get(ctx, client.ObjectKey{Namespace: its.Namespace, Name: its.Name + "-0"}, source); err != nil {
		t.Fatal(err)
	}
	its.Spec.Replicas = ptr.To[int32](2)
	its.Generation++
	if err := cli.Update(ctx, its); err != nil {
		t.Fatal(err)
	}
	source.Status = corev1.PodStatus{Phase: corev1.PodRunning, PodIP: "127.0.0.1", Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}}
	if err := cli.Status().Update(ctx, source); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		step()
	}
	target := &corev1.Pod{}
	if err := cli.Get(ctx, client.ObjectKey{Namespace: its.Namespace, Name: its.Name + "-1"}, target); err != nil {
		t.Fatal(err)
	}
	var actions []kbaproto.Action
	var streaming []string
	for _, env := range source.Spec.Containers[1].Env {
		if env.Name == "KB_AGENT_PROBE" {
			t.Fatal("fixture must have no probes")
		}
		if env.Name == "KB_AGENT_ACTION" {
			if err := json.Unmarshal([]byte(env.Value), &actions); err != nil {
				t.Fatal(err)
			}
		}
		if env.Name == "KB_AGENT_STREAMING" {
			streaming = strings.Split(env.Value, ",")
		}
	}
	services, err := kbaservice.New(ctrl.Log, actions, nil, streaming)
	if err != nil {
		t.Fatal(err)
	}
	var transport kbaservice.Service
	for _, service := range services {
		if service.Kind() == kbaproto.ServiceStreaming.Kind {
			transport = service
		}
	}
	if transport == nil {
		t.Fatal("generated dump action has no streaming transport")
	}
	for _, env := range target.Spec.InitContainers[0].Env {
		if env.ValueFrom == nil {
			t.Setenv(env.Name, env.Value)
		}
	}
	t.Setenv("KB_AGENT_POD_NAME", target.Name)
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing"))
	for attempt, exit := range []string{"1", "0"} {
		t.Setenv("LOAD_EXIT_CODE", exit)
		done := make(chan error, 1)
		go func() {
			conn, err := listener.Accept()
			if err != nil {
				done <- err
				return
			}
			defer conn.Close()
			done <- transport.HandleConn(ctx, conn)
		}()
		mode, err := kbagent.Launch(ctrl.Log, kbaserver.Config{})
		if mode || (attempt == 0 && err == nil) || (attempt == 1 && err != nil) {
			t.Fatalf("worker attempt %d: server=%t err=%v", attempt, mode, err)
		}
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(dataPath)
	if err != nil || string(data) != "workload data\n" {
		t.Fatalf("copied bytes=%q err=%v", data, err)
	}
	attempts, err := os.ReadFile(countPath)
	if err != nil || string(attempts) != "load\nload\n" {
		t.Fatalf("reentrant attempts=%q err=%v", attempts, err)
	}
}

func TestInstanceSetDataRegistrationWhileStopped(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, replicas := range []int32{0, 1} {
			t.Run(fmt.Sprintf("instanceAPI=%t/bootstrap=%d", instanceAPI, replicas), func(t *testing.T) {
				f := newDataLifecycleEntry(t, instanceAPI, replicas, appsv1.ParallelPodManagement)
				f.bootstrap(t, int(replicas))
				for _, status := range f.its.Status.InstanceStatus {
					if !ptr.Deref(status.DataLoaded, false) {
						t.Fatal("initial data step was not satisfied")
					}
				}
				f.its.Spec.Stop = ptr.To(true)
				f.its.Spec.Replicas = ptr.To(replicas + 1)
				f.its.Generation++
				if err := f.cli.Update(context.Background(), f.its); err != nil {
					t.Fatal(err)
				}
				for i := 0; i < 10; i++ {
					f.stepWithChildren(t)
				}
				status := f.its.FindInstanceStatus(fmt.Sprintf("%s-%d", f.its.Name, replicas))
				if status == nil || status.DataLoaded == nil || *status.DataLoaded {
					t.Fatalf("stopped expansion lost pending registration: %#v", status)
				}
				if len(lifecycleEntryPods(t, f.cli)) != 0 {
					t.Fatal("Stop retained or created a runtime")
				}
			})
		}
	}
}

func TestInstanceSetCompletedWorkerSurvivesDonorRebuild(t *testing.T) {
	for _, recordFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("recordFirst=%t", recordFirst), func(t *testing.T) {

			ctx := context.Background()
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = listener.Close() })
			f := newDataLifecycleEntry(t, true, 1, appsv1.ParallelPodManagement)
			f.its.Spec.Template.Spec.Containers[1].Ports[1].ContainerPort = int32(listener.Addr().(*net.TCPAddr).Port)
			copiedPath := filepath.Join(t.TempDir(), "copied-data")
			f.its.Spec.LifecycleActions.DataDump.Exec.Command = []string{"sh", "-c", "printf 'completed data'"}
			f.its.Spec.LifecycleActions.DataLoad.Exec.Command = []string{"sh", "-c", `cat > "$COPIED_PATH"`}
			f.its.Spec.LifecycleActions.TemplateVars = map[string]string{"COPIED_PATH": copiedPath}
			if err := f.cli.Update(ctx, f.its); err != nil {
				t.Fatal(err)
			}
			f.bootstrap(t, 1)
			originalDonor := &corev1.Pod{}
			if err := f.cli.Get(ctx, client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-0"}, originalDonor); err != nil {
				t.Fatal(err)
			}
			originalDonor.Status.PodIP = "127.0.0.1"
			if err := f.cli.Status().Update(ctx, originalDonor); err != nil {
				t.Fatal(err)
			}
			f.its.Spec.Replicas = ptr.To[int32](2)
			f.its.Generation++
			if err := f.cli.Update(context.Background(), f.its); err != nil {
				t.Fatal(err)
			}
			targetKey := client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-1"}
			for i := 0; i < 10; i++ {
				f.stepWithChildren(t)
			}
			workerPod := &corev1.Pod{}
			if err := f.cli.Get(ctx, targetKey, workerPod); err != nil {
				t.Fatal(err)
			}
			var actions []kbaproto.Action
			var streaming []string
			for _, env := range originalDonor.Spec.Containers[1].Env {
				if env.Name == "KB_AGENT_ACTION" {
					if err := json.Unmarshal([]byte(env.Value), &actions); err != nil {
						t.Fatal(err)
					}
				}
				if env.Name == "KB_AGENT_STREAMING" {
					streaming = strings.Split(env.Value, ",")
				}
			}
			services, err := kbaservice.New(ctrl.Log, actions, nil, streaming)
			if err != nil {
				t.Fatal(err)
			}
			var transport kbaservice.Service
			for _, service := range services {
				if service.Kind() == kbaproto.ServiceStreaming.Kind {
					transport = service
				}
			}
			if transport == nil {
				t.Fatal("no generated streaming service")
			}
			for _, env := range workerPod.Spec.InitContainers[0].Env {
				if env.ValueFrom == nil {
					t.Setenv(env.Name, env.Value)
				}
			}
			t.Setenv("KB_AGENT_POD_NAME", workerPod.Name)
			t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "missing"))
			done := make(chan error, 1)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					done <- err
					return
				}
				defer conn.Close()
				done <- transport.HandleConn(ctx, conn)
			}()
			serving, err := kbagent.Launch(ctrl.Log, kbaserver.Config{})
			if serving || err != nil {
				t.Fatalf("real init worker did not exit successfully: server=%t error=%v", serving, err)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			copied, err := os.ReadFile(copiedPath)
			if err != nil || string(copied) != "completed data" {
				t.Fatalf("actual loaded bytes=%q error=%v", copied, err)
			}
			t.Logf("real worker returned success and copied bytes=%q", copied)
			markDataWorkerComplete(t, f.cli, targetKey)
			target := &corev1.Pod{}
			if err := f.cli.Get(context.Background(), targetKey, target); err != nil {
				t.Fatal(err)
			}
			target.Status.Phase = corev1.PodRunning
			target.Status.PodIP = "10.0.0.2"
			target.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.Now()}}
			for _, c := range target.Spec.Containers {
				target.Status.ContainerStatuses = append(target.Status.ContainerStatuses, corev1.ContainerStatus{Name: c.Name, Image: c.Image, Ready: true})
			}
			if err := f.cli.Status().Update(context.Background(), target); err != nil {
				t.Fatal(err)
			}
			oldTask, err := workloadlifecycle.GeneratedTask(&target.Spec)
			if err != nil || oldTask == "" {
				t.Fatalf("missing task: %q %v", oldTask, err)
			}
			target.UID = "successful-target"
			if err := f.cli.Update(ctx, target); err != nil {
				t.Fatal(err)
			}
			if recordFirst {
				leaf := &InstanceReconciler{Client: f.cli, Scheme: f.cli.Scheme(), Recorder: record.NewFakeRecorder(100)}
				if _, err := leaf.Reconcile(ctx, ctrl.Request{NamespacedName: targetKey}); err != nil {
					t.Fatal(err)
				}
				if !ptr.Deref(its2LifecycleInstance(t, f.cli, targetKey.Name).Status.DataLoaded, false) {
					t.Fatal("control did not record worker result first")
				}
			}
			donorKey := client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-0"}
			donor := &corev1.Pod{}
			if err := f.cli.Get(context.Background(), donorKey, donor); err != nil {
				t.Fatal(err)
			}
			if err := f.cli.Delete(context.Background(), donor); err != nil {
				t.Fatal(err)
			}
			leaf := &InstanceReconciler{Client: f.cli, Scheme: f.cli.Scheme(), Recorder: record.NewFakeRecorder(100)}
			for i := 0; i < 8; i++ {
				if _, err := leaf.Reconcile(context.Background(), ctrl.Request{NamespacedName: donorKey}); err != nil {
					t.Fatal(err)
				}
			}
			f.setReady(t, donorKey.Name, true)
			if err := f.cli.Get(context.Background(), donorKey, donor); err != nil {
				t.Fatal(err)
			}
			donor.Status.PodIP = "10.0.0.9"
			if err := f.cli.Status().Update(context.Background(), donor); err != nil {
				t.Fatal(err)
			}
			agent := &its2LifecycleAgent{}
			kbacli.SetMockClient(agent, nil)
			defer kbacli.UnsetMockClient()
			for i := 0; i < 4; i++ {
				f.step(t)
			}
			inst := its2LifecycleInstance(t, f.cli, targetKey.Name)
			newTask, err := workloadlifecycle.GeneratedTask(&inst.Spec.Template.Spec)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("old=%s refreshed=%s", oldTask, newTask)
			for i := 0; i < 32; i++ {
				f.stepWithChildren(t)
			}
			if err := f.cli.Get(context.Background(), targetKey, target); err != nil {
				t.Fatal(err)
			}
			actual, err := workloadlifecycle.GeneratedTask(&target.Spec)
			if err != nil {
				t.Fatal(err)
			}
			inst = its2LifecycleInstance(t, f.cli, targetKey.Name)
			t.Logf("same Running target phase=%s applied=%s desired=%s child-data=%v", target.Status.Phase, actual, newTask, inst.Status.DataLoaded)
			if actual != oldTask || (!recordFirst && newTask == oldTask) {
				t.Fatal("fixture did not refresh desired while retaining applied task")
			}
			if target.UID != "successful-target" {
				t.Fatal("completion replaced the Running target")
			}
			if !ptr.Deref(inst.Status.DataLoaded, false) || its2TaskInput(inst) != "" {
				t.Fatal("leaf did not commit success and clean desired task")
			}
			if !ptr.Deref(f.its.FindInstanceStatus(targetKey.Name).MemberJoined, false) || len(agent.calls) != 1 || agent.calls[0] != "memberJoin" {
				t.Fatalf("completed target did not join: %v", agent.calls)
			}
			if !ptr.Deref(f.its.FindInstanceStatus(targetKey.Name).DataLoaded, false) {
				t.Fatalf("completed Running target stranded: calls=%v status=%+v", agent.calls, f.its.FindInstanceStatus(targetKey.Name))
			}

		})
	}
}

type reviewPVCUIDClient struct {
	client.Client
	creates int
}

func (c *reviewPVCUIDClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if pvc, ok := obj.(*corev1.PersistentVolumeClaim); ok {
		c.creates++
		pvc.UID = types.UID(fmt.Sprintf("review-created-pvc-%d", c.creates))
	}
	return c.Client.Create(ctx, obj, opts...)
}
func TestReviewNamedTemplateRetainedData(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, loaded := range []bool{false, true} {
			for _, remove := range []bool{false, true} {
				t.Run(fmt.Sprintf("instanceAPI=%t/loaded=%t/remove=%t", instanceAPI, loaded, remove), func(t *testing.T) {
					ctx := context.Background()
					initial := int32(1)
					if loaded {
						initial = 2
					}
					f := newDataLifecycleEntry(t, instanceAPI, initial, appsv1.ParallelPodManagement)
					uidClient := &reviewPVCUIDClient{Client: f.cli}
					f.cli = uidClient
					named := workloads.InstanceTemplate{Name: "named", Replicas: ptr.To[int32](1), VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}, Spec: corev1.PersistentVolumeClaimSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2Gi")}}}}}}
					if loaded {
						f.its.Spec.Instances = []workloads.InstanceTemplate{named}
					}
					f.its.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{WhenScaled: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType}
					if err := f.cli.Update(ctx, f.its); err != nil {
						t.Fatal(err)
					}
					agent := &its2LifecycleAgent{}
					kbacli.SetMockClient(agent, nil)
					defer kbacli.UnsetMockClient()
					f.bootstrap(t, int(initial))
					target := f.its.Name + "-named-0"
					targetKey := client.ObjectKey{Namespace: f.its.Namespace, Name: target}
					if !loaded {
						f.its.Spec.Replicas = ptr.To[int32](2)
						f.its.Spec.Instances = []workloads.InstanceTemplate{named}
						f.its.Generation++
						if err := f.cli.Update(ctx, f.its); err != nil {
							t.Fatal(err)
						}
						for i := 0; i < 16; i++ {
							f.stepWithChildren(t)
						}
						pod := &corev1.Pod{}
						if err := f.cli.Get(ctx, targetKey, pod); err != nil {
							t.Fatal(err)
						}
						pod.Status.Phase = corev1.PodPending
						if err := f.cli.Status().Update(ctx, pod); err != nil {
							t.Fatal(err)
						}
					}
					if s := f.its.FindInstanceStatus(target); s == nil || s.DataLoaded == nil || *s.DataLoaded != loaded {
						t.Fatalf("initial expected data %t missing: %#v", loaded, s)
					}
					claim := &corev1.PersistentVolumeClaim{}
					key := client.ObjectKey{Namespace: f.its.Namespace, Name: "data-" + target}
					if err := f.cli.Get(ctx, key, claim); err != nil {
						t.Fatal(err)
					}
					initialUID := claim.UID
					if initialUID == "" {
						t.Fatal("missing simulated API UID")
					}
					f.its.Spec.Replicas = ptr.To[int32](1)
					if remove {
						f.its.Spec.Instances = nil
					} else {
						f.its.Spec.Instances[0].Replicas = ptr.To[int32](0)
					}
					f.its.Generation++
					if err := f.cli.Update(ctx, f.its); err != nil {
						t.Fatal(err)
					}
					for i := 0; i < 30; i++ {
						f.stepWithChildren(t)
					}
					if err := f.cli.Get(ctx, key, claim); err != nil || claim.UID != initialUID {
						t.Fatal("retained same PVC missing", err)
					}
					if err := f.cli.Get(ctx, targetKey, &corev1.Pod{}); !apierrors.IsNotFound(err) {
						t.Fatal("pod still exists", err)
					}
					t.Logf("after shrink: UID=%s status=%#v", claim.UID, f.its.FindInstanceStatus(target))
					f.its.Spec.Replicas = ptr.To[int32](2)
					f.its.Spec.Instances = []workloads.InstanceTemplate{named}
					f.its.Generation++
					if err := f.cli.Update(ctx, f.its); err != nil {
						t.Fatal(err)
					}
					for i := 0; i < 16; i++ {
						f.stepWithChildren(t)
					}
					pod := &corev1.Pod{}
					if err := f.cli.Get(ctx, targetKey, pod); err != nil {
						t.Fatal(err)
					}
					if err := f.cli.Get(ctx, key, claim); err != nil || claim.UID != initialUID {
						t.Fatal("regrow changed PVC identity", err)
					}
					task, err := workloadlifecycle.GeneratedTask(&pod.Spec)
					if err != nil {
						t.Fatal(err)
					}
					if (task == "") != loaded {
						t.Errorf("same retained PVC UID=%s: loaded=%t task=%s status=%#v", claim.UID, loaded, task, f.its.FindInstanceStatus(target))
					}
					if s := f.its.FindInstanceStatus(target); s == nil || s.DataLoaded == nil || *s.DataLoaded != loaded {
						t.Errorf("regrow lost data result %t: %#v", loaded, s)
					}
				})
			}
		}
	}
}

func TestReviewNamedOnlyZeroRegrowRetainedData(t *testing.T) {
	for _, instanceAPI := range []bool{false, true} {
		for _, remove := range []bool{false, true} {
			t.Run(fmt.Sprintf("instanceAPI=%t/remove=%t", instanceAPI, remove), func(t *testing.T) {
				ctx := context.Background()
				f := newDataLifecycleEntry(t, instanceAPI, 1, appsv1.ParallelPodManagement)
				f.cli = &reviewPVCUIDClient{Client: f.cli}
				named := workloads.InstanceTemplate{Name: "named", Replicas: ptr.To[int32](1)}
				f.its.Spec.Instances = []workloads.InstanceTemplate{named}
				f.its.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{WhenScaled: kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType}
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				agent := &its2LifecycleAgent{}
				kbacli.SetMockClient(agent, nil)
				defer kbacli.UnsetMockClient()
				f.bootstrap(t, 1)
				target := f.its.Name + "-named-0"
				targetKey := client.ObjectKey{Namespace: f.its.Namespace, Name: target}
				s := f.its.FindInstanceStatus(target)
				if s == nil || !ptr.Deref(s.DataLoaded, false) {
					t.Fatal("initial named data not loaded")
				}
				claimKey := client.ObjectKey{Namespace: f.its.Namespace, Name: "data-" + target}
				claim := &corev1.PersistentVolumeClaim{}
				if err := f.cli.Get(ctx, claimKey, claim); err != nil {
					t.Fatal(err)
				}
				initialUID := claim.UID
				if initialUID == "" {
					t.Fatal("missing fake API UID")
				}
				f.its.Spec.Replicas = ptr.To[int32](0)
				if remove {
					f.its.Spec.Instances = nil
				} else {
					f.its.Spec.Instances[0].Replicas = ptr.To[int32](0)
				}
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for i := 0; i < 30; i++ {
					f.stepWithChildren(t)
				}
				if err := f.cli.Get(ctx, claimKey, claim); err != nil || claim.UID != initialUID {
					t.Fatal("same retained PVC missing", err)
				}
				if err := f.cli.Get(ctx, targetKey, &corev1.Pod{}); !apierrors.IsNotFound(err) {
					t.Fatal("zero still has runtime", err)
				}
				f.its.Spec.Replicas = ptr.To[int32](1)
				f.its.Spec.Instances = []workloads.InstanceTemplate{named}
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for i := 0; i < 30; i++ {
					f.stepWithChildren(t)
					for _, p := range lifecycleEntryPods(t, f.cli) {
						f.setReady(t, p.Name, true)
					}
				}
				if err := f.cli.Get(ctx, claimKey, claim); err != nil || claim.UID != initialUID {
					t.Fatal("regrow changed retained PVC", err)
				}
				pod := &corev1.Pod{}
				if err := f.cli.Get(ctx, targetKey, pod); err != nil {
					t.Fatalf("regrow cannot recreate runtime using retained UID=%s: %v status=%#v conditions=%#v", initialUID, err, f.its.FindInstanceStatus(target), f.its.Status.Conditions)
				}
				task, err := workloadlifecycle.GeneratedTask(&pod.Spec)
				if err != nil || task != "" {
					t.Fatalf("retained named data unexpectedly copied: %s %v", task, err)
				}
				s = f.its.FindInstanceStatus(target)
				if s == nil || !ptr.Deref(s.DataLoaded, false) || !ptr.Deref(s.MemberJoined, false) {
					t.Fatalf("retained named data did not resume: %#v", s)
				}
			})
		}
	}
}

func TestInstanceSetRoleProbeToolsUpgradePreservesPod(t *testing.T) {
	original := viper.GetString(constant.KBToolsImage)
	t.Cleanup(func() { viper.Set(constant.KBToolsImage, original) })
	viper.Set(constant.KBToolsImage, "docker.io/apecloud/kubeblocks-tools:1.1.0")
	resize := intctrlutil.SupportResizeSubResource
	intctrlutil.SupportResizeSubResource = func() (bool, error) { return false, nil }
	t.Cleanup(func() { intctrlutil.SupportResizeSubResource = resize })
	for _, instanceAPI := range []bool{false, true} {
		t.Run(fmt.Sprintf("instanceAPI=%t", instanceAPI), func(t *testing.T) {
			ctx := context.Background()
			var produced workloads.InstanceSet
			if err := json.Unmarshal([]byte(`{"metadata":{"name":"tools-db","namespace":"default","labels":{"app.kubernetes.io/instance":"tools","app.kubernetes.io/managed-by":"kubeblocks","apps.kubeblocks.io/component-name":"db"},"annotations":{"app.kubernetes.io/component":"tools-db","apps.kubeblocks.io/service-version":"1","kubeblocks.io/generation":"0"}},"spec":{"replicas":1,"ordinals":{},"selector":{"matchLabels":{"app.kubernetes.io/instance":"tools","app.kubernetes.io/managed-by":"kubeblocks","apps.kubeblocks.io/component-name":"db","apps.kubeblocks.io/release-phase":"stable"}},"template":{"metadata":{"creationTimestamp":null,"labels":{"app.kubernetes.io/component":"tools-db","app.kubernetes.io/instance":"tools","app.kubernetes.io/managed-by":"kubeblocks","apps.kubeblocks.io/component-name":"db","apps.kubeblocks.io/release-phase":"stable","apps.kubeblocks.io/service-version":"1"}},"spec":{"initContainers":[{"name":"kbagent-worker","image":"docker.io/apecloud/kubeblocks-tools:1.0.0","command":["/bin/kbagent"],"args":["--server=false"],"env":[{"name":"KB_AGENT_NAMESPACE","valueFrom":{"fieldRef":{"apiVersion":"v1","fieldPath":"metadata.namespace"}}},{"name":"KB_AGENT_POD_NAME","valueFrom":{"fieldRef":{"apiVersion":"v1","fieldPath":"metadata.name"}}},{"name":"KB_AGENT_POD_UID","valueFrom":{"fieldRef":{"apiVersion":"v1","fieldPath":"metadata.uid"}}},{"name":"KB_AGENT_NODE_NAME","valueFrom":{"fieldRef":{"apiVersion":"v1","fieldPath":"spec.nodeName"}}},{"name":"KB_AGENT_ACTION","value":"[{\"name\":\"roleProbe\",\"exec\":{\"command\":[\"echo\",\"leader\"]}}]"},{"name":"KB_AGENT_PROBE","value":"[{\"instance\":\"tools-db\",\"action\":\"roleProbe\",\"periodSeconds\":10}]"}],"resources":{},"imagePullPolicy":"IfNotPresent","securityContext":{"runAsGroup":1000}}],"containers":[{"name":"db","image":"database:1","resources":{"limits":{"cpu":"0","memory":"0"}}},{"name":"kbagent","image":"docker.io/apecloud/kubeblocks-tools:1.0.0","command":["/bin/kbagent"],"args":["--port","3501","--streaming-port","3502"],"ports":[{"name":"http","containerPort":3501,"protocol":"TCP"},{"name":"streaming","containerPort":3502,"protocol":"TCP"}],"env":[{"name":"KB_AGENT_NAMESPACE","valueFrom":{"fieldRef":{"apiVersion":"v1","fieldPath":"metadata.namespace"}}},{"name":"KB_AGENT_POD_NAME","valueFrom":{"fieldRef":{"apiVersion":"v1","fieldPath":"metadata.name"}}},{"name":"KB_AGENT_POD_UID","valueFrom":{"fieldRef":{"apiVersion":"v1","fieldPath":"metadata.uid"}}},{"name":"KB_AGENT_NODE_NAME","valueFrom":{"fieldRef":{"apiVersion":"v1","fieldPath":"spec.nodeName"}}},{"name":"KB_AGENT_ACTION","value":"[{\"name\":\"roleProbe\",\"exec\":{\"command\":[\"echo\",\"leader\"]}}]"},{"name":"KB_AGENT_PROBE","value":"[{\"instance\":\"tools-db\",\"action\":\"roleProbe\",\"periodSeconds\":10}]"}],"resources":{},"startupProbe":{"tcpSocket":{"port":3501}},"imagePullPolicy":"IfNotPresent","securityContext":{"runAsGroup":1000}}]}},"persistentVolumeClaimRetentionPolicy":{"whenDeleted":"Delete","whenScaled":"Delete"},"podManagementPolicy":"OrderedReady","parallelPodManagementConcurrency":"100%","podUpdatePolicy":"PreferInPlace","podUpgradePolicy":"PreferInPlace","memberUpdateStrategy":"Serial","lifecycleActions":{}},"status":{"replicas":0,"readyReplicas":0,"availableReplicas":0,"initReplicas":0}}`), &produced); err != nil {
				t.Fatal(err)
			}
			produced.Spec.EnableInstanceAPI = ptr.To(instanceAPI)
			specs := []*workloads.InstanceSet{produced.DeepCopy(), produced.DeepCopy()}
			specs[1].Spec.Template.Spec.InitContainers[0].Image = "docker.io/apecloud/kubeblocks-tools:1.1.0"
			specs[1].Spec.Template.Spec.Containers[1].Image = "docker.io/apecloud/kubeblocks-tools:1.1.0"
			its := specs[0]
			its.UID = "initial-its"
			its.Generation = 1
			its.Finalizers = []string{"instanceset.workloads.kubeblocks.io/finalizer"}
			cli := fake.NewClientBuilder().WithScheme(model.GetScheme()).WithStatusSubresource(&workloads.InstanceSet{}, &workloads.Instance{}, &corev1.Pod{}).WithObjects(its).WithInterceptorFuncs(interceptor.Funcs{
				Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
					if _, ok := obj.(*workloads.Instance); ok {
						obj.SetGeneration(1)
					}
					return c.Create(ctx, obj, opts...)
				},
				Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
					if inst, ok := obj.(*workloads.Instance); ok {
						previous := &workloads.Instance{}
						if err := c.Get(ctx, client.ObjectKeyFromObject(inst), previous); err != nil {
							return err
						}
						if !reflect.DeepEqual(previous.Spec, inst.Spec) {
							inst.Generation = previous.Generation + 1
						}
					}
					return c.Update(ctx, obj, opts...)
				},
			}).Build()
			f := &lifecycleEntry{cli: cli, its: its, instanceAPI: instanceAPI}
			for i := 0; i < 16; i++ {
				f.stepWithChildren(t)
				for _, pod := range lifecycleEntryPods(t, cli) {
					pod.Status.InitContainerStatuses = nil
					for _, c := range pod.Spec.InitContainers {
						pod.Status.InitContainerStatuses = append(pod.Status.InitContainerStatuses, corev1.ContainerStatus{Name: c.Name, Image: c.Image, State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}})
					}
					if err := cli.Status().Update(ctx, &pod); err != nil {
						t.Fatal(err)
					}
					f.setReady(t, pod.Name, true)
				}
			}
			pods := lifecycleEntryPods(t, cli)
			if len(pods) != 1 || !pods[0].DeletionTimestamp.IsZero() || f.its.Status.ReadyReplicas != 1 {
				t.Fatalf("bootstrap not stable: pods=%+v status=%+v", pods, f.its.Status)
			}
			key := client.ObjectKeyFromObject(&pods[0])
			pods[0].UID = "tools-upgrade-target"
			if err := cli.Update(ctx, &pods[0]); err != nil {
				t.Fatal(err)
			}
			for _, status := range f.its.Status.InstanceStatus {
				if status.DataLoaded != nil {
					t.Fatal("no DataLoad must remain untracked")
				}
			}
			t.Logf("stable before update: %+v", f.its.Status)
			f.its.Spec = specs[1].Spec
			f.its.Generation++
			if err := cli.Update(ctx, f.its); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 12; i++ {
				f.stepWithChildren(t)
				pod := &corev1.Pod{}
				if err := cli.Get(ctx, key, pod); err != nil {
					t.Fatalf("round %d: roleProbe-only tools upgrade deleted DB Pod: %v", i, err)
				}
				if !pod.DeletionTimestamp.IsZero() {
					t.Fatalf("round %d: roleProbe-only tools upgrade requested DB Pod recreation", i)
				}
				f.setReady(t, pod.Name, true)
			}
			pod := &corev1.Pod{}
			if err := cli.Get(ctx, key, pod); err != nil {
				t.Fatal(err)
			}
			_, worker := intctrlutil.GetContainerByName(pod.Spec.InitContainers, kbagent.ContainerName4Worker)
			if worker == nil || worker.Image != "docker.io/apecloud/kubeblocks-tools:1.1.0" {
				t.Fatalf("tools image did not converge in place: %+v", worker)
			}
			if pod.UID != "tools-upgrade-target" || pod.Spec.Containers[0].Image != "database:1" {
				t.Fatal("tools upgrade changed database identity or image")
			}
			if task, err := workloadlifecycle.GeneratedTask(&pod.Spec); err != nil || task != "" {
				t.Fatalf("tools upgrade generated data task: %s %v", task, err)
			}
			for _, status := range f.its.Status.InstanceStatus {
				if status.DataLoaded != nil {
					t.Fatal("tools upgrade tracked a data result")
				}
			}
		})
	}
}

func TestInstanceSetJoinOnlyReallocationSeparatesData(t *testing.T) {
	for _, retain := range []bool{false, true} {
		for _, instanceAPI := range []bool{false, true} {
			t.Run(fmt.Sprintf("instanceAPI=%t/retain=%t", instanceAPI, retain), func(t *testing.T) {
				ctx := context.Background()
				f := newDataLifecycleEntry(t, instanceAPI, 2, appsv1.ParallelPodManagement)
				f.cli = &reviewPVCUIDClient{Client: f.cli}
				f.its.Spec.LifecycleActions.MemberLeave = nil
				f.its.Spec.PersistentVolumeClaimRetentionPolicy = &workloads.PersistentVolumeClaimRetentionPolicy{WhenScaled: kbappsv1.DeletePersistentVolumeClaimRetentionPolicyType}
				if retain {
					f.its.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled = kbappsv1.RetainPersistentVolumeClaimRetentionPolicyType
				}
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				f.bootstrap(t, 2)
				key := client.ObjectKey{Namespace: f.its.Namespace, Name: f.its.Name + "-1"}
				oldPod := &corev1.Pod{}
				if err := f.cli.Get(ctx, key, oldPod); err != nil {
					t.Fatal(err)
				}
				claimName, err := lifecycleTestDataPVCName(oldPod)
				if err != nil {
					t.Fatal(err)
				}
				claimKey := client.ObjectKey{Namespace: f.its.Namespace, Name: claimName}
				original := &corev1.PersistentVolumeClaim{}
				if err := f.cli.Get(ctx, claimKey, original); err != nil {
					t.Fatal(err)
				}
				agent := &its2LifecycleAgent{}
				kbacli.SetMockClient(agent, nil)
				defer kbacli.UnsetMockClient()
				f.its.Spec.Replicas = ptr.To[int32](1)
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				for i := 0; i < 24; i++ {
					f.stepWithChildren(t)
				}
				if err := f.cli.Get(ctx, key, &corev1.Pod{}); !apierrors.IsNotFound(err) {
					t.Fatalf("shrink did not delete runtime: %v", err)
				}
				claimErr := f.cli.Get(ctx, claimKey, &corev1.PersistentVolumeClaim{})
				if retain && claimErr != nil || !retain && !apierrors.IsNotFound(claimErr) {
					t.Fatalf("wrong PVC retention: %v", claimErr)
				}
				if instanceAPI {
					if err := f.cli.Get(ctx, key, &workloads.Instance{}); !apierrors.IsNotFound(err) {
						t.Fatalf("shrink did not delete Instance: %v", err)
					}
				}
				t.Logf("after runtime and PVC release, status=%#v", f.its.FindInstanceStatus(key.Name))
				f.its.Spec.Replicas = ptr.To[int32](2)
				f.its.Generation++
				if err := f.cli.Update(ctx, f.its); err != nil {
					t.Fatal(err)
				}
				target := &corev1.Pod{}
				for i := 0; i < 24; i++ {
					f.stepWithChildren(t)
					if err := f.cli.Get(ctx, key, target); err == nil {
						break
					} else if !apierrors.IsNotFound(err) {
						t.Fatal(err)
					}
				}
				if err := f.cli.Get(ctx, key, target); err != nil {
					t.Fatalf("reexpansion did not create runtime: %v", err)
				}
				if err := f.cli.Get(ctx, claimKey, &corev1.PersistentVolumeClaim{}); err != nil {
					t.Fatalf("reexpansion did not create fresh PVC: %v", err)
				}
				f.setReady(t, key.Name, true)
				for i := 0; i < 24; i++ {
					f.stepWithChildren(t)
				}
				if err := f.cli.Get(ctx, key, target); err != nil {
					t.Fatal(err)
				}
				task, err := workloadlifecycle.GeneratedTask(&target.Spec)
				status := f.its.FindInstanceStatus(key.Name)
				t.Logf("fresh PVC and Pod exist; task=%q status=%#v calls=%v", task, status, agent.calls)
				if err != nil || (task == "") != retain {
					t.Fatalf("wrong load decision retain=%t task=%q err=%v", retain, task, err)
				}
				claim := &corev1.PersistentVolumeClaim{}
				if err := f.cli.Get(ctx, claimKey, claim); err != nil {
					t.Fatal(err)
				}
				if (claim.UID == original.UID) != retain {
					t.Fatal("PVC identity does not match retention policy")
				}
				if status == nil || status.DataLoaded == nil || *status.DataLoaded != retain {
					t.Fatalf("wrong data result: %#v", status)
				}
				if !retain {
					if status.MemberJoined == nil || *status.MemberJoined {
						t.Fatalf("fresh allocation reused membership: %#v", status)
					}
					if len(agent.calls) != 0 {
						t.Fatalf("pending data joined: %v", agent.calls)
					}
					f.markDataLoaded(t, key.Name)
					for i := 0; i < 12; i++ {
						f.stepWithChildren(t)
					}
				}
				status = f.its.FindInstanceStatus(key.Name)
				if !ptr.Deref(status.DataLoaded, false) || !ptr.Deref(status.MemberJoined, false) || len(agent.calls) != 1 || agent.calls[0] != "memberJoin" {
					t.Fatalf("reallocation did not join once after data satisfaction: %#v calls=%v", status, agent.calls)
				}
			})
		}
	}
}
