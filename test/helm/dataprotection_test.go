/*
Copyright (C) 2026 ApeCloud Co., Ltd.

This file is part of KubeBlocks project.

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

package helm

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const defaultRegistry = "apecloud-registry.cn-zhangjiakou.cr.aliyuncs.com"

func renderChart(t *testing.T, settings ...string) []map[string]interface{} {
	t.Helper()
	helm, err := exec.LookPath("helm")
	if err != nil {
		t.Skip("Helm is required for chart rendering tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	args := []string{"template", "kb", "../../deploy/helm", "--namespace", "kb-system",
		"--set-string", "dataProtection.encryptionKey=chart-render-test-key"}
	for _, setting := range settings {
		args = append(args, "--set", setting)
	}
	cmd := exec.CommandContext(ctx, helm, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("helm template: %v\n%s", err, stderr.String())
	}
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(output), 4096)
	var objects []map[string]interface{}
	for {
		var object map[string]interface{}
		err := decoder.Decode(&object)
		if err == io.EOF {
			return objects
		}
		if err != nil {
			t.Fatalf("decode rendered manifest: %v", err)
		}
		if len(object) > 0 {
			objects = append(objects, object)
		}
	}
}

func datasafedEnv(t *testing.T, objects []map[string]interface{}) (map[string]interface{}, []interface{}, map[string]interface{}) {
	t.Helper()
	for _, object := range objects {
		name, _, _ := unstructured.NestedString(object, "metadata", "name")
		if object["kind"] != "Deployment" || name != "kb-kubeblocks-dataprotection" {
			continue
		}
		containers, _, err := unstructured.NestedSlice(object, "spec", "template", "spec", "containers")
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range containers {
			container := item.(map[string]interface{})
			if container["name"] != "dataprotection" {
				continue
			}
			for _, item := range container["env"].([]interface{}) {
				env := item.(map[string]interface{})
				if env["name"] == "DATASAFED_IMAGE" {
					return object, containers, env
				}
			}
		}
	}
	t.Fatal("DATASAFED_IMAGE not found in dataprotection Deployment")
	return nil, nil, nil
}

func TestDataSafedRegistry(t *testing.T) {
	tests := []struct {
		name     string
		settings []string
		image    string
	}{
		{"default", nil, defaultRegistry + "/apecloud/datasafed:0.2.3"},
		{"global fallback", []string{"image.registry=global.example.com"}, "global.example.com/apecloud/datasafed:0.2.3"},
		{"DP fallback", []string{"image.registry=global.example.com", "dataProtection.image.registry=dp.example.com"}, "dp.example.com/apecloud/datasafed:0.2.3"},
		{"independent registry", []string{"dataProtection.image.datasafed.registry=public.example.com"}, "public.example.com/apecloud/datasafed:0.2.3"},
		{"independent precedence", []string{"image.registry=global.example.com", "dataProtection.image.registry=dp.example.com", "dataProtection.image.datasafed.registry=public.example.com:5000"}, "public.example.com:5000/apecloud/datasafed:0.2.3"},
		{"empty registry", []string{"dataProtection.image.registry=dp.example.com", "dataProtection.image.datasafed.registry="}, "dp.example.com/apecloud/datasafed:0.2.3"},
		{"null registry", []string{"dataProtection.image.registry=dp.example.com", "dataProtection.image.datasafed.registry=null"}, "dp.example.com/apecloud/datasafed:0.2.3"},
		{"empty registries", []string{"image.registry=", "dataProtection.image.registry=", "dataProtection.image.datasafed.registry="}, defaultRegistry + "/apecloud/datasafed:0.2.3"},
		{"custom repository and tag", []string{"dataProtection.image.datasafed.registry=public.example.com", "dataProtection.image.datasafed.repository=custom/datasafed", "dataProtection.image.datasafed.tag=custom"}, "public.example.com/custom/datasafed:custom"},
		{"empty tag fallback", []string{"dataProtection.image.datasafed.registry=public.example.com", "dataProtection.image.datasafed.tag="}, "public.example.com/apecloud/datasafed:latest"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, env := datasafedEnv(t, renderChart(t, tt.settings...))
			if env["value"] != tt.image {
				t.Fatalf("DATASAFED_IMAGE = %q, want %q", env["value"], tt.image)
			}
		})
	}
}

func TestDataSafedRegistryOnlyChangesDataSafedImage(t *testing.T) {
	settings := []string{"image.registry=controller.example.com", "dataProtection.image.registry=harbor.example.com:5000"}
	want := renderChart(t, settings...)
	got := renderChart(t, append(settings, "dataProtection.image.datasafed.registry=public.example.com")...)
	object, containers, env := datasafedEnv(t, want)
	if env["value"] != "harbor.example.com:5000/apecloud/datasafed:0.2.3" {
		t.Fatalf("unexpected legacy image: %v", env["value"])
	}
	env["value"] = "public.example.com/apecloud/datasafed:0.2.3"
	if err := unstructured.SetNestedSlice(object, containers, "spec", "template", "spec", "containers"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("independent registry must change only DATASAFED_IMAGE across all rendered objects")
	}
}

func TestDataSafedRegistryWithDataProtectionDisabled(t *testing.T) {
	for _, object := range renderChart(t, "dataProtection.enabled=false", "dataProtection.image.datasafed.registry=public.example.com") {
		name, _, _ := unstructured.NestedString(object, "metadata", "name")
		if object["kind"] == "Deployment" && strings.HasSuffix(name, "-dataprotection") {
			t.Fatal("dataprotection Deployment rendered while disabled")
		}
	}
}
