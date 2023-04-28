/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgutil "github.com/apecloud/kubeblocks/internal/configuration"
	testutil "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

func TestIsSupportReload(t *testing.T) {
	type args struct {
		reload *appsv1alpha1.ReloadOptions
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		name: "reload_test",
		args: args{
			reload: nil,
		},
		want: false,
	}, {
		name: "reload_test",
		args: args{
			reload: &appsv1alpha1.ReloadOptions{},
		},
		want: false,
	}, {
		name: "reload_test",
		args: args{
			reload: &appsv1alpha1.ReloadOptions{
				UnixSignalTrigger: &appsv1alpha1.UnixSignalTrigger{
					ProcessName: "test",
					Signal:      appsv1alpha1.SIGHUP,
				},
			},
		},
		want: true,
	}, {
		name: "reload_test",
		args: args{
			reload: &appsv1alpha1.ReloadOptions{
				ShellTrigger: &appsv1alpha1.ShellTrigger{
					Command: strings.Fields("pg_ctl reload"),
				},
			},
		},
		want: true,
	}, {
		name: "reload_test",
		args: args{
			reload: &appsv1alpha1.ReloadOptions{
				TPLScriptTrigger: &appsv1alpha1.TPLScriptTrigger{
					ScriptConfig: appsv1alpha1.ScriptConfig{
						ScriptConfigMapRef: "cm",
						Namespace:          "default",
					},
				},
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

		mockK8sCli.MockGetMethod(
			testutil.WithFailed(cfgutil.MakeError("failed to get resource."), testutil.WithTimes(1)),
			testutil.WithSucceed(testutil.WithTimes(1)),
		)
	})

	AfterEach(func() {
		DeferCleanup(mockK8sCli.Finish)
	})

	Context("TestValidateReloadOptions", func() {
		It("Should succeed with no error", func() {
			type args struct {
				reloadOptions *appsv1alpha1.ReloadOptions
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
					reloadOptions: &appsv1alpha1.ReloadOptions{
						UnixSignalTrigger: &appsv1alpha1.UnixSignalTrigger{
							Signal: appsv1alpha1.SIGHUP,
						}},
				},
				wantErr: false,
			}, {
				name: "unixSignalTest",
				args: args{
					reloadOptions: &appsv1alpha1.ReloadOptions{
						UnixSignalTrigger: &appsv1alpha1.UnixSignalTrigger{
							Signal: "SIGNOEXIST",
						}},
				},
				wantErr: true,
			}, {
				name: "shellTest",
				args: args{
					reloadOptions: &appsv1alpha1.ReloadOptions{
						ShellTrigger: &appsv1alpha1.ShellTrigger{
							Command: nil,
						}},
				},
				wantErr: true,
			}, {
				name: "shellTest",
				args: args{
					reloadOptions: &appsv1alpha1.ReloadOptions{
						ShellTrigger: &appsv1alpha1.ShellTrigger{
							Command: strings.Fields("go"),
						}},
				},
				wantErr: false,
			}, {
				name: "TPLScriptTest",
				args: args{
					reloadOptions: &appsv1alpha1.ReloadOptions{
						TPLScriptTrigger: &appsv1alpha1.TPLScriptTrigger{
							ScriptConfig: appsv1alpha1.ScriptConfig{
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
					reloadOptions: &appsv1alpha1.ReloadOptions{
						TPLScriptTrigger: &appsv1alpha1.TPLScriptTrigger{
							ScriptConfig: appsv1alpha1.ScriptConfig{
								ScriptConfigMapRef: "test",
							},
						}},
					cli: mockK8sCli.Client(),
					ctx: context.TODO(),
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
})
