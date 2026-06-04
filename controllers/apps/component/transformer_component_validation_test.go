/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package component

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

func TestValidateExternalManagedConfigSources(t *testing.T) {
	const (
		namespace   = "default"
		clusterName = "test-cluster"
		compName    = "mysql"
		configName  = "mysql-config"
		sidecarName = "sidecar-config"
		cmName      = "mysql-cm"
	)

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name                   string
		generation             int64
		config                 appsv1.ClusterComponentConfig
		compDefExternalManaged bool
		sidecarExternalManaged bool
		runningWorkload        *workloads.InstanceSet
		cm                     *corev1.ConfigMap
		wantErr                bool
	}{
		{
			name:                   "rejects external-managed configmap when referenced configmap is missing",
			generation:             1,
			config:                 externalManagedConfig(configName, cmName),
			compDefExternalManaged: true,
			wantErr:                true,
		},
		{
			name:                   "rejects external-managed configmap without kb component labels",
			generation:             1,
			config:                 externalManagedConfig(configName, cmName),
			compDefExternalManaged: true,
			cm:                     configMap(namespace, cmName, nil),
			wantErr:                true,
		},
		{
			name:                   "accepts existing external-managed configmap source already mounted by workload",
			generation:             2,
			config:                 externalManagedConfig(configName, cmName),
			compDefExternalManaged: true,
			runningWorkload:        workloadWithConfigVolume(configName+"-volume", cmName),
			cm:                     configMap(namespace, cmName, nil),
		},
		{
			name:                   "rejects changed external-managed configmap source without kb component labels",
			generation:             2,
			config:                 externalManagedConfig(configName, cmName),
			compDefExternalManaged: true,
			runningWorkload:        workloadWithConfigVolume(configName+"-volume", "old-"+cmName),
			cm:                     configMap(namespace, cmName, nil),
			wantErr:                true,
		},
		{
			name:                   "accepts external-managed configmap with only kb component labels",
			generation:             1,
			config:                 externalManagedConfig(configName, cmName),
			compDefExternalManaged: true,
			cm: configMap(namespace, cmName, map[string]string{
				constant.AppManagedByLabelKey:   constant.AppName,
				constant.AppInstanceLabelKey:    clusterName,
				constant.KBAppComponentLabelKey: compName,
			}),
		},
		{
			name:       "ignores user-specified configmap when config is not external-managed",
			generation: 1,
			config: appsv1.ClusterComponentConfig{
				Name: ptr.To(configName),
				ClusterComponentConfigSource: appsv1.ClusterComponentConfigSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
					},
				},
			},
			compDefExternalManaged: false,
		},
		{
			name:                   "rejects sidecar external-managed configmap without kb component labels",
			generation:             1,
			config:                 externalManagedConfig(sidecarName, cmName),
			sidecarExternalManaged: true,
			cm:                     configMap(namespace, cmName, nil),
			wantErr:                true,
		},
		{
			name:                   "accepts sidecar external-managed configmap with only kb component labels",
			generation:             1,
			config:                 externalManagedConfig(sidecarName, cmName),
			sidecarExternalManaged: true,
			cm: configMap(namespace, cmName, map[string]string{
				constant.AppManagedByLabelKey:   constant.AppName,
				constant.AppInstanceLabelKey:    clusterName,
				constant.KBAppComponentLabelKey: compName,
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objs []client.Object
			if tt.cm != nil {
				objs = append(objs, tt.cm)
			}

			transCtx := &componentTransformContext{
				Context: context.Background(),
				Client:  fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build(),
				Component: &appsv1.Component{
					ObjectMeta: metav1ObjectMeta(namespace,
						constant.GenerateClusterComponentName(clusterName, compName),
						tt.generation,
						constant.GetCompLabels(clusterName, compName)),
					Spec: appsv1.ComponentSpec{
						Configs: []appsv1.ClusterComponentConfig{tt.config},
					},
				},
				CompDef: &appsv1.ComponentDefinition{
					Spec: appsv1.ComponentDefinitionSpec{
						Configs: []appsv1.ComponentFileTemplate{{
							Name:            configName,
							ExternalManaged: ptr.To(tt.compDefExternalManaged),
						}},
					},
				},
				SynthesizeComponent: synthesizedComponentWithFileTemplates(
					configName, tt.compDefExternalManaged,
					sidecarName, tt.sidecarExternalManaged),
			}
			if tt.runningWorkload != nil {
				transCtx.RunningWorkload = tt.runningWorkload
			}
			err := validateExternalManagedConfigSources(transCtx)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func externalManagedConfig(name, cmName string) appsv1.ClusterComponentConfig {
	return appsv1.ClusterComponentConfig{
		Name: ptr.To(name),
		ClusterComponentConfigSource: appsv1.ClusterComponentConfigSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
			},
		},
	}
}

func configMap(namespace, name string, labels map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1ObjectMeta(namespace, name, 0, labels),
		Data:       map[string]string{"config": "value"},
	}
}

func synthesizedComponentWithFileTemplates(configName string, compDefExternalManaged bool, sidecarName string, sidecarExternalManaged bool) *component.SynthesizedComponent {
	return &component.SynthesizedComponent{
		FileTemplates: []component.SynthesizedFileTemplate{
			{
				ComponentFileTemplate: appsv1.ComponentFileTemplate{
					Name:            configName,
					VolumeName:      configName + "-volume",
					ExternalManaged: ptr.To(compDefExternalManaged),
				},
				Config: true,
			},
			{
				ComponentFileTemplate: appsv1.ComponentFileTemplate{
					Name:            sidecarName,
					VolumeName:      sidecarName + "-volume",
					ExternalManaged: ptr.To(sidecarExternalManaged),
				},
				Config: true,
			},
		},
	}
}

func workloadWithConfigVolume(volumeName, cmName string) *workloads.InstanceSet {
	return &workloads.InstanceSet{
		Spec: workloads.InstanceSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{{
						Name: volumeName,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
							},
						},
					}},
				},
			},
		},
	}
}

func metav1ObjectMeta(namespace, name string, generation int64, labels map[string]string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace:  namespace,
		Name:       name,
		Generation: generation,
		Labels:     labels,
	}
}
