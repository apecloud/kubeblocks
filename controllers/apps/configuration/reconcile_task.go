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
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/plan"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

type Task struct {
	intctrlutil.ResourceFetcher[Task]

	Status *appsv1alpha1.ConfigurationItemDetailStatus
	Name   string

	Do func(task *Task, component *component.SynthesizedComponent) error
}

func NewTask(item appsv1alpha1.ConfigurationItemDetail, status *appsv1alpha1.ConfigurationItemDetailStatus) Task {
	return Task{
		Name: item.Name,
		Do: func(fetcher *Task, component *component.SynthesizedComponent) error {
			return plan.NewReconcilePipeline(plan.ReconcileCtx{
				ResourceCtx: fetcher.ResourceCtx,
				Cluster:     fetcher.ClusterObj,
				ClusterVer:  fetcher.ClusterVerObj,
				Component:   component,
				PodSpec:     component.PodSpec,
			}, item, status).ConfigMap().
				Prepare().
				ConfigConstraints().
				// ConfigMap().
				RerenderTemplate().
				ApplyParameters().
				UpdateConfigVersion().
				Sync().
				SyncStatus().
				Complete()
		},
		Status: status,
	}
}
