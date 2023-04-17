/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package lifecycle

import (
	"fmt"

	"golang.org/x/exp/slices"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dataprotectionv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	types2 "github.com/apecloud/kubeblocks/internal/controller/client"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

// backupPolicyTPLTransformer transforms the backup policy template to the backup policy.
type backupPolicyTPLTransformer struct {
	cr  clusterRefResources
	cli types2.ReadonlyClient
	ctx intctrlutil.RequestCtx
}

func (r *backupPolicyTPLTransformer) Transform(dag *graph.DAG) error {
	clusterDefName := r.cr.cd.Name
	backupPolicyTPLs := &appsv1alpha1.BackupPolicyTemplateList{}
	if err := r.cli.List(r.ctx.Ctx, backupPolicyTPLs, client.MatchingLabels{constant.ClusterDefLabelKey: clusterDefName}); err != nil {
		return err
	}
	if len(backupPolicyTPLs.Items) == 0 {
		return nil
	}
	rootVertex, err := findRootVertex(dag)
	if err != nil {
		return err
	}
	origCluster, _ := rootVertex.oriObj.(*appsv1alpha1.Cluster)
	for _, tpl := range backupPolicyTPLs.Items {
		for _, v := range tpl.Spec.BackupPolicies {
			compDef := r.cr.cd.GetComponentDefByName(v.ComponentDefRef)
			if compDef == nil {
				return intctrlutil.NewNotFound("componentDef %s not found in ClusterDefinition: %s ", v.ComponentDefRef, clusterDefName)
			}
			// build the backup policy from the template.
			backupPolicy := r.transformBackupPolicy(v, origCluster, compDef.WorkloadType, tpl.Name)
			vertex := &lifecycleVertex{obj: backupPolicy}
			dag.AddVertex(vertex)
			dag.Connect(rootVertex, vertex)
		}
	}
	return nil
}

// transformBackupPolicy transform backup policy template to backup policy.
func (r *backupPolicyTPLTransformer) transformBackupPolicy(policyTPL appsv1alpha1.BackupPolicy,
	cluster *appsv1alpha1.Cluster,
	workloadType appsv1alpha1.WorkloadType,
	tplName string) *dataprotectionv1alpha1.BackupPolicy {
	backupPolicyName := DeriveBackupPolicyName(cluster.Name, policyTPL.ComponentDefRef)
	backupPolicy := &dataprotectionv1alpha1.BackupPolicy{}
	if err := r.cli.Get(r.ctx.Ctx, client.ObjectKey{Namespace: cluster.Namespace, Name: backupPolicyName}, backupPolicy); err != nil && !apierrors.IsNotFound(err) {
		return nil
	}
	if len(backupPolicy.Name) == 0 {
		// build a new backup policy from the backup policy template.
		return r.buildBackupPolicy(policyTPL, cluster, workloadType, tplName)
	}
	// sync the existing backup policy with the cluster changes
	r.syncBackupPolicy(backupPolicy, cluster, policyTPL, workloadType, tplName)
	return backupPolicy
}

// syncBackupPolicy syncs labels and annotations of the backup policy with the cluster changes.
func (r *backupPolicyTPLTransformer) syncBackupPolicy(backupPolicy *dataprotectionv1alpha1.BackupPolicy,
	cluster *appsv1alpha1.Cluster,
	policyTPL appsv1alpha1.BackupPolicy,
	workloadType appsv1alpha1.WorkloadType,
	tplName string) {
	// update labels and annotations of the backup policy.
	if backupPolicy.Annotations == nil {
		backupPolicy.Annotations = map[string]string{}
	}
	backupPolicy.Annotations[constant.DefaultBackupPolicyAnnotationKey] = "true"
	backupPolicy.Annotations[constant.BackupPolicyTemplateAnnotationKey] = tplName
	if backupPolicy.Labels == nil {
		backupPolicy.Labels = map[string]string{}
	}
	backupPolicy.Labels[constant.AppInstanceLabelKey] = cluster.Name
	backupPolicy.Labels[constant.KBAppComponentDefRefLabelKey] = policyTPL.ComponentDefRef

	// REVIEW/TODO: (wangyelei)
	// 1. following is rather hack-ish, as Backup target criteria has no direct relation with workloadType,
	// need extra attributes for the target selector.
	// 2. need to update workloadType API attributes documentation for current design implementation.
	//
	// only update the role labelSelector of the backup target instance when component workload is Replication/Consensus.
	if !slices.Contains([]appsv1alpha1.WorkloadType{appsv1alpha1.Replication, appsv1alpha1.Consensus}, workloadType) {
		return
	}
	component := r.getFirstComponent(cluster, policyTPL.ComponentDefRef)
	if component == nil {
		return
	}
	// convert role labelSelector based on the replicas of the component automatically.
	syncTheRoleLabel := func(target dataprotectionv1alpha1.TargetCluster,
		basePolicy appsv1alpha1.BasePolicy) dataprotectionv1alpha1.TargetCluster {
		role := basePolicy.Target.Role
		if len(role) == 0 {
			return target
		}
		if target.LabelsSelector == nil || target.LabelsSelector.MatchLabels == nil {
			target.LabelsSelector = &metav1.LabelSelector{MatchLabels: map[string]string{}}
		}
		if component.Replicas == 1 {
			// if replicas is 1, remove the role label selector.
			delete(target.LabelsSelector.MatchLabels, constant.RoleLabelKey)
		} else {
			target.LabelsSelector.MatchLabels[constant.RoleLabelKey] = role
		}
		return target
	}
	if backupPolicy.Spec.Snapshot != nil {
		backupPolicy.Spec.Snapshot.Target = syncTheRoleLabel(backupPolicy.Spec.Snapshot.Target,
			policyTPL.Snapshot.BasePolicy)
	}
	if backupPolicy.Spec.Full != nil {
		backupPolicy.Spec.Full.Target = syncTheRoleLabel(backupPolicy.Spec.Full.Target,
			policyTPL.Full.BasePolicy)
	}
	if backupPolicy.Spec.Incremental != nil {
		backupPolicy.Spec.Incremental.Target = syncTheRoleLabel(backupPolicy.Spec.Incremental.Target,
			policyTPL.Incremental.BasePolicy)
	}
}

// buildBackupPolicy builds a new backup policy from the backup policy template.
func (r *backupPolicyTPLTransformer) buildBackupPolicy(policyTPL appsv1alpha1.BackupPolicy,
	cluster *appsv1alpha1.Cluster,
	workloadType appsv1alpha1.WorkloadType,
	tplName string) *dataprotectionv1alpha1.BackupPolicy {
	backupPolicy := &dataprotectionv1alpha1.BackupPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeriveBackupPolicyName(cluster.Name, policyTPL.ComponentDefRef),
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				constant.AppInstanceLabelKey:          cluster.Name,
				constant.KBAppComponentDefRefLabelKey: policyTPL.ComponentDefRef,
			},
			Annotations: map[string]string{
				constant.DefaultBackupPolicyAnnotationKey:  "true",
				constant.BackupPolicyTemplateAnnotationKey: tplName,
			},
		},
	}
	bpSpec := backupPolicy.Spec
	bpSpec.TTL = policyTPL.TTL
	bpSpec.Schedule.BaseBackup = r.convertBaseBackupSchedulePolicy(policyTPL.Schedule.BaseBackup)
	bpSpec.Schedule.Incremental = r.convertSchedulePolicy(policyTPL.Schedule.Incremental)
	component := r.getFirstComponent(cluster, policyTPL.ComponentDefRef)
	if component != nil {
		bpSpec.Full = r.convertCommonPolicy(policyTPL.Full, cluster.Name, *component, workloadType)
		bpSpec.Incremental = r.convertCommonPolicy(policyTPL.Incremental, cluster.Name, *component, workloadType)
		bpSpec.Snapshot = r.convertSnapshotPolicy(policyTPL.Snapshot, cluster.Name, *component, workloadType)
	}
	backupPolicy.Spec = bpSpec
	return backupPolicy
}

// getFirstComponent returns the first component name of the componentDefRef.
func (r *backupPolicyTPLTransformer) getFirstComponent(cluster *appsv1alpha1.Cluster,
	componentDefRef string) *appsv1alpha1.ClusterComponentSpec {
	for _, v := range cluster.Spec.ComponentSpecs {
		if v.ComponentDefRef == componentDefRef {
			return &v
		}
	}
	return nil
}

// convertSchedulePolicy converts the schedulePolicy from backupPolicyTemplate.
func (r *backupPolicyTPLTransformer) convertSchedulePolicy(sp *appsv1alpha1.SchedulePolicy) *dataprotectionv1alpha1.SchedulePolicy {
	if sp == nil {
		return nil
	}
	return &dataprotectionv1alpha1.SchedulePolicy{
		Enable:         sp.Enable,
		CronExpression: sp.CronExpression,
	}
}

// convertBaseBackupSchedulePolicy converts the baseBackupSchedulePolicy from backupPolicyTemplate.
func (r *backupPolicyTPLTransformer) convertBaseBackupSchedulePolicy(sp *appsv1alpha1.BaseBackupSchedulePolicy) *dataprotectionv1alpha1.BaseBackupSchedulePolicy {
	if sp == nil {
		return nil
	}
	schedulePolicy := r.convertSchedulePolicy(&sp.SchedulePolicy)
	return &dataprotectionv1alpha1.BaseBackupSchedulePolicy{
		Type:           dataprotectionv1alpha1.BaseBackupType(sp.Type),
		SchedulePolicy: *schedulePolicy,
	}
}

// convertBasePolicy converts the basePolicy from backupPolicyTemplate.
func (r *backupPolicyTPLTransformer) convertBasePolicy(bp appsv1alpha1.BasePolicy,
	clusterName string,
	component appsv1alpha1.ClusterComponentSpec,
	workloadType appsv1alpha1.WorkloadType) dataprotectionv1alpha1.BasePolicy {
	basePolicy := dataprotectionv1alpha1.BasePolicy{
		Target: dataprotectionv1alpha1.TargetCluster{
			LabelsSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					constant.AppInstanceLabelKey:    clusterName,
					constant.KBAppComponentLabelKey: component.Name,
				},
			},
		},
		BackupsHistoryLimit: bp.BackupsHistoryLimit,
		OnFailAttempted:     bp.OnFailAttempted,
	}
	if len(bp.BackupStatusUpdates) != 0 {
		backupStatusUpdates := make([]dataprotectionv1alpha1.BackupStatusUpdate, len(bp.BackupStatusUpdates))
		for i, v := range bp.BackupStatusUpdates {
			backupStatusUpdates[i] = dataprotectionv1alpha1.BackupStatusUpdate{
				Path:          v.Path,
				ContainerName: v.ContainerName,
				Script:        v.Script,
				UpdateStage:   dataprotectionv1alpha1.BackupStatusUpdateStage(v.UpdateStage),
			}
		}
		basePolicy.BackupStatusUpdates = backupStatusUpdates
	}
	switch workloadType {
	case appsv1alpha1.Replication, appsv1alpha1.Consensus:
		if len(bp.Target.Role) > 0 && component.Replicas > 1 {
			// the role only works when the component has multiple replicas.
			basePolicy.Target.LabelsSelector.MatchLabels[constant.RoleLabelKey] = bp.Target.Role
		}
	}
	// build the target secret.
	if len(bp.Target.Account) > 0 {
		basePolicy.Target.Secret = &dataprotectionv1alpha1.BackupPolicySecret{
			Name:        fmt.Sprintf("%s-%s-%s", clusterName, component.Name, bp.Target.Account),
			PasswordKey: constant.AccountPasswdForSecret,
			UsernameKey: constant.AccountNameForSecret,
		}
	} else {
		basePolicy.Target.Secret = &dataprotectionv1alpha1.BackupPolicySecret{
			Name: fmt.Sprintf("%s-conn-credential", clusterName),
		}
		connectionCredentialKey := bp.Target.ConnectionCredentialKey
		if connectionCredentialKey.PasswordKey != nil {
			basePolicy.Target.Secret.PasswordKey = *connectionCredentialKey.PasswordKey
		}
		if connectionCredentialKey.UsernameKey != nil {
			basePolicy.Target.Secret.UsernameKey = *connectionCredentialKey.UsernameKey
		}
	}
	return basePolicy
}

// convertBaseBackupSchedulePolicy converts the snapshotPolicy from backupPolicyTemplate.
func (r *backupPolicyTPLTransformer) convertSnapshotPolicy(sp *appsv1alpha1.SnapshotPolicy,
	clusterName string,
	component appsv1alpha1.ClusterComponentSpec,
	workloadType appsv1alpha1.WorkloadType) *dataprotectionv1alpha1.SnapshotPolicy {
	if sp == nil {
		return nil
	}
	snapshotPolicy := &dataprotectionv1alpha1.SnapshotPolicy{
		BasePolicy: r.convertBasePolicy(sp.BasePolicy, clusterName, component, workloadType),
	}
	if sp.Hooks != nil {
		snapshotPolicy.Hooks = &dataprotectionv1alpha1.BackupPolicyHook{
			PreCommands:   sp.Hooks.PreCommands,
			PostCommands:  sp.Hooks.PostCommands,
			ContainerName: sp.Hooks.ContainerName,
			Image:         sp.Hooks.Image,
		}
	}
	return snapshotPolicy
}

// convertBaseBackupSchedulePolicy converts the commonPolicy from backupPolicyTemplate.
func (r *backupPolicyTPLTransformer) convertCommonPolicy(bp *appsv1alpha1.CommonBackupPolicy,
	clusterName string,
	component appsv1alpha1.ClusterComponentSpec,
	workloadType appsv1alpha1.WorkloadType) *dataprotectionv1alpha1.CommonBackupPolicy {
	if bp == nil {
		return nil
	}
	return &dataprotectionv1alpha1.CommonBackupPolicy{
		BackupToolName: bp.BackupToolName,
		BasePolicy:     r.convertBasePolicy(bp.BasePolicy, clusterName, component, workloadType),
	}
}

// DeriveBackupPolicyName generates the backup policy name which is created from backup policy template.
func DeriveBackupPolicyName(clusterName, componentDef string) string {
	return fmt.Sprintf("%s-%s-backup-policy", clusterName, componentDef)
}
