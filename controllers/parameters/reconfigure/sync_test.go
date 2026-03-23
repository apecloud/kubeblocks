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

package reconfigure

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/parameters/core"
)

var _ = ginkgo.Describe("syncPolicy test", func() {
	ginkgo.Context("sync policy", func() {
		var (
			rctx Context
		)

		ginkgo.BeforeEach(func() {
			configHash := "test-config-hash"
			rctx = Context{
				RequestCtx: intctrlutil.RequestCtx{
					Ctx: context.Background(),
					Log: log.FromContext(context.Background()),
				},
				Client: nil,
				ConfigTemplate: appsv1.ComponentFileTemplate{
					Name: cfgName,
				},
				ConfigHash: &configHash,
				Cluster: &appsv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						Namespace: "default",
					},
				},
				ClusterComponent: &appsv1.ClusterComponentSpec{
					Name:     "test-component",
					Replicas: 3,
					Configs: []appsv1.ClusterComponentConfig{
						{
							Name: ptr.To(cfgName),
						},
					},
				},
				ITS: &workloads.InstanceSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-instanceset",
						Namespace: "default",
					},
					Spec: workloads.InstanceSetSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{{
									Name: "config-manager",
									Ports: []corev1.ContainerPort{{
										Name:          "config-manager",
										ContainerPort: 9901,
									}},
								}},
							},
						},
					},
				},
				ConfigDescription: &parametersv1alpha1.ComponentConfigDescription{
					Name: cfgName,
					FileFormatConfig: &parametersv1alpha1.FileFormatConfig{
						Format: parametersv1alpha1.RedisCfg,
					},
				},
				ParametersDef: &parametersv1alpha1.ParametersDefinitionSpec{
					MergeReloadAndRestart:           ptr.To(false),
					ReloadStaticParamsBeforeRestart: ptr.To(true),
				},
				Patch: &core.ConfigPatchInfo{
					IsModify: true,
					UpdateConfig: map[string][]byte{
						cfgName: []byte(`{"a":"c b e f"}`),
					},
				},
			}
		})

		ginkgo.It("update cluster spec", func() {
			ginkgo.By("update cluster spec")
			status, err := syncPolicy(rctx)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(StatusRetry))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(3))
			Expect(status.SucceedCount).Should(BeEquivalentTo(0))

			Expect(*rctx.ClusterComponent.Configs[0].ConfigHash).Should(Equal(*rctx.getTargetConfigHash()))
			Expect(rctx.ClusterComponent.Configs[0].Variables).Should(HaveKeyWithValue("a", "c b e f"))
		})

		ginkgo.It("status replicas - partially updated", func() {
			ginkgo.By("update cluster spec")
			status, err := syncPolicy(rctx)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(StatusRetry))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(3))
			Expect(status.SucceedCount).Should(BeEquivalentTo(0))

			ginkgo.By("mock the instance status")
			rctx.ITS.Status.InstanceStatus = []workloads.InstanceStatus{
				{
					PodName: "pod-0",
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       rctx.ConfigTemplate.Name,
							ConfigHash: rctx.getTargetConfigHash(),
						},
					},
				},
			}

			ginkgo.By("status check")
			status, err = syncPolicy(rctx)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(StatusRetry))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(3))
			Expect(status.SucceedCount).Should(BeEquivalentTo(1))
		})

		ginkgo.It("status replicas - all", func() {
			ginkgo.By("update cluster spec")
			status, err := syncPolicy(rctx)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(StatusRetry))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(3))
			Expect(status.SucceedCount).Should(BeEquivalentTo(0))

			ginkgo.By("mock the instance status")
			rctx.ITS.Status.InstanceStatus = []workloads.InstanceStatus{
				{
					PodName: "pod-0",
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       rctx.ConfigTemplate.Name,
							ConfigHash: rctx.getTargetConfigHash(),
						},
					},
				},
				{
					PodName: "pod-1",
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       rctx.ConfigTemplate.Name,
							ConfigHash: rctx.getTargetConfigHash(),
						},
					},
				},
				{
					PodName: "pod-2",
					Configs: []workloads.InstanceConfigStatus{
						{
							Name:       rctx.ConfigTemplate.Name,
							ConfigHash: rctx.getTargetConfigHash(),
						},
					},
				},
			}

			ginkgo.By("status check")
			status, err = syncPolicy(rctx)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(BeEquivalentTo(StatusNone))
			Expect(status.ExpectedCount).Should(BeEquivalentTo(3))
			Expect(status.SucceedCount).Should(BeEquivalentTo(3))
		})
	})

	ginkgo.Context("reconfigure conditions", func() {
		var (
			rctx Context
		)

		ginkgo.BeforeEach(func() {
			configHash := "test-config-hash"
			rctx = Context{
				RequestCtx: intctrlutil.RequestCtx{
					Ctx: context.Background(),
					Log: log.FromContext(context.Background()),
				},
				Client: nil,
				ConfigTemplate: appsv1.ComponentFileTemplate{
					Name: cfgName,
				},
				ConfigHash: &configHash,
				Cluster: &appsv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						Namespace: "default",
					},
				},
				ClusterComponent: &appsv1.ClusterComponentSpec{
					Name:     "test-component",
					Replicas: 1,
					Configs: []appsv1.ClusterComponentConfig{
						{
							Name: ptr.To(cfgName),
						},
					},
				},
				ITS: &workloads.InstanceSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-instanceset",
						Namespace: "default",
					},
					Spec: workloads.InstanceSetSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{{
									Name: "config-manager",
									Ports: []corev1.ContainerPort{{
										Name:          "config-manager",
										ContainerPort: 9901,
									}},
								}},
							},
						},
					},
				},
				ConfigDescription: &parametersv1alpha1.ComponentConfigDescription{
					Name: cfgName,
					FileFormatConfig: &parametersv1alpha1.FileFormatConfig{
						Format: parametersv1alpha1.RedisCfg,
					},
				},
				ParametersDef: &parametersv1alpha1.ParametersDefinitionSpec{
					MergeReloadAndRestart:           ptr.To(false),
					ReloadStaticParamsBeforeRestart: ptr.To(true),
					ReloadAction: &parametersv1alpha1.ReloadAction{
						ShellTrigger: &parametersv1alpha1.ShellTrigger{
							Command: []string{"bash", "-c", "reload"},
							Sync:    ptr.To(true),
						},
					},
				},
				Patch: &core.ConfigPatchInfo{
					IsModify: true,
					UpdateConfig: map[string][]byte{
						cfgName: []byte(`{"a":"c b e f"}`),
					},
				},
			}
		})

		ginkgo.It("set reconfigure when sync reload with shell trigger", func() {
			_, err := syncPolicy(rctx)
			Expect(err).Should(Succeed())
			config := rctx.ClusterComponent.Configs[0]
			Expect(config.Reconfigure).ShouldNot(BeNil())
			Expect(*config.Reconfigure).Should(BeTrue())
			Expect(config.ReconfigureAction).ShouldNot(BeNil())
			Expect(config.ReconfigureAction.GRPC).ShouldNot(BeNil())
			Expect(config.ReconfigureAction.GRPC.Port).Should(Equal("9901"))
			Expect(config.ReconfigureAction.GRPC.Service).Should(Equal("proto.Reconfigure"))
			Expect(config.ReconfigureAction.GRPC.Method).Should(Equal("OnlineUpgradeParams"))
			Expect(config.ReconfigureAction.GRPC.Request).Should(HaveKeyWithValue("configSpec", cfgName))
			Expect(config.ReconfigureAction.GRPC.Request).Should(HaveKeyWithValue("configFile", cfgName))
			Expect(config.ReconfigureAction.GRPC.Request).Should(HaveKey("params"))
			params := map[string]string{}
			Expect(json.Unmarshal([]byte(config.ReconfigureAction.GRPC.Request["params"]), &params)).Should(Succeed())
			Expect(params).Should(HaveKeyWithValue("a", "c b e f"))
			Expect(config.ReconfigureAction.GRPC.Response.Status).Should(Equal("errMessage"))
			Expect(config.Restart).ShouldNot(BeNil())
			Expect(*config.Restart).Should(BeFalse())
		})

		ginkgo.It("use config manager port from workload template", func() {
			rctx.ITS.Spec.Template.Spec.Containers = []corev1.Container{{
				Name: "config-manager",
				Ports: []corev1.ContainerPort{{
					Name:          "config-manager",
					ContainerPort: 19901,
				}},
			}}
			_, err := syncPolicy(rctx)
			Expect(err).Should(Succeed())
			config := rctx.ClusterComponent.Configs[0]
			Expect(config.Reconfigure).ShouldNot(BeNil())
			Expect(*config.Reconfigure).Should(BeTrue())
			Expect(config.ReconfigureAction).ShouldNot(BeNil())
			Expect(config.ReconfigureAction.GRPC).ShouldNot(BeNil())
			Expect(config.ReconfigureAction.GRPC.Port).Should(Equal("19901"))
		})

		ginkgo.It("fail when legacy config manager is not injected", func() {
			rctx.ITS.Spec.Template.Spec.Containers = nil
			status, err := syncPolicy(rctx)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(Equal(StatusFailed))
			Expect(status.Reason).Should(ContainSubstring("config-manager"))
		})

		ginkgo.It("fail when legacy config manager port is not exposed", func() {
			rctx.ITS.Spec.Template.Spec.Containers = []corev1.Container{{
				Name: "config-manager",
			}}
			status, err := syncPolicy(rctx)
			Expect(err).Should(Succeed())
			Expect(status.Status).Should(Equal(StatusFailed))
			Expect(status.Reason).Should(ContainSubstring("port"))
		})

		ginkgo.It("skip reconfigure when auto trigger", func() {
			rctx.ParametersDef.ReloadAction.AutoTrigger = &parametersv1alpha1.AutoTrigger{
				ProcessName: "mysqld",
			}
			_, err := syncPolicy(rctx)
			Expect(err).Should(Succeed())
			config := rctx.ClusterComponent.Configs[0]
			Expect(config.Reconfigure).ShouldNot(BeNil())
			Expect(*config.Reconfigure).Should(BeFalse())
			Expect(config.ReconfigureAction).Should(BeNil())
		})

		ginkgo.It("skip reconfigure when no reload action", func() {
			rctx.ParametersDef.ReloadAction = nil
			_, err := syncPolicy(rctx)
			Expect(err).Should(Succeed())
			config := rctx.ClusterComponent.Configs[0]
			Expect(config.Reconfigure).ShouldNot(BeNil())
			Expect(*config.Reconfigure).Should(BeFalse())
			Expect(config.ReconfigureAction).Should(BeNil())
		})

		ginkgo.It("skip reconfigure when restart merged", func() {
			rctx.ParametersDef.MergeReloadAndRestart = ptr.To(true)
			_, err := syncNRestartPolicy(rctx)
			Expect(err).Should(Succeed())
			config := rctx.ClusterComponent.Configs[0]
			Expect(config.Reconfigure).ShouldNot(BeNil())
			Expect(*config.Reconfigure).Should(BeFalse())
			Expect(config.ReconfigureAction).Should(BeNil())
			Expect(config.Restart).ShouldNot(BeNil())
			Expect(*config.Restart).Should(BeTrue())
		})

		ginkgo.It("set reconfigure when reload before restart enabled", func() {
			rctx.ParametersDef.MergeReloadAndRestart = ptr.To(false)
			_, err := syncNRestartPolicy(rctx)
			Expect(err).Should(Succeed())
			config := rctx.ClusterComponent.Configs[0]
			Expect(config.Reconfigure).ShouldNot(BeNil())
			Expect(*config.Reconfigure).Should(BeTrue())
			Expect(config.ReconfigureAction).ShouldNot(BeNil())
			Expect(config.ReconfigureAction.GRPC).ShouldNot(BeNil())
			Expect(config.ReconfigureAction.GRPC.Port).Should(Equal("9901"))
			Expect(config.ReconfigureAction.GRPC.Service).Should(Equal("proto.Reconfigure"))
			Expect(config.ReconfigureAction.GRPC.Method).Should(Equal("OnlineUpgradeParams"))
			Expect(config.ReconfigureAction.GRPC.Request).Should(HaveKeyWithValue("configSpec", cfgName))
			Expect(config.ReconfigureAction.GRPC.Request).Should(HaveKeyWithValue("configFile", cfgName))
			Expect(config.ReconfigureAction.GRPC.Request).Should(HaveKey("params"))
			Expect(config.Restart).ShouldNot(BeNil())
			Expect(*config.Restart).Should(BeTrue())
		})

		ginkgo.It("skip reconfigure but keep restart when no reloadable params remain", func() {
			rctx.ParametersDef.StaticParameters = []string{"a"}
			rctx.ParametersDef.ReloadStaticParamsBeforeRestart = ptr.To(false)
			_, err := syncNRestartPolicy(rctx)
			Expect(err).Should(Succeed())
			config := rctx.ClusterComponent.Configs[0]
			Expect(config.Reconfigure).ShouldNot(BeNil())
			Expect(*config.Reconfigure).Should(BeFalse())
			Expect(config.ReconfigureAction).Should(BeNil())
			Expect(config.Restart).ShouldNot(BeNil())
			Expect(*config.Restart).Should(BeTrue())
		})
	})
})

func TestShouldBuildLegacyReconfigureAction(t *testing.T) {
	newContext := func() Context {
		return Context{
			ParametersDef: &parametersv1alpha1.ParametersDefinitionSpec{
				MergeReloadAndRestart:           ptr.To(false),
				ReloadStaticParamsBeforeRestart: ptr.To(true),
				ReloadAction: &parametersv1alpha1.ReloadAction{
					ShellTrigger: &parametersv1alpha1.ShellTrigger{
						Command: []string{"bash", "-c", "reload"},
						Sync:    ptr.To(true),
					},
				},
			},
		}
	}

	tests := []struct {
		name    string
		ctx     Context
		params  map[string]string
		restart bool
		want    bool
	}{
		{name: "no updated params", ctx: newContext(), params: nil, want: false},
		{
			name: "auto trigger does not build grpc action",
			ctx: func() Context {
				ctx := newContext()
				ctx.ParametersDef.ReloadAction.AutoTrigger = &parametersv1alpha1.AutoTrigger{ProcessName: "mysqld"}
				ctx.ParametersDef.ReloadAction.ShellTrigger = nil
				return ctx
			}(),
			params: map[string]string{"a": "b"},
			want:   false,
		},
		{
			name: "reload action without shell trigger does not build grpc action",
			ctx: func() Context {
				ctx := newContext()
				ctx.ParametersDef.ReloadAction.ShellTrigger = nil
				return ctx
			}(),
			params: map[string]string{"a": "b"},
			want:   false,
		},
		{
			name: "sync shell trigger builds grpc action",
			ctx:  newContext(),
			params: map[string]string{
				"a": "b",
			},
			want: true,
		},
		{
			name: "merged restart skips grpc action",
			ctx: func() Context {
				ctx := newContext()
				ctx.ParametersDef.MergeReloadAndRestart = ptr.To(true)
				return ctx
			}(),
			params:  map[string]string{"a": "b"},
			restart: true,
			want:    false,
		},
		{
			name: "reload before restart keeps grpc action",
			ctx:  newContext(),
			params: map[string]string{
				"a": "b",
			},
			restart: true,
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldBuildLegacyReconfigureAction(tt.ctx, tt.params, tt.restart); got != tt.want {
				t.Fatalf("shouldBuildLegacyReconfigureAction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyChangesToClusterLegacyReconfigure(t *testing.T) {
	newContext := func() Context {
		return Context{
			ConfigTemplate: appsv1.ComponentFileTemplate{Name: "my.cnf"},
			ConfigDescription: &parametersv1alpha1.ComponentConfigDescription{
				Name: "my.cnf",
			},
			ConfigHash:       ptr.To("hash"),
			ClusterComponent: &appsv1.ClusterComponentSpec{},
			ParametersDef: &parametersv1alpha1.ParametersDefinitionSpec{
				MergeReloadAndRestart: ptr.To(false),
				ReloadAction: &parametersv1alpha1.ReloadAction{
					ShellTrigger: &parametersv1alpha1.ShellTrigger{
						Command: []string{"bash", "-c", "reload"},
						Sync:    ptr.To(true),
					},
				},
			},
			ITS: &workloads.InstanceSet{
				Spec: workloads.InstanceSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: nil,
						},
					},
				},
			},
		}
	}

	t.Run("set grpc action when sync reload is required", func(t *testing.T) {
		ctx := newContext()
		ctx.ITS.Spec.Template.Spec.Containers = []corev1.Container{{
			Name: "config-manager",
			Ports: []corev1.ContainerPort{{
				Name:          "config-manager",
				ContainerPort: 19901,
			}},
		}}
		config := &appsv1.ClusterComponentConfig{Name: ptr.To("my.cnf")}
		applyChangesToCluster(ctx, config, map[string]string{"a": "b"}, false)
		if config.Reconfigure == nil || !*config.Reconfigure {
			t.Fatalf("expected reconfigure intent to be true")
		}
		if config.ReconfigureAction == nil || config.ReconfigureAction.GRPC == nil {
			t.Fatalf("expected grpc reconfigure action to be set")
		}
		if config.ReconfigureAction.GRPC.Port != "19901" {
			t.Fatalf("expected grpc port 19901, got %s", config.ReconfigureAction.GRPC.Port)
		}
		params := map[string]string{}
		if err := json.Unmarshal([]byte(config.ReconfigureAction.GRPC.Request["params"]), &params); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if params["a"] != "b" {
			t.Fatalf("expected grpc params to include updated values, got %v", params)
		}
	})

	t.Run("clear grpc action when restart absorbs reload", func(t *testing.T) {
		ctx := newContext()
		ctx.ParametersDef.MergeReloadAndRestart = ptr.To(true)
		config := &appsv1.ClusterComponentConfig{Name: ptr.To("my.cnf")}
		applyChangesToCluster(ctx, config, map[string]string{"a": "b"}, true)
		if config.Reconfigure == nil || *config.Reconfigure {
			t.Fatalf("expected reconfigure intent to be false")
		}
		if config.ReconfigureAction != nil {
			t.Fatalf("expected grpc reconfigure action to be nil")
		}
		if config.Restart == nil || !*config.Restart {
			t.Fatalf("expected restart to remain set")
		}
	})
}

func TestApplyChangesToClusterTemplateReconfigure(t *testing.T) {
	configHash := "test-config-hash"
	ctx := Context{
		ConfigTemplate: appsv1.ComponentFileTemplate{
			Name: "my.cnf",
			Reconfigure: &appsv1.Action{
				Exec: &appsv1.ExecAction{Command: []string{"bash", "-c", "reload"}},
			},
		},
		ConfigHash: &configHash,
		ClusterComponent: &appsv1.ClusterComponentSpec{
			Replicas: 1,
			Configs: []appsv1.ClusterComponentConfig{{
				Name: ptr.To("my.cnf"),
			}},
		},
		ConfigDescription: &parametersv1alpha1.ComponentConfigDescription{
			Name: "my.cnf",
		},
		ParametersDef: &parametersv1alpha1.ParametersDefinitionSpec{
			DynamicParameters: []string{"binlog_expire_logs_seconds"},
		},
		Patch: &core.ConfigPatchInfo{
			IsModify: true,
			UpdateConfig: map[string][]byte{
				"my.cnf": []byte(`{"binlog_expire_logs_seconds":"432000"}`),
			},
		},
	}

	status, err := syncPolicy(ctx)
	if err != nil {
		t.Fatalf("syncPolicy returned error: %v", err)
	}
	if status.Status != StatusRetry {
		t.Fatalf("expected status %q, got %q", StatusRetry, status.Status)
	}
	config := ctx.ClusterComponent.Configs[0]
	if config.Restart == nil || *config.Restart {
		t.Fatalf("expected restart to be false, got %v", config.Restart)
	}
	if config.Reconfigure == nil || !*config.Reconfigure {
		t.Fatalf("expected reconfigure intent to be true")
	}
	if config.ReconfigureAction != nil {
		t.Fatalf("expected template reconfigure to use default action instead of override")
	}
	if ctx.ConfigTemplate.Reconfigure == nil || ctx.ConfigTemplate.Reconfigure.Exec == nil {
		t.Fatalf("expected template reconfigure action to be propagated")
	}
	if got := ctx.ConfigTemplate.Reconfigure.Exec.Command; len(got) != 3 || got[2] != "reload" {
		t.Fatalf("unexpected reconfigure exec command: %v", got)
	}
}

func TestApplyChangesToClusterClearsHistoricalRestartFlag(t *testing.T) {
	configHash := "test-config-hash"
	ctx := Context{
		ConfigTemplate: appsv1.ComponentFileTemplate{
			Name: "my.cnf",
			Reconfigure: &appsv1.Action{
				Exec: &appsv1.ExecAction{Command: []string{"bash", "-c", "reload"}},
			},
		},
		ConfigHash: &configHash,
		ClusterComponent: &appsv1.ClusterComponentSpec{
			Replicas: 1,
		},
		ConfigDescription: &parametersv1alpha1.ComponentConfigDescription{
			Name: "my.cnf",
		},
		ParametersDef: &parametersv1alpha1.ParametersDefinitionSpec{
			DynamicParameters: []string{"binlog_expire_logs_seconds"},
		},
		Patch: &core.ConfigPatchInfo{
			IsModify: true,
			UpdateConfig: map[string][]byte{
				"my.cnf": []byte(`{"binlog_expire_logs_seconds":"259200"}`),
			},
		},
	}

	config := &appsv1.ClusterComponentConfig{
		Name:    ptr.To("my.cnf"),
		Restart: ptr.To(true),
	}

	applyChangesToCluster(ctx, config, map[string]string{"binlog_expire_logs_seconds": "259200"}, false)

	if config.Restart == nil || *config.Restart {
		t.Fatalf("expected restart to be cleared to false, got %v", config.Restart)
	}
	if config.Reconfigure == nil || !*config.Reconfigure {
		t.Fatalf("expected reconfigure intent to stay true")
	}
	if config.ReconfigureAction != nil {
		t.Fatalf("expected no override action for template-level reconfigure")
	}
}

func TestApplyChangesToClusterTemplateReconfigureWithRestartSemantics(t *testing.T) {
	baseContext := func() Context {
		configHash := "test-config-hash"
		return Context{
			ConfigTemplate: appsv1.ComponentFileTemplate{
				Name: "my.cnf",
				Reconfigure: &appsv1.Action{
					Exec: &appsv1.ExecAction{Command: []string{"bash", "-c", "reload"}},
				},
			},
			ConfigHash: &configHash,
			ClusterComponent: &appsv1.ClusterComponentSpec{
				Replicas: 1,
				Configs: []appsv1.ClusterComponentConfig{{
					Name: ptr.To("my.cnf"),
				}},
			},
			ConfigDescription: &parametersv1alpha1.ComponentConfigDescription{
				Name: "my.cnf",
			},
		}
	}

	t.Run("restart-only with template action does not propagate reconfigure", func(t *testing.T) {
		ctx := baseContext()
		ctx.ParametersDef = &parametersv1alpha1.ParametersDefinitionSpec{}
		config := &ctx.ClusterComponent.Configs[0]

		applyChangesToCluster(ctx, config, nil, true)

		if config.Reconfigure == nil || *config.Reconfigure {
			t.Fatalf("expected reconfigure intent to be false for restart-only path")
		}
		if config.ReconfigureAction != nil {
			t.Fatalf("expected reconfigure action to be nil for restart-only path")
		}
		if config.Restart == nil || !*config.Restart {
			t.Fatalf("expected restart to remain true")
		}
	})

	t.Run("static reload-before-restart propagates template action", func(t *testing.T) {
		ctx := baseContext()
		ctx.ParametersDef = &parametersv1alpha1.ParametersDefinitionSpec{
			ReloadStaticParamsBeforeRestart: ptr.To(true),
		}
		config := &ctx.ClusterComponent.Configs[0]

		applyChangesToCluster(ctx, config, map[string]string{"performance_schema": "ON"}, true)

		if config.Reconfigure == nil || !*config.Reconfigure {
			t.Fatalf("expected reconfigure intent to be true for reload-before-restart")
		}
		if config.ReconfigureAction != nil {
			t.Fatalf("expected template reconfigure to use default action")
		}
		if config.Restart == nil || !*config.Restart {
			t.Fatalf("expected restart to remain true")
		}
	})

	t.Run("mixed split update propagates template action", func(t *testing.T) {
		ctx := baseContext()
		ctx.ParametersDef = &parametersv1alpha1.ParametersDefinitionSpec{
			DynamicParameters:     []string{"binlog_expire_logs_seconds"},
			MergeReloadAndRestart: ptr.To(false),
		}
		config := &ctx.ClusterComponent.Configs[0]

		applyChangesToCluster(ctx, config, map[string]string{"binlog_expire_logs_seconds": "432000"}, true)

		if config.Reconfigure == nil || !*config.Reconfigure {
			t.Fatalf("expected reconfigure intent to be true for split mixed update")
		}
		if config.ReconfigureAction != nil {
			t.Fatalf("expected template reconfigure to use default action")
		}
		if config.Restart == nil || !*config.Restart {
			t.Fatalf("expected restart to remain true")
		}
	})

	t.Run("mixed merged update clears template action", func(t *testing.T) {
		ctx := baseContext()
		ctx.ParametersDef = &parametersv1alpha1.ParametersDefinitionSpec{
			DynamicParameters:     []string{"binlog_expire_logs_seconds"},
			MergeReloadAndRestart: ptr.To(true),
		}
		config := &ctx.ClusterComponent.Configs[0]

		applyChangesToCluster(ctx, config, map[string]string{"binlog_expire_logs_seconds": "432000"}, true)

		if config.Reconfigure == nil || *config.Reconfigure {
			t.Fatalf("expected reconfigure intent to be false when restart absorbs mixed update")
		}
		if config.ReconfigureAction != nil {
			t.Fatalf("expected reconfigure action to be nil when restart absorbs mixed update")
		}
		if config.Restart == nil || !*config.Restart {
			t.Fatalf("expected restart to remain true")
		}
	})
}
