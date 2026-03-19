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

package component

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
	kbagentproto "github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("Component Workload Operations Test", func() {
	const (
		clusterName    = "test-cluster"
		compName       = "test-comp"
		kubeblocksName = "kubeblocks"
	)

	var (
		reader         *appsutil.MockReader
		dag            *graph.DAG
		comp           *appsv1.Component
		synthesizeComp *component.SynthesizedComponent
	)

	roles := []appsv1.ReplicaRole{
		{Name: "leader", UpdatePriority: 3},
		{Name: "follower", UpdatePriority: 2},
	}

	newDAG := func(graphCli model.GraphClient, comp *appsv1.Component) *graph.DAG {
		d := graph.NewDAG()
		graphCli.Root(d, comp, comp, model.ActionStatusPtr())
		return d
	}

	BeforeEach(func() {
		reader = &appsutil.MockReader{}
		comp = &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testCtx.DefaultNamespace,
				Name:      constant.GenerateClusterComponentName(clusterName, compName),
				Labels: map[string]string{
					constant.AppManagedByLabelKey:   constant.AppName,
					constant.AppInstanceLabelKey:    clusterName,
					constant.KBAppComponentLabelKey: compName,
				},
			},
			Spec: appsv1.ComponentSpec{},
		}

		synthesizeComp = &component.SynthesizedComponent{
			Namespace:   testCtx.DefaultNamespace,
			ClusterName: clusterName,
			Name:        compName,
			Roles:       roles,
			LifecycleActions: component.SynthesizedLifecycleActions{
				ComponentLifecycleActions: &appsv1.ComponentLifecycleActions{
					MemberJoin: &appsv1.Action{
						Exec: &appsv1.ExecAction{
							Image: "test-image",
						},
					},
					MemberLeave: &appsv1.Action{
						Exec: &appsv1.ExecAction{
							Image: "test-image",
						},
					},
					Switchover: &appsv1.Action{
						Exec: &appsv1.ExecAction{
							Image: "test-image",
						},
					},
				},
			},
		}

		graphCli := model.NewGraphClient(reader)
		dag = newDAG(graphCli, comp)
	})

	Context("Member Leave Operations", func() {
		var (
			ops  *componentWorkloadOps
			pod0 *corev1.Pod
			pod1 *corev1.Pod
			pods []*corev1.Pod
		)

		BeforeEach(func() {
			pod0 = testapps.NewPodFactory(testCtx.DefaultNamespace, "test-pod-0").
				AddContainer(corev1.Container{
					Image: "test-image",
					Name:  "test-container",
				}).
				AddLabels(
					constant.AppManagedByLabelKey, kubeblocksName,
					constant.AppInstanceLabelKey, clusterName,
					constant.KBAppComponentLabelKey, compName,
				).
				GetObject()

			pod1 = testapps.NewPodFactory(testCtx.DefaultNamespace, "test-pod-1").
				AddContainer(corev1.Container{
					Image: "test-image",
					Name:  "test-container",
				}).
				AddLabels(
					constant.AppManagedByLabelKey, kubeblocksName,
					constant.AppInstanceLabelKey, clusterName,
					constant.KBAppComponentLabelKey, compName,
				).
				GetObject()

			pods = []*corev1.Pod{pod0, pod1}

			container := corev1.Container{
				Name:            "mock-container-name",
				Image:           testapps.ApeCloudMySQLImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
			}

			mockITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"test-its", clusterName, compName).
				AddFinalizers([]string{constant.DBClusterFinalizerName}).
				AddContainer(container).
				AddAppInstanceLabel(clusterName).
				AddAppComponentLabel(compName).
				AddAppManagedByLabel().
				SetReplicas(2).
				SetRoles(roles).
				GetObject()

			ops = &componentWorkloadOps{
				transCtx: &componentTransformContext{
					Context:       ctx,
					Logger:        logger,
					EventRecorder: clusterRecorder,
				},
				cli:            k8sClient,
				component:      comp,
				synthesizeComp: synthesizeComp,
				runningITS:     mockITS,
				protoITS:       mockITS.DeepCopy(),
				dag:            dag,
			}
		})

		It("should handle switchover for when scale in", func() {
			testapps.MockKBAgentClient(func(recorder *kbacli.MockClientMockRecorder) {
				recorder.Action(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(func(ctx context.Context, req kbagentproto.ActionRequest) (kbagentproto.ActionResponse, error) {
					GinkgoWriter.Printf("ActionRequest: %#v\n", req)
					switch req.Action {
					case "switchover":
						Expect(req.Parameters["KB_SWITCHOVER_CURRENT_NAME"]).Should(Equal(pod1.Name))
					case "memberLeave":
						Expect(req.Parameters["KB_LEAVE_MEMBER_POD_NAME"]).Should(Equal(pod1.Name))
					}
					rsp := kbagentproto.ActionResponse{Message: "mock success"}
					return rsp, nil
				})
			})

			By("setting up leader pod")
			pod1.Labels[constant.RoleLabelKey] = "follower"
			pod1.Labels[constant.RoleLabelKey] = "leader"

			By("executing leave member for leader")
			Expect(ops.leaveMemberForPod(pod1, pods)).Should(Succeed())
		})

		It("should eliminate upgrade-only diff by preserving legacy config-manager", func() {
			oldITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"old-its", clusterName, compName).
				AddContainer(corev1.Container{
					Name:  "main",
					Image: "test-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				}).
				AddVolume(corev1.Volume{
					Name: "kb-tools",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}).
				GetObject()
			oldITS.Spec.Template.Spec.Containers = append(oldITS.Spec.Template.Spec.Containers,
				corev1.Container{
					Name:  "config-manager",
					Image: "cm-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				},
				corev1.Container{
					Name:  "metrics",
					Image: "metrics-image",
				},
			)
			oldITS.Spec.Template.Spec.InitContainers = append(oldITS.Spec.Template.Spec.InitContainers, corev1.Container{
				Name:  "install-config-manager-tool",
				Image: "tools-image",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "kb-tools",
					MountPath: "/opt/kb-tools",
				}},
			})

			newITS := oldITS.DeepCopy()
			newITS.Spec.Template.Spec.Containers = []corev1.Container{
				{
					Name:  "main",
					Image: "test-image",
				},
				{
					Name:  "metrics",
					Image: "metrics-image",
				},
			}
			newITS.Spec.Template.Spec.InitContainers = nil
			newITS.Spec.Template.Spec.Volumes = nil

			merged := copyAndMergeITS(oldITS, newITS, legacyConfigManagerPolicyKeep)
			Expect(merged).Should(BeNil())
		})

		It("should not reintroduce legacy config-manager for workloads that never had it", func() {
			oldITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"old-its-no-legacy", clusterName, compName).
				AddContainer(corev1.Container{
					Name:  "main",
					Image: "test-image",
				}).
				GetObject()

			newITS := oldITS.DeepCopy()
			newITS.Spec.Template.Spec.Containers = []corev1.Container{
				{
					Name:  "main",
					Image: "new-image",
				},
			}

			merged := copyAndMergeITS(oldITS, newITS, legacyConfigManagerPolicyKeep)
			Expect(merged).ShouldNot(BeNil())
			Expect(merged.Spec.Template.Spec.Containers).Should(HaveLen(1))
			Expect(merged.Spec.Template.Spec.Containers[0].Name).Should(Equal("main"))
			Expect(merged.Spec.Template.Spec.Containers[0].Image).Should(Equal("new-image"))
			_, cfg := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.Containers, "config-manager")
			Expect(cfg).Should(BeNil())
			_, init := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.InitContainers, "install-config-manager-tool")
			Expect(init).Should(BeNil())
			Expect(merged.Spec.Template.Spec.Volumes).Should(BeEmpty())
		})

		It("should preserve only the legacy config-manager resources that still exist on the live template", func() {
			// Some clusters may carry partially migrated legacy resources. The compatibility logic should
			// keep only what still exists on the live template instead of synthesizing a full legacy bundle.
			buildBaseITS := func(name string) *workloads.InstanceSet {
				its := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace, name, clusterName, compName).
					AddContainer(corev1.Container{
						Name:  "main",
						Image: "test-image",
					}).
					GetObject()
				its.Spec.Template.Spec.Containers = append(its.Spec.Template.Spec.Containers, corev1.Container{
					Name:  "config-manager",
					Image: "cm-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				})
				return its
			}

			tests := []struct {
				name            string
				oldITS          *workloads.InstanceSet
				wantInit        bool
				wantVolume      bool
				wantMainMount   bool
				wantConfigMount bool
			}{
				{
					name: "missing legacy init container",
					oldITS: func() *workloads.InstanceSet {
						its := buildBaseITS("old-its-no-legacy-init")
						its.Spec.Template.Spec.Volumes = append(its.Spec.Template.Spec.Volumes, corev1.Volume{
							Name: "kb-tools",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						})
						its.Spec.Template.Spec.Containers[0].VolumeMounts = append(its.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
							Name:      "kb-tools",
							MountPath: "/opt/kb-tools",
						})
						return its
					}(),
					wantInit:        false,
					wantVolume:      true,
					wantMainMount:   true,
					wantConfigMount: true,
				},
				{
					name: "missing legacy volume",
					oldITS: func() *workloads.InstanceSet {
						its := buildBaseITS("old-its-no-legacy-volume")
						its.Spec.Template.Spec.InitContainers = append(its.Spec.Template.Spec.InitContainers, corev1.Container{
							Name:  "install-config-manager-tool",
							Image: "tools-image",
							VolumeMounts: []corev1.VolumeMount{{
								Name:      "kb-tools",
								MountPath: "/opt/kb-tools",
							}},
						})
						its.Spec.Template.Spec.Containers[0].VolumeMounts = append(its.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
							Name:      "kb-tools",
							MountPath: "/opt/kb-tools",
						})
						return its
					}(),
					wantInit:        true,
					wantVolume:      false,
					wantMainMount:   true,
					wantConfigMount: true,
				},
				{
					name: "missing business mount for legacy volume",
					oldITS: func() *workloads.InstanceSet {
						its := buildBaseITS("old-its-no-business-mount")
						its.Spec.Template.Spec.InitContainers = append(its.Spec.Template.Spec.InitContainers, corev1.Container{
							Name:  "install-config-manager-tool",
							Image: "tools-image",
							VolumeMounts: []corev1.VolumeMount{{
								Name:      "kb-tools",
								MountPath: "/opt/kb-tools",
							}},
						})
						its.Spec.Template.Spec.Volumes = append(its.Spec.Template.Spec.Volumes, corev1.Volume{
							Name: "kb-tools",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						})
						return its
					}(),
					wantInit:        true,
					wantVolume:      true,
					wantMainMount:   false,
					wantConfigMount: true,
				},
			}

			for _, tt := range tests {
				By(tt.name)
				newITS := tt.oldITS.DeepCopy()
				newITS.Spec.Template.Spec.Containers = []corev1.Container{
					{
						Name:  "main",
						Image: "new-image",
					},
				}
				newITS.Spec.Template.Spec.InitContainers = nil
				newITS.Spec.Template.Spec.Volumes = nil

				merged := copyAndMergeITS(tt.oldITS, newITS, legacyConfigManagerPolicyKeep)
				Expect(merged).ShouldNot(BeNil())

				_, cfg := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.Containers, "config-manager")
				Expect(cfg).ShouldNot(BeNil())
				Expect(hasVolumeMount(cfg.VolumeMounts, "kb-tools")).Should(Equal(tt.wantConfigMount))

				_, init := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.InitContainers, "install-config-manager-tool")
				if tt.wantInit {
					Expect(init).ShouldNot(BeNil())
				} else {
					Expect(init).Should(BeNil())
				}

				if tt.wantVolume {
					Expect(merged.Spec.Template.Spec.Volumes).ShouldNot(BeEmpty())
					Expect(hasVolumeByName(func() map[string]struct{} {
						names := map[string]struct{}{}
						for _, volume := range merged.Spec.Template.Spec.Volumes {
							names[volume.Name] = struct{}{}
						}
						return names
					}(), "kb-tools")).Should(BeTrue())
				} else {
					Expect(merged.Spec.Template.Spec.Volumes).Should(BeEmpty())
				}

				_, main := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.Containers, "main")
				Expect(main).ShouldNot(BeNil())
				Expect(hasVolumeMount(main.VolumeMounts, "kb-tools")).Should(Equal(tt.wantMainMount))
			}
		})

		It("should keep legacy config-manager ordering during in-place business changes", func() {
			oldITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"old-its-order", clusterName, compName).
				AddContainer(corev1.Container{
					Name:  "main",
					Image: "test-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				}).
				GetObject()
			oldITS.Spec.PodUpgradePolicy = appsv1.PreferInPlacePodUpdatePolicyType
			oldITS.Spec.Template.Spec.Containers = append(oldITS.Spec.Template.Spec.Containers,
				corev1.Container{
					Name:  "config-manager",
					Image: "cm-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				},
				corev1.Container{
					Name:  "exporter",
					Image: "exporter-image",
				},
			)
			oldITS.Spec.Template.Spec.InitContainers = append(oldITS.Spec.Template.Spec.InitContainers,
				corev1.Container{
					Name:    "business-init",
					Image:   "init-image",
					Command: []string{"prepare-old"},
				},
				corev1.Container{
					Name:  "install-config-manager-tool",
					Image: "tools-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				},
			)
			oldITS.Spec.Template.Spec.Volumes = append(oldITS.Spec.Template.Spec.Volumes, corev1.Volume{
				Name: "kb-tools",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			})

			newITS := oldITS.DeepCopy()
			newITS.Spec.Template.Spec.Containers = []corev1.Container{
				{
					Name:  "main",
					Image: "main-business-upgrade",
				},
				{
					Name:  "exporter",
					Image: "exporter-business-upgrade",
				},
			}
			newITS.Spec.Template.Spec.InitContainers = []corev1.Container{
				{
					Name:    "business-init",
					Image:   "init-image",
					Command: []string{"prepare-new"},
				},
			}
			newITS.Spec.Template.Spec.Volumes = nil

			merged := copyAndMergeITS(oldITS, newITS, legacyConfigManagerPolicyKeep)
			Expect(merged).ShouldNot(BeNil())
			_, cfg := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.Containers, "config-manager")
			Expect(cfg).ShouldNot(BeNil())
			_, init := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.InitContainers, "install-config-manager-tool")
			Expect(init).ShouldNot(BeNil())
			Expect(len(merged.Spec.Template.Spec.Volumes)).ShouldNot(BeZero())
			for _, c := range merged.Spec.Template.Spec.Containers {
				if c.Name == "main" {
					Expect(len(c.VolumeMounts)).Should(Equal(1))
					Expect(c.VolumeMounts[0].Name).Should(Equal("kb-tools"))
				}
			}
			Expect(merged.Spec.Template.Spec.Containers[0].Name).Should(Equal("main"))
			Expect(merged.Spec.Template.Spec.Containers[1].Name).Should(Equal("config-manager"))
			Expect(merged.Spec.Template.Spec.Containers[2].Name).Should(Equal("exporter"))
			Expect(merged.Spec.Template.Spec.InitContainers[0].Name).Should(Equal("business-init"))
			Expect(merged.Spec.Template.Spec.InitContainers[0].Command).Should(Equal([]string{"prepare-new"}))
			Expect(merged.Spec.Template.Spec.InitContainers[1].Name).Should(Equal("install-config-manager-tool"))
		})

		It("should drop legacy config-manager when business changes already recreate pods", func() {
			oldITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"old-its-recreate", clusterName, compName).
				AddContainer(corev1.Container{
					Name:  "main",
					Image: "test-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				}).
				AddVolume(corev1.Volume{
					Name: "kb-tools",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}).
				GetObject()
			oldITS.Spec.PodUpgradePolicy = appsv1.ReCreatePodUpdatePolicyType
			oldITS.Spec.Template.Spec.Containers = append(oldITS.Spec.Template.Spec.Containers, corev1.Container{
				Name:  "config-manager",
				Image: "cm-image",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "kb-tools",
					MountPath: "/opt/kb-tools",
				}},
			})
			oldITS.Spec.Template.Spec.InitContainers = append(oldITS.Spec.Template.Spec.InitContainers, corev1.Container{
				Name:  "install-config-manager-tool",
				Image: "tools-image",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "kb-tools",
					MountPath: "/opt/kb-tools",
				}},
			})
			oldITS.Spec.Template.Spec.InitContainers = append([]corev1.Container{{
				Name:    "business-init",
				Image:   "init-image",
				Command: []string{"prepare-old"},
			}}, oldITS.Spec.Template.Spec.InitContainers...)

			newITS := oldITS.DeepCopy()
			newITS.Spec.PodUpgradePolicy = appsv1.ReCreatePodUpdatePolicyType
			newITS.Spec.Template.Spec.Containers = []corev1.Container{
				{
					Name:  "main",
					Image: "test-image",
				},
			}
			newITS.Spec.Template.Spec.InitContainers = []corev1.Container{{
				Name:    "business-init",
				Image:   "init-image",
				Command: []string{"prepare-new"},
			}}
			newITS.Spec.Template.Spec.Volumes = nil

			merged := copyAndMergeITS(oldITS, newITS, legacyConfigManagerPolicyCleanup)
			Expect(merged).ShouldNot(BeNil())
			_, cfg := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.Containers, "config-manager")
			Expect(cfg).Should(BeNil())
			_, init := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.InitContainers, "install-config-manager-tool")
			Expect(init).Should(BeNil())
			Expect(merged.Spec.Template.Spec.Volumes).Should(BeEmpty())
		})

		It("should clean legacy config-manager when component annotation allows cleanup on recreate rollout", func() {
			oldITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"old-its-recreate-false-annotation", clusterName, compName).
				AddContainer(corev1.Container{
					Name:  "main",
					Image: "test-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				}).
				AddVolume(corev1.Volume{
					Name: "kb-tools",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}).
				GetObject()
			oldITS.Spec.PodUpgradePolicy = appsv1.ReCreatePodUpdatePolicyType
			oldITS.Spec.Template.Spec.Containers = append(oldITS.Spec.Template.Spec.Containers, corev1.Container{
				Name:  "config-manager",
				Image: "cm-image",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "kb-tools",
					MountPath: "/opt/kb-tools",
				}},
			})
			oldITS.Spec.Template.Spec.InitContainers = append(oldITS.Spec.Template.Spec.InitContainers,
				corev1.Container{
					Name:    "business-init",
					Image:   "init-image",
					Command: []string{"prepare-old"},
				},
				corev1.Container{
					Name:  "install-config-manager-tool",
					Image: "tools-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				},
			)

			comp := &appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constant.LegacyConfigManagerRequiredAnnotationKey: "false",
					},
				},
			}

			newITS := oldITS.DeepCopy()
			newITS.Spec.PodUpgradePolicy = appsv1.ReCreatePodUpdatePolicyType
			newITS.Spec.Template.Spec.Containers = []corev1.Container{{
				Name:  "main",
				Image: "test-image",
			}}
			newITS.Spec.Template.Spec.InitContainers = []corev1.Container{{
				Name:    "business-init",
				Image:   "init-image",
				Command: []string{"prepare-new"},
			}}
			newITS.Spec.Template.Spec.Volumes = nil

			merged := copyAndMergeITS(oldITS, newITS, legacyConfigManagerRequired(comp))
			Expect(legacyConfigManagerRequired(comp)).Should(Equal(legacyConfigManagerPolicyCleanup))
			Expect(merged).ShouldNot(BeNil())

			_, cfg := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.Containers, "config-manager")
			Expect(cfg).Should(BeNil())
			_, init := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.InitContainers, "install-config-manager-tool")
			Expect(init).Should(BeNil())
			Expect(hasVolumeByName(func() map[string]struct{} {
				names := map[string]struct{}{}
				for _, volume := range merged.Spec.Template.Spec.Volumes {
					names[volume.Name] = struct{}{}
				}
				return names
			}(), "kb-tools")).Should(BeFalse())
			_, main := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.Containers, "main")
			Expect(main).ShouldNot(BeNil())
			Expect(hasVolumeMount(main.VolumeMounts, "kb-tools")).Should(BeFalse())
		})

		It("should keep legacy config-manager when compatibility annotation is still required", func() {
			oldITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"old-its-annotation-required", clusterName, compName).
				AddContainer(corev1.Container{
					Name:  "main",
					Image: "test-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				}).
				AddVolume(corev1.Volume{
					Name: "kb-tools",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}).
				GetObject()
			oldITS.Spec.PodUpgradePolicy = appsv1.ReCreatePodUpdatePolicyType
			oldITS.Spec.Template.Spec.Containers = append(oldITS.Spec.Template.Spec.Containers, corev1.Container{
				Name:  "config-manager",
				Image: "cm-image",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "kb-tools",
					MountPath: "/opt/kb-tools",
				}},
			})
			oldITS.Spec.Template.Spec.InitContainers = append(oldITS.Spec.Template.Spec.InitContainers,
				corev1.Container{
					Name:    "business-init",
					Image:   "init-image",
					Command: []string{"prepare-old"},
				},
				corev1.Container{
					Name:  "install-config-manager-tool",
					Image: "tools-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				},
			)

			newITS := oldITS.DeepCopy()
			newITS.Spec.PodUpgradePolicy = appsv1.ReCreatePodUpdatePolicyType
			newITS.Spec.Template.Spec.Containers = []corev1.Container{
				{
					Name:  "main",
					Image: "test-image",
				},
			}
			newITS.Spec.Template.Spec.InitContainers = []corev1.Container{{
				Name:    "business-init",
				Image:   "init-image",
				Command: []string{"prepare-new"},
			}}
			newITS.Spec.Template.Spec.Volumes = nil

			merged := copyAndMergeITS(oldITS, newITS, legacyConfigManagerPolicyKeep)
			Expect(merged).ShouldNot(BeNil())
			_, cfg := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.Containers, "config-manager")
			Expect(cfg).ShouldNot(BeNil())
			_, init := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.InitContainers, "install-config-manager-tool")
			Expect(init).ShouldNot(BeNil())
			Expect(merged.Spec.Template.Spec.Volumes).Should(HaveLen(1))
			Expect(merged.Spec.Template.Spec.Volumes[0].Name).Should(Equal("kb-tools"))
		})

		It("should keep legacy config-manager when business changes stay in-place", func() {
			oldITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"old-its-inplace", clusterName, compName).
				AddContainer(corev1.Container{
					Name:  "main",
					Image: "test-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				}).
				AddVolume(corev1.Volume{
					Name: "kb-tools",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}).
				GetObject()
			oldITS.Spec.PodUpgradePolicy = appsv1.PreferInPlacePodUpdatePolicyType
			oldITS.Spec.Template.Spec.Containers = append(oldITS.Spec.Template.Spec.Containers, corev1.Container{
				Name:  "config-manager",
				Image: "cm-image",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "kb-tools",
					MountPath: "/opt/kb-tools",
				}},
			})
			oldITS.Spec.Template.Spec.InitContainers = append(oldITS.Spec.Template.Spec.InitContainers, corev1.Container{
				Name:  "install-config-manager-tool",
				Image: "tools-image",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "kb-tools",
					MountPath: "/opt/kb-tools",
				}},
			})
			oldITS.Spec.Template.Spec.InitContainers = append([]corev1.Container{{
				Name:    "business-init",
				Image:   "init-image",
				Command: []string{"prepare-old"},
			}}, oldITS.Spec.Template.Spec.InitContainers...)

			newITS := oldITS.DeepCopy()
			newITS.Spec.PodUpgradePolicy = appsv1.PreferInPlacePodUpdatePolicyType
			newITS.Spec.Template.Spec.Containers = []corev1.Container{
				{
					Name:  "main",
					Image: "test-image",
				},
			}
			newITS.Spec.Template.Spec.InitContainers = []corev1.Container{{
				Name:    "business-init",
				Image:   "init-image",
				Command: []string{"prepare-new"},
			}}
			newITS.Spec.Template.Spec.Volumes = nil

			merged := copyAndMergeITS(oldITS, newITS, legacyConfigManagerPolicyKeep)
			Expect(merged).ShouldNot(BeNil())
			_, cfg := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.Containers, "config-manager")
			Expect(cfg).ShouldNot(BeNil())
			Expect(merged.Spec.Template.Spec.InitContainers[0].Name).Should(Equal("business-init"))
			Expect(merged.Spec.Template.Spec.InitContainers[0].Command).Should(Equal([]string{"prepare-new"}))
			Expect(merged.Spec.Template.Spec.InitContainers[1].Name).Should(Equal("install-config-manager-tool"))
		})

		It("should keep legacy config-manager when annotation is absent and rollout stays in-place", func() {
			oldITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"old-its-inplace-no-annotation", clusterName, compName).
				AddContainer(corev1.Container{
					Name:  "main",
					Image: "test-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				}).
				AddVolume(corev1.Volume{
					Name: "kb-tools",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}).
				GetObject()
			oldITS.Spec.PodUpgradePolicy = appsv1.PreferInPlacePodUpdatePolicyType
			oldITS.Spec.Template.Spec.Containers = append(oldITS.Spec.Template.Spec.Containers, corev1.Container{
				Name:  "config-manager",
				Image: "cm-image",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "kb-tools",
					MountPath: "/opt/kb-tools",
				}},
			})
			oldITS.Spec.Template.Spec.InitContainers = append(oldITS.Spec.Template.Spec.InitContainers,
				corev1.Container{
					Name:    "business-init",
					Image:   "init-image",
					Command: []string{"prepare-old"},
				},
				corev1.Container{
					Name:  "install-config-manager-tool",
					Image: "tools-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				},
			)

			newITS := oldITS.DeepCopy()
			newITS.Spec.PodUpgradePolicy = appsv1.PreferInPlacePodUpdatePolicyType
			newITS.Spec.Template.Spec.Containers = []corev1.Container{
				{
					Name:  "main",
					Image: "test-image",
				},
			}
			newITS.Spec.Template.Spec.InitContainers = []corev1.Container{{
				Name:    "business-init",
				Image:   "init-image",
				Command: []string{"prepare-new"},
			}}
			newITS.Spec.Template.Spec.Volumes = nil

			merged := copyAndMergeITS(oldITS, newITS, legacyConfigManagerPolicyKeep)
			Expect(merged).ShouldNot(BeNil())
			_, cfg := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.Containers, "config-manager")
			Expect(cfg).ShouldNot(BeNil())
			_, init := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.InitContainers, "install-config-manager-tool")
			Expect(init).ShouldNot(BeNil())
			Expect(merged.Spec.Template.Spec.Volumes).Should(HaveLen(1))
			Expect(merged.Spec.Template.Spec.Volumes[0].Name).Should(Equal("kb-tools"))
		})

		It("should keep legacy config-manager when annotation is absent and rollout recreates pods", func() {
			oldITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"old-its-recreate-no-annotation", clusterName, compName).
				AddContainer(corev1.Container{
					Name:  "main",
					Image: "test-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				}).
				AddVolume(corev1.Volume{
					Name: "kb-tools",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}).
				GetObject()
			oldITS.Spec.PodUpgradePolicy = appsv1.ReCreatePodUpdatePolicyType
			oldITS.Spec.Template.Spec.Containers = append(oldITS.Spec.Template.Spec.Containers, corev1.Container{
				Name:  "config-manager",
				Image: "cm-image",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "kb-tools",
					MountPath: "/opt/kb-tools",
				}},
			})
			oldITS.Spec.Template.Spec.InitContainers = append(oldITS.Spec.Template.Spec.InitContainers,
				corev1.Container{
					Name:    "business-init",
					Image:   "init-image",
					Command: []string{"prepare-old"},
				},
				corev1.Container{
					Name:  "install-config-manager-tool",
					Image: "tools-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				},
			)

			newITS := oldITS.DeepCopy()
			newITS.Spec.PodUpgradePolicy = appsv1.ReCreatePodUpdatePolicyType
			newITS.Spec.Template.Spec.Containers = []corev1.Container{{
				Name:  "main",
				Image: "test-image",
			}}
			newITS.Spec.Template.Spec.InitContainers = []corev1.Container{{
				Name:    "business-init",
				Image:   "init-image",
				Command: []string{"prepare-new"},
			}}
			newITS.Spec.Template.Spec.Volumes = nil

			merged := copyAndMergeITS(oldITS, newITS, legacyConfigManagerPolicyKeep)
			Expect(merged).ShouldNot(BeNil())
			_, cfg := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.Containers, "config-manager")
			Expect(cfg).ShouldNot(BeNil())
			_, init := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.InitContainers, "install-config-manager-tool")
			Expect(init).ShouldNot(BeNil())
			Expect(merged.Spec.Template.Spec.Volumes).Should(HaveLen(1))
		})

		It("should keep legacy config-manager when annotation is false but rollout stays in-place", func() {
			oldITS := testapps.NewInstanceSetFactory(testCtx.DefaultNamespace,
				"old-its-inplace-false-annotation", clusterName, compName).
				AddContainer(corev1.Container{
					Name:  "main",
					Image: "test-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				}).
				AddVolume(corev1.Volume{
					Name: "kb-tools",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}).
				GetObject()
			oldITS.Spec.PodUpgradePolicy = appsv1.PreferInPlacePodUpdatePolicyType
			oldITS.Spec.Template.Spec.Containers = append(oldITS.Spec.Template.Spec.Containers, corev1.Container{
				Name:  "config-manager",
				Image: "cm-image",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "kb-tools",
					MountPath: "/opt/kb-tools",
				}},
			})
			oldITS.Spec.Template.Spec.InitContainers = append(oldITS.Spec.Template.Spec.InitContainers,
				corev1.Container{
					Name:    "business-init",
					Image:   "init-image",
					Command: []string{"prepare-old"},
				},
				corev1.Container{
					Name:  "install-config-manager-tool",
					Image: "tools-image",
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "kb-tools",
						MountPath: "/opt/kb-tools",
					}},
				},
			)

			newITS := oldITS.DeepCopy()
			newITS.Spec.PodUpgradePolicy = appsv1.PreferInPlacePodUpdatePolicyType
			newITS.Spec.Template.Spec.Containers = []corev1.Container{{
				Name:  "main",
				Image: "test-image",
			}}
			newITS.Spec.Template.Spec.InitContainers = []corev1.Container{{
				Name:    "business-init",
				Image:   "init-image",
				Command: []string{"prepare-new"},
			}}
			newITS.Spec.Template.Spec.Volumes = nil

			merged := copyAndMergeITS(oldITS, newITS, legacyConfigManagerPolicyCleanup)
			Expect(merged).ShouldNot(BeNil())
			_, cfg := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.Containers, "config-manager")
			Expect(cfg).ShouldNot(BeNil())
			_, init := intctrlutil.GetContainerByName(merged.Spec.Template.Spec.InitContainers, "install-config-manager-tool")
			Expect(init).ShouldNot(BeNil())
			Expect(merged.Spec.Template.Spec.Volumes).Should(HaveLen(1))
		})

		It("should resolve legacy config-manager policy conservatively when annotation is missing", func() {
			Expect(legacyConfigManagerRequired(nil)).Should(Equal(legacyConfigManagerPolicyKeep))
			Expect(legacyConfigManagerRequired(&appsv1.Component{})).Should(Equal(legacyConfigManagerPolicyKeep))
			Expect(legacyConfigManagerRequired(&appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constant.LegacyConfigManagerRequiredAnnotationKey: "true",
					},
				},
			})).Should(Equal(legacyConfigManagerPolicyKeep))
			Expect(legacyConfigManagerRequired(&appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constant.LegacyConfigManagerRequiredAnnotationKey: "false",
					},
				},
			})).Should(Equal(legacyConfigManagerPolicyCleanup))
			Expect(legacyConfigManagerRequired(&appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constant.LegacyConfigManagerRequiredAnnotationKey: "",
					},
				},
			})).Should(Equal(legacyConfigManagerPolicyKeep))
		})
	})
})
