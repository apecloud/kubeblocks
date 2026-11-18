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

package instancetemplate

import (
	"reflect"
	"testing"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func TestResolveConfigs(t *testing.T) {
	base := []workloads.ConfigTemplate{
		{Name: "main", ConfigHash: ptr.To("global"), Restart: ptr.To(true), Parameters: map[string]string{"global": "value"}},
		{Name: "other", ConfigHash: ptr.To("other")},
	}
	source := InstanceTemplateExt{PodTemplateSpec: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
		Volumes: []corev1.Volume{{Name: "config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
			LocalObjectReference: corev1.LocalObjectReference{Name: "global"}, Optional: ptr.To(true), DefaultMode: ptr.To[int32](0400), Items: []corev1.KeyToPath{{Key: "a", Path: "b"}},
		}}}, {Name: "data", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}},
		Containers: []corev1.Container{{Name: "db", VolumeMounts: []corev1.VolumeMount{{Name: "config", MountPath: "/etc/db", SubPath: "db.conf"}}}},
	}}}
	t.Run("whole entry replacement and inherited volume options", func(t *testing.T) {
		got := source
		override := workloads.ConfigTemplate{Name: "main", ConfigHash: ptr.To("specific"), Volume: &workloads.ConfigMapVolumeBinding{VolumeName: "config", ConfigMapName: "specific"}}
		if err := ResolveConfigs(&got, base, []workloads.ConfigTemplate{override}); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got.Configs, []workloads.ConfigTemplate{override, base[1]}) {
			t.Fatalf("unexpected configs: %#v", got.Configs)
		}
		expected := source.PodTemplateSpec.DeepCopy()
		expected.Spec.Volumes[0].ConfigMap.Name = "specific"
		if !reflect.DeepEqual(got.PodTemplateSpec, *expected) {
			t.Fatal("binding changed inherited volume options or mounts")
		}
		if source.Spec.Volumes[0].ConfigMap.Name != "global" || *base[0].ConfigHash != "global" {
			t.Fatal("resolution mutated source")
		}
	})
	for _, overrides := range [][]workloads.ConfigTemplate{nil, {}} {
		got := source
		if err := ResolveConfigs(&got, base, overrides); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got.Configs, base) || !reflect.DeepEqual(got.PodTemplateSpec, source.PodTemplateSpec) {
			t.Fatal("empty overrides did not inherit")
		}
	}
	cases := map[string][]workloads.ConfigTemplate{
		"empty name":              {{Name: ""}},
		"duplicate name":          {{Name: "main"}, {Name: "main"}},
		"unknown config":          {{Name: "missing"}},
		"unknown volume":          {{Name: "main", Volume: &workloads.ConfigMapVolumeBinding{VolumeName: "missing", ConfigMapName: "new"}}},
		"non ConfigMap volume":    {{Name: "main", Volume: &workloads.ConfigMapVolumeBinding{VolumeName: "data", ConfigMapName: "new"}}},
		"empty ConfigMap name":    {{Name: "main", Volume: &workloads.ConfigMapVolumeBinding{VolumeName: "config"}}},
		"conflicting destination": {{Name: "main", Volume: &workloads.ConfigMapVolumeBinding{VolumeName: "config", ConfigMapName: "one"}}, {Name: "other", Volume: &workloads.ConfigMapVolumeBinding{VolumeName: "config", ConfigMapName: "two"}}},
	}
	for name, overrides := range cases {
		t.Run(name, func(t *testing.T) {
			got := source
			if err := ResolveConfigs(&got, base, overrides); err == nil {
				t.Fatal("expected invalid override to fail")
			}
			if !reflect.DeepEqual(got, source) {
				t.Fatal("failed resolution published partial state")
			}
		})
	}
	t.Run("legacy global names remain accepted without overrides", func(t *testing.T) {
		got := source
		legacy := []workloads.ConfigTemplate{{Name: ""}, {Name: "main"}, {Name: "main"}}
		if err := ResolveConfigs(&got, legacy, nil); err != nil {
			t.Fatal(err)
		}
		if err := ResolveConfigs(&got, legacy, []workloads.ConfigTemplate{{Name: "main"}}); err == nil {
			t.Fatal("ambiguous override target was accepted")
		}
	})
}
