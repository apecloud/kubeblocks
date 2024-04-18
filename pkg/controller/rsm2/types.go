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

package rsm2

type ReplicaProvider string

const (
	StatefulSetProvider ReplicaProvider = "StatefulSet"
	PodProvider         ReplicaProvider = "Pod"
)

const (
	// FeatureGateRSMReplicaProvider determines the instance provider for the RSM controller.
	// A instance provider is responsible for managing the underlying API resources required for the smooth operation of the RSM.
	// The currently supported instance providers are StatefulSet and Pod.
	// Planned supported instance providers include OpenKruise Advanced StatefulSet and KB Replica.
	FeatureGateRSMReplicaProvider = "RSM_REPLICA_PROVIDER"

	defaultReplicaProvider = PodProvider

	// MaxPlainRevisionCount specified max number of plain revision stored in rsm.status.updateRevisions.
	// All revisions will be compressed if exceeding this value.
	MaxPlainRevisionCount = "MAX_PLAIN_REVISION_COUNT"

	templateRefAnnotationKey = "kubeblocks.io/template-ref"
	templateRefDataKey       = "instances"
	revisionsZSTDKey         = "zstd"

	FeatureGateIgnorePodVerticalScaling = "IGNORE_POD_VERTICAL_SCALING"

	finalizer = "instanceset.workloads.kubeblocks.io/finalizer"
)
