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

// GetKBGenerationAnnotation returns the annotation for kubeblocks generation.
func GetKBGenerationAnnotation(generation string) map[string]string {
	return map[string]string{
		KubeBlocksGenerationKey: generation,
	}
}

func GetServiceVersionAnnotation(serviceVersion string) map[string]string {
	return map[string]string{
		KBAppServiceVersionKey: serviceVersion,
	}
}

// GetKBReservedAnnotationKeys returns the reserved annotation keys for KubeBlocks
func GetKBReservedAnnotationKeys() []string {
	return []string{
		KubeBlocksGenerationKey,
		ReconcileAnnotationKey,
		KBAppServiceVersionKey,
	}
}

const (
	HorizontalScaleBackupPolicyTemplateKey = "apps.kubeblocks.io/horizontal-scale-backup-policy-template"
)

// annotations for multi-cluster
const (
	KBAppMultiClusterPlacementKey   = "apps.kubeblocks.io/multi-cluster-placement"
	MultiClusterServicePlacementKey = "apps.kubeblocks.io/multi-cluster-service-placement"

	OpsDependentOnSuccessfulOpsAnnoKey = "ops.kubeblocks.io/dependent-on-successful-ops" // OpsDependentOnSuccessfulOpsAnnoKey wait for the dependent ops to succeed before executing the current ops. If it fails, this ops will also fail.
	RelatedOpsAnnotationKey            = "ops.kubeblocks.io/related-ops"
)
