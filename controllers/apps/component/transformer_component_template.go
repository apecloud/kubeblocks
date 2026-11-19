/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	kubeBlockFileTemplateLabelKey    = "apps.kubeblocks.io/file-template"
	fileTemplateChangesAnnotationKey = "apps.kubeblocks.io/file-template-changes"
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

	renderCtx, err := t.fileTemplateRenderContext(transCtx, dag, nil)
	if err != nil {
		return err
	}
	runningObjs, protoObjs, err := t.prepareFileTemplateObjects(renderCtx)
	if err != nil {
		return err
	}

	toCreate, toDelete, toUpdate := mapDiff(runningObjs, protoObjs)

	if err = t.buildPodVolumes(transCtx); err != nil {
		return err
	}
	synth := transCtx.SynthesizeComponent
	for _, tpl := range synth.FileTemplates {
		if !tpl.Config {
			continue
		}
		name := fileTemplateObjectName(synth, tpl.Name)
		obj := protoObjs[name]
		if obj != nil {
			t.preserveFileTemplateChanges(transCtx, tpl.Name, runningObjs[name], obj)
		}
		synth.Configs = append(synth.Configs, t.buildConfigTemplate(transCtx, tpl, obj))
	}
	slices.SortFunc(synth.Configs, func(a, b workloads.ConfigTemplate) int {
		return strings.Compare(a.Name, b.Name)
	})
	instanceRunning, instanceDesired, err := t.prepareInstanceFileTemplateObjects(transCtx, dag, protoObjs)
	if err != nil {
		return err
	}

	t.handleTemplateObjectChanges(transCtx, dag, runningObjs, protoObjs, toCreate, toDelete, toUpdate)
	instanceCreate, _, instanceUpdate := mapDiff(instanceRunning, instanceDesired)
	t.handleTemplateObjectChanges(transCtx, dag, instanceRunning, instanceDesired, instanceCreate, nil, instanceUpdate)

	for _, tpl := range transCtx.SynthesizeComponent.FileTemplates {
		if obj := t.instanceAssistantObject(transCtx, tpl, protoObjs); obj != nil {
			component.AddInstanceAssistantObject(transCtx.SynthesizeComponent, obj)
		}
	}

	return nil
}

func (t *componentFileTemplateTransformer) fileTemplateRenderContext(transCtx *componentTransformContext,
	dag *graph.DAG, instance *kbappsv1.InstanceTemplate) (*componentTransformContext, error) {
	graphCli, _ := transCtx.Client.(model.GraphClient)
	reader := &varsReader{transCtx.Client, graphCli, dag}
	vars := slices.Clone(transCtx.CompDef.Spec.Vars)
	vars = append(vars, transCtx.SynthesizeComponent.SidecarVars...)
	resolved, err := component.ResolveFileTemplateVars(transCtx.Context, reader, transCtx.SynthesizeComponent, vars, instance)
	if err != nil {
		return nil, err
	}
	synth := *transCtx.SynthesizeComponent
	synth.TemplateVars = make(map[string]string)
	maps.Copy(synth.TemplateVars, transCtx.SynthesizeComponent.TemplateVars)
	maps.Copy(synth.TemplateVars, resolved)
	result := *transCtx
	result.SynthesizeComponent = &synth
	return &result, nil
}

func (t *componentFileTemplateTransformer) prepareInstanceFileTemplateObjects(transCtx *componentTransformContext,
	dag *graph.DAG, baseDesired map[string]*corev1.ConfigMap) (map[string]*corev1.ConfigMap, map[string]*corev1.ConfigMap, error) {
	running, desired := make(map[string]*corev1.ConfigMap), make(map[string]*corev1.ConfigMap)
	synth := transCtx.SynthesizeComponent
	synth.InstanceConfigs = make(map[string][]workloads.ConfigTemplate)
	for i := range synth.Instances {
		instance := &synth.Instances[i]
		templates, err := component.InstanceFileTemplates(synth, instance)
		if err != nil {
			return nil, nil, err
		}
		if ptr.Deref(instance.Replicas, int32(1)) == 0 || len(templates) == 0 {
			continue
		}
		renderCtx, err := t.fileTemplateRenderContext(transCtx, dag, instance)
		if err != nil {
			return nil, nil, err
		}
		for _, tpl := range templates {
			obj, err := t.buildFileTemplateObject(renderCtx, tpl)
			if err != nil {
				return nil, nil, fmt.Errorf("render instance template %s config %s: %w", instance.Name, tpl.Name, err)
			}
			baseName := fileTemplateObjectName(synth, tpl.Name)
			base := baseDesired[baseName]
			sameData := base != nil && reflect.DeepEqual(base.Data, obj.Data)
			configObj := base
			if !sameData {
				current, err := t.prepareSpecializedConfigMap(transCtx, instance.Name, tpl, obj)
				if err != nil {
					return nil, nil, err
				}
				desired[obj.Name] = obj
				if current != nil {
					running[obj.Name] = current
				}
				configObj = obj
			}
			config := t.buildConfigTemplate(transCtx, tpl, configObj)
			if config.Volume == nil && !sameData {
				return nil, nil, fmt.Errorf("instance template %s config %s has no ConfigMap volume %s", instance.Name, tpl.Name, tpl.VolumeName)
			}
			if config.Volume != nil {
				config.Volume.ConfigMapName = configObj.Name
			}
			baseConfig, found := slices.BinarySearchFunc(synth.Configs, config.Name, func(config workloads.ConfigTemplate, name string) int {
				return strings.Compare(config.Name, name)
			})
			if sameData && found && reflect.DeepEqual(config, synth.Configs[baseConfig]) {
				continue
			}
			synth.InstanceConfigs[instance.Name] = append(synth.InstanceConfigs[instance.Name], config)
		}
		slices.SortFunc(synth.InstanceConfigs[instance.Name], func(a, b workloads.ConfigTemplate) int {
			return strings.Compare(a.Name, b.Name)
		})
	}
	return running, desired, nil
}

func (t *componentFileTemplateTransformer) prepareSpecializedConfigMap(transCtx *componentTransformContext,
	instanceName string, tpl component.SynthesizedFileTemplate, desired *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	desired.Name = instanceFileTemplateObjectName(transCtx.SynthesizeComponent, instanceName, tpl.Name)
	desired.Labels[instanceConfigMapLabelKey] = "true"
	desired.Labels[instanceConfigMapOwnerLabelKey] = string(transCtx.Component.UID)
	current := &corev1.ConfigMap{}
	if err := transCtx.Client.Get(transCtx.Context, client.ObjectKeyFromObject(desired), current); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		current = nil
	} else if !isCurrentComponentConfigMap(transCtx.Component, current) {
		return nil, fmt.Errorf("the ConfigMap name for instance template %s config %s conflicts", instanceName, tpl.Name)
	}
	placements := sets.New[string]()
	if current != nil {
		desired.ResourceVersion = current.ResourceVersion
		placements = configMapPlacements(current.Annotations[instanceConfigMapPlacementsAnnotationKey])
	}
	if ptr.Deref(transCtx.SynthesizeComponent.EnableInstanceAPI, false) {
		placements.Insert(sets.List(configMapPlacements(transCtx.Component.Annotations[constant.KBAppMultiClusterPlacementKey]))...)
	}
	if len(placements) > 0 {
		desired.Annotations[instanceConfigMapPlacementsAnnotationKey] = strings.Join(sets.List(placements), ",")
	}
	t.preserveFileTemplateChanges(transCtx, tpl.Name, current, desired)
	if tpl.ConfigHash != nil {
		hash, err := intctrlutil.ComputeHash(struct {
			Version string
			Data    map[string]string
		}{*tpl.ConfigHash, desired.Data})
		if err != nil {
			return nil, err
		}
		desired.Annotations[constant.CMInsConfigurationHashLabelKey] = hash
	}
	return current, nil
}

func instanceFileTemplateObjectName(synth *component.SynthesizedComponent, instance, config string) string {
	// Length prefixes distinguish names containing separators without relying on truncation.
	hash := sha256.Sum256([]byte(fmt.Sprintf("%d:%s%d:%s%d:%s", len(synth.FullCompName), synth.FullCompName,
		len(instance), instance, len(config), config)))
	prefix := strings.TrimRight(synth.FullCompName[:min(len(synth.FullCompName), 220)], ".-")
	return fmt.Sprintf("%s-itcfg-%x", prefix, hash[:10])
}

func (t *componentFileTemplateTransformer) instanceAssistantObject(transCtx *componentTransformContext,
	tpl component.SynthesizedFileTemplate, protoObjs map[string]*corev1.ConfigMap) client.Object {
	if isExternalManaged(tpl) {
		if len(tpl.Template) == 0 {
			return nil
		}
		namespace := tpl.Namespace
		if len(namespace) == 0 {
			namespace = transCtx.SynthesizeComponent.Namespace
		}
		return &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      tpl.Template,
			},
		}
	}
	objName := fileTemplateObjectName(transCtx.SynthesizeComponent, tpl.Name)
	return protoObjs[objName]
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
		runningObj, ok := runningObjs[fileTemplateObjectName(transCtx.SynthesizeComponent, tpl.Name)]
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
		if obj.Labels[instanceConfigMapLabelKey] == "true" {
			continue
		}
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
		ptpl, err := tpl.Clone()
		if err != nil {
			return nil, err
		}
		ptpl, err = ptpl.Parse(val)
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

func isExternalManaged(tpl component.SynthesizedFileTemplate) bool {
	return ptr.Deref(tpl.ExternalManaged, false)
}

func (t *componentFileTemplateTransformer) buildConfigTemplate(transCtx *componentTransformContext,
	tpl component.SynthesizedFileTemplate, cm *corev1.ConfigMap) workloads.ConfigTemplate {
	config := workloads.ConfigTemplate{
		Name:            tpl.Name,
		Volume:          t.configVolumeBinding(transCtx, tpl),
		Restart:         tpl.RestartOnFileChange,
		Parameters:      tpl.Variables,
		ReconfigureArgs: tpl.ReconfigureArgs,
	}
	if action := transCtx.SynthesizeComponent.LifecycleActions.ComponentLifecycleActions; action != nil {
		config.Reconfigure = action.Reconfigure
	}
	switch {
	case !ptr.Deref(tpl.ReconfigureRequired, true):
		config.Reconfigure = nil
	case ptr.Deref(tpl.ReconfigureRequired, false) && tpl.ReconfigureAction != nil:
		config.Reconfigure = tpl.ReconfigureAction
		config.ReconfigureActionName = component.UserReconfigureActionName(tpl)
	case tpl.Reconfigure != nil:
		config.Reconfigure = tpl.Reconfigure
		config.ReconfigureActionName = component.CMPDReconfigureActionName(tpl)
	}
	if isExternalManaged(tpl) {
		config.ConfigHash = tpl.ConfigHash
	}
	if cm != nil {
		if hash, ok := cm.Annotations[constant.CMInsConfigurationHashLabelKey]; ok {
			config.ConfigHash = &hash
		}
		if encoded := cm.Annotations[fileTemplateChangesAnnotationKey]; encoded != "" {
			var changes fileTemplateChanges
			if json.Unmarshal([]byte(encoded), &changes) == nil {
				config.Parameters = lifecycle.FileTemplateChanges(changes.Created, changes.Removed, changes.Updated)
				maps.Copy(config.Parameters, tpl.Variables)
			}
		}
	}
	return config
}

func (t *componentFileTemplateTransformer) configVolumeBinding(transCtx *componentTransformContext,
	tpl component.SynthesizedFileTemplate) *workloads.ConfigMapVolumeBinding {
	if isExternalManaged(tpl) {
		return nil
	}
	for _, volume := range transCtx.SynthesizeComponent.PodSpec.Volumes {
		if volume.Name == tpl.VolumeName && volume.ConfigMap != nil {
			return &workloads.ConfigMapVolumeBinding{VolumeName: volume.Name, ConfigMapName: volume.ConfigMap.Name}
		}
	}
	return nil
}

func (t *componentFileTemplateTransformer) preserveFileTemplateChanges(transCtx *componentTransformContext,
	tplName string, running, desired *corev1.ConfigMap) {
	if running == nil {
		return
	}
	if reflect.DeepEqual(running.Data, desired.Data) {
		if changes, ok := running.Annotations[fileTemplateChangesAnnotationKey]; ok {
			desired.Annotations[fileTemplateChangesAnnotationKey] = changes
		}
		return
	}
	changes := t.fileTemplateChanges(transCtx, tplName, running.Data, desired.Data)
	encoded, _ := json.Marshal(changes)
	desired.Annotations[fileTemplateChangesAnnotationKey] = string(encoded)
}

func (t *componentFileTemplateTransformer) fileTemplateChanges(transCtx *componentTransformContext,
	tplName string, rData, pData map[string]string) fileTemplateChanges {
	var (
		items = make([][]string, 3)
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
