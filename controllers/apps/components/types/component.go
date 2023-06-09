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

package types

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	// ComponentPhaseTransition the event reason indicates that the component transits to a new phase.
	ComponentPhaseTransition = "ComponentPhaseTransition"

	// PodContainerFailedTimeout the timeout for container of pod failures, the component phase will be set to Failed/Abnormal after this time.
	PodContainerFailedTimeout = 10 * time.Second

	// PodScheduledFailedTimeout timeout for scheduling failure.
	PodScheduledFailedTimeout = 30 * time.Second
)

type Component interface {
	GetName() string
	GetNamespace() string
	GetClusterName() string
	GetDefinitionName() string
	GetWorkloadType() appsv1alpha1.WorkloadType

	GetCluster() *appsv1alpha1.Cluster
	GetClusterVersion() *appsv1alpha1.ClusterVersion
	GetSynthesizedComponent() *component.SynthesizedComponent

	GetMatchingLabels() client.MatchingLabels

	GetReplicas() int32

	GetConsensusSpec() *appsv1alpha1.ConsensusSetSpec
	GetPrimaryIndex() int32

	GetPhase() appsv1alpha1.ClusterComponentPhase
	// GetStatus() appsv1alpha1.ClusterComponentStatus

	// GetBuiltObjects returns all objects that will be created by this component
	GetBuiltObjects(reqCtx intctrlutil.RequestCtx, cli client.Client) ([]client.Object, error)

	Create(reqCtx intctrlutil.RequestCtx, cli client.Client) error
	Delete(reqCtx intctrlutil.RequestCtx, cli client.Client) error
	Update(reqCtx intctrlutil.RequestCtx, cli client.Client) error
	Status(reqCtx intctrlutil.RequestCtx, cli client.Client) error

	Restart(reqCtx intctrlutil.RequestCtx, cli client.Client) error

	ExpandVolume(reqCtx intctrlutil.RequestCtx, cli client.Client) error

	HorizontalScale(reqCtx intctrlutil.RequestCtx, cli client.Client) error

	// TODO(impl): impl-related, replace them with component workload
	SetWorkload(obj client.Object, action *ictrltypes.LifecycleAction, parent *ictrltypes.LifecycleVertex)
	AddResource(obj client.Object, action *ictrltypes.LifecycleAction, parent *ictrltypes.LifecycleVertex) *ictrltypes.LifecycleVertex
}

// TODO(impl): replace it with ComponentWorkload and <*>Set implementation.

type ComponentSet interface {
	SetComponent(component Component)

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
	GetPhaseWhenPodsReadyAndProbeTimeout(pods []*corev1.Pod) (appsv1alpha1.ClusterComponentPhase, appsv1alpha1.ComponentMessageMap)

	// GetPhaseWhenPodsNotReady when the pods of component are not ready, calculate the component phase is Failed or Abnormal.
	// if return an empty phase, means the pods of component are ready and skips it.
	GetPhaseWhenPodsNotReady(ctx context.Context, componentName string) (appsv1alpha1.ClusterComponentPhase, appsv1alpha1.ComponentMessageMap, error)

	HandleRestart(ctx context.Context, obj client.Object) ([]graph.Vertex, error)

	HandleRoleChange(ctx context.Context, obj client.Object) ([]graph.Vertex, error)

	HandleHA(ctx context.Context, obj client.Object) ([]graph.Vertex, error)
}

// ComponentSetBase is a common component set base struct.
type ComponentSetBase struct {
	Cli           client.Client
	Cluster       *appsv1alpha1.Cluster
	ComponentSpec *appsv1alpha1.ClusterComponentSpec
	ComponentDef  *appsv1alpha1.ClusterComponentDefinition
	Component     Component
}
