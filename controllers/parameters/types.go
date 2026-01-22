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

package parameters

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	cfgproto "github.com/apecloud/kubeblocks/pkg/parameters/proto"
)

type createReconfigureClient func(addr string) (cfgproto.ReconfigureClient, error)

type GetPodsFunc func(params reconfigureContext) ([]corev1.Pod, error)
type RestartComponent func(client client.Client, ctx intctrlutil.RequestCtx, key string, version string, cluster *appsv1.Cluster, compName string) error

type OnlineUpdatePodFunc func(pod *corev1.Pod, ctx context.Context, createClient createReconfigureClient, configSpec string, configFile string, updatedParams map[string]string) error

// Node: Distinguish between implementation and interface.

type RollingUpgradeFuncs struct {
	GetPodsFunc         GetPodsFunc
	OnlineUpdatePodFunc OnlineUpdatePodFunc
	RestartComponent    RestartComponent
}

func GetInstanceSetRollingUpgradeFuncs() RollingUpgradeFuncs {
	return RollingUpgradeFuncs{
		GetPodsFunc:         getPodsForOnlineUpdate,
		OnlineUpdatePodFunc: commonOnlineUpdateWithPod,
		RestartComponent:    restartComponent,
	}
}

type ReloadAction interface {
	ExecReload() (returnedStatus, error)
	ReloadType() string
}

type reconfigureTask struct {
	parametersv1alpha1.ReloadPolicy
	taskCtx reconfigureContext
}
