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
		{Name: "main", ConfigHash: ptr.To("global"), Volume: &workloads.ConfigVolumeBinding{VolumeName: "config", ConfigMapName: "global"}, Restart: ptr.To(true), Parameters: map[string]string{"global": "value"}},
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
		override := workloads.ConfigTemplate{Name: "main", ConfigHash: ptr.To("specific"), Volume: &workloads.ConfigVolumeBinding{VolumeName: "config", ConfigMapName: "specific"}}
		if err := resolveConfigs(&got, base, []workloads.ConfigTemplate{override}); err != nil {
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
	t.Run("omitted binding inherits only base association", func(t *testing.T) {
		got := source
		override := workloads.ConfigTemplate{Name: "main", ConfigHash: ptr.To("specific")}
		if err := resolveConfigs(&got, base, []workloads.ConfigTemplate{override}); err != nil {
			t.Fatal(err)
		}
		expected := override.DeepCopy()
		expected.Volume = base[0].Volume.DeepCopy()
		if !reflect.DeepEqual(got.Configs[0], *expected) || !reflect.DeepEqual(got.PodTemplateSpec, source.PodTemplateSpec) {
			t.Fatal("override did not inherit binding independently of other settings")
		}
		got.Configs[0].Volume.ConfigMapName = "mutated"
		if base[0].Volume.ConfigMapName != "global" {
			t.Fatal("inherited binding aliases base config")
		}
	})
	t.Run("infer the default ConfigMap and inherit the volume for a source override", func(t *testing.T) {
		defaultConfig := workloads.ConfigTemplate{Name: "main", Volume: &workloads.ConfigVolumeBinding{VolumeName: "config"}}
		instanceConfig := workloads.ConfigTemplate{Name: "main", Volume: &workloads.ConfigVolumeBinding{ConfigMapName: "specific"}}

		got := source
		if err := resolveConfigs(&got, []workloads.ConfigTemplate{defaultConfig}, []workloads.ConfigTemplate{instanceConfig}); err != nil {
			t.Fatal(err)
		}
		if got.Spec.Volumes[0].ConfigMap.Name != "specific" ||
			got.Configs[0].Volume == nil || got.Configs[0].Volume.VolumeName != "config" {
			t.Fatalf("expected the override to mount its ConfigMap in the inherited volume: %#v", got.Configs)
		}
		if defaultConfig.Volume.ConfigMapName != "" || instanceConfig.Volume.VolumeName != "" {
			t.Fatal("resolution mutated the source configurations")
		}

		got = source
		if err := resolveConfigs(&got, []workloads.ConfigTemplate{defaultConfig}, nil); err != nil {
			t.Fatal(err)
		}
		if got.Spec.Volumes[0].ConfigMap.Name != "global" || got.Configs[0].Volume.ConfigMapName != "global" {
			t.Fatal("removing the override did not restore the ConfigMap from the Pod template")
		}
	})
	for _, overrides := range [][]workloads.ConfigTemplate{nil, {}} {
		got := source
		if err := resolveConfigs(&got, base, overrides); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got.Configs, base) || !reflect.DeepEqual(got.PodTemplateSpec, source.PodTemplateSpec) {
			t.Fatal("empty overrides did not inherit")
		}
	}
	cases := map[string][]workloads.ConfigTemplate{
		"empty name":                    {{Name: ""}},
		"duplicate name":                {{Name: "main"}, {Name: "main"}},
		"unknown config":                {{Name: "missing"}},
		"unknown volume":                {{Name: "main", Volume: &workloads.ConfigVolumeBinding{VolumeName: "missing", ConfigMapName: "new"}}},
		"non ConfigMap volume":          {{Name: "main", Volume: &workloads.ConfigVolumeBinding{VolumeName: "data", ConfigMapName: "new"}}},
		"empty ConfigMap name":          {{Name: "main", Volume: &workloads.ConfigVolumeBinding{VolumeName: "config"}}},
		"override without base binding": {{Name: "other", Volume: &workloads.ConfigVolumeBinding{ConfigMapName: "new"}}},
		"empty override binding":        {{Name: "main", Volume: &workloads.ConfigVolumeBinding{}}},
	}
	for name, overrides := range cases {
		t.Run(name, func(t *testing.T) {
			got := source
			if err := resolveConfigs(&got, base, overrides); err == nil {
				t.Fatal("expected invalid override to fail")
			}
			if !reflect.DeepEqual(got, source) {
				t.Fatal("failed resolution published partial state")
			}
		})
	}
	t.Run("conflicting destinations for shared association", func(t *testing.T) {
		got := source
		shared := []workloads.ConfigTemplate{*base[0].DeepCopy(), {Name: "other", Volume: &workloads.ConfigVolumeBinding{VolumeName: "config"}}}
		override := workloads.ConfigTemplate{Name: "main", Volume: &workloads.ConfigVolumeBinding{ConfigMapName: "specific"}}
		if err := resolveConfigs(&got, shared, []workloads.ConfigTemplate{override}); err == nil {
			t.Fatal("conflicting destinations for a shared volume were accepted")
		}
		if !reflect.DeepEqual(got, source) {
			t.Fatal("failed resolution published partial state")
		}
	})
	for name, binding := range map[string]*workloads.ConfigVolumeBinding{
		"missing volume":        {VolumeName: "missing", ConfigMapName: "global"},
		"non ConfigMap volume":  {VolumeName: "data", ConfigMapName: "global"},
		"missing volume name":   {ConfigMapName: "global"},
		"empty base binding":    {},
		"different base source": {VolumeName: "config", ConfigMapName: "different"},
	} {
		t.Run("invalid base binding/"+name, func(t *testing.T) {
			got := source
			config := *base[0].DeepCopy()
			config.Volume = binding
			if err := resolveConfigs(&got, []workloads.ConfigTemplate{config}, nil); err == nil {
				t.Fatal("invalid base binding was accepted")
			}
			if !reflect.DeepEqual(got, source) {
				t.Fatal("failed resolution published partial state")
			}
		})
	}
	t.Run("an associated Pod volume requires a ConfigMap source name", func(t *testing.T) {
		got := source
		got.PodTemplateSpec = *source.PodTemplateSpec.DeepCopy()
		got.Spec.Volumes[0].ConfigMap.Name = ""
		config := workloads.ConfigTemplate{Name: "main", Volume: &workloads.ConfigVolumeBinding{VolumeName: "config"}}
		if err := resolveConfigs(&got, []workloads.ConfigTemplate{config}, nil); err == nil {
			t.Fatal("accepted a volume without a default ConfigMap")
		}
	})
	t.Run("legacy global names remain accepted without overrides", func(t *testing.T) {
		got := source
		legacy := []workloads.ConfigTemplate{{Name: ""}, {Name: "main"}, {Name: "main"}}
		if err := resolveConfigs(&got, legacy, nil); err != nil {
			t.Fatal(err)
		}
		if err := resolveConfigs(&got, legacy, []workloads.ConfigTemplate{{Name: "main"}}); err == nil {
			t.Fatal("ambiguous override target was accepted")
		}
	})
}
