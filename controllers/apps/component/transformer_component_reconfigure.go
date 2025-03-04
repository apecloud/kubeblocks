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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type componentReconfigureTransformer struct{}

var _ graph.Transformer = &componentReconfigureTransformer{}

func (t *componentReconfigureTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	if model.IsObjectDeleting(transCtx.ComponentOrig) {
		return nil
	}

	runningObjs, err := getFileTemplateObjects(transCtx)
	if err != nil {
		return err
	}

	protoObjs, err := buildFileTemplateObjects(transCtx)
	if err != nil {
		return err
	}

	toCreate, toDelete, toUpdate := mapDiff(runningObjs, protoObjs)

	return t.handleReconfigure(transCtx, dag, runningObjs, protoObjs, toCreate, toDelete, toUpdate)
}

func (t *componentReconfigureTransformer) handleReconfigure(transCtx *componentTransformContext, dag *graph.DAG,
	runningObjs, protoObjs map[string]*corev1.ConfigMap, toCreate, toDelete, toUpdate sets.Set[string]) error {
	if len(toCreate) > 0 || len(toDelete) > 0 {
		// since pod volumes changed, the workload will be restarted, cancel the queued reconfigure.
		return t.cancelQueuedReconfigure(transCtx, dag)
	}

	changes := t.templateFileChanges(transCtx, runningObjs, protoObjs, toUpdate)
	if len(changes) > 0 {
		msg, err := json.Marshal(changes)
		if err != nil {
			return err
		}
		if err := t.queueReconfigure(transCtx, dag, string(msg)); err != nil {
			return err
		}
		return intctrlutil.NewDelayedRequeueError(time.Second, fmt.Sprintf("pending reconfigure task: %s", msg))
	}

	return t.reconfigure(transCtx, dag)
}

func (t *componentReconfigureTransformer) templateFileChanges(transCtx *componentTransformContext,
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

func (t *componentReconfigureTransformer) absoluteFilePath(transCtx *componentTransformContext, tpl, file string) string {
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

func (t *componentReconfigureTransformer) reconfigure(transCtx *componentTransformContext, dag *graph.DAG) error {
	replicas, changes, err := t.reconfigureStatus(transCtx, dag)
	if err != nil {
		return err
	}
	for _, replica := range replicas {
		if err := t.reconfigureReplica(transCtx, changes, replica); err != nil {
			return err
		}
		if err := t.reconfigured(transCtx, dag, []string{replica}); err != nil {
			return err
		}
	}
	return nil
}

func (t *componentReconfigureTransformer) reconfigureReplica(transCtx *componentTransformContext,
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

func (t *componentReconfigureTransformer) reconfigureReplicaTemplate(transCtx *componentTransformContext,
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
				return nil // disabled by the external
			}
		}
		// TODO: variables, dynamic render
		if tpl.Reconfigure != nil {
			actionName := component.UDFReconfigureActionName(tpl)
			args := lifecycle.FileTemplateChanges(changes.Created, changes.Removed, changes.Updated)
			return lfa.UserDefined(transCtx.Context, transCtx.Client, nil, actionName, tpl.Reconfigure, args)
		}
		return lfa.Reconfigure(transCtx.Context, transCtx.Client, nil, changes.Created, changes.Removed, changes.Updated)
	}

	lfa, err := lifecycle.New(synthesizedComp.Namespace, synthesizedComp.ClusterName, synthesizedComp.Name,
		lifecycleActions, synthesizedComp.TemplateVars, pod)
	if err != nil {
		return err
	}

	if err := reconfigure(lfa); err != nil {
		if errors.Is(err, lifecycle.ErrPreconditionFailed) {
			return intctrlutil.NewDelayedRequeueError(time.Second,
				fmt.Sprintf("replicas not up-to-date when reconfiguring: %s", err.Error()))
		}
		return err
	}
	return nil
}

func (t *componentReconfigureTransformer) queueReconfigure(transCtx *componentTransformContext, dag *graph.DAG, changes string) error {
	return t.updateReconfigureStatus(transCtx, dag, func(s *component.ReplicaStatus) {
		s.Reconfigured = ptr.To(changes)
	})
}

func (t *componentReconfigureTransformer) cancelQueuedReconfigure(transCtx *componentTransformContext, dag *graph.DAG) error {
	return t.updateReconfigureStatus(transCtx, dag, func(s *component.ReplicaStatus) {
		s.Reconfigured = nil
	})
}

func (t *componentReconfigureTransformer) reconfigured(transCtx *componentTransformContext, dag *graph.DAG, replicas []string) error {
	replicasSet := sets.New(replicas...)
	return t.updateReconfigureStatus(transCtx, dag, func(s *component.ReplicaStatus) {
		if replicasSet.Has(s.Name) {
			s.Reconfigured = ptr.To("")
		}
	})
}

func (t *componentReconfigureTransformer) updateReconfigureStatus(
	transCtx *componentTransformContext, dag *graph.DAG, f func(*component.ReplicaStatus)) error {
	its, inDag := t.itsObject(transCtx, dag)
	if its == nil {
		return nil
	}
	err := component.UpdateReplicasStatusFunc(its, func(r *component.ReplicasStatus) error {
		for i := range r.Status {
			f(&r.Status[i])
		}
		return nil
	})
	if err != nil {
		return err
	}

	if !inDag {
		runningIts := transCtx.RunningWorkload.(*workloads.InstanceSet)
		if !reflect.DeepEqual(runningIts.Annotations, its.Annotations) {
			graphCli, _ := transCtx.Client.(model.GraphClient)
			// its is copied from running its, and only the annotation is modified.
			graphCli.Update(dag, nil, its)
		}
	}
	return nil
}

func (t *componentReconfigureTransformer) reconfigureStatus(
	transCtx *componentTransformContext, dag *graph.DAG) ([]string, map[string]fileTemplateChanges, error) {
	its, _ := t.itsObject(transCtx, dag)
	if its == nil {
		return nil, nil, nil
	}

	var err1 error
	changes := map[string]fileTemplateChanges{}
	replicas, err2 := component.GetReplicasStatusFunc(its, func(r component.ReplicaStatus) bool {
		if r.Reconfigured == nil || len(*r.Reconfigured) == 0 {
			return false
		}
		if len(changes) == 0 {
			err1 = json.Unmarshal([]byte(*r.Reconfigured), &changes)
		}
		return true
	})
	if err2 != nil {
		return nil, nil, err2
	}
	if err1 != nil {
		return nil, nil, err1
	}
	return replicas, changes, nil
}

func (t *componentReconfigureTransformer) itsObject(transCtx *componentTransformContext, dag *graph.DAG) (*workloads.InstanceSet, bool) {
	graphCli, _ := transCtx.Client.(model.GraphClient)
	objs := graphCli.FindAll(dag, &workloads.InstanceSet{})
	if len(objs) > 0 {
		return objs[0].(*workloads.InstanceSet), true // reuse it
	}
	if transCtx.RunningWorkload != nil {
		return transCtx.RunningWorkload.(*workloads.InstanceSet).DeepCopy(), false
	}
	return nil, false
}
