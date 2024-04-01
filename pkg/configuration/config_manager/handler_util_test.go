/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package configmanager

import (
	"context"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/core"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

func TestIsSupportReload(t *testing.T) {
	type args struct {
		reload *v1.DynamicReloadAction
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		name: "reload_test_with_nil_reload_options",
		args: args{
			reload: nil,
		},
		want: false,
	}, {
		name: "reload_test_with_empty_reload_options",
		args: args{
			reload: &v1.DynamicReloadAction{},
		},
		want: false,
	}, {
		name: "reload_test_with_unix_signal",
		args: args{
			reload: &v1.DynamicReloadAction{
				UnixSignalTrigger: &v1.UnixSignalTrigger{
					ProcessName: "test",
					Signal:      v1.SIGHUP,
				},
			},
		},
		want: true,
	}, {
		name: "reload_test_with_shell",
		args: args{
			reload: &v1.DynamicReloadAction{
				ShellTrigger: &v1.ShellTrigger{
					Command: strings.Fields("pg_ctl reload"),
				},
			},
		},
		want: true,
	}, {
		name: "reload_test_with_tpl_script",
		args: args{
			reload: &v1.DynamicReloadAction{
				TPLScriptTrigger: &v1.TPLScriptTrigger{
					ScriptConfig: v1.ScriptConfig{
						ScriptConfigMapRef: "cm",
						Namespace:          "default",
					},
				},
			},
		},
		want: true,
	}, {
		name: "auto_trigger_reload_test_with_process_name",
		args: args{
			reload: &v1.DynamicReloadAction{
				AutoTrigger: &v1.AutoTrigger{
					ProcessName: "test",
				},
			},
		},
		want: true,
	}, {
		name: "auto_trigger_reload_test",
		args: args{
			reload: &v1.DynamicReloadAction{
				AutoTrigger: &v1.AutoTrigger{},
			},
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSupportReload(tt.args.reload); got != tt.want {
				t.Errorf("IsSupportReload() = %v, want %v", got, tt.want)
			}
		})
	}
}

var _ = Describe("Handler Util Test", func() {

	var mockK8sCli *testutil.K8sClientMockHelper

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		mockK8sCli = testutil.NewK8sMockClient()
	})

	AfterEach(func() {
		DeferCleanup(mockK8sCli.Finish)
	})

	mockConfigConstraint := func(ccName string, reloadOptions *v1.DynamicReloadAction) *v1.ConfigConstraint {
		return &v1.ConfigConstraint{
			ObjectMeta: metav1.ObjectMeta{
				Name: ccName,
			},
			Spec: v1.ConfigConstraintSpec{
				DynamicReloadAction: reloadOptions,
				FormatterConfig: &v1.FormatterConfig{
					Format: v1.Properties,
				},
			}}
	}

	mockConfigSpec := func(ccName string) appsv1alpha1.ComponentConfigSpec {
		return appsv1alpha1.ComponentConfigSpec{
			ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
				Name:        "test",
				TemplateRef: "config_template",
				Namespace:   "default",
				VolumeName:  "for_test",
			},
			ConfigConstraintRef: ccName,
		}
	}

	Context("TestValidateReloadOptions", func() {
		It("Should succeed with no error", func() {
			mockK8sCli.MockGetMethod(
				testutil.WithFailed(cfgutil.MakeError("failed to get resource."), testutil.WithTimes(1)),
				testutil.WithSucceed(testutil.WithTimes(1)),
			)

			type args struct {
				reloadOptions *v1.DynamicReloadAction
				cli           client.Client
				ctx           context.Context
			}
			tests := []struct {
				name    string
				args    args
				wantErr bool
			}{{
				name: "unixSignalTest",
				args: args{
					reloadOptions: &v1.DynamicReloadAction{
						UnixSignalTrigger: &v1.UnixSignalTrigger{
							Signal: v1.SIGHUP,
						}},
				},
				wantErr: false,
			}, {
				name: "unixSignalTest",
				args: args{
					reloadOptions: &v1.DynamicReloadAction{
						UnixSignalTrigger: &v1.UnixSignalTrigger{
							Signal: "SIGNOEXIST",
						}},
				},
				wantErr: true,
			}, {
				name: "shellTest",
				args: args{
					reloadOptions: &v1.DynamicReloadAction{
						ShellTrigger: &v1.ShellTrigger{
							Command: nil,
						}},
				},
				wantErr: true,
			}, {
				name: "shellTest",
				args: args{
					reloadOptions: &v1.DynamicReloadAction{
						ShellTrigger: &v1.ShellTrigger{
							Command: strings.Fields("go"),
						}},
				},
				wantErr: false,
			}, {
				name: "TPLScriptTest",
				args: args{
					reloadOptions: &v1.DynamicReloadAction{
						TPLScriptTrigger: &v1.TPLScriptTrigger{
							ScriptConfig: v1.ScriptConfig{
								ScriptConfigMapRef: "test",
							},
						}},
					cli: mockK8sCli.Client(),
					ctx: context.TODO(),
				},
				wantErr: true,
			}, {
				name: "TPLScriptTest",
				args: args{
					reloadOptions: &v1.DynamicReloadAction{
						TPLScriptTrigger: &v1.TPLScriptTrigger{
							ScriptConfig: v1.ScriptConfig{
								ScriptConfigMapRef: "test",
							},
						}},
					cli: mockK8sCli.Client(),
					ctx: context.TODO(),
				},
				wantErr: false,
			}, {
				name: "autoTriggerTest",
				args: args{
					reloadOptions: &v1.DynamicReloadAction{
						AutoTrigger: &v1.AutoTrigger{
							ProcessName: "test",
						}},
				},
				wantErr: false,
			}, {
				name: "autoTriggerTest",
				args: args{
					reloadOptions: &v1.DynamicReloadAction{
						AutoTrigger: &v1.AutoTrigger{}},
				},
				wantErr: false,
			}}
			for _, tt := range tests {
				By(tt.name)
				err := ValidateReloadOptions(tt.args.reloadOptions, tt.args.cli, tt.args.ctx)
				Expect(err != nil).Should(BeEquivalentTo(tt.wantErr))
			}
		})
	})

	Context("TestGetSupportReloadConfigSpecs", func() {
		It("not support reload", func() {
			configSpecs, err := GetSupportReloadConfigSpecs([]appsv1alpha1.ComponentConfigSpec{{
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name: "test",
				}}}, nil, nil)
			Expect(err).Should(Succeed())
			Expect(len(configSpecs)).Should(BeEquivalentTo(0))
		})

		It("not ConfigConstraint ", func() {
			configSpecs, err := GetSupportReloadConfigSpecs([]appsv1alpha1.ComponentConfigSpec{{
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					Name:        "test",
					TemplateRef: "config_template",
					Namespace:   "default",
				}}}, nil, nil)
			Expect(err).Should(Succeed())
			Expect(len(configSpecs)).Should(BeEquivalentTo(0))
		})

		It("not support reload", func() {
			ccName := "config_constraint"
			mockK8sCli.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]client.Object{
				mockConfigConstraint(ccName, nil),
			}), testutil.WithTimes(1)))

			configSpecs, err := GetSupportReloadConfigSpecs(
				[]appsv1alpha1.ComponentConfigSpec{mockConfigSpec(ccName)},
				mockK8sCli.Client(), ctx)

			Expect(err).Should(Succeed())
			Expect(len(configSpecs)).Should(BeEquivalentTo(0))
		})

		It("normal test", func() {
			ccName := "config_constraint"
			cc := mockConfigConstraint(ccName, &v1.DynamicReloadAction{
				UnixSignalTrigger: &v1.UnixSignalTrigger{
					ProcessName: "test",
					Signal:      v1.SIGHUP,
				},
			})
			mockK8sCli.MockGetMethod(testutil.WithGetReturned(
				testutil.WithConstructSimpleGetResult([]client.Object{cc}),
				testutil.WithTimes(1)))

			configSpecs, err := GetSupportReloadConfigSpecs(
				[]appsv1alpha1.ComponentConfigSpec{mockConfigSpec(ccName)},
				mockK8sCli.Client(), ctx)

			Expect(err).Should(Succeed())
			Expect(len(configSpecs)).Should(BeEquivalentTo(1))
			Expect(configSpecs[0].ConfigSpec).Should(BeEquivalentTo(mockConfigSpec(ccName)))
			Expect(configSpecs[0].ReloadType).Should(BeEquivalentTo(v1.UnixSignalType))
			Expect(configSpecs[0].FormatterConfig).Should(BeEquivalentTo(*cc.Spec.FormatterConfig))
		})

		It("auto trigger test", func() {
			ccName := "auto_trigger_config_constraint"
			cc := mockConfigConstraint(ccName, &v1.DynamicReloadAction{
				AutoTrigger: &v1.AutoTrigger{
					ProcessName: "test",
				},
			})
			mockK8sCli.MockGetMethod(testutil.WithGetReturned(
				testutil.WithConstructSimpleGetResult([]client.Object{cc}),
				testutil.WithTimes(1)))

			configSpecs, err := GetSupportReloadConfigSpecs(
				[]appsv1alpha1.ComponentConfigSpec{mockConfigSpec(ccName)},
				mockK8sCli.Client(), ctx)

			Expect(err).Should(Succeed())
			Expect(len(configSpecs)).Should(BeEquivalentTo(0))
		})
	})

	Context("TestFromReloadTypeConfig", func() {
		It("TestSignalTrigger", func() {
			Expect(v1.UnixSignalType).Should(BeEquivalentTo(FromReloadTypeConfig(&v1.DynamicReloadAction{
				UnixSignalTrigger: &v1.UnixSignalTrigger{
					ProcessName: "test",
					Signal:      v1.SIGHUP,
				}})))
		})

		It("TestAutoTrigger", func() {
			Expect(v1.AutoType).Should(BeEquivalentTo(FromReloadTypeConfig(&v1.DynamicReloadAction{
				AutoTrigger: &v1.AutoTrigger{
					ProcessName: "test",
				}})))
		})

		It("TestShellTrigger", func() {
			Expect(v1.ShellType).Should(BeEquivalentTo(FromReloadTypeConfig(&v1.DynamicReloadAction{
				ShellTrigger: &v1.ShellTrigger{
					Command: []string{"/bin/true"},
				}})))
		})

		It("TestTplScriptsTrigger", func() {
			Expect(v1.TPLScriptType).Should(BeEquivalentTo(FromReloadTypeConfig(&v1.DynamicReloadAction{
				TPLScriptTrigger: &v1.TPLScriptTrigger{
					ScriptConfig: v1.ScriptConfig{
						ScriptConfigMapRef: "test",
						Namespace:          "default",
					},
				}})))
		})

		It("TestInvalidTrigger", func() {
			Expect("").Should(BeEquivalentTo(FromReloadTypeConfig(&v1.DynamicReloadAction{})))
		})
	})

	Context("TestValidateReloadOptions", func() {
		It("TestSignalTrigger", func() {
			Expect(ValidateReloadOptions(&v1.DynamicReloadAction{
				UnixSignalTrigger: &v1.UnixSignalTrigger{
					ProcessName: "test",
					Signal:      v1.SIGHUP,
				}}, nil, nil),
			).Should(Succeed())
		})

		It("TestSignalTrigger", func() {
			Expect(ValidateReloadOptions(&v1.DynamicReloadAction{
				AutoTrigger: &v1.AutoTrigger{
					ProcessName: "test",
				}}, nil, nil),
			).Should(Succeed())
		})

		It("TestShellTrigger", func() {
			Expect(ValidateReloadOptions(&v1.DynamicReloadAction{
				ShellTrigger: &v1.ShellTrigger{
					Command: []string{"/bin/true"},
				}}, nil, nil),
			).Should(Succeed())
		})

		It("TestTplScriptsTrigger", func() {
			ns := "default"
			testName1 := "test1"
			testName2 := "not_test1"
			mockK8sCli.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testName1,
						Namespace: ns,
					},
				},
			}), testutil.WithTimes(2)))

			By("Test valid")
			Expect(ValidateReloadOptions(&v1.DynamicReloadAction{
				TPLScriptTrigger: &v1.TPLScriptTrigger{
					ScriptConfig: v1.ScriptConfig{
						ScriptConfigMapRef: testName1,
						Namespace:          ns,
					},
				}}, mockK8sCli.Client(), ctx),
			).Should(Succeed())

			By("Test invalid")
			Expect(ValidateReloadOptions(&v1.DynamicReloadAction{
				TPLScriptTrigger: &v1.TPLScriptTrigger{
					ScriptConfig: v1.ScriptConfig{
						ScriptConfigMapRef: testName2,
						Namespace:          ns,
					},
				}}, mockK8sCli.Client(), ctx),
			).ShouldNot(Succeed())
		})

		It("TestInvalidTrigger", func() {
			Expect(ValidateReloadOptions(&v1.DynamicReloadAction{}, nil, nil)).ShouldNot(Succeed())
		})
	})
})

func TestFilterSubPathVolumeMount(t *testing.T) {
	createConfigMeta := func(volumeName string, reloadType v1.CfgReloadType) ConfigSpecMeta {
		return ConfigSpecMeta{ConfigSpecInfo: ConfigSpecInfo{
			ReloadType: reloadType,
			ConfigSpec: appsv1alpha1.ComponentConfigSpec{
				ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
					VolumeName: volumeName,
				}}}}
	}

	type args struct {
		metas   []ConfigSpecMeta
		volumes []corev1.VolumeMount
	}
	tests := []struct {
		name string
		args args
		want []ConfigSpecMeta
	}{{
		name: "test1",
		args: args{
			metas: []ConfigSpecMeta{
				createConfigMeta("test1", v1.UnixSignalType),
				createConfigMeta("test2", v1.ShellType),
				createConfigMeta("test3", v1.TPLScriptType),
			},
			volumes: []corev1.VolumeMount{
				{Name: "test1", SubPath: "test1"},
				{Name: "test2", SubPath: "test2"},
				{Name: "test3", SubPath: "test3"},
			},
		},
		want: []ConfigSpecMeta{
			createConfigMeta("test3", v1.TPLScriptType),
		},
	}, {
		name: "test2",
		args: args{
			metas: []ConfigSpecMeta{
				createConfigMeta("test1", v1.UnixSignalType),
				createConfigMeta("test2", v1.ShellType),
				createConfigMeta("test3", v1.TPLScriptType),
			},
			volumes: []corev1.VolumeMount{
				{Name: "test1"},
				{Name: "test2"},
				{Name: "test3"},
			},
		},
		want: []ConfigSpecMeta{
			createConfigMeta("test1", v1.UnixSignalType),
			createConfigMeta("test2", v1.ShellType),
			createConfigMeta("test3", v1.TPLScriptType),
		},
	}, {
		name: "test3",
		args: args{
			metas: []ConfigSpecMeta{
				createConfigMeta("test1", v1.UnixSignalType),
				createConfigMeta("test2", v1.ShellType),
				createConfigMeta("test3", v1.TPLScriptType),
			},
			volumes: []corev1.VolumeMount{},
		},
		want: []ConfigSpecMeta{
			createConfigMeta("test1", v1.UnixSignalType),
			createConfigMeta("test2", v1.ShellType),
			createConfigMeta("test3", v1.TPLScriptType),
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, FilterSubPathVolumeMount(tt.args.metas, tt.args.volumes), "FilterSubPathVolumeMount(%v, %v)", tt.args.metas, tt.args.volumes)
		})
	}
}
