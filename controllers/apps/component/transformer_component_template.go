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
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	kubeBlockFileTemplate         = "kubeblock-file-templates"
	kubeBlockFileTemplateLabelKey = "apps.kubeblocks.io/file-template"
)

type componentFileTemplateTransformer struct{}

var _ graph.Transformer = &componentFileTemplateTransformer{}

func (t *componentFileTemplateTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	if model.IsObjectDeleting(transCtx.ComponentOrig) {
		return nil
	}

	if err := t.precheck(transCtx); err != nil {
		return err
	}

	runningObjs, err := t.getTemplateObjects(transCtx)
	if err != nil {
		return err
	}

	protoObjs, err := t.buildTemplateObjects(transCtx)
	if err != nil {
		return err
	}

	toCreate, toDelete, toUpdate := mapDiff(runningObjs, protoObjs)

	t.handleTemplateObjectChanges(transCtx, dag, runningObjs, protoObjs, toCreate, toDelete, toUpdate)

	if err = t.buildPodVolumes(transCtx); err != nil {
		return err
	}

	return t.handleReconfigure(transCtx, runningObjs, protoObjs, toCreate, toDelete, toUpdate)
}

func (t *componentFileTemplateTransformer) precheck(transCtx *componentTransformContext) error {
	for _, tpl := range transCtx.SynthesizeComponent.FileTemplates {
		if len(tpl.Template) == 0 {
			return fmt.Errorf("config/script template has no template specified: %s", tpl.Name)
		}
	}
	return nil
}

func (t *componentFileTemplateTransformer) getTemplateObjects(transCtx *componentTransformContext) (map[string]*corev1.ConfigMap, error) {
	synthesizedComp := transCtx.SynthesizeComponent

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

func (t *componentFileTemplateTransformer) buildTemplateObjects(transCtx *componentTransformContext) (map[string]*corev1.ConfigMap, error) {
	objs := make(map[string]*corev1.ConfigMap)
	for _, tpl := range transCtx.SynthesizeComponent.FileTemplates {
		obj, err := t.buildTemplateObject(transCtx, tpl)
		if err != nil {
			return nil, err
		}
		objs[obj.Name] = obj
	}
	return objs, nil
}

func (t *componentFileTemplateTransformer) buildTemplateObject(
	transCtx *componentTransformContext, tpl component.SynthesizedFileTemplate) (*corev1.ConfigMap, error) {
	var (
		compDef         = transCtx.CompDef
		synthesizedComp = transCtx.SynthesizeComponent
	)

	data, err := t.buildTemplateData(transCtx, tpl)
	if err != nil {
		return nil, err
	}

	obj := builder.NewConfigMapBuilder(synthesizedComp.Name, t.templateObjectName(transCtx, tpl.Name)).
		AddLabelsInMap(synthesizedComp.StaticLabels).
		AddLabelsInMap(constant.GetCompLabelsWithDef(synthesizedComp.ClusterName, synthesizedComp.Name, compDef.Name)).
		AddLabels(kubeBlockFileTemplateLabelKey, "true").
		AddAnnotationsInMap(synthesizedComp.StaticAnnotations).
		SetData(data).
		GetObject()
	return obj, nil
}

func (t *componentFileTemplateTransformer) templateObjectName(transCtx *componentTransformContext, tplName string) string {
	synthesizedComp := transCtx.SynthesizeComponent
	return fmt.Sprintf("%s-%s", synthesizedComp.FullCompName, tplName)
}

func (t *componentFileTemplateTransformer) templateNameFromObject(transCtx *componentTransformContext, obj *corev1.ConfigMap) string {
	synthesizedComp := transCtx.SynthesizeComponent
	name, _ := strings.CutPrefix(obj.Name, fmt.Sprintf("%s-", synthesizedComp.FullCompName))
	return name
}

func (t *componentFileTemplateTransformer) buildTemplateData(transCtx *componentTransformContext, tpl component.SynthesizedFileTemplate) (map[string]string, error) {
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
	return t.renderTemplateData(transCtx, tpl, cmObj.Data)
}

func (t *componentFileTemplateTransformer) renderTemplateData(transCtx *componentTransformContext,
	fileTemplate component.SynthesizedFileTemplate, data map[string]string) (map[string]string, error) {
	var (
		synthesizedComp = transCtx.SynthesizeComponent
		rendered        = make(map[string]string)
	)

	variables := synthesizedComp.TemplateVars
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

func (t *componentFileTemplateTransformer) handleTemplateObjectChanges(transCtx *componentTransformContext,
	dag *graph.DAG, runningObjs, protoObjs map[string]*corev1.ConfigMap, toCreate, toDelete, toUpdate sets.Set[string]) {
	graphCli, _ := transCtx.Client.(model.GraphClient)
	for name := range toCreate {
		graphCli.Create(dag, protoObjs[name], appsutil.InDataContext4G())
	}
	for name := range toDelete {
		graphCli.Delete(dag, runningObjs[name], appsutil.InDataContext4G())
	}
	for name := range toUpdate {
		runningObj, protoObj := runningObjs[name], protoObjs[name]
		if !reflect.DeepEqual(runningObj.Data, protoObj.Data) ||
			!reflect.DeepEqual(runningObj.Labels, protoObj.Labels) ||
			!reflect.DeepEqual(runningObj.Annotations, protoObj.Annotations) {
			graphCli.Update(dag, runningObj, protoObj, appsutil.InDataContext4G())
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
		objName := t.templateObjectName(transCtx, tpl.Name)
		synthesizedComp.PodSpec.Volumes = append(synthesizedComp.PodSpec.Volumes, t.newVolume(tpl, objName))
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

func (t *componentFileTemplateTransformer) handleReconfigure(transCtx *componentTransformContext,
	runningObjs, protoObjs map[string]*corev1.ConfigMap, toCreate, toDelete, toUpdate sets.Set[string]) error {
	if len(toCreate) > 0 || len(toDelete) > 0 {
		// since pod volumes changed, the workload will be restarted, cancel the queued reconfigure.
		return t.cancelQueuedReconfigure(transCtx)
	}

	changes := t.templateFileChanges(transCtx, runningObjs, protoObjs, toUpdate)
	if len(changes) > 0 {
		msg, err := json.Marshal(changes)
		if err != nil {
			return err
		}
		if err := t.queueReconfigure(transCtx, string(msg)); err != nil {
			return err
		}
		return intctrlutil.NewDelayedRequeueError(time.Second, fmt.Sprintf("pending reconfigure task: %s", msg))
	}

	return t.reconfigure(transCtx)
}

func (t *componentFileTemplateTransformer) templateFileChanges(transCtx *componentTransformContext,
	runningObjs, protoObjs map[string]*corev1.ConfigMap, update sets.Set[string]) map[string]fileTemplateChanges {
	diff := func(obj *corev1.ConfigMap, rData, pData map[string]string) fileTemplateChanges {
		var (
			tplName = t.templateNameFromObject(transCtx, obj)
			items   = make([][]string, 0, 3)
		)

		toAdd, toDelete, toUpdate := mapDiff(rData, pData)

		items[0], items[1] = sets.List(toAdd), sets.List(toDelete)
		for item := range toUpdate {
			if !reflect.DeepEqual(rData[item], pData[item]) {
				absPath := t.absoluteFilePath(transCtx, tplName, item)
				if len(absPath) > 0 {
					checksum := sha256.Sum256([]byte(pData[item]))
					items[2] = append(items[2], fmt.Sprintf("%s@%x", absPath, checksum))
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
			result[name] = diff(runningObjs[name], rData, pData)
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

func (t *componentFileTemplateTransformer) reconfigure(transCtx *componentTransformContext) error {
	replicas, changes, err := t.getReconfigureStatus(transCtx)
	if err != nil {
		return err
	}
	for _, replica := range replicas {
		if err := t.reconfigureReplica(transCtx, changes, replica); err != nil {
			return err
		}
		if err := t.reconfigured(transCtx, []string{replica}); err != nil {
			return err
		}
	}
	return nil
}

func (t *componentFileTemplateTransformer) reconfigureReplica(transCtx *componentTransformContext,
	changes map[string]fileTemplateChanges, replica string) error {
	var (
		synthesizedComp = transCtx.SynthesizeComponent
	)
	pod := &corev1.Pod{}
	podKey := types.NamespacedName{
		Namespace: synthesizedComp.Namespace,
		Name:      replica,
	}
	if err := transCtx.Client.Get(transCtx.Context, podKey, pod); err != nil {
		return client.IgnoreNotFound(err)
	}
	for _, tpl := range synthesizedComp.FileTemplates {
		if change, ok := changes[tpl.Name]; ok {
			if err := t.reconfigureReplicaTemplate(transCtx, tpl, change, pod); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *componentFileTemplateTransformer) reconfigureReplicaTemplate(transCtx *componentTransformContext,
	tpl component.SynthesizedFileTemplate, changes fileTemplateChanges, pod *corev1.Pod) error {
	var (
		synthesizedComp  = transCtx.SynthesizeComponent
		lifecycleActions = synthesizedComp.LifecycleActions
	)
	if (lifecycleActions == nil || lifecycleActions.Reconfigure == nil) && tpl.Reconfigure == nil {
		return nil // has no reconfigure action defined
	}

	reconfigure := func(lfa lifecycle.Lifecycle) error {
		if tpl.ExternalManaged != nil && *tpl.ExternalManaged {
			if tpl.Reconfigure == nil {
				return nil // disabled
			}
		}
		if tpl.Reconfigure != nil {
			name := component.UDFReconfigureActionName(tpl)
			args := lifecycle.FileTemplateChanges(changes.Created, changes.Removed, changes.Updated)
			return lfa.UserDefined(transCtx.Context, transCtx.Client, nil, name, tpl.Reconfigure, args)
		}
		return lfa.Reconfigure(transCtx.Context, transCtx.Client, nil, changes.Created, changes.Removed, changes.Updated)
	}

	lfa, err := lifecycle.New(synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name,
		lifecycleActions, synthesizedComp.TemplateVars, pod)
	if err != nil {
		return err
	}

	if err := reconfigure(lfa); err != nil {
		if errors.Is(err, lifecycle.ErrActionNotReady) {
			return intctrlutil.NewDelayedRequeueError(time.Second,
				fmt.Sprintf("replicas not up-to-date when reconfiguring: %s", err.Error()))
		}
		return err
	}
	return nil
}

type fileTemplateChanges struct {
	Created string `json:"created,omitempty"`
	Removed string `json:"removed,omitempty"`
	Updated string `json:"updated,omitempty"`
}

func (t *componentFileTemplateTransformer) queueReconfigure(transCtx *componentTransformContext, changes string) error {
	return t.updateReconfigureStatus(transCtx, func(s *component.ReplicaStatus) {
		s.Reconfigured = ptr.To(changes)
	})
}

func (t *componentFileTemplateTransformer) cancelQueuedReconfigure(transCtx *componentTransformContext) error {
	return t.updateReconfigureStatus(transCtx, func(s *component.ReplicaStatus) {
		s.Reconfigured = nil
	})
}

func (t *componentFileTemplateTransformer) reconfigured(transCtx *componentTransformContext, replicas []string) error {
	replicasSet := sets.New(replicas...)
	return t.updateReconfigureStatus(transCtx, func(s *component.ReplicaStatus) {
		if replicasSet.Has(s.Name) {
			s.Reconfigured = ptr.To("")
		}
	})
}

func (t *componentFileTemplateTransformer) updateReconfigureStatus(
	transCtx *componentTransformContext, f func(*component.ReplicaStatus)) error {
	var (
		synthesizedComp = transCtx.SynthesizeComponent
	)
	its := &workloads.InstanceSet{}
	itsKey := types.NamespacedName{
		Namespace: synthesizedComp.Namespace,
		Name:      constant.GenerateWorkloadNamePattern(synthesizedComp.ClusterName, synthesizedComp.Name),
	}
	if err := transCtx.Client.Get(transCtx.Context, itsKey, its); err != nil {
		return err
	}
	return component.UpdateReplicasStatusFunc(its, func(r *component.ReplicasStatus) error {
		for i := range r.Status {
			f(&r.Status[i])
		}
		return nil
	})
}

func (t *componentFileTemplateTransformer) getReconfigureStatus(
	transCtx *componentTransformContext) ([]string, map[string]fileTemplateChanges, error) {
	var (
		synthesizedComp = transCtx.SynthesizeComponent
	)

	its := &workloads.InstanceSet{}
	itsKey := types.NamespacedName{
		Namespace: synthesizedComp.Namespace,
		Name:      constant.GenerateWorkloadNamePattern(synthesizedComp.ClusterName, synthesizedComp.Name),
	}
	if err := transCtx.Client.Get(transCtx.Context, itsKey, its); err != nil {
		return nil, nil, err
	}

	var err2 error
	changes := map[string]fileTemplateChanges{}
	replicas, err1 := component.GetReplicasStatusFunc(its, func(r component.ReplicaStatus) bool {
		if r.Reconfigured == nil || len(*r.Reconfigured) == 0 {
			return false
		}
		if len(changes) == 0 {
			err2 = json.Unmarshal([]byte(*r.Reconfigured), &changes)
		}
		return true
	})
	if err1 != nil {
		return nil, nil, err1
	}
	if err2 != nil {
		return nil, nil, err2
	}
	return replicas, changes, nil
}
