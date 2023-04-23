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

package class

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/ghodss/yaml"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/constant"
)

// ValidateComponentClass check if component classDefRef or resource is invalid
func ValidateComponentClass(comp *v1alpha1.ClusterComponentSpec, compClasses map[string]map[string]*v1alpha1.ComponentClassInstance) (*v1alpha1.ComponentClassInstance, error) {
	classes := compClasses[comp.ComponentDefRef]
	var cls *v1alpha1.ComponentClassInstance
	switch {
	case comp.ClassDefRef != nil && comp.ClassDefRef.Class != "":
		if classes == nil {
			return nil, fmt.Errorf("can not find classes for component %s", comp.ComponentDefRef)
		}
		cls = classes[comp.ClassDefRef.Class]
		if cls == nil {
			return nil, fmt.Errorf("unknown component class %s", comp.ClassDefRef.Class)
		}
	case classes != nil:
		cls = ChooseComponentClasses(classes, comp.Resources.Requests)
		if cls == nil {
			return nil, fmt.Errorf("can not find matching class for component %s", comp.Name)
		}
	}
	return cls, nil
}

// GetCustomClassObjectName Returns the name of the ComponentClassDefinition object containing the custom classes
func GetCustomClassObjectName(cdName string, componentName string) string {
	return fmt.Sprintf("kb.classes.custom.%s.%s", cdName, componentName)
}

// ChooseComponentClasses Choose the classes to be used for a given component with some constraints
func ChooseComponentClasses(classes map[string]*v1alpha1.ComponentClassInstance, resources corev1.ResourceList) *v1alpha1.ComponentClassInstance {
	var candidates []*v1alpha1.ComponentClassInstance
	for _, cls := range classes {
		if !resources.Cpu().IsZero() && !resources.Cpu().Equal(cls.CPU) {
			continue
		}
		if !resources.Memory().IsZero() && !resources.Memory().Equal(cls.Memory) {
			continue
		}
		candidates = append(candidates, cls)
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.Sort(ByClassCPUAndMemory(candidates))
	return candidates[0]
}

func GetClasses(classDefinitionList v1alpha1.ComponentClassDefinitionList) (map[string]map[string]*v1alpha1.ComponentClassInstance, error) {
	var (
		compTypeLabel    = "apps.kubeblocks.io/component-def-ref"
		componentClasses = make(map[string]map[string]*v1alpha1.ComponentClassInstance)
	)
	for _, classDefinition := range classDefinitionList.Items {
		componentType := classDefinition.GetLabels()[compTypeLabel]
		if componentType == "" {
			return nil, fmt.Errorf("can not find component type label %s", compTypeLabel)
		}
		var (
			err     error
			classes = make(map[string]*v1alpha1.ComponentClassInstance)
		)
		if classDefinition.GetGeneration() != 0 &&
			classDefinition.Status.ObservedGeneration == classDefinition.GetGeneration() {
			for idx := range classDefinition.Status.Classes {
				cls := classDefinition.Status.Classes[idx]
				classes[cls.Name] = &cls
			}
		} else {
			classes, err = ParseComponentClasses(classDefinition)
			if err != nil {
				return nil, err
			}
		}
		if _, ok := componentClasses[componentType]; !ok {
			componentClasses[componentType] = classes
		} else {
			for k, v := range classes {
				if _, exists := componentClasses[componentType][k]; exists {
					return nil, fmt.Errorf("duplicate component class %s", k)
				}
				componentClasses[componentType][k] = v
			}
		}
	}

	return componentClasses, nil
}

// GetResourceConstraints get all resource constraints
func GetResourceConstraints(dynamic dynamic.Interface) (map[string]*v1alpha1.ComponentResourceConstraint, error) {
	objs, err := dynamic.Resource(types.ComponentResourceConstraintGVR()).List(context.TODO(), metav1.ListOptions{
		//LabelSelector: types.ResourceConstraintProviderLabelKey,
	})
	if err != nil {
		return nil, err
	}
	var constraintsList v1alpha1.ComponentResourceConstraintList
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(objs.UnstructuredContent(), &constraintsList); err != nil {
		return nil, err
	}

	result := make(map[string]*v1alpha1.ComponentResourceConstraint)
	for idx := range constraintsList.Items {
		cf := constraintsList.Items[idx]
		if _, ok := cf.GetLabels()[types.ResourceConstraintProviderLabelKey]; !ok {
			continue
		}
		result[cf.GetName()] = &cf
	}
	return result, nil
}

// ListClassesByClusterDefinition get all classes, including kubeblocks default classes and user custom classes
func ListClassesByClusterDefinition(client dynamic.Interface, cdName string) (map[string]map[string]*v1alpha1.ComponentClassInstance, error) {
	selector := fmt.Sprintf("%s=%s,%s", constant.ClusterDefLabelKey, cdName, types.ClassProviderLabelKey)
	objs, err := client.Resource(types.ComponentClassDefinitionGVR()).Namespace("").List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, err
	}
	var classDefinitionList v1alpha1.ComponentClassDefinitionList
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(objs.UnstructuredContent(), &classDefinitionList); err != nil {
		return nil, err
	}
	return GetClasses(classDefinitionList)
}

// ParseComponentClasses parse ComponentClassDefinition to component classes
func ParseComponentClasses(classDefinition v1alpha1.ComponentClassDefinition) (map[string]*v1alpha1.ComponentClassInstance, error) {
	genClass := func(nameTpl string, bodyTpl string, vars []string, args []string) (v1alpha1.ComponentClass, error) {
		var result v1alpha1.ComponentClass
		values := make(map[string]interface{})
		for index, key := range vars {
			values[key] = args[index]
		}

		classStr, err := renderTemplate(bodyTpl, values)
		if err != nil {
			return result, err
		}

		if err = yaml.Unmarshal([]byte(classStr), &result); err != nil {
			return result, err
		}

		name, err := renderTemplate(nameTpl, values)
		if err != nil {
			return result, err
		}
		result.Name = name
		return result, nil
	}

	parser := func(group v1alpha1.ComponentClassGroup, series v1alpha1.ComponentClassSeries, class v1alpha1.ComponentClass) (*v1alpha1.ComponentClassInstance, error) {
		if len(class.Args) > 0 {
			cls, err := genClass(series.NamingTemplate, group.Template, group.Vars, class.Args)
			if err != nil {
				return nil, err
			}

			if class.Name == "" && cls.Name != "" {
				class.Name = cls.Name
			}
			class.CPU = cls.CPU
			class.Memory = cls.Memory
			class.Volumes = cls.Volumes
		}
		result := &v1alpha1.ComponentClassInstance{
			ComponentClass: v1alpha1.ComponentClass{
				Name:   class.Name,
				CPU:    class.CPU,
				Memory: class.Memory,
			},
			ResourceConstraintRef: group.ResourceConstraintRef,
		}
		for _, volume := range class.Volumes {
			result.Volumes = append(result.Volumes, v1alpha1.Volume{
				Name:             volume.Name,
				StorageClassName: volume.StorageClassName,
				Size:             volume.Size,
			})
		}
		return result, nil
	}

	result := make(map[string]*v1alpha1.ComponentClassInstance)
	for _, group := range classDefinition.Spec.Groups {
		for _, series := range group.Series {
			for _, class := range series.Classes {
				out, err := parser(group, series, class)
				if err != nil {
					return nil, err
				}
				if _, exists := result[out.Name]; exists {
					return nil, fmt.Errorf("duplicate component class name: %s", out.Name)
				}
				result[out.Name] = out
			}
		}
	}
	return result, nil
}

func renderTemplate(tpl string, values map[string]interface{}) (string, error) {
	engine, err := template.New("").Parse(tpl)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err := engine.Execute(&buf, values); err != nil {
		return "", err
	}
	return buf.String(), nil
}
