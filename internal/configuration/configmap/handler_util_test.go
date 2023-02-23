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

package configmap

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
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
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSupportReload(tt.args.reload); got != tt.want {
				t.Errorf("IsSupportReload() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildSignalArgs(t *testing.T) {
	r := BuildSignalArgs(appsv1alpha1.UnixSignalTrigger{
		ProcessName: "postgres",
		Signal:      appsv1alpha1.SIGHUP,
	}, []corev1.VolumeMount{
		{
			MountPath: "/postgresql/conf",
			Name:      "pg_config",
		},
		{
			MountPath: "/postgresql/conf2",
			Name:      "pg_config",
		},
	})
	require.Regexp(t, `--process\s+postgres`, r)
	require.Regexp(t, `--signal\s+SIGHUP`, r)
	require.Regexp(t, `--volume-dir\s+/postgresql/conf`, r)
	require.Regexp(t, `--volume-dir\s+/postgresql/conf2`, r)

	require.Nil(t, NeedBuildConfigSidecar(&appsv1alpha1.ReloadOptions{
		UnixSignalTrigger: &appsv1alpha1.UnixSignalTrigger{
			Signal: appsv1alpha1.SIGHUP,
		},
	}))
	require.NotNil(t, NeedBuildConfigSidecar(&appsv1alpha1.ReloadOptions{
		UnixSignalTrigger: &appsv1alpha1.UnixSignalTrigger{
			Signal: "SIGNOEXIST",
		},
	}))
}
