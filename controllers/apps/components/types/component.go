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
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/component"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	// ComponentPhaseTransition the event reason indicates that the component transits to a new phase.
	ComponentPhaseTransition = "ComponentPhaseTransition"

	// PodContainerFailedTimeout the timeout for container of pod failures, the component phase will be set to Failed/Abnormal after this time.
	PodContainerFailedTimeout = time.Minute

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

	GetConsensusSpec() *appsv1alpha1.ConsensusSetSpec

	GetMatchingLabels() client.MatchingLabels

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
