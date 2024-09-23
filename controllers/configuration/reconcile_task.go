/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package configuration

import (
	"context"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	configurationv1alpha1 "github.com/apecloud/kubeblocks/apis/configuration/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	configctrl "github.com/apecloud/kubeblocks/pkg/controller/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type Task struct {
	configctrl.ResourceFetcher[Task]

	Status *configurationv1alpha1.ConfigTemplateItemDetailStatus
	Name   string

	Do func(fetcher *Task, component *component.SynthesizedComponent, revision string) error
}

type TaskContext struct {
	configuration *configurationv1alpha1.ComponentParameter
	ctx           context.Context
	fetcher       *Task
}

func NewTask(item configurationv1alpha1.ConfigTemplateItemDetail, status *configurationv1alpha1.ConfigTemplateItemDetailStatus) Task {
	return Task{
		Name: item.Name,
		Do: func(fetcher *Task, synComponent *component.SynthesizedComponent, revision string) error {
			configSpec := item.ConfigSpec
			if configSpec == nil {
				return core.MakeError("not found config spec: %s", item.Name)
			}
			if err := fetcher.ConfigMap(item.Name).Complete(); err != nil {
				if !apierrors.IsNotFound(err) {
					return err
				}
				if configctrl.InjectEnvEnabled(*item.ConfigSpec) && configctrl.ToSecret(*item.ConfigSpec) {
					return syncSecretStatus(status)
				}
				return err
			}
			// Do reconcile for config template
			configMap := fetcher.ConfigMapObj
			switch intctrlutil.GetConfigSpecReconcilePhase(configMap, item, status) {
			default:
				return syncStatus(configMap, status)
			case configurationv1alpha1.CPendingPhase,
				configurationv1alpha1.CMergeFailedPhase:
				return syncImpl(fetcher, item, status, synComponent, revision, configSpec)
			case configurationv1alpha1.CCreatingPhase:
				return nil
			}
		},
		Status: status,
	}
}

func syncSecretStatus(status *configurationv1alpha1.ConfigTemplateItemDetailStatus) error {
	status.Phase = configurationv1alpha1.CFinishedPhase
	if status.LastDoneRevision == "" {
		status.LastDoneRevision = status.UpdateRevision
	}
	return nil
}

func syncImpl(fetcher *Task,
	item configurationv1alpha1.ConfigTemplateItemDetail,
	status *configurationv1alpha1.ConfigTemplateItemDetailStatus,
	synthesizedComponent *component.SynthesizedComponent,
	revision string,
	configSpec *appsv1.ComponentConfigSpec) (err error) {
	err = configctrl.NewReconcilePipeline(configctrl.ReconcileCtx{
		ResourceCtx:          fetcher.ResourceCtx,
		Cluster:              fetcher.ClusterObj,
		Component:            fetcher.ComponentObj,
		SynthesizedComponent: synthesizedComponent,
		PodSpec:              synthesizedComponent.PodSpec,
	}, item, status, configSpec).
		ConfigMap(item.Name).
		ConfigConstraints(configSpec.ConfigConstraintRef).
		PrepareForTemplate().
		RerenderTemplate().
		ApplyParameters().
		UpdateConfigVersion(revision).
		Sync().
		Complete()

	if err != nil {
		status.Message = cfgutil.ToPointer(err.Error())
		status.Phase = configurationv1alpha1.CMergeFailedPhase
	} else {
		status.Message = nil
		status.Phase = configurationv1alpha1.CMergedPhase
	}
	status.UpdateRevision = revision
	return err
}

func syncStatus(configMap *corev1.ConfigMap, status *configurationv1alpha1.ConfigTemplateItemDetailStatus) (err error) {
	annotations := configMap.GetAnnotations()
	// status.CurrentRevision = GetCurrentRevision(annotations)
	revisions := RetrieveRevision(annotations)
	if len(revisions) == 0 {
		return
	}

	for i := 0; i < len(revisions); i++ {
		updateRevision(revisions[i], status)
		updateLastDoneRevision(revisions[i], status)
	}

	return
}

func updateLastDoneRevision(revision ConfigurationRevision, status *configurationv1alpha1.ConfigTemplateItemDetailStatus) {
	if revision.Phase == configurationv1alpha1.CFinishedPhase {
		status.LastDoneRevision = strconv.FormatInt(revision.Revision, 10)
	}
}

func updateRevision(revision ConfigurationRevision, status *configurationv1alpha1.ConfigTemplateItemDetailStatus) {
	if revision.StrRevision == status.UpdateRevision {
		status.Phase = revision.Phase
		status.ReconcileDetail = &configurationv1alpha1.ReconcileDetail{
			CurrentRevision: revision.StrRevision,
			Policy:          revision.Result.Policy,
			SucceedCount:    revision.Result.SucceedCount,
			ExpectedCount:   revision.Result.ExpectedCount,
			ExecResult:      revision.Result.ExecResult,
			ErrMessage:      revision.Result.Message,
		}
	}
}
