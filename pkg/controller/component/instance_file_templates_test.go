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
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

func TestInstanceFileTemplates(t *testing.T) {
	action := &appsv1.Action{Exec: &appsv1.ExecAction{Command: []string{"reload"}}}
	comp := &SynthesizedComponent{
		Namespace: "component-ns",
		FileTemplates: []SynthesizedFileTemplate{
			{
				ComponentFileTemplate: appsv1.ComponentFileTemplate{
					Name: "server", Template: "default-source", Namespace: "definition-ns", VolumeName: "config",
					DefaultMode: ptr.To(int32(0440)), RestartOnFileChange: ptr.To(true), Reconfigure: action,
				},
				Config: true, Variables: map[string]string{"keep": "parent", "override": "parent"},
				ConfigHash: ptr.To("parent-hash"), ReconfigureRequired: ptr.To(true), ReconfigureAction: action,
				ReconfigureArgs: [][]string{{"--reload"}},
			},
			{ComponentFileTemplate: appsv1.ComponentFileTemplate{Name: "script", Template: "script-source"}},
			{ComponentFileTemplate: appsv1.ComponentFileTemplate{Name: "external", ExternalManaged: ptr.To(true)}, Config: true},
		},
	}
	instance := &appsv1.InstanceTemplate{Name: "large", Configs: []appsv1.ClusterComponentConfig{{
		Name: ptr.To("server"), Variables: map[string]string{"override": "instance", "add": "instance"},
		ClusterComponentConfigSource: appsv1.ClusterComponentConfigSource{ConfigMap: &corev1.ConfigMapVolumeSource{
			LocalObjectReference: corev1.LocalObjectReference{Name: "instance-source"},
		}},
	}}}
	templates, err := InstanceFileTemplates(comp, instance)
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) != 1 {
		t.Fatalf("got %d templates, want one managed config", len(templates))
	}
	want := comp.FileTemplates[0]
	want.Namespace, want.Template = comp.Namespace, "instance-source"
	want.Variables = map[string]string{"keep": "parent", "override": "instance", "add": "instance"}
	if !reflect.DeepEqual(templates[0], want) {
		t.Fatalf("effective template differs: got %#v, want %#v", templates[0], want)
	}
	templates[0].Variables["keep"] = "changed"
	templates[0].Variables["add"] = "changed"
	if comp.FileTemplates[0].Variables["keep"] != "parent" || instance.Configs[0].Variables["add"] != "instance" {
		t.Fatal("derived variables alias Component or instance inputs")
	}
	inherited, err := InstanceFileTemplates(comp, &appsv1.InstanceTemplate{Name: "inherit"})
	if err != nil {
		t.Fatal(err)
	}
	if len(inherited) != 1 || !reflect.DeepEqual(inherited[0], comp.FileTemplates[0]) {
		t.Fatalf("empty overrides did not inherit managed configuration: %#v", inherited)
	}
	inherited[0].Variables["keep"] = "changed"
	if comp.FileTemplates[0].Variables["keep"] != "parent" {
		t.Fatal("inherited variables alias Component inputs")
	}
}

func TestInstanceFileTemplatesRejectsInvalidOverrides(t *testing.T) {
	config := func(name string) appsv1.ClusterComponentConfig {
		return appsv1.ClusterComponentConfig{Name: ptr.To(name)}
	}
	for _, tc := range []struct {
		name   string
		modify func(*appsv1.ClusterComponentConfig)
		extra  bool
		want   string
	}{
		{name: "missing name", modify: func(c *appsv1.ClusterComponentConfig) { c.Name = nil }, want: "name is required"},
		{name: "empty name", modify: func(c *appsv1.ClusterComponentConfig) { c.Name = ptr.To("") }, want: "name is required"},
		{name: "unknown name", modify: func(c *appsv1.ClusterComponentConfig) { c.Name = ptr.To("unknown") }, want: "not defined"},
		{name: "script target", modify: func(c *appsv1.ClusterComponentConfig) { c.Name = ptr.To("script") }, want: "not defined"},
		{name: "external target", modify: func(c *appsv1.ClusterComponentConfig) { c.Name = ptr.To("external") }, want: "externally managed"},
		{name: "duplicate name", extra: true, want: "duplicated"},
		{name: "external managed", modify: func(c *appsv1.ClusterComponentConfig) { c.ExternalManaged = ptr.To(false) }, want: "accepts only"},
		{name: "config hash", modify: func(c *appsv1.ClusterComponentConfig) { c.ConfigHash = ptr.To("") }, want: "accepts only"},
		{name: "restart", modify: func(c *appsv1.ClusterComponentConfig) { c.Restart = ptr.To(false) }, want: "accepts only"},
		{name: "reconfigure", modify: func(c *appsv1.ClusterComponentConfig) { c.Reconfigure = ptr.To(false) }, want: "accepts only"},
		{name: "action", modify: func(c *appsv1.ClusterComponentConfig) { c.ReconfigureAction = &appsv1.Action{} }, want: "accepts only"},
		{name: "args", modify: func(c *appsv1.ClusterComponentConfig) { c.ReconfigureArgs = [][]string{} }, want: "accepts only"},
		{name: "empty source", modify: func(c *appsv1.ClusterComponentConfig) { c.ConfigMap = &corev1.ConfigMapVolumeSource{} }, want: "invalid configMap.name"},
		{name: "invalid source", modify: func(c *appsv1.ClusterComponentConfig) {
			c.ConfigMap = &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "Invalid/Source"}}
		}, want: "invalid configMap.name"},
		{name: "source items", modify: func(c *appsv1.ClusterComponentConfig) { c.ConfigMap.Items = []corev1.KeyToPath{} }, want: "accepts only configMap.name"},
		{name: "source mode", modify: func(c *appsv1.ClusterComponentConfig) { c.ConfigMap.DefaultMode = ptr.To(int32(0444)) }, want: "accepts only configMap.name"},
		{name: "source optional", modify: func(c *appsv1.ClusterComponentConfig) { c.ConfigMap.Optional = ptr.To(false) }, want: "accepts only configMap.name"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			comp := &SynthesizedComponent{FileTemplates: []SynthesizedFileTemplate{
				{ComponentFileTemplate: appsv1.ComponentFileTemplate{Name: "server", Template: "source"}, Config: true},
				{ComponentFileTemplate: appsv1.ComponentFileTemplate{Name: "script"}},
				{ComponentFileTemplate: appsv1.ComponentFileTemplate{Name: "external", ExternalManaged: ptr.To(true)}, Config: true},
			}}
			override := config("server")
			override.ConfigMap = &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "source"}}
			if tc.modify != nil {
				tc.modify(&override)
			}
			instance := &appsv1.InstanceTemplate{Name: "large", Configs: []appsv1.ClusterComponentConfig{override}}
			if tc.extra {
				instance.Configs = append(instance.Configs, config("server"))
			}
			_, err := InstanceFileTemplates(comp, instance)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got error %v, want %q", err, tc.want)
			}
		})
	}
}
