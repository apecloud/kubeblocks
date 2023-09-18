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

package configuration

import (
	"strconv"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/configuration/core"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/configuration"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type Task struct {
	intctrlutil.ResourceFetcher[Task]

	Status *appsv1alpha1.ConfigurationItemDetailStatus
	Name   string

	Do         func(fetcher *Task, component *component.SynthesizedComponent, revision string) error
	SyncStatus func(fetcher *Task, status *appsv1alpha1.ConfigurationItemDetailStatus) error
}

func NewTask(item appsv1alpha1.ConfigurationItemDetail, status *appsv1alpha1.ConfigurationItemDetailStatus) Task {
	return Task{
		Name:   item.Name,
		Status: status,
		Do: func(fetcher *Task, synComponent *component.SynthesizedComponent, revision string) error {
			configSpec := component.GetConfigSpecByName(synComponent, item.Name)
			if configSpec == nil {
				return core.MakeError("not found config spec: %s", item.Name)
			}
			reconcileTask := configuration.NewReconcilePipeline(configuration.ReconcileCtx{
				ResourceCtx: fetcher.ResourceCtx,
				Cluster:     fetcher.ClusterObj,
				ClusterVer:  fetcher.ClusterVerObj,
				Component:   synComponent,
				PodSpec:     synComponent.PodSpec,
			}, item, status, configSpec)
			return reconcileTask.ConfigMap(item.Name).
				ConfigConstraints(configSpec.ConfigConstraintRef).
				PrepareForTemplate().
				RerenderTemplate().
				ApplyParameters().
				UpdateConfigVersion(revision).
				Sync().
				SyncStatus().
				Complete()
		},
		SyncStatus: syncStatus,
	}
}

func syncStatus(fetcher *Task, status *appsv1alpha1.ConfigurationItemDetailStatus) (err error) {
	err = fetcher.ConfigMap(status.Name).Complete()
	if err != nil {
		return
	}

	annotations := fetcher.ConfigMapObj.GetAnnotations()
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
	}
}
