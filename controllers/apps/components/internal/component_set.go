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

package internal

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
)

// TODO(impl): replace it with ComponentWorkload and <*>Set implementation.

type ComponentSet interface {
	// IsRunning when relevant k8s workloads changes, it checks whether the component is running.
	// you can also reconcile the pods of component till the component is Running here.
	IsRunning(ctx context.Context, obj client.Object) (bool, error)

	// PodsReady checks whether all pods of the component are ready.
	// it means the pods are available in StatefulSet or Deployment.
	PodsReady(ctx context.Context, obj client.Object) (bool, error)

	// PodIsAvailable checks whether a pod of the component is available.
	// if the component is Stateless/StatefulSet, the available conditions follows as:
	// 1. the pod is ready.
	// 2. readyTime reached minReadySeconds.
	// if the component is ConsensusSet,it will be available when the pod is ready and labeled with its role.
	PodIsAvailable(pod *corev1.Pod, minReadySeconds int32) bool

	// GetPhaseWhenPodsReadyAndProbeTimeout when the pods of component are ready but the probe timed-out,
	//  calculate the component phase is Failed or Abnormal.
	GetPhaseWhenPodsReadyAndProbeTimeout(pods []*corev1.Pod) (v1alpha1.ClusterComponentPhase, v1alpha1.ComponentMessageMap)

	// GetPhaseWhenPodsNotReady when the pods of component are not ready, calculate the component phase is Failed or Abnormal.
	// if return an empty phase, means the pods of component are ready and skips it.
	GetPhaseWhenPodsNotReady(ctx context.Context, componentName string) (v1alpha1.ClusterComponentPhase, v1alpha1.ComponentMessageMap, error)

	HandleRestart(ctx context.Context, obj client.Object) ([]graph.Vertex, error)

	HandleRoleChange(ctx context.Context, obj client.Object) ([]graph.Vertex, error)

	HandleHA(ctx context.Context, obj client.Object) ([]graph.Vertex, error)
}

// ComponentSetBase is a common component set base struct.
type ComponentSetBase struct {
	Cli                  client.Client
	Cluster              *v1alpha1.Cluster
	SynthesizedComponent *component.SynthesizedComponent
	ComponentSpec        *v1alpha1.ClusterComponentSpec       // for test cases used only
	ComponentDef         *v1alpha1.ClusterComponentDefinition // for test cases used only
}
