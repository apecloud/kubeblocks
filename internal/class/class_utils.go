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
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/ghodss/yaml"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/constant"
)

// GetCustomClassConfigMapName Returns the name of the ConfigMap containing the custom classes
func GetCustomClassConfigMapName(cdName string, componentName string) string {
	return fmt.Sprintf("kb.classes.custom.%s.%s", cdName, componentName)
}

// ChooseComponentClasses Choose the classes to be used for a given component with some constraints
func ChooseComponentClasses(classes map[string]*ComponentClass, filters map[string]resource.Quantity) *ComponentClass {
	var candidates []*ComponentClass
	for _, cls := range classes {
		cpu, ok := filters[corev1.ResourceCPU.String()]
		if ok && !cpu.Equal(cls.CPU) {
			continue
		}
		memory, ok := filters[corev1.ResourceMemory.String()]
		if ok && !memory.Equal(cls.Memory) {
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

func GetClassFamilies(dynamic dynamic.Interface) (map[string]*v1alpha1.ClassFamily, error) {
	objs, err := dynamic.Resource(types.ClassFamilyGVR()).Namespace("").List(context.TODO(), metav1.ListOptions{
		//LabelSelector: types.ClassFamilyProviderLabelKey,
	})
	if err != nil {
		return nil, err
	}
	var classFamilyList v1alpha1.ClassFamilyList
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(objs.UnstructuredContent(), &classFamilyList); err != nil {
		return nil, err
	}

	result := make(map[string]*v1alpha1.ClassFamily)
	for _, cf := range classFamilyList.Items {
		if _, ok := cf.GetLabels()[types.ClassFamilyProviderLabelKey]; !ok {
			continue
		}
		result[cf.GetName()] = &cf
	}
	return result, nil
}

// GetClasses Get all classes, including kubeblocks default classes and user custom classes
func GetClasses(client kubernetes.Interface, cdName string) (map[string]map[string]*ComponentClass, error) {
	selector := fmt.Sprintf("%s=%s,%s", constant.ClusterDefLabelKey, cdName, types.ClassProviderLabelKey)
	cmList, err := client.CoreV1().ConfigMaps(metav1.NamespaceAll).List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, err
	}
	return ParseClasses(cmList)
}

func ParseClasses(cmList *corev1.ConfigMapList) (map[string]map[string]*ComponentClass, error) {
	var (
		componentClasses = make(map[string]map[string]*ComponentClass)
	)
	for _, cm := range cmList.Items {
		if _, ok := cm.GetLabels()[types.ClassProviderLabelKey]; !ok {
			continue
		}
		level := cm.GetLabels()[types.ClassLevelLabelKey]
		switch level {
		case "component":
			componentType := cm.GetLabels()[constant.KBAppComponentDefRefLabelKey]
			if componentType == "" {
				return nil, fmt.Errorf("failed to find component type")
			}
			classes, err := ParseComponentClasses(cm.Data)
			if err != nil {
				return nil, err
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
		case "cluster":
			// TODO
		default:
			return nil, fmt.Errorf("invalid class level: %s", level)
		}
	}

	return componentClasses, nil
}

type classVersion int64

// ParseComponentClasses parse configmap.data to component classes
func ParseComponentClasses(data map[string]string) (map[string]*ComponentClass, error) {
	versions := make(map[classVersion][]*ComponentClassFamilyDef)

	for k, v := range data {
		// ConfigMap data key follows the format: families-[version]
		// version is the timestamp in unix microseconds which class is created
		parts := strings.SplitAfterN(k, "-", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid key: %s", k)
		}
		version, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid key: %s", k)
		}
		var families []*ComponentClassFamilyDef
		if err := yaml.Unmarshal([]byte(v), &families); err != nil {
			return nil, err
		}
		versions[classVersion(version)] = families
	}

	genClassDef := func(nameTpl string, bodyTpl string, vars []string, args []string) (ComponentClassDef, error) {
		var def ComponentClassDef
		values := make(map[string]interface{})
		for index, key := range vars {
			values[key] = args[index]
		}

		classStr, err := renderTemplate(bodyTpl, values)
		if err != nil {
			return def, err
		}

		if err = yaml.Unmarshal([]byte(classStr), &def); err != nil {
			return def, err
		}

		def.Name, err = renderTemplate(nameTpl, values)
		if err != nil {
			return def, err
		}
		return def, nil
	}

	parser := func(family *ComponentClassFamilyDef, series ComponentClassSeriesDef, class ComponentClassDef) (*ComponentClass, error) {
		var (
			err error
			def = class
		)

		if len(class.Args) > 0 {
			def, err = genClassDef(series.Name, family.Template, family.Vars, class.Args)
			if err != nil {
				return nil, err
			}

			if class.Name != "" {
				def.Name = class.Name
			}
		}

		result := &ComponentClass{
			Name:   def.Name,
			Family: family.Family,
			CPU:    resource.MustParse(def.CPU),
			Memory: resource.MustParse(def.Memory),
		}

		for _, disk := range def.Storage {
			result.Storage = append(result.Storage, &Disk{
				Name:  disk.Name,
				Class: disk.Class,
				Size:  resource.MustParse(disk.Size),
			})
		}

		return result, nil
	}

	result := make(map[string]*ComponentClass)
	for _, families := range versions {
		for _, family := range families {
			for _, series := range family.Series {
				for _, class := range series.Classes {
					out, err := parser(family, series, class)
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

// BuildClassDefinitionVersion generate the key in the configmap data field
func BuildClassDefinitionVersion() string {
	return fmt.Sprintf("version-%s", time.Now().Format("20060102150405"))
}
