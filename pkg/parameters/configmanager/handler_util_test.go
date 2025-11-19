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

package configmanager

import (
	"context"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/pkg/parameters/core"
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

	Context("TestValidateReloadOptions", func() {
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
