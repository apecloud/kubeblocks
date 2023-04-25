/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package class

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"text/template"

	"github.com/ghodss/yaml"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/constant"
)

var (
	Any = v1alpha1.ClassDefRef{}
)

type Manager struct {
	sync.RWMutex

	classes map[string][]*ComponentClassWithRef
}

func NewManager(classDefinitionList v1alpha1.ComponentClassDefinitionList) (*Manager, error) {
	classes, err := GetClasses(classDefinitionList)
	if err != nil {
		return nil, err
	}
	return &Manager{classes: classes}, nil
}

// HasClass returns true if the component has the specified class
func (r *Manager) HasClass(compType string, classDefRef v1alpha1.ClassDefRef) bool {
	compClasses, ok := r.classes[compType]
	if !ok || len(compClasses) == 0 {
		return false
	}
	if classDefRef == Any {
		return true
	}

	idx := slices.IndexFunc(compClasses, func(v *ComponentClassWithRef) bool {
		if classDefRef.Name != "" && classDefRef.Name != v.ClassDefRef.Name {
			return false
		}
		if classDefRef.Class != v.ClassDefRef.Class {
			return false
		}
		return true
	})
	return idx >= 0
}

// ChooseClass Choose the class to be used for a given component with some constraints
func (r *Manager) ChooseClass(comp *v1alpha1.ClusterComponentSpec) (*ComponentClassWithRef, error) {
	var (
		cls     *ComponentClassWithRef
		classes = r.classes[comp.ComponentDefRef]
	)
	switch {
	case comp.ClassDefRef != nil && comp.ClassDefRef.Class != "":
		if classes == nil {
			return nil, fmt.Errorf("can not find classes for component %s", comp.ComponentDefRef)
		}
		for _, v := range classes {
			if comp.ClassDefRef.Name != "" && comp.ClassDefRef.Name != v.ClassDefRef.Name {
				continue
			}

			if comp.ClassDefRef.Class != v.ClassDefRef.Class {
				continue
			}

			if cls == nil || cls.Cmp(&v.ComponentClassInstance) > 0 {
				cls = v
			}
		}
		if cls == nil {
			return nil, fmt.Errorf("unknown component class %s", comp.ClassDefRef.Class)
		}
	case classes != nil:
		candidates := filterClassByResources(classes, comp.Resources.Requests)
		if len(candidates) == 0 {
			return nil, fmt.Errorf("can not find matching class for component %s", comp.Name)
		}
		sort.Sort(ByClassResource(candidates))
		cls = candidates[0]
	}
	return cls, nil
}

func (r *Manager) GetClasses() map[string][]*ComponentClassWithRef {
	return r.classes
}

func filterClassByResources(classes []*ComponentClassWithRef, resources corev1.ResourceList) []*ComponentClassWithRef {
	var candidates []*ComponentClassWithRef
	for _, cls := range classes {
		if !resources.Cpu().IsZero() && !resources.Cpu().Equal(cls.CPU) {
			continue
		}
		if !resources.Memory().IsZero() && !resources.Memory().Equal(cls.Memory) {
			continue
		}
		candidates = append(candidates, cls)
	}
	return candidates
}

// GetManager get class manager which manages kubeblocks default classes and user custom classes
func GetManager(client dynamic.Interface, cdName string) (*Manager, error) {
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
	return NewManager(classDefinitionList)
}

// GetCustomClassObjectName Returns the name of the ComponentClassDefinition object containing the custom classes
func GetCustomClassObjectName(cdName string, componentName string) string {
	return fmt.Sprintf("kb.classes.custom.%s.%s", cdName, componentName)
}

func GetClasses(classDefinitionList v1alpha1.ComponentClassDefinitionList) (map[string][]*ComponentClassWithRef, error) {
	var (
		compTypeLabel    = "apps.kubeblocks.io/component-def-ref"
		componentClasses = make(map[string][]*ComponentClassWithRef)
	)
	for _, classDefinition := range classDefinitionList.Items {
		componentType := classDefinition.GetLabels()[compTypeLabel]
		if componentType == "" {
			return nil, fmt.Errorf("can not find component type label %s", compTypeLabel)
		}
		var (
			classes []*ComponentClassWithRef
		)
		if classDefinition.GetGeneration() != 0 &&
			classDefinition.Status.ObservedGeneration == classDefinition.GetGeneration() {
			for idx := range classDefinition.Status.Classes {
				cls := classDefinition.Status.Classes[idx]
				classes = append(classes, &ComponentClassWithRef{
					ComponentClassInstance: cls,
					ClassDefRef:            v1alpha1.ClassDefRef{Name: classDefinition.Name, Class: cls.Name},
				})
			}
		} else {
			classMap, err := ParseComponentClasses(classDefinition)
			if err != nil {
				return nil, err
			}
			for k, v := range classMap {
				classes = append(classes, &ComponentClassWithRef{
					ClassDefRef:            k,
					ComponentClassInstance: *v,
				})
			}
		}
		componentClasses[componentType] = append(componentClasses[componentType], classes...)
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

// ParseComponentClasses parse ComponentClassDefinition to component classes
func ParseComponentClasses(classDefinition v1alpha1.ComponentClassDefinition) (map[v1alpha1.ClassDefRef]*v1alpha1.ComponentClassInstance, error) {
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

	result := make(map[v1alpha1.ClassDefRef]*v1alpha1.ComponentClassInstance)
	for _, group := range classDefinition.Spec.Groups {
		for _, series := range group.Series {
			for _, class := range series.Classes {
				out, err := parser(group, series, class)
				if err != nil {
					return nil, err
				}
				key := v1alpha1.ClassDefRef{Name: classDefinition.Name, Class: out.Name}
				if _, exists := result[key]; exists {
					return nil, fmt.Errorf("component class name conflict: %s", out.Name)
				}
				result[key] = out
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
