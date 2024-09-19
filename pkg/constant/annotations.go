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

const (
	HorizontalScaleBackupPolicyTemplateKey = "apps.kubeblocks.io/horizontal-scale-backup-policy-template"
)

// annotations defined by KubeBlocks
const (
	ClusterSnapshotAnnotationKey         = "kubeblocks.io/cluster-snapshot"          // ClusterSnapshotAnnotationKey saves the snapshot of cluster.
	EncryptedSystemAccountsAnnotationKey = "kubeblocks.io/encrypted-system-accounts" // EncryptedSystemAccountsAnnotationKey saves the encrypted system accounts.
	OpsRequestAnnotationKey              = "kubeblocks.io/ops-request"               // OpsRequestAnnotationKey OpsRequest annotation key in Cluster
	ReconcileAnnotationKey               = "kubeblocks.io/reconcile"                 // ReconcileAnnotationKey Notify k8s object to reconcile
	RestartAnnotationKey                 = "kubeblocks.io/restart"                   // RestartAnnotationKey the annotation which notices the StatefulSet/DeploySet to restart
	RestoreFromBackupAnnotationKey       = "kubeblocks.io/restore-from-backup"
	RestoreDoneAnnotationKey             = "kubeblocks.io/restore-done"
	BackupSourceTargetAnnotationKey      = "kubeblocks.io/backup-source-target" // RestoreFromBackupAnnotationKey specifies the component to recover from the backup.
	BackupPolicyTemplateAnnotationKey    = "apps.kubeblocks.io/backup-policy-template"
	PVLastClaimPolicyAnnotationKey       = "apps.kubeblocks.io/pv-last-claim-policy"
	KubeBlocksGenerationKey              = "kubeblocks.io/generation"
	KBAppClusterUIDKey                   = "apps.kubeblocks.io/cluster-uid"
	LastRoleSnapshotVersionAnnotationKey = "apps.kubeblocks.io/last-role-snapshot-version"
	ComponentScaleInAnnotationKey        = "apps.kubeblocks.io/component-scale-in" // ComponentScaleInAnnotationKey specifies whether the component is scaled in

	// SkipImmutableCheckAnnotationKey specifies to skip the mutation check for the object.
	// The mutation check is only applied to the fields that are declared as immutable.
	SkipImmutableCheckAnnotationKey = "apps.kubeblocks.io/skip-immutable-check"

	// NodeSelectorOnceAnnotationKey adds nodeSelector in podSpec for one pod exactly once
	NodeSelectorOnceAnnotationKey = "workloads.kubeblocks.io/node-selector-once"
	MemberJoinStatusAnnotationKey = "workloads.kubeblocks.io/memberjoin-status"
)

// annotations for multi-cluster
const (
	KBAppMultiClusterPlacementKey   = "apps.kubeblocks.io/multi-cluster-placement"
	MultiClusterServicePlacementKey = "apps.kubeblocks.io/multi-cluster-service-placement"
)

func InheritedAnnotations() []string {
	return []string{
		RestoreFromBackupAnnotationKey,
		BackupSourceTargetAnnotationKey,
		HostNetworkAnnotationKey,
		FeatureReconciliationInCompactModeAnnotationKey,
		KBAppMultiClusterPlacementKey,
	}
}
