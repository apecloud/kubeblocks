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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("Config Builder Test", func() {

	const (
		scriptsName = "script_cm"
		scriptsNS   = "default"
	)

	var mockK8sCli *testutil.K8sClientMockHelper

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		mockK8sCli = testutil.NewK8sMockClient()
	})

	AfterEach(func() {
		DeferCleanup(mockK8sCli.Finish)
	})

	newVolumeMounts := func() []corev1.VolumeMount {
		return []corev1.VolumeMount{
			{
				MountPath: "/postgresql/conf",
				Name:      "pg_config",
			}}
	}
	newReloadOptions := func(t parametersv1alpha1.DynamicReloadType, sync *bool) *parametersv1alpha1.ReloadAction {
		shellHandle := &parametersv1alpha1.ShellTrigger{
			Command: []string{"pwd"},
		}
		autoHandle := &parametersv1alpha1.AutoTrigger{
			ProcessName: "postgres",
		}

		switch t {
		case parametersv1alpha1.ShellType:
			return &parametersv1alpha1.ReloadAction{
				ShellTrigger: shellHandle}
		case parametersv1alpha1.AutoType:
			return &parametersv1alpha1.ReloadAction{
				AutoTrigger: autoHandle}
		default:
			return nil
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

	Context("TestBuildConfigManagerContainer", func() {
		It("build shell reloader correctly", func() {
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
			Expect(BuildConfigManagerContainerParams(mockK8sCli.Client(), context.TODO(), param)).Should(Succeed())
		})
	})
})
