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

package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/controller/builder"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/plan"
	intctrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/apecloud/kubeblocks/internal/generics"
)

type componentedConfigSpec struct {
	component  string
	configSpec appsv1alpha1.ComponentTemplateSpec
}

type templateRenderWorkflow struct {
	renderedOpts  RenderedOptions
	clusterYaml   string
	clusterDefObj *appsv1alpha1.ClusterDefinition
	localObjects  []client.Object

	clusterDefComponents []appsv1alpha1.ClusterComponentDefinition
}

func (w *templateRenderWorkflow) Do(outputDir string) error {
	var err error
	var cluster *appsv1alpha1.Cluster
	var configSpecs []componentedConfigSpec

	cli := newMockClient(w.localObjects)
	ctx := intctrlutil.RequestCtx{
		Ctx: context.Background(),
		Log: log.Log.WithName("ctool"),
	}

	if cluster, err = w.createClusterObject(); err != nil {
		return err
	}
	ctx.Log.V(1).Info(fmt.Sprintf("cluster object : %v", cluster))

	if configSpecs, err = w.getRenderedConfigSpec(); err != nil {
		return err
	}

	ctx.Log.Info("rendering template:")
	for _, tplSpec := range configSpecs {
		ctx.Log.Info(fmt.Sprintf("config spec: %s, template name: %s in the component[%s]",
			tplSpec.configSpec.Name,
			tplSpec.configSpec.TemplateRef,
			tplSpec.component))
	}

	cache := make(map[string]*intctrltypes.ReconcileTask)
	for _, configSpec := range configSpecs {
		task, ok := cache[configSpec.component]
		if !ok {
			param, err := createComponentParams(w, ctx, cli, configSpec.component, cluster)
			if err != nil {
				return err
			}
			cache[configSpec.component] = param
			task = param
		}
		if err := renderTemplates(configSpec.configSpec, outputDir, task); err != nil {
			return err
		}
	}
	return nil
}

func (w *templateRenderWorkflow) Prepare(ctx intctrlutil.RequestCtx, componentType string, cluster *appsv1alpha1.Cluster) (*component.SynthesizedComponent, error) {
	clusterCompDef := w.clusterDefObj.GetComponentDefByName(componentType)
	clusterCompSpecMap := cluster.Spec.GetDefNameMappingComponents()
	clusterCompSpec := clusterCompSpecMap[componentType]

	if clusterCompDef == nil || len(clusterCompSpec) == 0 {
		return nil, cfgcore.MakeError("component[%s] is not defined in cluster definition", componentType)
	}

	return component.BuildComponent(ctx, *cluster, nil, *w.clusterDefObj, *clusterCompDef, clusterCompSpec[0])
}

func (w *templateRenderWorkflow) getRenderedConfigSpec() ([]componentedConfigSpec, error) {
	foundSpec := func(com appsv1alpha1.ClusterComponentDefinition, specName string) (appsv1alpha1.ComponentTemplateSpec, bool) {
		for _, spec := range com.ConfigSpecs {
			if spec.Name == specName {
				return spec.ComponentTemplateSpec, true
			}
		}
		for _, spec := range com.ScriptSpecs {
			if spec.Name == specName {
				return spec, true
			}
		}
		return appsv1alpha1.ComponentTemplateSpec{}, false
	}

	if w.renderedOpts.ConfigSpec != "" {
		for _, com := range w.clusterDefComponents {
			if spec, ok := foundSpec(com, w.renderedOpts.ConfigSpec); ok {
				return []componentedConfigSpec{{com.Name, spec}}, nil
			}
		}
		return nil, cfgcore.MakeError("config spec[%s] is not found", w.renderedOpts.ConfigSpec)
	}

	if !w.renderedOpts.AllConfigSpecs {
		return nil, cfgcore.MakeError("config spec[%s] is not found", w.renderedOpts.ConfigSpec)
	}
	configSpecs := make([]componentedConfigSpec, 0)
	for _, com := range w.clusterDefComponents {
		for _, configSpec := range com.ConfigSpecs {
			configSpecs = append(configSpecs, componentedConfigSpec{com.Name, configSpec.ComponentTemplateSpec})
		}
		for _, configSpec := range com.ScriptSpecs {
			configSpecs = append(configSpecs, componentedConfigSpec{com.Name, configSpec})
		}
	}
	return configSpecs, nil
}

func (w *templateRenderWorkflow) createClusterObject() (*appsv1alpha1.Cluster, error) {
	if w.clusterYaml != "" {
		return CustomizedObjFromYaml(w.clusterYaml, generics.ClusterSignature)
	}

	clusterVersionObj := GetTypedResourceObjectBySignature(w.localObjects, generics.ClusterVersionSignature)
	return mockClusterObject(w.clusterDefObj, w.renderedOpts, clusterVersionObj), nil
}

func NewWorkflowTemplateRender(helmTemplateDir string, opts RenderedOptions) (*templateRenderWorkflow, error) {
	if _, err := os.Stat(helmTemplateDir); err != nil {
		panic("cluster definition yaml file is required")
	}

	allObjects, err := CreateObjectsFromDirectory(helmTemplateDir)
	if err != nil {
		return nil, err
	}

	clusterDefObj := GetTypedResourceObjectBySignature(allObjects, generics.ClusterDefinitionSignature)
	if clusterDefObj == nil {
		return nil, cfgcore.MakeError("cluster definition object is not found in helm template directory[%s]", helmTemplateDir)
	}
	// hack apiserver auto filefield
	checkAndFillPortProtocol(clusterDefObj.Spec.ComponentDefs)

	components := clusterDefObj.Spec.ComponentDefs
	if opts.ComponentName != "" {
		component := clusterDefObj.GetComponentDefByName(opts.ComponentName)
		if component == nil {
			return nil, cfgcore.MakeError("component[%s] is not defined in cluster definition", opts.ComponentName)
		}
		components = []appsv1alpha1.ClusterComponentDefinition{*component}
	}
	return &templateRenderWorkflow{
		renderedOpts:         opts,
		clusterDefObj:        clusterDefObj,
		localObjects:         allObjects,
		clusterDefComponents: components,
	}, nil
}

func checkAndFillPortProtocol(clusterDefComponents []appsv1alpha1.ClusterComponentDefinition) {
	// fix failed to BuildHeadlessSvc
	// failed to render workflow: cue: marshal error: service.spec.ports.0.protocol: undefined field: protocol
	for i := range clusterDefComponents {
		for j := range clusterDefComponents[i].PodSpec.Containers {
			container := &clusterDefComponents[i].PodSpec.Containers[j]
			for k := range container.Ports {
				port := &container.Ports[k]
				if port.Protocol == "" {
					port.Protocol = corev1.ProtocolTCP
				}
			}
		}
	}
}

func renderTemplates(configSpec appsv1alpha1.ComponentTemplateSpec, outputDir string, task *intctrltypes.ReconcileTask) error {
	cfgName := cfgcore.GetComponentCfgName(task.Cluster.Name, task.Component.Name, configSpec.Name)
	output := filepath.Join(outputDir, cfgName)
	log.Log.Info(fmt.Sprintf("dump rendering template spec: %s, output directory: %s", configSpec.Name, output))

	if err := os.MkdirAll(output, 0755); err != nil {
		return err
	}

	var ok bool
	var cm *corev1.ConfigMap
	for _, obj := range *task.Resources {
		if cm, ok = obj.(*corev1.ConfigMap); !ok || cm.Name != cfgName {
			continue
		}
		for file, val := range cm.Data {
			if err := os.WriteFile(filepath.Join(output, file), []byte(val), 0755); err != nil {
				return err
			}
		}
		break
	}
	return nil
}

func createComponentParams(w *templateRenderWorkflow, ctx intctrlutil.RequestCtx, cli client.Client, componentType string, cluster *appsv1alpha1.Cluster) (*intctrltypes.ReconcileTask, error) {
	component, err := w.Prepare(ctx, componentType, cluster)
	if err != nil {
		return nil, err
	}
	clusterVersionObj := GetTypedResourceObjectBySignature(w.localObjects, generics.ClusterVersionSignature)
	task := intctrltypes.InitReconcileTask(w.clusterDefObj, clusterVersionObj, cluster, component)
	secret, err := builder.BuildConnCredential(task.GetBuilderParams())
	if err != nil {
		return nil, err
	}
	// must make sure secret resources are created before workloads resources
	task.AppendResource(secret)
	if err := plan.PrepareComponentResources(ctx, cli, task); err != nil {
		return nil, err
	}
	return task, nil
}
