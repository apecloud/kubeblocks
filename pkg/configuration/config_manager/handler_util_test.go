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

// import (
// 	"context"
// 	"strings"
// 	"testing"
//
// 	. "github.com/onsi/ginkgo/v2"
// 	. "github.com/onsi/gomega"
//
// 	"github.com/stretchr/testify/assert"
// 	corev1 "k8s.io/api/core/v1"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"sigs.k8s.io/controller-runtime/pkg/client"
//
// 	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
// 	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
// 	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
// 	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
// 	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
// )
//
// func TestIsSupportReload(t *testing.T) {
// 	type args struct {
// 		reload *appsv1beta1.ReloadAction
// 	}
// 	tests := []struct {
// 		name string
// 		args args
// 		want bool
// 	}{{
// 		name: "reload_test_with_nil_reload_options",
// 		args: args{
// 			reload: nil,
// 		},
// 		want: false,
// 	}, {
// 		name: "reload_test_with_empty_reload_options",
// 		args: args{
// 			reload: &appsv1beta1.ReloadAction{},
// 		},
// 		want: false,
// 	}, {
// 		name: "reload_test_with_unix_signal",
// 		args: args{
// 			reload: &appsv1beta1.ReloadAction{
// 				UnixSignalTrigger: &appsv1beta1.UnixSignalTrigger{
// 					ProcessName: "test",
// 					Signal:      appsv1beta1.SIGHUP,
// 				},
// 			},
// 		},
// 		want: true,
// 	}, {
// 		name: "reload_test_with_shell",
// 		args: args{
// 			reload: &appsv1beta1.ReloadAction{
// 				ShellTrigger: &appsv1beta1.ShellTrigger{
// 					Command: strings.Fields("pg_ctl reload"),
// 				},
// 			},
// 		},
// 		want: true,
// 	}, {
// 		name: "reload_test_with_tpl_script",
// 		args: args{
// 			reload: &appsv1beta1.ReloadAction{
// 				TPLScriptTrigger: &appsv1beta1.TPLScriptTrigger{
// 					ScriptConfig: appsv1beta1.ScriptConfig{
// 						ScriptConfigMapRef: "cm",
// 						Namespace:          "default",
// 					},
// 				},
// 			},
// 		},
// 		want: true,
// 	}, {
// 		name: "auto_trigger_reload_test_with_process_name",
// 		args: args{
// 			reload: &appsv1beta1.ReloadAction{
// 				AutoTrigger: &appsv1beta1.AutoTrigger{
// 					ProcessName: "test",
// 				},
// 			},
// 		},
// 		want: true,
// 	}, {
// 		name: "auto_trigger_reload_test",
// 		args: args{
// 			reload: &appsv1beta1.ReloadAction{
// 				AutoTrigger: &appsv1beta1.AutoTrigger{},
// 			},
// 		},
// 		want: true,
// 	}}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			if got := IsSupportReload(tt.args.reload); got != tt.want {
// 				t.Errorf("IsSupportReload() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }
//
// var _ = Describe("Handler Util Test", func() {
//
// 	var mockK8sCli *testutil.K8sClientMockHelper
//
// 	BeforeEach(func() {
// 		// Add any setup steps that needs to be executed before each test
// 		mockK8sCli = testutil.NewK8sMockClient()
// 	})
//
// 	AfterEach(func() {
// 		DeferCleanup(mockK8sCli.Finish)
// 	})
//
// 	mockConfigConstraint := func(ccName string, reloadOptions *appsv1beta1.ReloadAction) *appsv1beta1.ConfigConstraint {
// 		return &appsv1beta1.ConfigConstraint{
// 			ObjectMeta: metav1.ObjectMeta{
// 				Name: ccName,
// 			},
// 			Spec: appsv1beta1.ConfigConstraintSpec{
// 				ReloadAction: reloadOptions,
// 				FileFormatConfig: &appsv1beta1.FileFormatConfig{
// 					Format: appsv1beta1.Properties,
// 				},
// 			}}
// 	}
//
// 	mockConfigSpec := func(ccName string) appsv1.ComponentTemplateSpec {
// 		return appsv1.ComponentTemplateSpec{
// 			Name:        "test",
// 			TemplateRef: "config_template",
// 			Namespace:   "default",
// 			VolumeName:  "for_test",
// 		}
// 	}
//
// 	Context("TestValidateReloadOptions", func() {
// 		It("Should succeed with no error", func() {
// 			mockK8sCli.MockGetMethod(
// 				testutil.WithFailed(cfgcore.MakeError("failed to get resource."), testutil.WithTimes(1)),
// 				testutil.WithSucceed(testutil.WithTimes(1)),
// 			)
//
// 			type args struct {
// 				reloadAction *appsv1beta1.ReloadAction
// 				cli          client.Client
// 				ctx          context.Context
// 			}
// 			tests := []struct {
// 				name    string
// 				args    args
// 				wantErr bool
// 			}{{
// 				name: "unixSignalTest",
// 				args: args{
// 					reloadAction: &appsv1beta1.ReloadAction{
// 						UnixSignalTrigger: &appsv1beta1.UnixSignalTrigger{
// 							Signal: appsv1beta1.SIGHUP,
// 						}},
// 				},
// 				wantErr: false,
// 			}, {
// 				name: "unixSignalTest",
// 				args: args{
// 					reloadAction: &appsv1beta1.ReloadAction{
// 						UnixSignalTrigger: &appsv1beta1.UnixSignalTrigger{
// 							Signal: "SIGNOEXIST",
// 						}},
// 				},
// 				wantErr: true,
// 			}, {
// 				name: "shellTest",
// 				args: args{
// 					reloadAction: &appsv1beta1.ReloadAction{
// 						ShellTrigger: &appsv1beta1.ShellTrigger{
// 							Command: nil,
// 						}},
// 				},
// 				wantErr: true,
// 			}, {
// 				name: "shellTest",
// 				args: args{
// 					reloadAction: &appsv1beta1.ReloadAction{
// 						ShellTrigger: &appsv1beta1.ShellTrigger{
// 							Command: strings.Fields("go"),
// 						}},
// 				},
// 				wantErr: false,
// 			}, {
// 				name: "TPLScriptTest",
// 				args: args{
// 					reloadAction: &appsv1beta1.ReloadAction{
// 						TPLScriptTrigger: &appsv1beta1.TPLScriptTrigger{
// 							ScriptConfig: appsv1beta1.ScriptConfig{
// 								ScriptConfigMapRef: "test",
// 							},
// 						}},
// 					cli: mockK8sCli.Client(),
// 					ctx: context.TODO(),
// 				},
// 				wantErr: true,
// 			}, {
// 				name: "TPLScriptTest",
// 				args: args{
// 					reloadAction: &appsv1beta1.ReloadAction{
// 						TPLScriptTrigger: &appsv1beta1.TPLScriptTrigger{
// 							ScriptConfig: appsv1beta1.ScriptConfig{
// 								ScriptConfigMapRef: "test",
// 							},
// 						}},
// 					cli: mockK8sCli.Client(),
// 					ctx: context.TODO(),
// 				},
// 				wantErr: false,
// 			}, {
// 				name: "autoTriggerTest",
// 				args: args{
// 					reloadAction: &appsv1beta1.ReloadAction{
// 						AutoTrigger: &appsv1beta1.AutoTrigger{
// 							ProcessName: "test",
// 						}},
// 				},
// 				wantErr: false,
// 			}, {
// 				name: "autoTriggerTest",
// 				args: args{
// 					reloadAction: &appsv1beta1.ReloadAction{
// 						AutoTrigger: &appsv1beta1.AutoTrigger{}},
// 				},
// 				wantErr: false,
// 			}}
// 			for _, tt := range tests {
// 				By(tt.name)
// 				err := ValidateReloadOptions(tt.args.reloadAction, tt.args.cli, tt.args.ctx)
// 				Expect(err != nil).Should(BeEquivalentTo(tt.wantErr))
// 			}
// 		})
// 	})
//
// 	Context("TestGetSupportReloadConfigSpecs", func() {
// 		It("not support reload", func() {
// 			configSpecs, err := GetSupportReloadConfigSpecs([]appsv1.ComponentTemplateSpec{{
// 				Name: "test",
// 			}}, nil, nil)
// 			Expect(err).Should(Succeed())
// 			Expect(len(configSpecs)).Should(BeEquivalentTo(0))
// 		})
//
// 		It("not ConfigConstraint ", func() {
// 			configSpecs, err := GetSupportReloadConfigSpecs([]appsv1.ComponentTemplateSpec{{
// 				Name:        "test",
// 				TemplateRef: "config_template",
// 				Namespace:   "default",
// 			}}, nil, nil)
// 			Expect(err).Should(Succeed())
// 			Expect(len(configSpecs)).Should(BeEquivalentTo(0))
// 		})
//
// 		It("not support reload", func() {
// 			ccName := "config_constraint"
// 			mockK8sCli.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]client.Object{
// 				mockConfigConstraint(ccName, nil),
// 			}), testutil.WithTimes(1)))
//
// 			configSpecs, err := GetSupportReloadConfigSpecs(
// 				[]appsv1.ComponentTemplateSpec{mockConfigSpec(ccName)},
// 				mockK8sCli.Client(), ctx)
//
// 			Expect(err).Should(Succeed())
// 			Expect(len(configSpecs)).Should(BeEquivalentTo(0))
// 		})
//
// 		It("normal test", func() {
// 			ccName := "config_constraint"
// 			cc := mockConfigConstraint(ccName, &appsv1beta1.ReloadAction{
// 				UnixSignalTrigger: &appsv1beta1.UnixSignalTrigger{
// 					ProcessName: "test",
// 					Signal:      appsv1beta1.SIGHUP,
// 				},
// 			})
// 			mockK8sCli.MockGetMethod(testutil.WithGetReturned(
// 				testutil.WithConstructSimpleGetResult([]client.Object{cc}),
// 				testutil.WithTimes(1)))
//
// 			configSpecs, err := GetSupportReloadConfigSpecs(
// 				[]appsv1.ComponentConfigSpec{mockConfigSpec(ccName)},
// 				mockK8sCli.Client(), ctx)
//
// 			Expect(err).Should(Succeed())
// 			Expect(len(configSpecs)).Should(BeEquivalentTo(1))
// 			Expect(configSpecs[0].ConfigSpec).Should(BeEquivalentTo(mockConfigSpec(ccName)))
// 			Expect(configSpecs[0].ReloadType).Should(BeEquivalentTo(appsv1beta1.UnixSignalType))
// 			Expect(configSpecs[0].FormatterConfig).Should(BeEquivalentTo(*cc.Spec.FileFormatConfig))
// 		})
//
// 		It("auto trigger test", func() {
// 			ccName := "auto_trigger_config_constraint"
// 			cc := mockConfigConstraint(ccName, &appsv1beta1.ReloadAction{
// 				AutoTrigger: &appsv1beta1.AutoTrigger{
// 					ProcessName: "test",
// 				},
// 			})
// 			mockK8sCli.MockGetMethod(testutil.WithGetReturned(
// 				testutil.WithConstructSimpleGetResult([]client.Object{cc}),
// 				testutil.WithTimes(1)))
//
// 			configSpecs, err := GetSupportReloadConfigSpecs(
// 				[]appsv1.ComponentConfigSpec{mockConfigSpec(ccName)},
// 				mockK8sCli.Client(), ctx)
//
// 			Expect(err).Should(Succeed())
// 			Expect(len(configSpecs)).Should(BeEquivalentTo(0))
// 		})
// 	})
//
// 	Context("TestFromReloadTypeConfig", func() {
// 		It("TestSignalTrigger", func() {
// 			Expect(appsv1beta1.UnixSignalType).Should(BeEquivalentTo(FromReloadTypeConfig(&appsv1beta1.ReloadAction{
// 				UnixSignalTrigger: &appsv1beta1.UnixSignalTrigger{
// 					ProcessName: "test",
// 					Signal:      appsv1beta1.SIGHUP,
// 				}})))
// 		})
//
// 		It("TestAutoTrigger", func() {
// 			Expect(appsv1beta1.AutoType).Should(BeEquivalentTo(FromReloadTypeConfig(&appsv1beta1.ReloadAction{
// 				AutoTrigger: &appsv1beta1.AutoTrigger{
// 					ProcessName: "test",
// 				}})))
// 		})
//
// 		It("TestShellTrigger", func() {
// 			Expect(appsv1beta1.ShellType).Should(BeEquivalentTo(FromReloadTypeConfig(&appsv1beta1.ReloadAction{
// 				ShellTrigger: &appsv1beta1.ShellTrigger{
// 					Command: []string{"/bin/true"},
// 				}})))
// 		})
//
// 		It("TestTplScriptsTrigger", func() {
// 			Expect(appsv1beta1.TPLScriptType).Should(BeEquivalentTo(FromReloadTypeConfig(&appsv1beta1.ReloadAction{
// 				TPLScriptTrigger: &appsv1beta1.TPLScriptTrigger{
// 					ScriptConfig: appsv1beta1.ScriptConfig{
// 						ScriptConfigMapRef: "test",
// 						Namespace:          "default",
// 					},
// 				}})))
// 		})
//
// 		It("TestInvalidTrigger", func() {
// 			Expect("").Should(BeEquivalentTo(FromReloadTypeConfig(&appsv1beta1.ReloadAction{})))
// 		})
// 	})
//
// 	Context("TestValidateReloadOptions", func() {
// 		It("TestSignalTrigger", func() {
// 			Expect(ValidateReloadOptions(&appsv1beta1.ReloadAction{
// 				UnixSignalTrigger: &appsv1beta1.UnixSignalTrigger{
// 					ProcessName: "test",
// 					Signal:      appsv1beta1.SIGHUP,
// 				}}, nil, nil),
// 			).Should(Succeed())
// 		})
//
// 		It("TestSignalTrigger", func() {
// 			Expect(ValidateReloadOptions(&appsv1beta1.ReloadAction{
// 				AutoTrigger: &appsv1beta1.AutoTrigger{
// 					ProcessName: "test",
// 				}}, nil, nil),
// 			).Should(Succeed())
// 		})
//
// 		It("TestShellTrigger", func() {
// 			Expect(ValidateReloadOptions(&appsv1beta1.ReloadAction{
// 				ShellTrigger: &appsv1beta1.ShellTrigger{
// 					Command: []string{"/bin/true"},
// 				}}, nil, nil),
// 			).Should(Succeed())
// 		})
//
// 		It("TestTplScriptsTrigger", func() {
// 			ns := "default"
// 			testName1 := "test1"
// 			testName2 := "not_test1"
// 			mockK8sCli.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]client.Object{
// 				&corev1.ConfigMap{
// 					ObjectMeta: metav1.ObjectMeta{
// 						Name:      testName1,
// 						Namespace: ns,
// 					},
// 				},
// 			}), testutil.WithTimes(2)))
//
// 			By("Test valid")
// 			Expect(ValidateReloadOptions(&appsv1beta1.ReloadAction{
// 				TPLScriptTrigger: &appsv1beta1.TPLScriptTrigger{
// 					ScriptConfig: appsv1beta1.ScriptConfig{
// 						ScriptConfigMapRef: testName1,
// 						Namespace:          ns,
// 					},
// 				}}, mockK8sCli.Client(), ctx),
// 			).Should(Succeed())
//
// 			By("Test invalid")
// 			Expect(ValidateReloadOptions(&appsv1beta1.ReloadAction{
// 				TPLScriptTrigger: &appsv1beta1.TPLScriptTrigger{
// 					ScriptConfig: appsv1beta1.ScriptConfig{
// 						ScriptConfigMapRef: testName2,
// 						Namespace:          ns,
// 					},
// 				}}, mockK8sCli.Client(), ctx),
// 			).ShouldNot(Succeed())
// 		})
//
// 		It("TestInvalidTrigger", func() {
// 			Expect(ValidateReloadOptions(&appsv1beta1.ReloadAction{}, nil, nil)).ShouldNot(Succeed())
// 		})
// 	})
// })
//
// func TestFilterSubPathVolumeMount(t *testing.T) {
// 	createConfigMeta := func(volumeName string, reloadType appsv1beta1.DynamicReloadType, reloadAction *appsv1beta1.ReloadAction) ConfigSpecMeta {
// 		return ConfigSpecMeta{ConfigSpecInfo: ConfigSpecInfo{
// 			ReloadAction: reloadAction,
// 			ReloadType:   reloadType,
// 			ConfigSpec: appsv1.ComponentConfigSpec{
// 				ComponentTemplateSpec: appsv1.ComponentTemplateSpec{
// 					VolumeName: volumeName,
// 				}}}}
// 	}
//
// 	type args struct {
// 		metas   []ConfigSpecMeta
// 		volumes []corev1.VolumeMount
// 	}
// 	tests := []struct {
// 		name string
// 		args args
// 		want []ConfigSpecMeta
// 	}{{
// 		name: "test1",
// 		args: args{
// 			metas: []ConfigSpecMeta{
// 				createConfigMeta("test1", appsv1beta1.UnixSignalType, &appsv1beta1.ReloadAction{
// 					UnixSignalTrigger: &appsv1beta1.UnixSignalTrigger{},
// 				}),
// 				createConfigMeta("test2", appsv1beta1.ShellType, &appsv1beta1.ReloadAction{
// 					ShellTrigger: &appsv1beta1.ShellTrigger{
// 						Sync: cfgutil.ToPointer(true),
// 					},
// 				}),
// 				createConfigMeta("test3", appsv1beta1.TPLScriptType, &appsv1beta1.ReloadAction{
// 					TPLScriptTrigger: &appsv1beta1.TPLScriptTrigger{
// 						Sync: cfgutil.ToPointer(true),
// 					},
// 				}),
// 			},
// 			volumes: []corev1.VolumeMount{
// 				{Name: "test1", SubPath: "test1"},
// 				{Name: "test2", SubPath: "test2"},
// 				{Name: "test3", SubPath: "test3"},
// 			},
// 		},
// 		want: []ConfigSpecMeta{
// 			createConfigMeta("test2", appsv1beta1.ShellType, &appsv1beta1.ReloadAction{
// 				ShellTrigger: &appsv1beta1.ShellTrigger{
// 					Sync: cfgutil.ToPointer(true),
// 				},
// 			}),
// 			createConfigMeta("test3", appsv1beta1.TPLScriptType, &appsv1beta1.ReloadAction{
// 				TPLScriptTrigger: &appsv1beta1.TPLScriptTrigger{
// 					Sync: cfgutil.ToPointer(true),
// 				},
// 			}),
// 		},
// 	}, {
// 		name: "test2",
// 		args: args{
// 			metas: []ConfigSpecMeta{
// 				createConfigMeta("test1", appsv1beta1.UnixSignalType, &appsv1beta1.ReloadAction{
// 					UnixSignalTrigger: &appsv1beta1.UnixSignalTrigger{},
// 				}),
// 				createConfigMeta("test2", appsv1beta1.ShellType, &appsv1beta1.ReloadAction{
// 					ShellTrigger: &appsv1beta1.ShellTrigger{},
// 				}),
// 				createConfigMeta("test3", appsv1beta1.TPLScriptType, &appsv1beta1.ReloadAction{
// 					TPLScriptTrigger: &appsv1beta1.TPLScriptTrigger{},
// 				}),
// 			},
// 			volumes: []corev1.VolumeMount{
// 				{Name: "test1"},
// 				{Name: "test2"},
// 				{Name: "test3"},
// 			},
// 		},
// 		want: []ConfigSpecMeta{
// 			createConfigMeta("test1", appsv1beta1.UnixSignalType, &appsv1beta1.ReloadAction{
// 				UnixSignalTrigger: &appsv1beta1.UnixSignalTrigger{},
// 			}),
// 			createConfigMeta("test2", appsv1beta1.ShellType, &appsv1beta1.ReloadAction{
// 				ShellTrigger: &appsv1beta1.ShellTrigger{},
// 			}),
// 			createConfigMeta("test3", appsv1beta1.TPLScriptType, &appsv1beta1.ReloadAction{
// 				TPLScriptTrigger: &appsv1beta1.TPLScriptTrigger{},
// 			}),
// 		},
// 	}, {
// 		name: "test3",
// 		args: args{
// 			metas: []ConfigSpecMeta{
// 				createConfigMeta("test1", appsv1beta1.UnixSignalType, &appsv1beta1.ReloadAction{
// 					UnixSignalTrigger: &appsv1beta1.UnixSignalTrigger{},
// 				}),
// 				createConfigMeta("test2", appsv1beta1.ShellType, &appsv1beta1.ReloadAction{
// 					ShellTrigger: &appsv1beta1.ShellTrigger{},
// 				}),
// 				createConfigMeta("test3", appsv1beta1.TPLScriptType, &appsv1beta1.ReloadAction{
// 					TPLScriptTrigger: &appsv1beta1.TPLScriptTrigger{},
// 				}),
// 			},
// 			volumes: []corev1.VolumeMount{},
// 		},
// 		want: []ConfigSpecMeta{
// 			createConfigMeta("test1", appsv1beta1.UnixSignalType, &appsv1beta1.ReloadAction{
// 				UnixSignalTrigger: &appsv1beta1.UnixSignalTrigger{},
// 			}),
// 			createConfigMeta("test2", appsv1beta1.ShellType, &appsv1beta1.ReloadAction{
// 				ShellTrigger: &appsv1beta1.ShellTrigger{},
// 			}),
// 			createConfigMeta("test3", appsv1beta1.TPLScriptType, &appsv1beta1.ReloadAction{
// 				TPLScriptTrigger: &appsv1beta1.TPLScriptTrigger{},
// 			}),
// 		},
// 	}, {
// 		name: "test4",
// 		args: args{
// 			metas: []ConfigSpecMeta{
// 				createConfigMeta("test1", appsv1beta1.UnixSignalType, &appsv1beta1.ReloadAction{
// 					UnixSignalTrigger: &appsv1beta1.UnixSignalTrigger{},
// 				}),
// 				createConfigMeta("test2", appsv1beta1.ShellType, &appsv1beta1.ReloadAction{
// 					ShellTrigger: &appsv1beta1.ShellTrigger{
// 						Sync: cfgutil.ToPointer(false),
// 					},
// 				}),
// 				createConfigMeta("test3", appsv1beta1.TPLScriptType, &appsv1beta1.ReloadAction{
// 					TPLScriptTrigger: &appsv1beta1.TPLScriptTrigger{
// 						Sync: cfgutil.ToPointer(false),
// 					},
// 				}),
// 			},
// 			volumes: []corev1.VolumeMount{
// 				{Name: "test1", SubPath: "test1"},
// 				{Name: "test2", SubPath: "test2"},
// 				{Name: "test3", SubPath: "test3"},
// 			},
// 		},
// 		want: nil,
// 	}}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			assert.Equalf(t, tt.want, FilterSupportReloadActionConfigSpecs(tt.args.metas, tt.args.volumes), "FilterSupportReloadActionConfigSpecs(%v, %v)", tt.args.metas, tt.args.volumes)
// 		})
// 	}
// }
