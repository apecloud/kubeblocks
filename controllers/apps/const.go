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

package apps

const (
	// name of our custom finalizer
	clusterDefinitionFinalizerName   = "clusterdefinition.kubeblocks.io/finalizer"
	clusterVersionFinalizerName      = "clusterversion.kubeblocks.io/finalizer"
	opsDefinitionFinalizerName       = "opsdefinition.kubeblocks.io/finalizer"
	componentDefinitionFinalizerName = "componentdefinition.kubeblocks.io/finalizer"

	// annotations keys
	// debugClusterAnnotationKey is used when one wants to debug the cluster.
	// If debugClusterAnnotationKey = 'on',
	// logs will be recorded in more details, and some ephemeral pods (esp. those created by jobs) will retain after execution.
	debugClusterAnnotationKey = "cluster.kubeblocks.io/debug"
)

const (
	reasonOpsCancelActionNotSupported = "CancelActionNotSupported"
	reasonOpsCancelActionFailed       = "CancelActionFailed"
	reasonOpsReconcileStatusFailed    = "ReconcileStatusFailed"
	reasonOpsDoActionFailed           = "DoActionFailed"
)

const (
	trueVal = "true"
)
