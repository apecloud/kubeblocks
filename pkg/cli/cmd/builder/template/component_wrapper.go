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

package template

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/cli/printer"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/factory"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

type templateRenderWorkflow struct {
	renderedOpts RenderedOptions
	clusterYaml  string
	localObjects []client.Object

	clusterDefObj     *appsv1alpha1.ClusterDefinition
	clusterVersionObj *appsv1alpha1.ClusterVersion

	clusterDefComponents     []appsv1alpha1.ClusterComponentDefinition
	clusterVersionComponents []appsv1alpha1.ClusterComponentVersion
}

func (w *templateRenderWorkflow) Do(outputDir string) error {
	var err error
	var cluster *appsv1alpha1.Cluster

	cli := newMockClient(w.localObjects)
	ctx := intctrlutil.RequestCtx{
		Ctx: context.Background(),
		Log: log.Log.WithName("ctool"),
	}

	if cluster, err = w.createClusterObject(); err != nil {
		return err
	}
	ctx.Log.V(1).Info(fmt.Sprintf("cluster object : %v", cluster))

	components := w.clusterDefComponents
	if w.renderedOpts.ConfigSpec != "" {
		comp := w.getComponentWithConfigSpec(w.renderedOpts.ConfigSpec)
		if comp == nil {
			return core.MakeError("config spec[%s] is not found", w.renderedOpts.ConfigSpec)
		}
		components = []appsv1alpha1.ClusterComponentDefinition{*comp}
	}

	for _, component := range components {
		synthesizedComponent, objects, err := generateComponentObjects(w, ctx, cli, component.Name, cluster)
		if err != nil {
			return err
		}
		if len(objects) == 0 {
			continue
		}
		if err := w.dumpRenderedTemplates(outputDir, objects, component, w.renderedOpts.ConfigSpec, synthesizedComponent, cluster); err != nil {
			return err
		}
	}
	return nil
}

func (w *templateRenderWorkflow) getComponentName(componentType string, cluster *appsv1alpha1.Cluster) (string, error) {
	clusterCompSpec := cluster.Spec.GetDefNameMappingComponents()[componentType]
	if len(clusterCompSpec) == 0 {
		return "", core.MakeError("component[%s] is not defined in cluster definition", componentType)
	}
	return clusterCompSpec[0].Name, nil
}

func checkTemplateExist[T any](arrs []T, name string) bool {
	for _, a := range arrs {
		v := reflect.ValueOf(a)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		v = v.FieldByName("Name")
		if v.Kind() == reflect.String && v.String() == name {
			return true
		}
	}
	return false
}

func (w *templateRenderWorkflow) getComponentWithConfigSpec(name string) *appsv1alpha1.ClusterComponentDefinition {
	var compMap map[string]*appsv1alpha1.ClusterComponentVersion

	if w.clusterVersionObj != nil {
		compMap = w.clusterVersionObj.Spec.GetDefNameMappingComponents()
	}
	for _, component := range w.clusterDefComponents {
		if checkTemplateExist(component.ConfigSpecs, name) {
			return &component
		}
		if checkTemplateExist(component.ScriptSpecs, name) {
			return &component
		}
		if compMap != nil && compMap[component.Name] != nil {
			if checkTemplateExist(compMap[component.Name].ConfigSpecs, name) {
				return &component
			}
		}
	}
	return nil
}

func (w *templateRenderWorkflow) createClusterObject() (*appsv1alpha1.Cluster, error) {
	if w.clusterYaml != "" {
		return CustomizedObjFromYaml(w.clusterYaml, generics.ClusterSignature)
	}
	return mockClusterObject(w.clusterDefObj, w.renderedOpts, w.clusterVersionObj), nil
}

func (w *templateRenderWorkflow) dumpRenderedTemplates(outputDir string, objects []client.Object, componentDef appsv1alpha1.ClusterComponentDefinition, name string, synthesizedComponent *component.SynthesizedComponent, cluster *appsv1alpha1.Cluster) error {
	fromTemplate := func(component *component.SynthesizedComponent) []appsv1alpha1.ComponentTemplateSpec {
		templates := make([]appsv1alpha1.ComponentTemplateSpec, 0, len(component.ConfigTemplates)+len(component.ScriptTemplates))
		for _, tpl := range component.ConfigTemplates {
			templates = append(templates, tpl.ComponentTemplateSpec)
		}
		templates = append(templates, component.ScriptTemplates...)
		return templates
	}
	foundConfigSpec := func(component *component.SynthesizedComponent, name string) *appsv1alpha1.ComponentConfigSpec {
		for i := range component.ConfigTemplates {
			template := &component.ConfigTemplates[i]
			if template.Name == name {
				return template
			}
		}
		return nil
	}

	for _, template := range fromTemplate(synthesizedComponent) {
		if name != "" && name != template.Name {
			continue
		}
		comName, _ := w.getComponentName(componentDef.Name, cluster)
		cfgName := core.GetComponentCfgName(cluster.Name, comName, template.Name)
		if err := dumpTemplate(template, outputDir, objects, componentDef.Name, cfgName, foundConfigSpec(synthesizedComponent, template.Name)); err != nil {
			return err
		}
	}
	return nil
}

func getClusterDefComponents(clusterDefObj *appsv1alpha1.ClusterDefinition, componentName string) []appsv1alpha1.ClusterComponentDefinition {
	if componentName == "" {
		return clusterDefObj.Spec.ComponentDefs
	}
	component := clusterDefObj.GetComponentDefByName(componentName)
	if component == nil {
		return nil
	}
	return []appsv1alpha1.ClusterComponentDefinition{*component}
}

func getClusterVersionComponents(clusterVersionObj *appsv1alpha1.ClusterVersion, componentName string) []appsv1alpha1.ClusterComponentVersion {
	if clusterVersionObj == nil {
		return nil
	}
	if componentName == "" {
		return clusterVersionObj.Spec.ComponentVersions
	}
	componentMap := clusterVersionObj.Spec.GetDefNameMappingComponents()
	if component, ok := componentMap[componentName]; ok {
		return []appsv1alpha1.ClusterComponentVersion{*component}
	}
	return nil
}

func NewWorkflowTemplateRender(helmTemplateDir string, opts RenderedOptions, clusterDef, clusterVersion string) (*templateRenderWorkflow, error) {
	foundCVResource := func(allObjects []client.Object) *appsv1alpha1.ClusterVersion {
		cvObj := GetTypedResourceObjectBySignature(allObjects, generics.ClusterVersionSignature,
			func(object client.Object) bool {
				if clusterVersion != "" {
					return object.GetName() == clusterVersion
				}
				return object.GetAnnotations() != nil && object.GetAnnotations()[constant.DefaultClusterVersionAnnotationKey] == "true"
			})
		if clusterVersion == "" && cvObj == nil {
			cvObj = GetTypedResourceObjectBySignature(allObjects, generics.ClusterVersionSignature)
		}
		return cvObj
	}

	if _, err := os.Stat(helmTemplateDir); err != nil {
		panic("cluster definition yaml file is required")
	}

	allObjects, err := CreateObjectsFromDirectory(helmTemplateDir)
	if err != nil {
		return nil, err
	}

	clusterDefObj := GetTypedResourceObjectBySignature(allObjects, generics.ClusterDefinitionSignature, WithResourceName(clusterDef))
	if clusterDefObj == nil {
		return nil, core.MakeError("cluster definition object is not found in helm template directory[%s]", helmTemplateDir)
	}

	// hack apiserver auto filefield
	checkAndFillPortProtocol(clusterDefObj.Spec.ComponentDefs)

	var cdComponents []appsv1alpha1.ClusterComponentDefinition
	if cdComponents = getClusterDefComponents(clusterDefObj, opts.ComponentName); cdComponents == nil {
		return nil, core.MakeError("component[%s] is not defined in cluster definition", opts.ComponentName)
	}
	clusterVersionObj := foundCVResource(allObjects)
	return &templateRenderWorkflow{
		renderedOpts:             opts,
		clusterDefObj:            clusterDefObj,
		clusterVersionObj:        clusterVersionObj,
		localObjects:             allObjects,
		clusterDefComponents:     cdComponents,
		clusterVersionComponents: getClusterVersionComponents(clusterVersionObj, opts.ComponentName),
	}, nil
}

func dumpTemplate(template appsv1alpha1.ComponentTemplateSpec, outputDir string, objects []client.Object, componentDefName string, cfgName string, configSpec *appsv1alpha1.ComponentConfigSpec) error {
	output := filepath.Join(outputDir, cfgName)
	fmt.Printf("dump rendering template spec: %s, output directory: %s\n",
		printer.BoldYellow(fmt.Sprintf("%s.%s", componentDefName, template.Name)), output)

	if err := os.MkdirAll(output, 0755); err != nil {
		return err
	}

	var ok bool
	var cm *corev1.ConfigMap
	for _, obj := range objects {
		if cm, ok = obj.(*corev1.ConfigMap); !ok || !isTemplateOwner(cm, configSpec, cfgName) {
			continue
		}
		if isTemplateObject(cm, cfgName) {
			for file, val := range cm.Data {
				if err := os.WriteFile(filepath.Join(output, file), []byte(val), 0755); err != nil {
					return err
				}
			}
		}
		if isTemplateEnvFromObject(cm, configSpec, cfgName) {
			val, err := yaml.Marshal(cm)
			if err != nil {
				return err
			}
			yamlFile := fmt.Sprintf("%s.yaml", cm.Name[len(cfgName)+1:])
			if err := os.WriteFile(filepath.Join(output, yamlFile), val, 0755); err != nil {
				return err
			}
		}
	}
	return nil
}

func isTemplateObject(cm *corev1.ConfigMap, cfgName string) bool {
	return cm.Name == cfgName
}

func isTemplateEnvFromObject(cm *corev1.ConfigMap, configSpec *appsv1alpha1.ComponentConfigSpec, cfgName string) bool {
	if configSpec == nil || len(configSpec.AsEnvFrom) == 0 || configSpec.ConfigConstraintRef == "" || len(cm.Labels) == 0 {
		return false
	}
	return cm.Labels[constant.CMTemplateNameLabelKey] == configSpec.Name && strings.HasPrefix(cm.Name, cfgName)
}

func isTemplateOwner(cm *corev1.ConfigMap, configSpec *appsv1alpha1.ComponentConfigSpec, cfgName string) bool {
	return isTemplateObject(cm, cfgName) || isTemplateEnvFromObject(cm, configSpec, cfgName)
}

func generateComponentObjects(w *templateRenderWorkflow, reqCtx intctrlutil.RequestCtx, cli *mockClient,
	componentType string, cluster *appsv1alpha1.Cluster) (*component.SynthesizedComponent, []client.Object, error) {
	cmGVK := generics.ToGVK(&corev1.ConfigMap{})

	objs := make([]client.Object, 0)
	cli.SetResourceHandler(&ResourceHandler{
		Matcher: []ResourceMatcher{func(obj runtime.Object) bool {
			res := obj.(client.Object)
			return generics.ToGVK(res) == cmGVK &&
				res.GetLabels() != nil &&
				res.GetLabels()[constant.CMTemplateNameLabelKey] != ""
		}},
		Handler: func(obj runtime.Object) error {
			objs = append(objs, obj.(client.Object))
			return nil
		},
	})

	compName, err := w.getComponentName(componentType, cluster)
	if err != nil {
		return nil, nil, err
	}
	dag := graph.NewDAG()
	root := builder.NewReplicatedStateMachineBuilder(cluster.Namespace, fmt.Sprintf("%s-%s", cluster.Name, compName)).GetObject()
	model.NewGraphClient(nil).Root(dag, nil, root, nil)

	compSpec := cluster.Spec.GetComponentByName(compName)
	synthesizeComp, err := component.BuildSynthesizedComponentWrapper(reqCtx, cli, cluster, compSpec)
	if err != nil {
		return nil, nil, err
	}
	secret := factory.BuildConnCredential(w.clusterDefObj, cluster, synthesizeComp)
	cli.AppendMockObjects(secret)

	// TODO(xingran & zhangtao): This is the logic before the componentDefinition refactoring. component.Create has already been removed during the refactoring. If any functionality depends on it, please check to replace it with the new approach.
	// if err = component.Create(ctx, cli); err != nil {
	// 	return nil, nil, err
	// }

	return synthesizeComp, objs, nil
}
