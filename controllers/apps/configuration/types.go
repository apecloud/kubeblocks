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
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cfgproto "github.com/apecloud/kubeblocks/pkg/configuration/proto"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type createReconfigureClient func(addr string) (cfgproto.ReconfigureClient, error)

type GetPodsFunc func(params reconfigureParams) ([]corev1.Pod, error)
type RestartComponent func(client client.Client, ctx intctrlutil.RequestCtx, key string, version string, objs []client.Object, recordEvent func(obj client.Object)) (client.Object, error)

type RestartContainerFunc func(pod *corev1.Pod, ctx context.Context, containerName []string, createConnFn createReconfigureClient) error
type OnlineUpdatePodFunc func(pod *corev1.Pod, ctx context.Context, createClient createReconfigureClient, configSpec string, updatedParams map[string]string) error

// Node: Distinguish between implementation and interface.
// RollingUpgradeFuncs defines the interface, rsm is an implementation of Stateful, Replication and Consensus, not the only solution.

type RollingUpgradeFuncs struct {
	GetPodsFunc          GetPodsFunc
	RestartContainerFunc RestartContainerFunc
	OnlineUpdatePodFunc  OnlineUpdatePodFunc
	RestartComponent     RestartComponent
}

func GetRSMRollingUpgradeFuncs() RollingUpgradeFuncs {
	return RollingUpgradeFuncs{
		GetPodsFunc:          getPodsForOnlineUpdate,
		RestartContainerFunc: commonStopContainerWithPod,
		OnlineUpdatePodFunc:  commonOnlineUpdateWithPod,
		RestartComponent:     restartComponent,
	}
}
