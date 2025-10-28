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
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type componentWorkloadOps struct {
	transCtx       *componentTransformContext
	cli            client.Client
	component      *appsv1.Component
	synthesizeComp *component.SynthesizedComponent
	dag            *graph.DAG

	runningITS            *workloads.InstanceSet
	protoITS              *workloads.InstanceSet
	desiredCompPodNameSet sets.Set[string]
	runningItsPodNameSet  sets.Set[string]
}

func newComponentWorkloadOps(transCtx *componentTransformContext,
	cli client.Client,
	synthesizedComp *component.SynthesizedComponent,
	comp *appsv1.Component,
	runningITS *workloads.InstanceSet,
	protoITS *workloads.InstanceSet,
	dag *graph.DAG) (*componentWorkloadOps, error) {
	runningITSPodNames, err := component.GeneratePodNamesByITS(runningITS)
	if err != nil {
		return nil, err
	}
	protoITSCopy := protoITS.DeepCopy()
	protoITSCopy.Status = *runningITS.Status.DeepCopy()
	protoITSPodNames, err := component.GeneratePodNamesByITS(protoITSCopy)
	if err != nil {
		return nil, err
	}
	return &componentWorkloadOps{
		transCtx:              transCtx,
		cli:                   cli,
		component:             comp,
		synthesizeComp:        synthesizedComp,
		runningITS:            runningITS,
		protoITS:              protoITS,
		dag:                   dag,
		desiredCompPodNameSet: sets.New(protoITSPodNames...),
		runningItsPodNameSet:  sets.New(runningITSPodNames...),
	}, nil
}

func (r *componentWorkloadOps) horizontalScale() error {
	var (
		in  = r.runningItsPodNameSet.Difference(r.desiredCompPodNameSet)
		out = r.desiredCompPodNameSet.Difference(r.runningItsPodNameSet)
	)
	if err := r.dataReplicationTask(); err != nil {
		return err
	}
	if in.Len() != 0 || out.Len() != 0 {
		r.transCtx.EventRecorder.Eventf(r.component,
			corev1.EventTypeNormal,
			"HorizontalScale",
			"start horizontal scale component %s of cluster %s from %d to %d",
			r.synthesizeComp.Name, r.synthesizeComp.ClusterName, int(*r.runningITS.Spec.Replicas), r.synthesizeComp.Replicas)
	}
	return nil
}

func (r *componentWorkloadOps) dataReplicationTask() error {
	_, hasDataActionDefined := hasMemberJoinNDataActionDefined(r.synthesizeComp.LifecycleActions)
	if !hasDataActionDefined {
		return nil
	}

	var (
		// new replicas to be submitted to InstanceSet
		newReplicas = r.desiredCompPodNameSet.Difference(r.runningItsPodNameSet).UnsortedList()
		// replicas can be used as the source replica to dump data
		sourceReplicas = sets.New[string]()
		// replicas are in provisioning and the data has not been loaded
		provisioningReplicas []string
		// replicas are not provisioned
		unprovisionedReplicas = r.runningItsPodNameSet.Clone()
	)
	for _, replica := range r.runningITS.Status.InstanceStatus {
		if !r.runningItsPodNameSet.Has(replica.PodName) {
			continue // to be deleted
		}
		if replica.Provisioned {
			unprovisionedReplicas.Delete(replica.PodName)
		}
		if replica.DataLoaded != nil && !*replica.DataLoaded {
			provisioningReplicas = append(provisioningReplicas, replica.PodName)
			continue
		}
		if replica.MemberJoined == nil || *replica.MemberJoined {
			sourceReplicas.Insert(replica.PodName)
		}
	}

	if r.runningITS.IsInInitializing() || len(newReplicas) == 0 && unprovisionedReplicas.Len() == 0 && len(provisioningReplicas) == 0 {
		return nil
	}

	// choose the source replica
	source, err := r.sourceReplica(r.synthesizeComp.LifecycleActions.DataDump, sourceReplicas)
	if err != nil {
		return err
	}

	replicas := slices.Clone(newReplicas)
	replicas = append(replicas, unprovisionedReplicas.UnsortedList()...)
	replicas = append(replicas, provisioningReplicas...)
	slices.Sort(replicas)
	parameters, err := component.NewReplicaTask(r.synthesizeComp.FullCompName, r.synthesizeComp.Generation, source, replicas)
	if err != nil {
		return err
	}
	// apply the updated env to the env CM
	transCtx := &componentTransformContext{
		Context:             r.transCtx.Context,
		Client:              model.NewGraphClient(r.cli),
		SynthesizeComponent: r.synthesizeComp,
		Component:           r.component,
	}
	return createOrUpdateEnvConfigMap(transCtx, r.dag, nil, parameters)
}

func (r *componentWorkloadOps) sourceReplica(dataDump *appsv1.Action, sourceReplicas sets.Set[string]) (lifecycle.Replica, error) {
	var replicas []lifecycle.Replica
	for i, inst := range r.runningITS.Status.InstanceStatus {
		if sourceReplicas.Has(inst.PodName) {
			replicas = append(replicas, &lifecycleReplica{
				synthesizedComp: r.synthesizeComp,
				instance:        r.runningITS.Status.InstanceStatus[i],
			})
		}
	}
	if len(replicas) > 0 {
		if len(dataDump.TargetPodSelector) == 0 && (dataDump.Exec == nil || len(dataDump.Exec.TargetPodSelector) == 0) {
			dataDump.TargetPodSelector = appsv1.AnyReplica
		}
		// TODO: idempotence for provisioning replicas
		var err error
		replicas, err = lifecycle.SelectTargetPods(replicas, nil, dataDump)
		if err != nil {
			return nil, err
		}
		if len(replicas) > 0 {
			return replicas[0], nil
		}
	}
	return nil, fmt.Errorf("no available pod to dump data")
}

func (r *componentWorkloadOps) reconfigure() error {
	runningObjs, protoObjs, err := prepareFileTemplateObjects(r.transCtx)
	if err != nil {
		return err
	}

	toCreate, toDelete, toUpdate := mapDiff(runningObjs, protoObjs)

	return r.handleReconfigure(r.transCtx, runningObjs, protoObjs, toCreate, toDelete, toUpdate)
}

func (r *componentWorkloadOps) handleReconfigure(transCtx *componentTransformContext,
	runningObjs, protoObjs map[string]*corev1.ConfigMap, toCreate, toDelete, toUpdate sets.Set[string]) error {
	var (
		synthesizedComp = transCtx.SynthesizeComponent
	)

	if r.runningITS == nil {
		r.protoITS.Spec.Configs = nil
		return nil // the workload hasn't been provisioned
	}

	if len(toCreate) > 0 || len(toDelete) > 0 {
		// since pod volumes changed, the workload will be restarted
		r.protoITS.Spec.Configs = nil
		return nil
	}

	templateChanges := r.templateFileChanges(transCtx, runningObjs, protoObjs, toUpdate)
	for objName := range toUpdate {
		tplName := fileTemplateNameFromObject(transCtx.SynthesizeComponent, protoObjs[objName])
		if _, ok := templateChanges[tplName]; !ok {
			continue
		}
		for _, tpl := range synthesizedComp.FileTemplates {
			if tpl.Name == tplName {
				if ptr.Deref(tpl.RestartOnFileChange, false) {
					// restart
					if r.protoITS.Spec.Template.Annotations == nil {
						r.protoITS.Spec.Template.Annotations = map[string]string{}
					}
					r.protoITS.Spec.Template.Annotations[constant.RestartAnnotationKey] = metav1.NowMicro().Format(time.RFC3339)
					return nil
				}
			}
		}
	}

	reconfigure := func(tpl component.SynthesizedFileTemplate, changes fileTemplateChanges) {
		var (
			action     *appsv1.Action
			actionName string
		)
		if tpl.ExternalManaged != nil && *tpl.ExternalManaged {
			if tpl.Reconfigure == nil {
				return // disabled by the external system
			}
		}
		action = tpl.Reconfigure
		actionName = component.UDFReconfigureActionName(tpl)
		if action == nil && synthesizedComp.LifecycleActions != nil {
			action = synthesizedComp.LifecycleActions.Reconfigure
			actionName = "" // default reconfigure action
		}
		if action == nil {
			return // has no reconfigure action defined
		}

		config := workloads.ConfigTemplate{
			Name:                  tpl.Name,
			Generation:            r.component.Generation,
			Reconfigure:           action,
			ReconfigureActionName: actionName,
			Parameters:            lifecycle.FileTemplateChanges(changes.Created, changes.Removed, changes.Updated),
		}
		if r.protoITS.Spec.Configs == nil {
			r.protoITS.Spec.Configs = make([]workloads.ConfigTemplate, 0)
		}
		idx := slices.IndexFunc(r.protoITS.Spec.Configs, func(cfg workloads.ConfigTemplate) bool {
			return cfg.Name == tpl.Name
		})
		if idx >= 0 {
			r.protoITS.Spec.Configs[idx] = config
		} else {
			r.protoITS.Spec.Configs = append(r.protoITS.Spec.Configs, config)
		}
	}

	// make a copy of configs from the running ITS
	r.protoITS.Spec.Configs = slices.Clone(r.runningITS.Spec.Configs)

	for _, tpl := range synthesizedComp.FileTemplates {
		if changes, ok := templateChanges[tpl.Name]; ok {
			reconfigure(tpl, changes)
		}
	}
	return nil
}

func (r *componentWorkloadOps) templateFileChanges(transCtx *componentTransformContext,
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
				absPath := r.absoluteFilePath(transCtx, tplName, item)
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

func (r *componentWorkloadOps) absoluteFilePath(transCtx *componentTransformContext, tpl, file string) string {
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
