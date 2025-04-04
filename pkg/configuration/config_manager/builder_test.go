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
	"reflect"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("Config Builder Test", func() {

	const (
		scriptsName = "script_cm"
		scriptsNS   = "default"

		lazyRenderedTemplateName = "lazy-rendered-template"
	)

	var mockK8sCli *testutil.K8sClientMockHelper

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		mockK8sCli = testutil.NewK8sMockClient()
	})

	AfterEach(func() {
		DeferCleanup(mockK8sCli.Finish)
	})

	syncFn := func(sync bool) *bool { r := sync; return &r }

	newVolumeMounts := func() []corev1.VolumeMount {
		return []corev1.VolumeMount{
			{
				MountPath: "/postgresql/conf",
				Name:      "pg_config",
			}}
	}
	newVolumeMounts2 := func() []corev1.VolumeMount {
		return []corev1.VolumeMount{
			{
				MountPath: "/postgresql/conf",
				Name:      "pg_config",
			},
			{
				MountPath: "/postgresql/conf2",
				Name:      "pg_config",
			}}
	}
	newReloadOptions := func(t parametersv1alpha1.DynamicReloadType, sync *bool) *parametersv1alpha1.ReloadAction {
		signalHandle := &parametersv1alpha1.UnixSignalTrigger{
			ProcessName: "postgres",
			Signal:      parametersv1alpha1.SIGHUP,
		}
		shellHandle := &parametersv1alpha1.ShellTrigger{
			Command: []string{"pwd"},
		}
		scriptHandle := &parametersv1alpha1.TPLScriptTrigger{
			Sync: sync,
			ScriptConfig: parametersv1alpha1.ScriptConfig{
				ScriptConfigMapRef: "reload-script",
				Namespace:          scriptsNS,
			},
		}
		autoHandle := &parametersv1alpha1.AutoTrigger{
			ProcessName: "postgres",
		}

		switch t {
		default:
			return nil
		case parametersv1alpha1.UnixSignalType:
			return &parametersv1alpha1.ReloadAction{
				UnixSignalTrigger: signalHandle}
		case parametersv1alpha1.ShellType:
			return &parametersv1alpha1.ReloadAction{
				ShellTrigger: shellHandle}
		case parametersv1alpha1.TPLScriptType:
			return &parametersv1alpha1.ReloadAction{
				TPLScriptTrigger: scriptHandle}
		case parametersv1alpha1.AutoType:
			return &parametersv1alpha1.ReloadAction{
				AutoTrigger: autoHandle}
		}
	}
	newConfigSpecMeta := func() []ConfigSpecMeta {
		return []ConfigSpecMeta{
			{
				ConfigSpecInfo: ConfigSpecInfo{
					ConfigSpec: appsv1.ComponentFileTemplate{
						Name:       "pg_config",
						VolumeName: "pg_config",
					},
				},
			},
		}
	}

	newCMBuildParams := func(hasScripts bool) *CfgManagerBuildParams {
		param := &CfgManagerBuildParams{
			Cluster: &appsv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "abcd",
					Namespace: "default",
				},
			},
			ComponentName:          "test",
			Volumes:                newVolumeMounts(),
			ConfigSpecsBuildParams: newConfigSpecMeta(),
			DownwardAPIVolumes:     make([]corev1.VolumeMount, 0),
		}
		if hasScripts {
			param.ConfigSpecsBuildParams[0].ScriptConfig = []parametersv1alpha1.ScriptConfig{
				{
					Namespace:          scriptsNS,
					ScriptConfigMapRef: scriptsName,
				},
			}
		}
		return param
	}

	mockTplScriptCM := func() {
		mockK8sCli.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]client.Object{
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "reload-script",
					Namespace: scriptsNS,
				},
				Data: map[string]string{
					"reload.yaml": `
scripts: reload.tpl
fileRegex: my.cnf
formatterConfig:
  format: ini
`,
				}},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      lazyRenderedTemplateName,
					Namespace: scriptsNS,
				},
				Data: map[string]string{
					"my.cnf": "",
				}},
		}), testutil.WithAnyTimes()))
		mockK8sCli.MockCreateMethod(testutil.WithCreateReturned(testutil.WithCreatedSucceedResult(), testutil.WithAnyTimes()))
	}

	newDownwardAPIVolumes := func() []parametersv1alpha1.DownwardAPIChangeTriggeredAction {
		return []parametersv1alpha1.DownwardAPIChangeTriggeredAction{
			{
				Name:       "downward-api",
				MountPoint: "/etc/podinfo",
				Command:    []string{"/bin/true"},
				Items: []corev1.DownwardAPIVolumeFile{
					{
						Path: "labels/role",
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: `metadata.labels['kubeblocks.io/role']`,
						},
					},
				},
			},
		}
	}

	Context("TestBuildConfigManagerContainer", func() {
		It("builds unixSignal reloader correctly", func() {
			param := newCMBuildParams(false)
			mockTplScriptCM()
			reloadOptions := newReloadOptions(parametersv1alpha1.UnixSignalType, nil)
			for i := range param.ConfigSpecsBuildParams {
				buildParam := &param.ConfigSpecsBuildParams[i]
				buildParam.ReloadAction = reloadOptions
				buildParam.ReloadType = parametersv1alpha1.UnixSignalType
			}
			Expect(BuildConfigManagerContainerParams(mockK8sCli.Client(), ctx, param, newVolumeMounts2())).Should(Succeed())
			for _, arg := range []string{`--volume-dir`, `/postgresql/conf`, `--volume-dir`, `/postgresql/conf2`} {
				Expect(param.Args).Should(ContainElement(arg))
			}
		})

		It("builds shellTrigger reloader correctly", func() {
			mockK8sCli.MockGetMethod(testutil.WithGetReturned(testutil.WithConstructSimpleGetResult([]client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      scriptsName,
						Namespace: scriptsNS,
					},
				},
			}), testutil.WithTimes(3)))
			mockK8sCli.MockCreateMethod(testutil.WithCreateReturned(testutil.WithCreatedSucceedResult(), testutil.WithTimes(2)))

			param := newCMBuildParams(true)
			reloadOptions := newReloadOptions(parametersv1alpha1.ShellType, nil)
			for i := range param.ConfigSpecsBuildParams {
				buildParam := &param.ConfigSpecsBuildParams[i]
				buildParam.ReloadAction = reloadOptions
				buildParam.ReloadType = parametersv1alpha1.ShellType
			}
			Expect(BuildConfigManagerContainerParams(mockK8sCli.Client(), context.TODO(), param, newVolumeMounts())).Should(Succeed())
			for _, arg := range []string{`--volume-dir`, `/postgresql/conf`} {
				Expect(param.Args).Should(ContainElement(arg))
			}
		})

		It("builds tplScriptsTrigger reloader correctly", func() {
			mockTplScriptCM()
			param := newCMBuildParams(false)
			reloadOptions := newReloadOptions(parametersv1alpha1.TPLScriptType, syncFn(true))
			for i := range param.ConfigSpecsBuildParams {
				buildParam := &param.ConfigSpecsBuildParams[i]
				buildParam.ReloadAction = reloadOptions
				buildParam.ReloadType = parametersv1alpha1.TPLScriptType
			}
			Expect(BuildConfigManagerContainerParams(mockK8sCli.Client(), context.TODO(), param, newVolumeMounts())).Should(Succeed())
			for _, arg := range []string{`--operator-update-enable`} {
				Expect(param.Args).Should(ContainElement(arg))
			}
		})

		It("builds tplScriptsTrigger reloader correctly with sync", func() {
			mockTplScriptCM()
			param := newCMBuildParams(false)
			reloadOptions := newReloadOptions(parametersv1alpha1.TPLScriptType, syncFn(false))
			for i := range param.ConfigSpecsBuildParams {
				buildParam := &param.ConfigSpecsBuildParams[i]
				buildParam.ReloadAction = reloadOptions
				buildParam.ReloadType = parametersv1alpha1.TPLScriptType
			}
			Expect(BuildConfigManagerContainerParams(mockK8sCli.Client(), context.TODO(), param, newVolumeMounts())).Should(Succeed())
			for _, arg := range []string{`--volume-dir`, `/postgresql/conf`} {
				Expect(param.Args).Should(ContainElement(arg))
			}
		})

		It("builds downwardAPI correctly", func() {
			mockTplScriptCM()
			param := newCMBuildParams(false)
			buildParam := &param.ConfigSpecsBuildParams[0]
			buildParam.DownwardAPIOptions = newDownwardAPIVolumes()
			buildParam.ReloadAction = newReloadOptions(parametersv1alpha1.TPLScriptType, syncFn(true))
			Expect(BuildConfigManagerContainerParams(mockK8sCli.Client(), context.TODO(), param, newVolumeMounts())).Should(Succeed())
			Expect(FindVolumeMount(param.DownwardAPIVolumes, buildParam.DownwardAPIOptions[0].Name)).ShouldNot(BeNil())
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
		formatterConfig *parametersv1alpha1.FileFormatConfig
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
			formatterConfig: &parametersv1alpha1.FileFormatConfig{
				Format: parametersv1alpha1.Ini,
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
			formatterConfig: &parametersv1alpha1.FileFormatConfig{Format: parametersv1alpha1.Ini},
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
