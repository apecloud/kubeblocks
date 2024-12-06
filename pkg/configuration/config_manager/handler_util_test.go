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

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

func TestIsSupportReload(t *testing.T) {
	type args struct {
		reload *parametersv1alpha1.ReloadAction
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
			reload: &parametersv1alpha1.ReloadAction{},
		},
		want: false,
	}, {
		name: "reload_test_with_unix_signal",
		args: args{
			reload: &parametersv1alpha1.ReloadAction{
				UnixSignalTrigger: &parametersv1alpha1.UnixSignalTrigger{
					ProcessName: "test",
					Signal:      parametersv1alpha1.SIGHUP,
				},
			},
		},
		want: true,
	}, {
		name: "reload_test_with_shell",
		args: args{
			reload: &parametersv1alpha1.ReloadAction{
				ShellTrigger: &parametersv1alpha1.ShellTrigger{
					Command: strings.Fields("pg_ctl reload"),
				},
			},
		},
		want: true,
	}, {
		name: "reload_test_with_tpl_script",
		args: args{
			reload: &parametersv1alpha1.ReloadAction{
				TPLScriptTrigger: &parametersv1alpha1.TPLScriptTrigger{
					ScriptConfig: parametersv1alpha1.ScriptConfig{
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
			reload: &parametersv1alpha1.ReloadAction{
				AutoTrigger: &parametersv1alpha1.AutoTrigger{
					ProcessName: "test",
				},
			},
		},
		want: true,
	}, {
		name: "auto_trigger_reload_test",
		args: args{
			reload: &parametersv1alpha1.ReloadAction{
				AutoTrigger: &parametersv1alpha1.AutoTrigger{},
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

	mockParametersDef := func(ccName string, reloadOptions *parametersv1alpha1.ReloadAction) *parametersv1alpha1.ParametersDefinition {
		return &parametersv1alpha1.ParametersDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: ccName,
			},
			Spec: parametersv1alpha1.ParametersDefinitionSpec{
				FileName:     "test",
				ReloadAction: reloadOptions,
			}}
	}
	mockConfigDescription := func(tplName string, format parametersv1alpha1.CfgFileFormat) []parametersv1alpha1.ComponentConfigDescription {
		return []parametersv1alpha1.ComponentConfigDescription{
			{
				Name:         "test",
				TemplateName: tplName,
				FileFormatConfig: &parametersv1alpha1.FileFormatConfig{
					Format: format,
				},
			},
		}
	}

	mockConfigSpec := func(tplName string) appsv1.ComponentTemplateSpec {
		return appsv1.ComponentTemplateSpec{
			Name:        tplName,
			TemplateRef: "config_template",
			Namespace:   "default",
			VolumeName:  "for_test",
		}
	}

	Context("TestValidateReloadOptions", func() {
		It("Should succeed with no error", func() {
			mockK8sCli.MockGetMethod(
				testutil.WithFailed(cfgcore.MakeError("failed to get resource."), testutil.WithTimes(1)),
				testutil.WithSucceed(testutil.WithTimes(1)),
			)

			type args struct {
				reloadAction *parametersv1alpha1.ReloadAction
				cli          client.Client
				ctx          context.Context
			}
			tests := []struct {
				name    string
				args    args
				wantErr bool
			}{{
				name: "unixSignalTest",
				args: args{
					reloadAction: &parametersv1alpha1.ReloadAction{
						UnixSignalTrigger: &parametersv1alpha1.UnixSignalTrigger{
							Signal: parametersv1alpha1.SIGHUP,
						}},
				},
				wantErr: false,
			}, {
				name: "unixSignalTest",
				args: args{
					reloadAction: &parametersv1alpha1.ReloadAction{
						UnixSignalTrigger: &parametersv1alpha1.UnixSignalTrigger{
							Signal: "SIGNOEXIST",
						}},
				},
				wantErr: true,
			}, {
				name: "shellTest",
				args: args{
					reloadAction: &parametersv1alpha1.ReloadAction{
						ShellTrigger: &parametersv1alpha1.ShellTrigger{
							Command: nil,
						}},
				},
				wantErr: true,
			}, {
				name: "shellTest",
				args: args{
					reloadAction: &parametersv1alpha1.ReloadAction{
						ShellTrigger: &parametersv1alpha1.ShellTrigger{
							Command: strings.Fields("go"),
						}},
				},
				wantErr: false,
			}, {
				name: "TPLScriptTest",
				args: args{
					reloadAction: &parametersv1alpha1.ReloadAction{
						TPLScriptTrigger: &parametersv1alpha1.TPLScriptTrigger{
							ScriptConfig: parametersv1alpha1.ScriptConfig{
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
					reloadAction: &parametersv1alpha1.ReloadAction{
						TPLScriptTrigger: &parametersv1alpha1.TPLScriptTrigger{
							ScriptConfig: parametersv1alpha1.ScriptConfig{
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
					reloadAction: &parametersv1alpha1.ReloadAction{
						AutoTrigger: &parametersv1alpha1.AutoTrigger{
							ProcessName: "test",
						}},
				},
				wantErr: false,
			}, {
				name: "autoTriggerTest",
				args: args{
					reloadAction: &parametersv1alpha1.ReloadAction{
						AutoTrigger: &parametersv1alpha1.AutoTrigger{}},
				},
				wantErr: false,
			}}
			for _, tt := range tests {
				By(tt.name)
				err := ValidateReloadOptions(tt.args.reloadAction, tt.args.cli, tt.args.ctx)
				Expect(err != nil).Should(BeEquivalentTo(tt.wantErr))
			}
		})
	})

	Context("TestGetSupportReloadConfigSpecs", func() {
		It("not support reload", func() {
			configSpecs, err := GetSupportReloadConfigSpecs([]appsv1.ComponentTemplateSpec{{
				Name: "test",
			}}, nil, nil)
			Expect(err).Should(Succeed())
			Expect(len(configSpecs)).Should(BeEquivalentTo(0))
		})

		It("not ComponentConfigDescription", func() {
			configSpecs, err := GetSupportReloadConfigSpecs([]appsv1.ComponentTemplateSpec{{
				Name:        "test",
				TemplateRef: "config_template",
				Namespace:   "default",
			}}, nil, []*parametersv1alpha1.ParametersDefinition{mockParametersDef("test", nil)})
			Expect(err).Should(Succeed())
			Expect(len(configSpecs)).Should(BeEquivalentTo(0))
		})

		It("not support reload for paramsDef", func() {
			ccName := "config_constraint"
			configtpl := mockConfigSpec(ccName)
			configSpecs, err := GetSupportReloadConfigSpecs(
				[]appsv1.ComponentTemplateSpec{configtpl},
				mockConfigDescription(configtpl.Name, parametersv1alpha1.Ini),
				nil,
			)

			Expect(err).Should(Succeed())
			Expect(len(configSpecs)).Should(BeEquivalentTo(0))
		})

		It("normal test", func() {
			ccName := "config_constraint"
			pd := mockParametersDef(ccName, &parametersv1alpha1.ReloadAction{
				UnixSignalTrigger: &parametersv1alpha1.UnixSignalTrigger{
					ProcessName: "test",
					Signal:      parametersv1alpha1.SIGHUP,
				},
			})

			cd := mockConfigDescription(ccName, parametersv1alpha1.Ini)
			configSpecs, err := GetSupportReloadConfigSpecs(
				[]appsv1.ComponentTemplateSpec{mockConfigSpec(ccName)},
				cd,
				[]*parametersv1alpha1.ParametersDefinition{pd},
			)

			Expect(err).Should(Succeed())
			Expect(len(configSpecs)).Should(BeEquivalentTo(1))
			Expect(configSpecs[0].ConfigSpec).Should(BeEquivalentTo(mockConfigSpec(ccName)))
			Expect(configSpecs[0].ReloadType).Should(BeEquivalentTo(parametersv1alpha1.UnixSignalType))
			Expect(&configSpecs[0].FormatterConfig).Should(BeEquivalentTo(cd[0].FileFormatConfig))
		})
	})

	Context("TestFromReloadTypeConfig", func() {
		It("TestSignalTrigger", func() {
			Expect(parametersv1alpha1.UnixSignalType).Should(BeEquivalentTo(FromReloadTypeConfig(&parametersv1alpha1.ReloadAction{
				UnixSignalTrigger: &parametersv1alpha1.UnixSignalTrigger{
					ProcessName: "test",
					Signal:      parametersv1alpha1.SIGHUP,
				}})))
		})

		It("TestAutoTrigger", func() {
			Expect(parametersv1alpha1.AutoType).Should(BeEquivalentTo(FromReloadTypeConfig(&parametersv1alpha1.ReloadAction{
				AutoTrigger: &parametersv1alpha1.AutoTrigger{
					ProcessName: "test",
				}})))
		})

		It("TestShellTrigger", func() {
			Expect(parametersv1alpha1.ShellType).Should(BeEquivalentTo(FromReloadTypeConfig(&parametersv1alpha1.ReloadAction{
				ShellTrigger: &parametersv1alpha1.ShellTrigger{
					Command: []string{"/bin/true"},
				}})))
		})

		It("TestTplScriptsTrigger", func() {
			Expect(parametersv1alpha1.TPLScriptType).Should(BeEquivalentTo(FromReloadTypeConfig(&parametersv1alpha1.ReloadAction{
				TPLScriptTrigger: &parametersv1alpha1.TPLScriptTrigger{
					ScriptConfig: parametersv1alpha1.ScriptConfig{
						ScriptConfigMapRef: "test",
						Namespace:          "default",
					},
				}})))
		})

		It("TestInvalidTrigger", func() {
			Expect("").Should(BeEquivalentTo(FromReloadTypeConfig(&parametersv1alpha1.ReloadAction{})))
		})
	})

	Context("TestValidateReloadOptions", func() {
		It("TestSignalTrigger", func() {
			Expect(ValidateReloadOptions(&parametersv1alpha1.ReloadAction{
				UnixSignalTrigger: &parametersv1alpha1.UnixSignalTrigger{
					ProcessName: "test",
					Signal:      parametersv1alpha1.SIGHUP,
				}}, nil, nil),
			).Should(Succeed())
		})

		It("TestSignalTrigger", func() {
			Expect(ValidateReloadOptions(&parametersv1alpha1.ReloadAction{
				AutoTrigger: &parametersv1alpha1.AutoTrigger{
					ProcessName: "test",
				}}, nil, nil),
			).Should(Succeed())
		})

		It("TestShellTrigger", func() {
			Expect(ValidateReloadOptions(&parametersv1alpha1.ReloadAction{
				ShellTrigger: &parametersv1alpha1.ShellTrigger{
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
			Expect(ValidateReloadOptions(&parametersv1alpha1.ReloadAction{
				TPLScriptTrigger: &parametersv1alpha1.TPLScriptTrigger{
					ScriptConfig: parametersv1alpha1.ScriptConfig{
						ScriptConfigMapRef: testName1,
						Namespace:          ns,
					},
				}}, mockK8sCli.Client(), ctx),
			).Should(Succeed())

			By("Test invalid")
			Expect(ValidateReloadOptions(&parametersv1alpha1.ReloadAction{
				TPLScriptTrigger: &parametersv1alpha1.TPLScriptTrigger{
					ScriptConfig: parametersv1alpha1.ScriptConfig{
						ScriptConfigMapRef: testName2,
						Namespace:          ns,
					},
				}}, mockK8sCli.Client(), ctx),
			).ShouldNot(Succeed())
		})

		It("TestInvalidTrigger", func() {
			Expect(ValidateReloadOptions(&parametersv1alpha1.ReloadAction{}, nil, nil)).ShouldNot(Succeed())
		})
	})
})

func TestFilterSubPathVolumeMount(t *testing.T) {
	createConfigMeta := func(volumeName string, reloadType parametersv1alpha1.DynamicReloadType, reloadAction *parametersv1alpha1.ReloadAction) ConfigSpecMeta {
		return ConfigSpecMeta{ConfigSpecInfo: ConfigSpecInfo{
			ReloadAction: reloadAction,
			ReloadType:   reloadType,
			ConfigSpec: appsv1.ComponentTemplateSpec{
				VolumeName: volumeName,
			}}}
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
				createConfigMeta("test1", parametersv1alpha1.UnixSignalType, &parametersv1alpha1.ReloadAction{
					UnixSignalTrigger: &parametersv1alpha1.UnixSignalTrigger{},
				}),
				createConfigMeta("test2", parametersv1alpha1.ShellType, &parametersv1alpha1.ReloadAction{
					ShellTrigger: &parametersv1alpha1.ShellTrigger{
						Sync: cfgutil.ToPointer(true),
					},
				}),
				createConfigMeta("test3", parametersv1alpha1.TPLScriptType, &parametersv1alpha1.ReloadAction{
					TPLScriptTrigger: &parametersv1alpha1.TPLScriptTrigger{
						Sync: cfgutil.ToPointer(true),
					},
				}),
			},
			volumes: []corev1.VolumeMount{
				{Name: "test1", SubPath: "test1"},
				{Name: "test2", SubPath: "test2"},
				{Name: "test3", SubPath: "test3"},
			},
		},
		want: []ConfigSpecMeta{
			createConfigMeta("test2", parametersv1alpha1.ShellType, &parametersv1alpha1.ReloadAction{
				ShellTrigger: &parametersv1alpha1.ShellTrigger{
					Sync: cfgutil.ToPointer(true),
				},
			}),
			createConfigMeta("test3", parametersv1alpha1.TPLScriptType, &parametersv1alpha1.ReloadAction{
				TPLScriptTrigger: &parametersv1alpha1.TPLScriptTrigger{
					Sync: cfgutil.ToPointer(true),
				},
			}),
		},
	}, {
		name: "test2",
		args: args{
			metas: []ConfigSpecMeta{
				createConfigMeta("test1", parametersv1alpha1.UnixSignalType, &parametersv1alpha1.ReloadAction{
					UnixSignalTrigger: &parametersv1alpha1.UnixSignalTrigger{},
				}),
				createConfigMeta("test2", parametersv1alpha1.ShellType, &parametersv1alpha1.ReloadAction{
					ShellTrigger: &parametersv1alpha1.ShellTrigger{},
				}),
				createConfigMeta("test3", parametersv1alpha1.TPLScriptType, &parametersv1alpha1.ReloadAction{
					TPLScriptTrigger: &parametersv1alpha1.TPLScriptTrigger{},
				}),
			},
			volumes: []corev1.VolumeMount{
				{Name: "test1"},
				{Name: "test2"},
				{Name: "test3"},
			},
		},
		want: []ConfigSpecMeta{
			createConfigMeta("test1", parametersv1alpha1.UnixSignalType, &parametersv1alpha1.ReloadAction{
				UnixSignalTrigger: &parametersv1alpha1.UnixSignalTrigger{},
			}),
			createConfigMeta("test2", parametersv1alpha1.ShellType, &parametersv1alpha1.ReloadAction{
				ShellTrigger: &parametersv1alpha1.ShellTrigger{},
			}),
			createConfigMeta("test3", parametersv1alpha1.TPLScriptType, &parametersv1alpha1.ReloadAction{
				TPLScriptTrigger: &parametersv1alpha1.TPLScriptTrigger{},
			}),
		},
	}, {
		name: "test3",
		args: args{
			metas: []ConfigSpecMeta{
				createConfigMeta("test1", parametersv1alpha1.UnixSignalType, &parametersv1alpha1.ReloadAction{
					UnixSignalTrigger: &parametersv1alpha1.UnixSignalTrigger{},
				}),
				createConfigMeta("test2", parametersv1alpha1.ShellType, &parametersv1alpha1.ReloadAction{
					ShellTrigger: &parametersv1alpha1.ShellTrigger{},
				}),
				createConfigMeta("test3", parametersv1alpha1.TPLScriptType, &parametersv1alpha1.ReloadAction{
					TPLScriptTrigger: &parametersv1alpha1.TPLScriptTrigger{},
				}),
			},
			volumes: []corev1.VolumeMount{},
		},
		want: []ConfigSpecMeta{
			createConfigMeta("test1", parametersv1alpha1.UnixSignalType, &parametersv1alpha1.ReloadAction{
				UnixSignalTrigger: &parametersv1alpha1.UnixSignalTrigger{},
			}),
			createConfigMeta("test2", parametersv1alpha1.ShellType, &parametersv1alpha1.ReloadAction{
				ShellTrigger: &parametersv1alpha1.ShellTrigger{},
			}),
			createConfigMeta("test3", parametersv1alpha1.TPLScriptType, &parametersv1alpha1.ReloadAction{
				TPLScriptTrigger: &parametersv1alpha1.TPLScriptTrigger{},
			}),
		},
	}, {
		name: "test4",
		args: args{
			metas: []ConfigSpecMeta{
				createConfigMeta("test1", parametersv1alpha1.UnixSignalType, &parametersv1alpha1.ReloadAction{
					UnixSignalTrigger: &parametersv1alpha1.UnixSignalTrigger{},
				}),
				createConfigMeta("test2", parametersv1alpha1.ShellType, &parametersv1alpha1.ReloadAction{
					ShellTrigger: &parametersv1alpha1.ShellTrigger{
						Sync: cfgutil.ToPointer(false),
					},
				}),
				createConfigMeta("test3", parametersv1alpha1.TPLScriptType, &parametersv1alpha1.ReloadAction{
					TPLScriptTrigger: &parametersv1alpha1.TPLScriptTrigger{
						Sync: cfgutil.ToPointer(false),
					},
				}),
			},
			volumes: []corev1.VolumeMount{
				{Name: "test1", SubPath: "test1"},
				{Name: "test2", SubPath: "test2"},
				{Name: "test3", SubPath: "test3"},
			},
		},
		want: nil,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, FilterSupportReloadActionConfigSpecs(tt.args.metas, tt.args.volumes), "FilterSupportReloadActionConfigSpecs(%v, %v)", tt.args.metas, tt.args.volumes)
		})
	}
}
