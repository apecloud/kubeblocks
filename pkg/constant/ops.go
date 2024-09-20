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

package constant

// labels
const (
	OpsRequestTypeLabelKey      = "operations.kubeblocks.io/ops-type"
	OpsRequestNameLabelKey      = "operations.kubeblocks.io/ops-name"
	OpsRequestNamespaceLabelKey = "operations.kubeblocks.io/ops-namespace"
)

// annotations
const (
	DisableHAAnnotationKey             = "operations.kubeblocks.io/disable-ha"
	RelatedOpsAnnotationKey            = "operations.kubeblocks.io/related-ops"
	OpsDependentOnSuccessfulOpsAnnoKey = "operations.kubeblocks.io/dependent-on-successful-ops" // OpsDependentOnSuccessfulOpsAnnoKey wait for the dependent ops to succeed before executing the current ops. If it fails, this ops will also fail.
)
