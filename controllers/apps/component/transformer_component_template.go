/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package component

import (
	"fmt"
	"maps"
	"reflect"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	kubeBlockFileTemplateLabelKey = "apps.kubeblocks.io/file-template"
)

type componentFileTemplateTransformer struct{}

var _ graph.Transformer = &componentFileTemplateTransformer{}

func (t *componentFileTemplateTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	if isCompDeleting(transCtx.ComponentOrig) {
		return nil
	}

	if err := t.precheck(transCtx); err != nil {
		return err
	}

	runningObjs, protoObjs, err := prepareFileTemplateObjects(transCtx)
	if err != nil {
		return err
	}

	toCreate, toDelete, toUpdate := mapDiff(runningObjs, protoObjs)

	t.handleTemplateObjectChanges(transCtx, dag, runningObjs, protoObjs, toCreate, toDelete, toUpdate)

	for _, obj := range protoObjs {
		component.AddInstanceAssistantObject(transCtx.SynthesizeComponent, obj)
	}

	return t.buildPodVolumes(transCtx)
}

func (t *componentFileTemplateTransformer) precheck(transCtx *componentTransformContext) error {
	for _, tpl := range transCtx.SynthesizeComponent.FileTemplates {
		if len(tpl.Template) == 0 {
			return fmt.Errorf("config/script template has no template specified: %s", tpl.Name)
		}
	}
	return nil
}

func (t *componentFileTemplateTransformer) handleTemplateObjectChanges(transCtx *componentTransformContext,
	dag *graph.DAG, runningObjs, protoObjs map[string]*corev1.ConfigMap, toCreate, toDelete, toUpdate sets.Set[string]) {
	graphCli, _ := transCtx.Client.(model.GraphClient)
	for name := range toCreate {
		graphCli.Create(dag, protoObjs[name])
	}
	for name := range toDelete {
		graphCli.Delete(dag, runningObjs[name])
	}
	for name := range toUpdate {
		runningObj, protoObj := runningObjs[name], protoObjs[name]
		if !reflect.DeepEqual(runningObj.Data, protoObj.Data) ||
			!reflect.DeepEqual(runningObj.Labels, protoObj.Labels) ||
			!reflect.DeepEqual(runningObj.Annotations, protoObj.Annotations) {
			graphCli.Update(dag, runningObj, protoObj)
		}
	}
}

func (t *componentFileTemplateTransformer) buildPodVolumes(transCtx *componentTransformContext) error {
	var (
		synthesizedComp = transCtx.SynthesizeComponent
	)
	if synthesizedComp.PodSpec.Volumes == nil {
		synthesizedComp.PodSpec.Volumes = []corev1.Volume{}
	}
	for _, tpl := range synthesizedComp.FileTemplates {
		objName := fileTemplateObjectName(transCtx.SynthesizeComponent, tpl.Name)
		// If the file template is managed by external, the volume mount object should be the external object.
		if isExternalManaged(tpl) {
			objName = tpl.Template
		}
		createFn := func(_ string) corev1.Volume {
			return t.newVolume(tpl, objName)
		}
		synthesizedComp.PodSpec.Volumes =
			intctrlutil.CreateVolumeIfNotExist(synthesizedComp.PodSpec.Volumes, tpl.VolumeName, createFn)
	}
	return nil
}

func (t *componentFileTemplateTransformer) newVolume(tpl component.SynthesizedFileTemplate, objName string) corev1.Volume {
	vol := corev1.Volume{
		Name: tpl.VolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: objName,
				},
				DefaultMode: tpl.DefaultMode,
			},
		},
	}
	if vol.VolumeSource.ConfigMap.DefaultMode == nil {
		vol.VolumeSource.ConfigMap.DefaultMode = ptr.To[int32](0444)
	}
	return vol
}

func prepareFileTemplateObjects(transCtx *componentTransformContext) (map[string]*corev1.ConfigMap, map[string]*corev1.ConfigMap, error) {
	runningObjs, err := getFileTemplateObjects(transCtx)
	if err != nil {
		return nil, nil, err
	}

	protoObjs, err := buildFileTemplateObjects(transCtx)
	if err != nil {
		return nil, nil, err
	}

	for _, tpl := range transCtx.SynthesizeComponent.FileTemplates {
		runningObj, ok := runningObjs[tpl.Name]
		if ok && !model.IsOwnerOf(transCtx.Component, runningObj) {
			return nil, nil, fmt.Errorf("the CM object name for file template %s conflicts", tpl.Name)
		}
	}

	return runningObjs, protoObjs, nil
}

func getFileTemplateObjects(transCtx *componentTransformContext) (map[string]*corev1.ConfigMap, error) {
	var (
		synthesizedComp = transCtx.SynthesizeComponent
	)
	labels := constant.GetCompLabels(synthesizedComp.ClusterName, synthesizedComp.Name)
	labels[kubeBlockFileTemplateLabelKey] = "true"
	opts := []client.ListOption{
		client.MatchingLabels(labels),
		client.InNamespace(synthesizedComp.Namespace),
	}

	cmList := &corev1.ConfigMapList{}
	if err := transCtx.Client.List(transCtx.Context, cmList, opts...); err != nil {
		return nil, err
	}

	objs := make(map[string]*corev1.ConfigMap)
	for i, obj := range cmList.Items {
		objs[obj.Name] = &cmList.Items[i]
	}
	return objs, nil
}

func buildFileTemplateObjects(transCtx *componentTransformContext) (map[string]*corev1.ConfigMap, error) {
	objs := make(map[string]*corev1.ConfigMap)
	for _, tpl := range transCtx.SynthesizeComponent.FileTemplates {
		// If the file template is managed by external, the cm object has been rendered by the external manager.
		if isExternalManaged(tpl) {
			continue
		}
		obj, err := buildFileTemplateObject(transCtx, tpl)
		if err != nil {
			return nil, err
		}
		objs[obj.Name] = obj
	}
	return objs, nil
}

func buildFileTemplateObject(transCtx *componentTransformContext, tpl component.SynthesizedFileTemplate) (*corev1.ConfigMap, error) {
	var (
		compDef         = transCtx.CompDef
		synthesizedComp = transCtx.SynthesizeComponent
	)

	data, err := buildFileTemplateData(transCtx, tpl)
	if err != nil {
		return nil, err
	}

	objName := fileTemplateObjectName(transCtx.SynthesizeComponent, tpl.Name)
	obj := builder.NewConfigMapBuilder(synthesizedComp.Namespace, objName).
		AddLabelsInMap(synthesizedComp.StaticLabels).
		AddLabelsInMap(constant.GetCompLabelsWithDef(synthesizedComp.ClusterName, synthesizedComp.Name, compDef.Name)).
		AddLabels(kubeBlockFileTemplateLabelKey, "true").
		AddAnnotationsInMap(synthesizedComp.StaticAnnotations).
		SetData(data).
		GetObject()
	if err := setCompOwnershipNFinalizer(transCtx.Component, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func buildFileTemplateData(transCtx *componentTransformContext, tpl component.SynthesizedFileTemplate) (map[string]string, error) {
	cmObj, err := func() (*corev1.ConfigMap, error) {
		cm := &corev1.ConfigMap{}
		cmKey := types.NamespacedName{
			Namespace: func() string {
				if len(tpl.Namespace) > 0 {
					return tpl.Namespace
				}
				return "default"
			}(),
			Name: tpl.Template,
		}
		if err := transCtx.Client.Get(transCtx.Context, cmKey, cm); err != nil {
			return nil, err
		}
		return cm, nil
	}()
	if err != nil {
		return nil, err
	}
	return renderFileTemplateData(transCtx, tpl, cmObj.Data)
}

func renderFileTemplateData(transCtx *componentTransformContext,
	fileTemplate component.SynthesizedFileTemplate, data map[string]string) (map[string]string, error) {
	var (
		synthesizedComp = transCtx.SynthesizeComponent
		rendered        = make(map[string]string)
	)

	variables := make(map[string]string)
	if synthesizedComp.TemplateVars != nil {
		maps.Copy(variables, synthesizedComp.TemplateVars)
	}
	for k, v := range fileTemplate.Variables {
		variables[k] = v // override
	}

	tpl := template.New(fileTemplate.Name).Option("missingkey=error").Funcs(sprig.TxtFuncMap())
	for key, val := range data {
		ptpl, err := tpl.Parse(val)
		if err != nil {
			return nil, err
		}
		var buf strings.Builder
		if err = ptpl.Execute(&buf, variables); err != nil {
			return nil, err
		}
		rendered[key] = buf.String()
	}
	return rendered, nil
}

func fileTemplateObjectName(synthesizedComp *component.SynthesizedComponent, tplName string) string {
	return fmt.Sprintf("%s-%s", synthesizedComp.FullCompName, tplName)
}

func fileTemplateNameFromObject(synthesizedComp *component.SynthesizedComponent, obj *corev1.ConfigMap) string {
	name, _ := strings.CutPrefix(obj.Name, fmt.Sprintf("%s-", synthesizedComp.FullCompName))
	return name
}

func isExternalManaged(tpl component.SynthesizedFileTemplate) bool {
	return ptr.Deref(tpl.ExternalManaged, false)
}
