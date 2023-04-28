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
	"reflect"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

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
					`--volume-dir`, `/postgresql/conf`,
					`--volume-dir`, `/postgresql/conf2`,
				},
			}, {
				name: "buildCfgContainerParams",
				args: args{
					reloadOptions: &appsv1alpha1.ReloadOptions{
						ShellTrigger: &appsv1alpha1.ShellTrigger{
							Command: []string{"pwd"},
						}},
					volumeDirs: []corev1.VolumeMount{
						{
							MountPath: "/postgresql/conf",
							Name:      "pg_config",
						}},
				},
				expectedArgs: []string{
					`--volume-dir`, `/postgresql/conf`,
				},
			}, {
				name: "buildCfgContainerParams",
				args: args{
					reloadOptions: &appsv1alpha1.ReloadOptions{
						ShellTrigger: &appsv1alpha1.ShellTrigger{
							Command: []string{"pwd"},
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
						Volumes: []corev1.VolumeMount{
							{
								Name:      "pg_config",
								MountPath: "/postgresql/conf",
							},
						},
						ConfigSpecsBuildParams: []ConfigSpecMeta{
							{
								ConfigSpec: appsv1alpha1.ComponentConfigSpec{
									ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
										Name:       "pg_config",
										VolumeName: "pg_config",
									},
								},
								ScriptConfig: []appsv1alpha1.ScriptConfig{
									{
										Namespace:          "default",
										ScriptConfigMapRef: "script_cm",
									},
								},
							},
						},
					},
				},
				expectedArgs: []string{
					`--volume-dir`, `/postgresql/conf`,
				},
			}, {
				name: "buildCfgContainerParams",
				args: args{
					reloadOptions: &appsv1alpha1.ReloadOptions{
						TPLScriptTrigger: &appsv1alpha1.TPLScriptTrigger{
							ScriptConfig: appsv1alpha1.ScriptConfig{
								ScriptConfigMapRef: "script_cm",
								Namespace:          "default",
							},
							Sync: func() *bool { b := true; return &b }()}},
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
					`--operator-update-enable`,
				},
				wantErr: false,
			}, {
				name: "buildCfgContainerParamsWithOutSync",
				args: args{
					reloadOptions: &appsv1alpha1.ReloadOptions{
						TPLScriptTrigger: &appsv1alpha1.TPLScriptTrigger{
							ScriptConfig: appsv1alpha1.ScriptConfig{
								ScriptConfigMapRef: "script_cm",
								Namespace:          "default",
							},
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
					`--volume-dir`, `/postgresql/conf`,
				},
				wantErr: false,
			}}
			for _, tt := range tests {
				param := tt.args.param
				if param == nil {
					param = &CfgManagerBuildParams{}
				}
				for i := range param.ConfigSpecsBuildParams {
					buildParam := &param.ConfigSpecsBuildParams[i]
					buildParam.ReloadOptions = tt.args.reloadOptions
					buildParam.ReloadType = FromReloadTypeConfig(tt.args.reloadOptions)
				}
				err := BuildConfigManagerContainerParams(tt.args.cli, tt.args.ctx, param, tt.args.volumeDirs)
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

func TestCheckAndUpdateReloadYaml(t *testing.T) {
	customEqual := func(l, r map[string]string) bool {
		if len(l) != len(r) {
			return false
		}
		var err error
		for k, v := range l {
			var lv any
			var rv any
			err = yaml.Unmarshal([]byte(v), &lv)
			assert.Nil(t, err)
			err = yaml.Unmarshal([]byte(r[k]), &rv)
			assert.Nil(t, err)
			if !reflect.DeepEqual(lv, rv) {
				return false
			}
		}
		return true
	}

	type args struct {
		data            map[string]string
		reloadConfig    string
		formatterConfig *appsv1alpha1.FormatterConfig
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr bool
	}{{
		name: "testCheckAndUpdateReloadYaml",
		args: args{
			data: map[string]string{"reload.yaml": `
fileRegex: my.cnf
scripts: reload.tpl
`},
			reloadConfig: "reload.yaml",
			formatterConfig: &appsv1alpha1.FormatterConfig{
				Format: appsv1alpha1.Ini,
			},
		},
		wantErr: false,
		want: map[string]string{"reload.yaml": `
scripts: reload.tpl
fileRegex: my.cnf
formatterConfig:
  format: ini
`,
		},
	}, {
		name: "testCheckAndUpdateReloadYaml",
		args: args{
			data:            map[string]string{},
			reloadConfig:    "reload.yaml",
			formatterConfig: &appsv1alpha1.FormatterConfig{Format: appsv1alpha1.Ini},
		},
		wantErr: true,
		want:    map[string]string{},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := checkAndUpdateReloadYaml(tt.args.data, tt.args.reloadConfig, *tt.args.formatterConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkAndUpdateReloadYaml() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !customEqual(got, tt.want) {
				t.Errorf("checkAndUpdateReloadYaml() got = %v, want %v", got, tt.want)
			}
		})
	}
}
