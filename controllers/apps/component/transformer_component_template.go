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
	"crypto/sha256"
	"fmt"
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
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

	runningObjs, protoObjs, err := t.prepareFileTemplateObjects(transCtx)
	if err != nil {
		return err
	}

	toCreate, toDelete, toUpdate := mapDiff(runningObjs, protoObjs)

	t.handleTemplateObjectChanges(transCtx, dag, runningObjs, protoObjs, toCreate, toDelete, toUpdate)

	for _, obj := range protoObjs {
		component.AddInstanceAssistantObject(transCtx.SynthesizeComponent, obj)
	}

	if err = t.buildPodVolumes(transCtx); err != nil {
		return err
	}

	// build config templates for the workload
	return t.buildConfigTemplates(transCtx, runningObjs, protoObjs, toUpdate)
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

func (t *componentFileTemplateTransformer) prepareFileTemplateObjects(transCtx *componentTransformContext) (map[string]*corev1.ConfigMap, map[string]*corev1.ConfigMap, error) {
	runningObjs, err := t.getFileTemplateObjects(transCtx)
	if err != nil {
		return nil, nil, err
	}

	protoObjs, err := t.buildFileTemplateObjects(transCtx)
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

func (t *componentFileTemplateTransformer) getFileTemplateObjects(transCtx *componentTransformContext) (map[string]*corev1.ConfigMap, error) {
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

func (t *componentFileTemplateTransformer) buildFileTemplateObjects(transCtx *componentTransformContext) (map[string]*corev1.ConfigMap, error) {
	objs := make(map[string]*corev1.ConfigMap)
	for _, tpl := range transCtx.SynthesizeComponent.FileTemplates {
		// If the file template is managed by external, the CM object has been rendered and created by the external manager.
		if isExternalManaged(tpl) {
			continue
		}

		obj, err := t.buildFileTemplateObject(transCtx, tpl)
		if err != nil {
			return nil, err
		}
		objs[obj.Name] = obj
	}
	return objs, nil
}

func (t *componentFileTemplateTransformer) buildFileTemplateObject(transCtx *componentTransformContext, tpl component.SynthesizedFileTemplate) (*corev1.ConfigMap, error) {
	var (
		compDef         = transCtx.CompDef
		synthesizedComp = transCtx.SynthesizeComponent
	)

	data, err := t.buildFileTemplateData(transCtx, tpl)
	if err != nil {
		return nil, err
	}

	configHash := tpl.ConfigHash
	if configHash == nil {
		hash, err1 := intctrlutil.ComputeHash(data)
		if err1 != nil {
			return nil, err1
		}
		configHash = &hash
	}

	objName := fileTemplateObjectName(transCtx.SynthesizeComponent, tpl.Name)
	obj := builder.NewConfigMapBuilder(synthesizedComp.Namespace, objName).
		AddLabelsInMap(synthesizedComp.StaticLabels).
		AddLabelsInMap(constant.GetCompLabelsWithDef(synthesizedComp.ClusterName, synthesizedComp.Name, compDef.Name)).
		AddLabels(kubeBlockFileTemplateLabelKey, "true").
		AddAnnotationsInMap(synthesizedComp.StaticAnnotations).
		AddAnnotations(constant.CMInsConfigurationHashLabelKey, *configHash).
		SetData(data).
		GetObject()
	if err := setCompOwnershipNFinalizer(transCtx.Component, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func (t *componentFileTemplateTransformer) buildFileTemplateData(transCtx *componentTransformContext, tpl component.SynthesizedFileTemplate) (map[string]string, error) {
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
	return t.renderFileTemplateData(transCtx, tpl, cmObj.Data)
}

func (t *componentFileTemplateTransformer) renderFileTemplateData(transCtx *componentTransformContext,
	fileTemplate component.SynthesizedFileTemplate, data map[string]string) (map[string]string, error) {
	var (
		synthesizedComp = transCtx.SynthesizeComponent
		rendered        = make(map[string]string)
	)

	variables := make(map[string]any)
	for k, v := range synthesizedComp.TemplateVars {
		variables[k] = v
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

func (t *componentFileTemplateTransformer) buildConfigTemplates(transCtx *componentTransformContext,
	runningObjs, protoObjs map[string]*corev1.ConfigMap, toUpdate sets.Set[string]) error {
	var (
		synthesizedComp = transCtx.SynthesizeComponent
		configHash      = func(tpl component.SynthesizedFileTemplate) *string {
			if isExternalManaged(tpl) {
				return tpl.ConfigHash
			}
			objName := fileTemplateObjectName(transCtx.SynthesizeComponent, tpl.Name)
			obj := protoObjs[objName]
			if obj == nil {
				return nil
			}
			val, ok := obj.Annotations[constant.CMInsConfigurationHashLabelKey]
			if !ok {
				return nil
			}
			return &val
		}
		action = func(tpl component.SynthesizedFileTemplate) *kbappsv1.Action {
			if tpl.ReconfigureRequired != nil && !*tpl.ReconfigureRequired {
				return nil
			}
			if tpl.ReconfigureRequired != nil && *tpl.ReconfigureRequired && tpl.ReconfigureAction != nil {
				return tpl.ReconfigureAction
			}
			if tpl.Reconfigure != nil {
				return tpl.Reconfigure
			}
			if synthesizedComp.LifecycleActions.ComponentLifecycleActions != nil {
				return synthesizedComp.LifecycleActions.ComponentLifecycleActions.Reconfigure
			}
			return nil
		}
		actionName = func(tpl component.SynthesizedFileTemplate) string {
			if tpl.ReconfigureRequired != nil && !*tpl.ReconfigureRequired {
				return ""
			}
			if tpl.ReconfigureRequired != nil && *tpl.ReconfigureRequired && tpl.ReconfigureAction != nil {
				return component.UserReconfigureActionName(tpl)
			}
			if tpl.Reconfigure != nil {
				return component.CMPDReconfigureActionName(tpl)
			}
			return "" // lifecycle default reconfigure action or empty
		}
		templateChanges = t.templateFileChanges(transCtx, runningObjs, protoObjs, toUpdate)
		parameters      = func(tpl component.SynthesizedFileTemplate) map[string]string {
			changes, ok := templateChanges[tpl.Name]
			if !ok {
				return tpl.Variables
			}
			result := lifecycle.FileTemplateChanges(changes.Created, changes.Removed, changes.Updated)
			maps.Copy(result, tpl.Variables)
			return result
		}
	)
	for _, tpl := range synthesizedComp.FileTemplates {
		if tpl.Config {
			config := workloads.ConfigTemplate{
				Name:                  tpl.Name,
				ConfigHash:            configHash(tpl),
				Restart:               tpl.RestartOnFileChange,
				Reconfigure:           action(tpl),
				ReconfigureActionName: actionName(tpl),
				Parameters:            parameters(tpl),
			}
			synthesizedComp.Configs = append(synthesizedComp.Configs, config)
		}
	}
	slices.SortFunc(synthesizedComp.Configs, func(a, b workloads.ConfigTemplate) int {
		return strings.Compare(a.Name, b.Name)
	})
	return nil
}

func (t *componentFileTemplateTransformer) templateFileChanges(transCtx *componentTransformContext,
	runningObjs, protoObjs map[string]*corev1.ConfigMap, update sets.Set[string]) map[string]fileTemplateChanges {
	diff := func(obj *corev1.ConfigMap, rData, pData map[string]string) fileTemplateChanges {
		var (
			tplName = fileTemplateNameFromObject(transCtx.SynthesizeComponent, obj)
			items   = make([][]string, 3)
		)

		toAdd, toDelete, toUpdate := mapDiff(rData, pData)

		items[0], items[1] = sets.List(toAdd), sets.List(toDelete)
		for item := range toUpdate {
			if !reflect.DeepEqual(rData[item], pData[item]) {
				absPath := t.absoluteFilePath(transCtx, tplName, item)
				if len(absPath) > 0 {
					checksum := sha256.Sum256([]byte(pData[item]))
					items[2] = append(items[2], fmt.Sprintf("%s:%x", absPath, checksum))
				}
			}
		}

		for i := range items {
			slices.Sort(items[i])
		}

		return fileTemplateChanges{
			Created: strings.Join(items[0], ","),
			Removed: strings.Join(items[1], ","),
			Updated: strings.Join(items[2], ","),
		}
	}

	result := make(map[string]fileTemplateChanges)
	for name := range update {
		rData, pData := runningObjs[name].Data, protoObjs[name].Data
		if !reflect.DeepEqual(rData, pData) {
			tplName := fileTemplateNameFromObject(transCtx.SynthesizeComponent, runningObjs[name])
			result[tplName] = diff(runningObjs[name], rData, pData)
		}
	}
	return result
}

func (t *componentFileTemplateTransformer) absoluteFilePath(transCtx *componentTransformContext, tpl, file string) string {
	var (
		synthesizedComp = transCtx.SynthesizeComponent
	)

	var volName, mountPath string
	for _, fileTpl := range synthesizedComp.FileTemplates {
		if fileTpl.Name == tpl {
			volName = fileTpl.VolumeName
			break
		}
	}
	if volName == "" {
		return "" // has no volumes specified
	}

	for _, container := range synthesizedComp.PodSpec.Containers {
		for _, mount := range container.VolumeMounts {
			if mount.Name == volName {
				mountPath = mount.MountPath
				break
			}
		}
		if mountPath != "" {
			break
		}
	}
	if mountPath == "" {
		return "" // the template is not mounted, ignore it
	}

	return filepath.Join(mountPath, file)
}

type fileTemplateChanges struct {
	Created string `json:"created,omitempty"`
	Removed string `json:"removed,omitempty"`
	Updated string `json:"updated,omitempty"`
}
