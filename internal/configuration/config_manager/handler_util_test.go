/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package configmanager

import (
	"context"
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
					Exec: "pg_ctl reload",
				},
			},
		},
		want: true,
	}, {
		name: "reload_test",
		args: args{
			reload: &appsv1alpha1.ReloadOptions{
				TPLScriptTrigger: &appsv1alpha1.TPLScriptTrigger{
					Namespace:          "default",
					ScriptConfigMapRef: "cm",
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
		It("Should success with no error", func() {
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
							Exec: "",
						}},
				},
				wantErr: true,
			}, {
				name: "shellTest",
				args: args{
					reloadOptions: &appsv1alpha1.ReloadOptions{
						ShellTrigger: &appsv1alpha1.ShellTrigger{
							Exec: "go",
						}},
				},
				wantErr: false,
			}, {
				name: "TPLScriptTest",
				args: args{
					reloadOptions: &appsv1alpha1.ReloadOptions{
						TPLScriptTrigger: &appsv1alpha1.TPLScriptTrigger{
							ScriptConfigMapRef: "test",
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
							ScriptConfigMapRef: "test",
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
