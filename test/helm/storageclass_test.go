/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func storageClassParameters(t *testing.T, objects []map[string]interface{}) map[string]interface{} {
	t.Helper()
	for _, object := range objects {
		name, _, _ := unstructured.NestedString(object, "metadata", "name")
		if object["kind"] != "StorageClass" || name != "kb-default-sc" {
			continue
		}
		parameters, found, err := unstructured.NestedMap(object, "parameters")
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			t.Fatal("parameters not found in default StorageClass")
		}
		return parameters
	}
	t.Fatal("default StorageClass not found")
	return nil
}

func TestAliyunStorageClassPerformanceLevel(t *testing.T) {
	parameters := storageClassParameters(t, renderChart(t, "provider=aliyun"))
	if _, found := parameters["performanceLevel"]; found {
		t.Fatal("performanceLevel must be omitted by default")
	}

	parameters = storageClassParameters(t, renderChart(t,
		"provider=aliyun",
		"storageClass.provider.aliyun.performanceLevel=PL0",
	))
	if parameters["performanceLevel"] != "PL0" {
		t.Fatalf("performanceLevel = %q, want PL0", parameters["performanceLevel"])
	}
}
