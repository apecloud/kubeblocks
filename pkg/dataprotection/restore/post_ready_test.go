/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package restore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func TestRestoreManagerPostReadyTargetEnv(t *testing.T) {
	const (
		namespace     = "default"
		clusterName   = "target"
		componentName = "mysql"
	)
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: clusterName},
		Spec:       appsv1.ClusterSpec{Topology: "shared-nothing"},
	}
	component := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      constant.GenerateClusterComponentName(clusterName, componentName),
		},
		Spec: appsv1.ComponentSpec{
			ServiceVersion: "3.3.2",
			Instances: []appsv1.InstanceTemplate{
				{Name: "canary", ServiceVersion: "3.4.0"},
				{Name: "inherited"},
			},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, component).Build()
	manager := &RestoreManager{Restore: &dpv1alpha1.Restore{}}
	reqCtx := intctrlutil.RequestCtx{Ctx: context.Background()}

	tests := []struct {
		name             string
		instanceTemplate string
		wantVersion      string
	}{
		{name: "component default", wantVersion: "3.3.2"},
		{name: "instance override", instanceTemplate: "canary", wantVersion: "3.4.0"},
		{name: "instance inherits component", instanceTemplate: "inherited", wantVersion: "3.3.2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "target-mysql-0",
				Labels: map[string]string{
					constant.AppInstanceLabelKey:           clusterName,
					constant.KBAppComponentLabelKey:        componentName,
					constant.KBAppInstanceTemplateLabelKey: tt.instanceTemplate,
				},
			}}

			env, err := manager.postReadyTargetEnv(reqCtx, cli, pod)

			require.NoError(t, err)
			require.Equal(t, []corev1.EnvVar{
				{Name: dptypes.DPTargetClusterTopology, Value: "shared-nothing"},
				{Name: dptypes.DPTargetComponentServiceVersion, Value: tt.wantVersion},
			}, env)
		})
	}

	env, err := manager.postReadyTargetEnv(reqCtx, cli, &corev1.Pod{})
	require.NoError(t, err)
	require.Empty(t, env, "non-KubeBlocks target Pods must keep their existing behavior")
}

func TestRestoreJobBuilderOverridePostReadyTargetEnv(t *testing.T) {
	builder := &restoreJobBuilder{env: []corev1.EnvVar{
		{Name: "KEEP", Value: "kept"},
		{Name: dptypes.DPTargetClusterTopology, Value: "restore-value"},
		{Name: dptypes.DPTargetClusterTopology, Value: "pod-value"},
		{Name: dptypes.DPTargetComponentServiceVersion, Value: "spoofed-version"},
	}}
	builder.overridePostReadyTargetEnv([]corev1.EnvVar{
		{Name: dptypes.DPTargetClusterTopology, Value: "shared-nothing"},
		{Name: dptypes.DPTargetComponentServiceVersion, Value: "3.4.0"},
	})

	values := func(name string) []string {
		var result []string
		for i := range builder.env {
			if builder.env[i].Name == name {
				result = append(result, builder.env[i].Value)
			}
		}
		return result
	}
	require.Equal(t, []string{"kept"}, values("KEEP"))
	require.Equal(t, []string{"shared-nothing"}, values(dptypes.DPTargetClusterTopology))
	require.Equal(t, []string{"3.4.0"}, values(dptypes.DPTargetComponentServiceVersion))
}
