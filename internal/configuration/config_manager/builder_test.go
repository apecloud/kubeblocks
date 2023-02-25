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
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func TestBuildConfigManagerContainerArgs(t *testing.T) {
	type args struct {
		reloadOptions *appsv1alpha1.ReloadOptions
		volumeDirs    []corev1.VolumeMount
		cli           client.Client
		ctx           context.Context
		manager       *ConfigManagerSidecar
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// add ut
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := BuildConfigManagerContainerArgs(tt.args.reloadOptions, tt.args.volumeDirs, tt.args.cli, tt.args.ctx, tt.args.manager); (err != nil) != tt.wantErr {
				t.Errorf("BuildConfigManagerContainerArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
