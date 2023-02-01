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
	"os"
	"path/filepath"
	"runtime"

	"github.com/sethvargo/go-password/password"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

type KBResource interface {
	dbaasv1alpha1.Cluster |
	dbaasv1alpha1.ClusterDefinition |
	dbaasv1alpha1.ClusterVersion |
	dbaasv1alpha1.ConfigConstraint |
	dbaasv1alpha1.OpsRequest |
	corev1.ConfigMap |
	appsv1.StatefulSet |
	appsv1.Deployment
}

type ResourceOptions func(obj client.Object)

var testDataRoot string

func init() {
	_, file, _, _ := runtime.Caller(0)
	testDataRoot = filepath.Dir(file)
}

// SubTestDataPath gets the file path which belongs to test data directory or its subdirectories.
func SubTestDataPath(subPath string) string {
	return filepath.Join(testDataRoot, subPath)
}

func ScanDirectoryPath(subPath string) ([]string, error) {
	dirs, err := os.ReadDir(SubTestDataPath(subPath))
	if err != nil {
		return nil, err
	}
	resourceList := make([]string, len(dirs))
	for i, d := range dirs {
		if d.IsDir() {
			continue
		}
		resourceList[i] = filepath.Join(subPath, d.Name())
	}
	return resourceList, nil
}

// GetTestDataFileContent gets the file content which belongs to test data directory or its subdirectories.
func GetTestDataFileContent(filePath string) ([]byte, error) {
	return os.ReadFile(SubTestDataPath(filePath))
}

func GetResourceFromTestData[T KBResource](yamlFile string, opts ...ResourceOptions) (*T, error) {
	yamlBytes, err := os.ReadFile(SubTestDataPath(yamlFile))
	if err != nil {
		return nil, err
	}
	return GetResourceFromContext[T](yamlBytes, opts...)
}

func GetResourceFromContext[T KBResource](yamlBytes []byte, opts ...ResourceOptions) (*T, error) {
	toK8sResource := func(o interface{}) client.Object {
		obj, _ := o.(client.Object)
		return obj
	}

	obj := new(T)
	if err := yaml.Unmarshal(yamlBytes, obj); err != nil {
		return nil, err
	}
	k8sObj := toK8sResource(obj)
	for _, ops := range opts {
		ops(k8sObj)
	}
	return obj, nil
}

func WithName(resourceName string) ResourceOptions {
	return func(obj client.Object) {
		obj.SetName(resourceName)
	}
}

func WithNamespace(ns string) ResourceOptions {
	return func(obj client.Object) {
		obj.SetNamespace(ns)
	}
}

func WithNamespacedName(resourceName, ns string) ResourceOptions {
	return func(k8sObject client.Object) {
		k8sObject.SetNamespace(ns)
		k8sObject.SetName(resourceName)
	}
}

func WithLabels(keysAndValues ...string) ResourceOptions {
	return func(k8sObject client.Object) {
		// ignore mismatching for kvs
		labels := make(map[string]string, len(keysAndValues)/2)
		for i := 0; i+1 < len(keysAndValues); i += 2 {
			labels[keysAndValues[i]] = keysAndValues[i+1]
		}
		k8sObject.SetLabels(labels)
	}
}

func WithAnnotations(keysAndValues ...string) ResourceOptions {
	return func(k8sObject client.Object) {
		// ignore mismatching for kvs
		annotations := make(map[string]string, len(keysAndValues)/2)
		for i := 0; i+1 < len(keysAndValues); i += 2 {
			annotations[keysAndValues[i]] = keysAndValues[i+1]
		}
		k8sObject.SetAnnotations(annotations)
	}
}

type ComponentSelector = func(*dbaasv1alpha1.ClusterDefinitionSpec) *dbaasv1alpha1.ClusterDefinitionComponent

func CustomSelector(fn func(*dbaasv1alpha1.ClusterDefinitionComponent) bool) ComponentSelector {
	return func(spec *dbaasv1alpha1.ClusterDefinitionSpec) *dbaasv1alpha1.ClusterDefinitionComponent {
		for i := range spec.Components {
			com := &spec.Components[i]
			if fn(com) {
				return com
			}
		}
		return nil
	}
}

func IndexSelector(index int) ComponentSelector {
	return func(spec *dbaasv1alpha1.ClusterDefinitionSpec) *dbaasv1alpha1.ClusterDefinitionComponent {
		if len(spec.Components) <= index {
			return nil
		}
		return &spec.Components[index]
	}
}

func NamedSelector(typeName string) ComponentSelector {
	return CustomSelector(func(component *dbaasv1alpha1.ClusterDefinitionComponent) bool {
		return component.TypeName == typeName
	})
}

func ComponentTypeSelector(componentType dbaasv1alpha1.ComponentType) ComponentSelector {
	return CustomSelector(func(component *dbaasv1alpha1.ClusterDefinitionComponent) bool {
		return component.ComponentType == componentType
	})
}

func WithUpdateComponent(selector ComponentSelector, fn func(component *dbaasv1alpha1.ClusterDefinitionComponent)) ResourceOptions {
	return func(obj client.Object) {
		cd, ok := obj.(*dbaasv1alpha1.ClusterDefinition)
		if !ok {
			return
		}
		if component := selector(&cd.Spec); component != nil {
			fn(component)
		}
	}
}

func WithConfigTemplate(tpls []dbaasv1alpha1.ConfigTemplate, selector ComponentSelector) ResourceOptions {
	return WithUpdateComponent(selector, func(component *dbaasv1alpha1.ClusterDefinitionComponent) {
		component.ConfigSpec = &dbaasv1alpha1.ConfigurationSpec{
			ConfigTemplateRefs: tpls,
		}
	})
}

func GetResourceMeta(yamlBytes []byte) (metav1.TypeMeta, error) {
	type k8sObj struct {
		metav1.TypeMeta `json:",inline"`
	}
	var o k8sObj
	err := yaml.Unmarshal(yamlBytes, &o)
	if err != nil {
		return metav1.TypeMeta{}, err
	}
	return o.TypeMeta, nil
}

func GenRandomString() string {
	const (
		numDigits    = 2
		numSymbols   = 0
		randomLength = 12
	)

	randomStr, _ := password.Generate(randomLength, numDigits, numSymbols, true, false)
	return randomStr
}
