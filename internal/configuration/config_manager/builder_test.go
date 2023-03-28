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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	testutil "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("ConfigManager Test", func() {

	var mockK8sCli *testutil.K8sClientMockHelper

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		mockK8sCli = testutil.NewK8sMockClient()
		mockK8sCli.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructGetResult(&corev1.ConfigMap{
			Data: map[string]string{
				"reload.tpl": "{{}}",
			},
		})), testutil.WithAnyTimes())
	})

	AfterEach(func() {
		DeferCleanup(mockK8sCli.Finish)
	})

	Context("TestBuildConfigManagerContainerArgs", func() {
		It("Should success with no error", func() {
			type args struct {
				reloadOptions *appsv1alpha1.ReloadOptions
				volumeDirs    []corev1.VolumeMount
				cli           client.Client
				ctx           context.Context
				param         *CfgManagerBuildParams
			}
			tests := []struct {
				name         string
				args         args
				expectedArgs []string
				wantErr      bool
			}{{
				name: "buildCfgContainerParams",
				args: args{
					reloadOptions: &appsv1alpha1.ReloadOptions{
						UnixSignalTrigger: &appsv1alpha1.UnixSignalTrigger{
							ProcessName: "postgres",
							Signal:      appsv1alpha1.SIGHUP,
						}},
					volumeDirs: []corev1.VolumeMount{
						{
							MountPath: "/postgresql/conf",
							Name:      "pg_config",
						},
						{
							MountPath: "/postgresql/conf2",
							Name:      "pg_config",
						}},
				},
				expectedArgs: []string{
					`--notify-type`, `signal`,
					`--process`, `postgres`,
					`--signal`, `SIGHUP`,
					`--volume-dir`, `/postgresql/conf`,
					`--volume-dir`, `/postgresql/conf2`,
				},
			}, {
				name: "buildCfgContainerParams",
				args: args{
					reloadOptions: &appsv1alpha1.ReloadOptions{
						ShellTrigger: &appsv1alpha1.ShellTrigger{
							Exec: "pwd",
						}},
					volumeDirs: []corev1.VolumeMount{
						{
							MountPath: "/postgresql/conf",
							Name:      "pg_config",
						}},
				},
				expectedArgs: []string{
					`--notify-type`, `exec`,
					`--volume-dir`, `/postgresql/conf`,
					`---command`, `pwd`,
				},
			}, {
				name: "buildCfgContainerParams",
				args: args{
					reloadOptions: &appsv1alpha1.ReloadOptions{
						TPLScriptTrigger: &appsv1alpha1.TPLScriptTrigger{
							ScriptConfigMapRef: "script_cm",
							Namespace:          "default",
						}},
					volumeDirs: []corev1.VolumeMount{
						{
							MountPath: "/postgresql/conf",
							Name:      "pg_config",
						}},
					cli: mockK8sCli.Client(),
					ctx: context.TODO(),
					param: &CfgManagerBuildParams{
						Cluster: &appsv1alpha1.Cluster{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "abcd",
								Namespace: "default",
							},
						},
					},
				},
				expectedArgs: []string{
					`--notify-type`, `tpl`,
					`--tpl-config`, `/opt/config/reload/reload.yaml`,
					`--volume-dir`, `/postgresql/conf`,
				},
				wantErr: false,
			}}
			for _, tt := range tests {
				param := tt.args.param
				if param == nil {
					param = &CfgManagerBuildParams{}
				}
				err := BuildConfigManagerContainerArgs(tt.args.reloadOptions, tt.args.volumeDirs, tt.args.cli, tt.args.ctx, param)
				Expect(err != nil).Should(BeEquivalentTo(tt.wantErr))
				if !tt.wantErr {
					for _, arg := range tt.expectedArgs {
						Expect(param.Args).Should(ContainElement(arg))
					}
				}
			}
		})
	})

})
