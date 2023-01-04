/*
Copyright ApeCloud Inc.

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

package testdata

import (
	"k8s.io/apimachinery/pkg/util/yaml"
	"os"
	"path/filepath"
	"runtime"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

var testDataRoot string

func init() {
	_, file, _, _ := runtime.Caller(0)
	testDataRoot = filepath.Dir(file)
}

func SubTestDataPath(subPath string) string {
	return filepath.Join(testDataRoot, subPath)
}

type KBResource interface {
	dbaasv1alpha1.Cluster |
	dbaasv1alpha1.ClusterDefinition |
	dbaasv1alpha1.ClusterVersion |
	dbaasv1alpha1.ConfigurationTemplate |
	corev1.ConfigMap |
	appsv1.StatefulSet
}

func GetResourceFromTestData[T KBResource](yamlFile string) (*T, error) {
	yamlContext, err := os.ReadFile(SubTestDataPath(yamlFile))
	if err != nil {
		return nil, err
	}

	obj := new(T)
	if err := yaml.Unmarshal(yamlContext, obj); err != nil {
		return nil, err
	}
	return obj, nil
}
