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

package parameters

import (
	"context"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	configctrl "github.com/apecloud/kubeblocks/pkg/controller/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type Task struct {
	configctrl.ResourceFetcher[Task]

	Status *appsv1alpha1.ConfigurationItemDetailStatus
	Name   string

	Do func(fetcher *Task, component *component.SynthesizedComponent, revision string) error
}

type TaskContext struct {
	configuration *appsv1alpha1.Configuration
	ctx           context.Context
	fetcher       *Task
}

func NewTask(item appsv1alpha1.ConfigurationItemDetail, status *appsv1alpha1.ConfigurationItemDetailStatus) Task {
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
				if item.ConfigSpec.InjectEnvEnabled() && item.ConfigSpec.ToSecret() {
					return syncSecretStatus(status)
				}
				return err
			}
			// Do reconcile for config template
			configMap := fetcher.ConfigMapObj
			switch intctrlutil.GetConfigSpecReconcilePhase(configMap, item, status) {
			default:
				return syncStatus(configMap, status)
			case appsv1alpha1.CPendingPhase,
				appsv1alpha1.CMergeFailedPhase:
				return syncImpl(fetcher, item, status, synComponent, revision, builder.ToV1ConfigSpec(configSpec))
			case appsv1alpha1.CCreatingPhase:
				return nil
			}
		},
		Status: status,
	}
}

func syncSecretStatus(status *appsv1alpha1.ConfigurationItemDetailStatus) error {
	status.Phase = appsv1alpha1.CFinishedPhase
	if status.LastDoneRevision == "" {
		status.LastDoneRevision = status.UpdateRevision
	}
	return nil
}

func syncImpl(fetcher *Task,
	item appsv1alpha1.ConfigurationItemDetail,
	status *appsv1alpha1.ConfigurationItemDetailStatus,
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
		status.Phase = appsv1alpha1.CMergeFailedPhase
	} else {
		status.Message = nil
		status.Phase = appsv1alpha1.CMergedPhase
	}
	status.UpdateRevision = revision
	return err
}

func syncStatus(configMap *corev1.ConfigMap, status *appsv1alpha1.ConfigurationItemDetailStatus) (err error) {
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

func updateLastDoneRevision(revision ConfigurationRevision, status *appsv1alpha1.ConfigurationItemDetailStatus) {
	if revision.Phase == appsv1alpha1.CFinishedPhase {
		status.LastDoneRevision = strconv.FormatInt(revision.Revision, 10)
	}
}

func updateRevision(revision ConfigurationRevision, status *appsv1alpha1.ConfigurationItemDetailStatus) {
	if revision.StrRevision == status.UpdateRevision {
		status.Phase = revision.Phase
		status.ReconcileDetail = &appsv1alpha1.ReconcileDetail{
			CurrentRevision: revision.StrRevision,
			Policy:          revision.Result.Policy,
			SucceedCount:    revision.Result.SucceedCount,
			ExpectedCount:   revision.Result.ExpectedCount,
			ExecResult:      revision.Result.ExecResult,
			ErrMessage:      revision.Result.Message,
		}
	}
}
