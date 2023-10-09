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

import (
	"fmt"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/controller/graph"
	ictrltypes "github.com/apecloud/kubeblocks/internal/controller/types"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/internal/dataprotection/types"
)

// BackupPolicyTplTransformer transforms the backup policy template to the data
// protection backup policy and backup schedule.
type BackupPolicyTplTransformer struct {
	*ClusterTransformContext

	tplCount          int
	tplIdentifier     string
	isDefaultTemplate string

	backupPolicyTpl  *appsv1alpha1.BackupPolicyTemplate
	backupPolicy     *appsv1alpha1.BackupPolicy
	compWorkloadType appsv1alpha1.WorkloadType
}

var _ graph.Transformer = &BackupPolicyTplTransformer{}

// Transform transforms the backup policy template to the backup policy and
// backup schedule.
func (r *BackupPolicyTplTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	rootVertex, err := ictrltypes.FindRootVertex(dag)
	if err != nil {
		return err
	}

	r.ClusterTransformContext = ctx.(*ClusterTransformContext)
	clusterDefName := r.ClusterDef.Name
	backupPolicyTpls := &appsv1alpha1.BackupPolicyTemplateList{}
	if err = r.Client.List(r.Context, backupPolicyTpls,
		client.MatchingLabels{constant.ClusterDefLabelKey: clusterDefName}); err != nil {
		return err
	}
	r.tplCount = len(backupPolicyTpls.Items)
	if r.tplCount == 0 {
		return nil
	}

	backupPolicyNames := map[string]struct{}{}
	backupScheduleNames := map[string]struct{}{}
	for _, tpl := range backupPolicyTpls.Items {
		r.isDefaultTemplate = tpl.Annotations[dptypes.DefaultBackupPolicyTemplateAnnotationKey]
		r.tplIdentifier = tpl.Spec.Identifier
		r.backupPolicyTpl = &tpl

		for i, bp := range tpl.Spec.BackupPolicies {
			compDef := r.ClusterDef.GetComponentDefByName(bp.ComponentDefRef)
			if compDef == nil {
				return intctrlutil.NewNotFound("componentDef %s not found in ClusterDefinition: %s ",
					bp.ComponentDefRef, clusterDefName)
			}

			r.backupPolicy = &tpl.Spec.BackupPolicies[i]
			r.compWorkloadType = compDef.WorkloadType

			transformBackupPolicy := func() (*dpv1alpha1.BackupPolicy, *ictrltypes.LifecycleVertex) {
				// build the data protection backup policy from the template.
				dpBackupPolicy, action := r.transformBackupPolicy()
				if dpBackupPolicy == nil {
					return nil, nil
				}

				// if exist multiple backup policy templates and duplicate spec.identifier,
				// the generated backupPolicy may have duplicate names, so it is
				// necessary to check if it already exists.
				if _, ok := backupPolicyNames[dpBackupPolicy.Name]; ok {
					return dpBackupPolicy, nil
				}

				vertex := &ictrltypes.LifecycleVertex{Obj: dpBackupPolicy, Action: action}
				dag.AddVertex(vertex)
				dag.Connect(rootVertex, vertex)
				backupPolicyNames[dpBackupPolicy.Name] = struct{}{}
				return dpBackupPolicy, vertex
			}

			transformBackupSchedule := func(
				backupPolicy *dpv1alpha1.BackupPolicy,
				bpVertex *ictrltypes.LifecycleVertex) {
				// if backup policy is nil, it means that the backup policy template
				// is invalid, backup schedule depends on backup policy, so we do
				// not need to transform backup schedule.
				if backupPolicy == nil {
					return
				}

				// only create backup schedule for the default backup policy template
				// if there are more than one backup policy templates.
				if r.isDefaultTemplate != trueVal && r.tplCount > 1 {
					return
				}

				// build the data protection backup schedule from the template.
				dpBackupSchedule, action := r.transformBackupSchedule(backupPolicy)

				// merge cluster backup configuration into the backup schedule.
				// If the backup schedule is nil, create a new backup schedule
				// based on the cluster backup configuration.
				if dpBackupSchedule == nil {
					action = ictrltypes.ActionCreatePtr()
				} else if action == nil {
					action = ictrltypes.ActionUpdatePtr()
				}

				// for a cluster, the default backup schedule is created by backup
				// policy template, user can also configure cluster backup in the
				// cluster custom object, such as enable cluster backup, set backup
				// schedule, etc.
				// We always prioritize the cluster backup configuration in the
				// cluster object, so we need to merge the cluster backup configuration
				// into the default backup schedule created by backup policy template
				// if it exists.
				dpBackupSchedule = r.mergeClusterBackup(backupPolicy, dpBackupSchedule)
				if dpBackupSchedule == nil {
					return
				}

				// if exist multiple backup policy templates and duplicate spec.identifier,
				// the backupPolicy that may be generated may have duplicate names,
				// and it is necessary to check if it already exists.
				if _, ok := backupScheduleNames[dpBackupSchedule.Name]; ok {
					return
				}

				parent := rootVertex
				if bpVertex != nil {
					parent = bpVertex
				}
				vertex := &ictrltypes.LifecycleVertex{Obj: dpBackupSchedule, Action: action}
				dag.AddVertex(vertex)
				dag.Connect(parent, vertex)
				backupScheduleNames[dpBackupSchedule.Name] = struct{}{}
			}

			// transform backup policy template to data protection backupPolicy
			// and backupSchedule
			policy, policyVertex := transformBackupPolicy()
			transformBackupSchedule(policy, policyVertex)
		}
	}
	return nil
}

// transformBackupPolicy transforms backup policy template to backup policy.
func (r *BackupPolicyTplTransformer) transformBackupPolicy() (*dpv1alpha1.BackupPolicy, *ictrltypes.LifecycleAction) {
	cluster := r.OrigCluster
	backupPolicyName := generateBackupPolicyName(cluster.Name, r.backupPolicy.ComponentDefRef, r.tplIdentifier)
	backupPolicy := &dpv1alpha1.BackupPolicy{}
	if err := r.Client.Get(r.Context, client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      backupPolicyName,
	}, backupPolicy); client.IgnoreNotFound(err) != nil {
		return nil, nil
	}

	if len(backupPolicy.Name) == 0 {
		// build a new backup policy by the backup policy template.
		return r.buildBackupPolicy(backupPolicyName), ictrltypes.ActionCreatePtr()
	}

	// sync the existing backup policy with the cluster changes
	r.syncBackupPolicy(backupPolicy)
	return backupPolicy, ictrltypes.ActionUpdatePtr()
}

func (r *BackupPolicyTplTransformer) transformBackupSchedule(
	backupPolicy *dpv1alpha1.BackupPolicy) (*dpv1alpha1.BackupSchedule, *ictrltypes.LifecycleAction) {
	cluster := r.OrigCluster
	scheduleName := generateBackupScheduleName(cluster.Name, r.backupPolicy.ComponentDefRef, r.tplIdentifier)
	backupSchedule := &dpv1alpha1.BackupSchedule{}
	if err := r.Client.Get(r.Context, client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      scheduleName,
	}, backupSchedule); client.IgnoreNotFound(err) != nil {
		return nil, nil
	}

	if len(backupSchedule.Name) == 0 {
		// build a new backup schedule from the backup policy template.
		return r.buildBackupSchedule(scheduleName, backupPolicy), ictrltypes.ActionCreatePtr()
	}
	return backupSchedule, nil
}

func (r *BackupPolicyTplTransformer) buildBackupSchedule(
	name string,
	backupPolicy *dpv1alpha1.BackupPolicy) *dpv1alpha1.BackupSchedule {
	cluster := r.OrigCluster
	backupSchedule := &dpv1alpha1.BackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   cluster.Namespace,
			Labels:      r.buildLabels(),
			Annotations: r.buildAnnotations(),
		},
		Spec: dpv1alpha1.BackupScheduleSpec{
			BackupPolicyName: backupPolicy.Name,
		},
	}

	var schedules []dpv1alpha1.SchedulePolicy
	for _, s := range r.backupPolicy.Schedules {
		schedules = append(schedules, dpv1alpha1.SchedulePolicy{
			BackupMethod:    s.BackupMethod,
			CronExpression:  s.CronExpression,
			Enabled:         s.Enabled,
			RetentionPeriod: r.backupPolicy.RetentionPeriod,
		})
	}
	backupSchedule.Spec.Schedules = schedules
	return backupSchedule
}

// syncBackupPolicy syncs labels and annotations of the backup policy with the cluster changes.
func (r *BackupPolicyTplTransformer) syncBackupPolicy(backupPolicy *dpv1alpha1.BackupPolicy) {
	// update labels and annotations of the backup policy.
	if backupPolicy.Annotations == nil {
		backupPolicy.Annotations = map[string]string{}
	}
	if backupPolicy.Labels == nil {
		backupPolicy.Labels = map[string]string{}
	}
	mergeMap(backupPolicy.Annotations, r.buildAnnotations())
	mergeMap(backupPolicy.Labels, r.buildLabels())

	// only update the role labelSelector of the backup target instance when
	// component workload is Replication/Consensus. Because the replicas of
	// component will change, such as 2->1. then if the target role is 'follower'
	// and replicas is 1, the target instance can not be found. so we sync the
	// label selector automatically.
	if !workloadHasRoleLabel(r.compWorkloadType) {
		return
	}

	comp := r.getClusterComponentSpec()
	if comp == nil {
		return
	}

	// convert role labelSelector based on the replicas of the component automatically.
	// TODO(ldm): need more review.
	role := r.backupPolicy.Target.Role
	if len(role) == 0 {
		return
	}

	podSelector := backupPolicy.Spec.Target.PodSelector
	if podSelector.LabelSelector == nil || podSelector.LabelSelector.MatchLabels == nil {
		podSelector.LabelSelector = &metav1.LabelSelector{MatchLabels: map[string]string{}}
	}
	if r.getCompReplicas() == 1 {
		delete(podSelector.LabelSelector.MatchLabels, constant.RoleLabelKey)
	} else {
		podSelector.LabelSelector.MatchLabels[constant.RoleLabelKey] = role
	}
}

func (r *BackupPolicyTplTransformer) getCompReplicas() int32 {
	rsm := &workloads.ReplicatedStateMachine{}
	compSpec := r.getClusterComponentSpec()
	rsmName := fmt.Sprintf("%s-%s", r.Cluster.Name, compSpec.Name)
	if err := r.Client.Get(r.Context, client.ObjectKey{Name: rsmName, Namespace: r.Cluster.Namespace}, rsm); err != nil {
		return compSpec.Replicas
	}
	return *rsm.Spec.Replicas
}

// buildBackupPolicy builds a new backup policy by the backup policy template.
func (r *BackupPolicyTplTransformer) buildBackupPolicy(backupPolicyName string) *dpv1alpha1.BackupPolicy {
	comp := r.getClusterComponentSpec()
	if comp == nil {
		return nil
	}

	cluster := r.OrigCluster
	backupPolicy := &dpv1alpha1.BackupPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:        backupPolicyName,
			Namespace:   cluster.Namespace,
			Labels:      r.buildLabels(),
			Annotations: r.buildAnnotations(),
		},
	}

	bpSpec := backupPolicy.Spec
	bpSpec.BackupMethods = r.backupPolicy.BackupMethods
	bpSpec.PathPrefix = buildBackupPathPrefix(cluster, comp.Name)
	bpSpec.Target = r.buildBackupTarget(comp)
	backupPolicy.Spec = bpSpec
	return backupPolicy
}

func (r *BackupPolicyTplTransformer) buildBackupTarget(
	comp *appsv1alpha1.ClusterComponentSpec) *dpv1alpha1.BackupTarget {
	targetTpl := r.backupPolicy.Target
	clusterName := r.OrigCluster.Name

	getSAName := func() string {
		if comp.ServiceAccountName != "" {
			return comp.ServiceAccountName
		}
		return "kb-" + r.Cluster.Name
	}

	// build the target connection credential
	cc := dpv1alpha1.ConnectionCredential{}
	if len(targetTpl.Account) > 0 {
		cc.SecretName = fmt.Sprintf("%s-%s-%s", clusterName, comp.Name, targetTpl.Account)
		cc.PasswordKey = constant.AccountPasswdForSecret
		cc.PasswordKey = constant.AccountNameForSecret
	} else {
		cc.SecretName = fmt.Sprintf("%s-conn-credential", clusterName)
		ccKey := targetTpl.ConnectionCredentialKey
		if ccKey.PasswordKey != nil {
			cc.PasswordKey = *ccKey.PasswordKey
		}
		if ccKey.UsernameKey != nil {
			cc.UsernameKey = *ccKey.UsernameKey
		}
		if ccKey.PortKey != nil {
			cc.PortKey = *ccKey.PortKey
		}
		if ccKey.HostKey != nil {
			cc.HostKey = *ccKey.HostKey
		}
	}

	target := &dpv1alpha1.BackupTarget{
		PodSelector: &dpv1alpha1.PodSelector{
			Strategy: dpv1alpha1.PodSelectionStrategyAny,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: r.buildTargetPodLabels(comp),
			},
		},
		ConnectionCredential: &cc,
		ServiceAccountName:   getSAName(),
	}
	return target
}

func (r *BackupPolicyTplTransformer) mergeClusterBackup(
	backupPolicy *dpv1alpha1.BackupPolicy,
	backupSchedule *dpv1alpha1.BackupSchedule) *dpv1alpha1.BackupSchedule {
	cluster := r.OrigCluster
	backupEnabled := func() bool {
		return cluster.Spec.Backup != nil && boolValue(cluster.Spec.Backup.Enabled)
	}

	if backupPolicy == nil || cluster.Spec.Backup == nil {
		// backup policy is nil, can not enable cluster backup, so record event and return.
		if backupEnabled() {
			r.EventRecorder.Event(r.Cluster, corev1.EventTypeWarning,
				"BackupPolicyNotFound", "backup policy is nil, can not enable cluster backup")
		}
		return nil
	}

	backup := cluster.Spec.Backup
	// there is no backup schedule created by backup policy template, so we need to
	// create a new backup schedule for cluster backup.
	if backupSchedule == nil {
		backupSchedule = &dpv1alpha1.BackupSchedule{
			ObjectMeta: metav1.ObjectMeta{
				Name:        generateBackupScheduleName(cluster.Name, r.backupPolicy.ComponentDefRef, r.tplIdentifier),
				Namespace:   cluster.Namespace,
				Labels:      r.buildLabels(),
				Annotations: r.buildAnnotations(),
			},
			Spec: dpv1alpha1.BackupScheduleSpec{
				BackupPolicyName:        backupPolicy.Name,
				StartingDeadlineMinutes: backup.StartingDeadlineMinutes,
				Schedules:               []dpv1alpha1.SchedulePolicy{},
			},
		}
	}

	// build backup schedule policy by cluster backup spec
	sp := &dpv1alpha1.SchedulePolicy{
		Enabled:         backup.Enabled,
		RetentionPeriod: backup.RetentionPeriod,
		BackupMethod:    backup.Method,
		CronExpression:  backup.CronExpression,
	}

	// merge cluster backup schedule policy into backup schedule, if the backup
	// schedule with specified method already exists, we need to update it
	// using the cluster backup schedule policy. Otherwise, we need to append
	// it to the backup schedule.
	for i, s := range backupSchedule.Spec.Schedules {
		if s.BackupMethod == backup.Method {
			mergeSchedulePolicy(sp, &backupSchedule.Spec.Schedules[i])
			return backupSchedule
		}
	}
	backupSchedule.Spec.Schedules = append(backupSchedule.Spec.Schedules, *sp)
	return backupSchedule
}

// getClusterComponentSpec returns the first component name of the componentDefRef.
func (r *BackupPolicyTplTransformer) getClusterComponentSpec() *appsv1alpha1.ClusterComponentSpec {
	for _, v := range r.OrigCluster.Spec.ComponentSpecs {
		if v.ComponentDefRef == r.backupPolicy.ComponentDefRef {
			return &v
		}
	}
	return nil
}

func (r *BackupPolicyTplTransformer) defaultPolicyAnnotationValue() string {
	if r.tplCount > 1 && r.isDefaultTemplate != trueVal {
		return "false"
	}
	return trueVal
}

func (r *BackupPolicyTplTransformer) buildAnnotations() map[string]string {
	annotations := map[string]string{
		dptypes.DefaultBackupPolicyAnnotationKey:   r.defaultPolicyAnnotationValue(),
		constant.BackupPolicyTemplateAnnotationKey: r.backupPolicyTpl.Name,
	}
	if r.backupPolicyTpl.Annotations[dptypes.ReconfigureRefAnnotationKey] != "" {
		annotations[dptypes.ReconfigureRefAnnotationKey] = r.backupPolicyTpl.Annotations[dptypes.ReconfigureRefAnnotationKey]
	}
	return annotations
}

func (r *BackupPolicyTplTransformer) buildLabels() map[string]string {
	return map[string]string{
		constant.AppInstanceLabelKey:          r.OrigCluster.Name,
		constant.KBAppComponentDefRefLabelKey: r.backupPolicy.ComponentDefRef,
		constant.AppManagedByLabelKey:         constant.AppName,
	}
}

// buildTargetPodLabels builds the target labels for the backup policy that will be
// used to select the target pod.
func (r *BackupPolicyTplTransformer) buildTargetPodLabels(comp *appsv1alpha1.ClusterComponentSpec) map[string]string {
	labels := map[string]string{
		constant.AppInstanceLabelKey:    r.OrigCluster.Name,
		constant.KBAppComponentLabelKey: comp.Name,
		constant.AppManagedByLabelKey:   constant.AppName,
	}
	// append label to filter specific role of the component.
	targetTpl := &r.backupPolicy.Target
	if workloadHasRoleLabel(r.compWorkloadType) &&
		len(targetTpl.Role) > 0 && r.getCompReplicas() > 1 {
		// the role only works when the component has multiple replicas.
		labels[constant.RoleLabelKey] = targetTpl.Role
	}
	return labels
}

// generateBackupPolicyName generates the backup policy name which is created from backup policy template.
func generateBackupPolicyName(clusterName, componentDef, identifier string) string {
	if len(identifier) == 0 {
		return fmt.Sprintf("%s-%s-backup-policy", clusterName, componentDef)
	}
	return fmt.Sprintf("%s-%s-backup-policy-%s", clusterName, componentDef, identifier)
}

// generateBackupScheduleName generates the backup schedule name which is created from backup policy template.
func generateBackupScheduleName(clusterName, componentDef, identifier string) string {
	if len(identifier) == 0 {
		return fmt.Sprintf("%s-%s-backup-schedule", clusterName, componentDef)
	}
	return fmt.Sprintf("%s-%s-backup-schedule-%s", clusterName, componentDef, identifier)
}

func buildBackupPathPrefix(cluster *appsv1alpha1.Cluster, compName string) string {
	return fmt.Sprintf("/%s-%s/%s", cluster.Name, cluster.UID, compName)
}

func workloadHasRoleLabel(workloadType appsv1alpha1.WorkloadType) bool {
	return slices.Contains([]appsv1alpha1.WorkloadType{appsv1alpha1.Replication, appsv1alpha1.Consensus}, workloadType)
}

func mergeSchedulePolicy(src *dpv1alpha1.SchedulePolicy, dst *dpv1alpha1.SchedulePolicy) {
	if src.Enabled != nil {
		dst.Enabled = src.Enabled
	}
	if src.RetentionPeriod.String() != "" {
		dst.RetentionPeriod = src.RetentionPeriod
	}
	if src.BackupMethod != "" {
		dst.BackupMethod = src.BackupMethod
	}
	if src.CronExpression != "" {
		dst.CronExpression = src.CronExpression
	}
}
