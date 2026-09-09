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

package parameters

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestAutomaticResourcePayload(t *testing.T) {
	comp := &appsv1.ComponentSpec{
		Replicas:  2,
		Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")}},
		TLSConfig: &appsv1.TLSConfig{Enable: true},
		VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{
			{Name: "data", Spec: corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
			}}},
		},
	}
	sharding := &appsv1.ClusterSharding{Shards: 3, Template: appsv1.ClusterComponentSpec{Replicas: 2}}
	payload := func(c *appsv1.ComponentSpec, s *appsv1.ClusterSharding) parametersv1alpha1.Payload {
		spec := &parametersv1alpha1.ComponentParameterSpec{ConfigItemDetails: []parametersv1alpha1.ConfigTemplateItemDetail{{Name: "config"}}}
		require.NoError(t, UpdateConfigPayload(spec, c, s))
		return spec.ConfigItemDetails[0].Payload
	}
	original := payload(comp, sharding)
	require.JSONEq(t, `{"data":{"requests":1073741824}}`, string(original[constant.VolumeClaimTemplatesPayload]))
	for _, tc := range []struct {
		name, key string
		change    func(*appsv1.ComponentSpec, *appsv1.ClusterSharding)
	}{
		{"vscale", constant.ComponentResourcePayload, func(c *appsv1.ComponentSpec, _ *appsv1.ClusterSharding) {
			c.Resources.Requests[corev1.ResourceCPU] = resource.MustParse("2")
		}},
		{"hscale", constant.ReplicasPayload, func(c *appsv1.ComponentSpec, _ *appsv1.ClusterSharding) { c.Replicas++ }},
		{"tls", constant.TLSPayload, func(c *appsv1.ComponentSpec, _ *appsv1.ClusterSharding) { c.TLSConfig.Enable = false }},
		{"sharding shards", constant.ShardingPayload, func(_ *appsv1.ComponentSpec, s *appsv1.ClusterSharding) { s.Shards++ }},
		{"sharding replicas", constant.ShardingPayload, func(_ *appsv1.ComponentSpec, s *appsv1.ClusterSharding) { s.Template.Replicas++ }},
		{"storage request", constant.VolumeClaimTemplatesPayload, func(c *appsv1.ComponentSpec, _ *appsv1.ClusterSharding) {
			c.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
		}},
		{"storage limit", constant.VolumeClaimTemplatesPayload, func(c *appsv1.ComponentSpec, _ *appsv1.ClusterSharding) {
			c.VolumeClaimTemplates[0].Spec.Resources.Limits = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2Gi")}
		}},
		{"storage removal", constant.VolumeClaimTemplatesPayload, func(c *appsv1.ComponentSpec, _ *appsv1.ClusterSharding) { c.VolumeClaimTemplates = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, s := comp.DeepCopy(), sharding.DeepCopy()
			tc.change(c, s)
			updated := payload(c, s)
			require.NotEqual(t, original[tc.key], updated[tc.key])
			for key := range original {
				if key != tc.key {
					require.Equal(t, original[key], updated[key])
				}
			}
		})
	}

	equivalent := comp.DeepCopy()
	equivalent.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("1024Mi")
	equivalent.VolumeClaimTemplates[0].Spec.StorageClassName = new(string)
	require.Equal(t, original, payload(equivalent, sharding))
	equivalent.VolumeClaimTemplates = append(equivalent.VolumeClaimTemplates, appsv1.PersistentVolumeClaimTemplate{Name: "logs"})
	ordered := payload(equivalent, sharding)
	equivalent.VolumeClaimTemplates[0], equivalent.VolumeClaimTemplates[1] = equivalent.VolumeClaimTemplates[1], equivalent.VolumeClaimTemplates[0]
	require.Equal(t, ordered, payload(equivalent, sharding))

	// Removing optional inputs clears old payload keys as well.
	spec := &parametersv1alpha1.ComponentParameterSpec{ConfigItemDetails: []parametersv1alpha1.ConfigTemplateItemDetail{{Payload: original}}}
	require.NoError(t, UpdateConfigPayload(spec, &appsv1.ComponentSpec{}, nil))
	require.NotContains(t, spec.ConfigItemDetails[0].Payload, constant.TLSPayload)
	require.NotContains(t, spec.ConfigItemDetails[0].Payload, constant.ShardingPayload)
	require.NotContains(t, spec.ConfigItemDetails[0].Payload, constant.ComponentResourcePayload)
}
