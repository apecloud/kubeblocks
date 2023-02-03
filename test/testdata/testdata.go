/*
Copyright ApeCloud, Inc.

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
type FilterOptions = func(string) bool

var testDataRoot string

func init() {
	_, file, _, _ := runtime.Caller(0)
	testDataRoot = filepath.Dir(file)
}

// SubTestDataPath gets the file path which belongs to test data directory or its subdirectories.
func SubTestDataPath(subPath string) string {
	return filepath.Join(testDataRoot, subPath)
}

func ScanDirectoryPath(subPath string, filter FilterOptions) ([]string, error) {
	dirs, err := os.ReadDir(SubTestDataPath(subPath))
	if err != nil {
		return nil, err
	}
	resourceList := make([]string, 0, len(dirs))
	for _, d := range dirs {
		if d.IsDir() {
			continue
		}
		if filter != nil && !filter(d.Name()) {
			continue
		}
		resourceList = append(resourceList, filepath.Join(subPath, d.Name()))
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

func WithCMData(fetch func() map[string]string) ResourceOptions {
	return func(obj client.Object) {
		cm, ok := obj.(*corev1.ConfigMap)
		if !ok {
			return
		}
		cm.Data = fetch()
	}
}

func WithNewFakeCMData(keysAndValues ...string) func() map[string]string {
	return func() map[string]string {
		return WithMap(keysAndValues...)
	}
}

func WithMap(keysAndValues ...string) map[string]string {
	// ignore mismatching for kvs
	m := make(map[string]string, len(keysAndValues)/2)
	for i := 0; i+1 < len(keysAndValues); i += 2 {
		m[keysAndValues[i]] = keysAndValues[i+1]
	}
	return m
}

func WithLabels(keysAndValues ...string) ResourceOptions {
	return func(k8sObject client.Object) {
		k8sObject.SetLabels(WithMap(keysAndValues...))
	}
}

func WithAnnotations(keysAndValues ...string) ResourceOptions {
	return func(k8sObject client.Object) {
		k8sObject.SetAnnotations(WithMap(keysAndValues...))
	}
}

type ComponentSelector = func(*dbaasv1alpha1.ClusterDefinitionSpec) *dbaasv1alpha1.ClusterDefinitionComponent
type ClusterComponentSelector = func(spec *dbaasv1alpha1.ClusterSpec) *dbaasv1alpha1.ClusterComponent
type ContainerSelector = func(containers []corev1.Container) *corev1.Container

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

func WithClusterComponent(selector ClusterComponentSelector, fn func(component *dbaasv1alpha1.ClusterComponent)) ResourceOptions {
	return func(obj client.Object) {
		cluster, ok := obj.(*dbaasv1alpha1.Cluster)
		if !ok {
			return
		}
		if component := selector(&cluster.Spec); component != nil {
			fn(component)
		}
	}
}

func ComponentIndexSelector(index int) ClusterComponentSelector {
	return func(spec *dbaasv1alpha1.ClusterSpec) *dbaasv1alpha1.ClusterComponent {
		if len(spec.Components) <= index {
			return nil
		}
		return &spec.Components[index]
	}
}

func WithComponentTypeName(name string, typeName string) func(component *dbaasv1alpha1.ClusterComponent) {
	return func(component *dbaasv1alpha1.ClusterComponent) {
		component.Type = typeName
		component.Name = name
	}
}

func WithClusterDef(clusterDefRef string) ResourceOptions {
	return func(obj client.Object) {
		if cluster, ok := obj.(*dbaasv1alpha1.Cluster); ok {
			cluster.Spec.ClusterDefRef = clusterDefRef
		}
	}
}

func WithClusterVersion(clusterVersionRef string) ResourceOptions {
	return func(obj client.Object) {
		if cluster, ok := obj.(*dbaasv1alpha1.Cluster); ok {
			cluster.Spec.ClusterVersionRef = clusterVersionRef
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

// for statefulset
type PodOptions = func(spec *corev1.PodSpec)

func WithPodTemplate(options ...PodOptions) ResourceOptions {
	return func(obj client.Object) {
		stsObj, ok := obj.(*appsv1.StatefulSet)
		if !ok {
			return
		}
		podSpec := &stsObj.Spec.Template.Spec
		for _, ops := range options {
			ops(podSpec)
		}
	}
}

func WithPodVolumeMount(selector ContainerSelector, fn func(container *corev1.Container)) PodOptions {
	return func(spec *corev1.PodSpec) {
		if container := selector(spec.Containers); container != nil {
			fn(container)
		}
	}
}

func WithConfigmapVolume(cmName string, volumeName string) PodOptions {
	return func(spec *corev1.PodSpec) {
		if spec.Volumes == nil {
			spec.Volumes = make([]corev1.Volume, 0)
		}
		spec.Volumes = append(spec.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
				},
			},
		})
	}
}

func WithContainerIndexSelector(index int) ContainerSelector {
	return func(containers []corev1.Container) *corev1.Container {
		if len(containers) <= index {
			return nil
		}
		return &containers[index]
	}
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
