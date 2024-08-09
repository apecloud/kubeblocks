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

package apps

import (
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/gengo/examples/set-gen/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	dputils "github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils/boolptr"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	defaultCronExpression = "0 18 * * *"
)

// clusterBackupPolicyTransformer transforms the backup policy template to the data protection backup policy and backup schedule.
type clusterBackupPolicyTransformer struct {
	*clusterTransformContext
	tplCount          int
	tplIdentifier     string
	isDefaultTemplate string

	backupPolicyTpl *appsv1alpha1.BackupPolicyTemplate
	backupPolicy    *appsv1alpha1.BackupPolicy
}

type componentItem struct {
	compSpec *appsv1alpha1.ClusterComponentSpec
	// shardingSpec.Name or componentSpec.Name
	componentName string
	isSharding    bool
	// componentSpec.Name or component name label which creates by shardingSpec.
	fullComponentName string
}

var _ graph.Transformer = &clusterBackupPolicyTransformer{}

// Transform transforms the backup policy template to the backup policy and backup schedule.
func (r *clusterBackupPolicyTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	r.clusterTransformContext = ctx.(*clusterTransformContext)
	if model.IsObjectDeleting(r.clusterTransformContext.OrigCluster) {
		return nil
	}
	if common.IsCompactMode(r.clusterTransformContext.OrigCluster.Annotations) {
		r.clusterTransformContext.V(1).Info("Cluster is in compact mode, no need to create backup related objects",
			"cluster", client.ObjectKeyFromObject(r.clusterTransformContext.OrigCluster))
		return nil
	}

	graphCli, _ := r.clusterTransformContext.Client.(model.GraphClient)
	backupPolicyTPLs, err := r.getBackupPolicyTemplates()
	if err != nil {
		return err
	}
	r.tplCount = len(backupPolicyTPLs.Items)
	backupPolicyNames := map[string]struct{}{}
	backupScheduleNames := map[string]struct{}{}
	for _, tpl := range backupPolicyTPLs.Items {
		r.isDefaultTemplate = tpl.Annotations[dptypes.DefaultBackupPolicyTemplateAnnotationKey]
		r.tplIdentifier = tpl.Spec.Identifier
		r.backupPolicyTpl = &tpl

		for i := range tpl.Spec.BackupPolicies {
			r.backupPolicy = &tpl.Spec.BackupPolicies[i]

			transformBackupPolicy := func(comp componentItem) *dpv1alpha1.BackupPolicy {
				// build the data protection backup policy from the template.
				oldBackupPolicy, newBackupPolicy := r.transformBackupPolicy(comp)
				if newBackupPolicy == nil {
					return nil
				}

				// if exist multiple backup policy templates and duplicate spec.identifier,
				// the generated backupPolicy may have duplicate names, so it is
				// necessary to check if it already exists.
				if _, ok := backupPolicyNames[newBackupPolicy.Name]; ok {
					return nil
				}

				if oldBackupPolicy == nil {
					graphCli.Create(dag, newBackupPolicy)
				} else {
					graphCli.Patch(dag, oldBackupPolicy, newBackupPolicy)
				}
				backupPolicyNames[newBackupPolicy.Name] = struct{}{}
				return newBackupPolicy
			}

			transformBackupSchedule := func(comp componentItem, backupPolicy *dpv1alpha1.BackupPolicy, needMergeClusterBackup bool) {
				// if backup policy is nil, it means that the backup policy template
				// is invalid, backup schedule depends on backup policy, so we do
				// not need to transform backup schedule.
				if backupPolicy == nil {
					return
				}

				// only create backup schedule for the default backup policy template
				// if there are more than one backup policy templates.
				if r.isDefaultTemplate != trueVal && r.tplCount > 1 {
					r.V(1).Info("Skip creating backup schedule for non-default backup policy template", "template", tpl.Name)
					return
				}

				// build the data protection backup schedule from the template.
				oldBackupSchedule, newBackupSchedule := r.transformBackupSchedule(comp, backupPolicy)

				// merge cluster backup configuration into the backup schedule.
				// If the backup schedule is nil, create a new backup schedule
				// based on the cluster backup configuration.
				// For a cluster, the default backup schedule is created by backup
				// policy template, user can also configure cluster backup in the
				// cluster custom object, such as enable cluster backup, set backup
				// schedule, etc.
				// We always prioritize the cluster backup configuration in the
				// cluster object, so we need to merge the cluster backup configuration
				// into the default backup schedule created by backup policy template
				// if it exists.
				if needMergeClusterBackup {
					newBackupSchedule = r.mergeClusterBackup(comp, backupPolicy, newBackupSchedule)
				}
				if newBackupSchedule == nil {
					return
				}
				// if exist multiple backup policy templates and duplicate spec.identifier,
				// the backupSchedule that may be generated may have duplicate names,
				// and it is necessary to check if it already exists.
				if _, ok := backupScheduleNames[newBackupSchedule.Name]; ok {
					return
				}

				if oldBackupSchedule == nil {
					graphCli.Create(dag, newBackupSchedule)
				} else {
					graphCli.Patch(dag, oldBackupSchedule, newBackupSchedule)
				}
				graphCli.DependOn(dag, backupPolicy, newBackupSchedule)
				comps := graphCli.FindAll(dag, &appsv1alpha1.Component{})
				graphCli.DependOn(dag, backupPolicy, comps...)
				backupScheduleNames[newBackupSchedule.Name] = struct{}{}
			}

			// transform backup policy template to data protection backupPolicy
			// and backupSchedule
			compItems := r.getClusterComponentItems()
			for j, v := range compItems {
				policy := transformBackupPolicy(v)
				// only merge the first backupSchedule for the cluster backup.
				transformBackupSchedule(v, policy, j == 0)
			}
		}
	}
	return nil
}

// getBackupPolicyTemplates gets the backupPolicyTemplate for the cluster.
func (r *clusterBackupPolicyTransformer) getBackupPolicyTemplates() (*appsv1alpha1.BackupPolicyTemplateList, error) {
	backupPolicyTPLs := &appsv1alpha1.BackupPolicyTemplateList{}
	tplMap := map[string]sets.Empty{}
	for _, v := range r.ComponentDefs {
		tmpTPLs := &appsv1alpha1.BackupPolicyTemplateList{}
		// TODO: prefix match for componentDef name?
		if err := r.Client.List(r.Context, tmpTPLs, client.MatchingLabels{v.Name: v.Name}); err != nil {
			return nil, err
		}
		for i := range tmpTPLs.Items {
			if _, ok := tplMap[tmpTPLs.Items[i].Name]; !ok {
				backupPolicyTPLs.Items = append(backupPolicyTPLs.Items, tmpTPLs.Items[i])
				tplMap[tmpTPLs.Items[i].Name] = sets.Empty{}
			}
		}
	}
	return backupPolicyTPLs, nil
}

// transformBackupPolicy transforms backup policy template to backup policy.
func (r *clusterBackupPolicyTransformer) transformBackupPolicy(comp componentItem) (*dpv1alpha1.BackupPolicy, *dpv1alpha1.BackupPolicy) {
	cluster := r.OrigCluster
	backupPolicyName := generateBackupPolicyName(cluster.Name, comp.componentName, r.tplIdentifier)
	backupPolicy := &dpv1alpha1.BackupPolicy{}
	if err := r.Client.Get(r.Context, client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      backupPolicyName,
	}, backupPolicy); client.IgnoreNotFound(err) != nil {
		r.Error(err, "failed to get backup policy", "backupPolicy", backupPolicyName)
		return nil, nil
	}

	if len(backupPolicy.Name) == 0 {
		// build a new backup policy by the backup policy template.
		return nil, r.buildBackupPolicy(comp, backupPolicyName)
	}

	// sync the existing backup policy with the cluster changes
	old := backupPolicy.DeepCopy()
	r.syncBackupPolicy(comp, backupPolicy)
	return old, backupPolicy
}

func (r *clusterBackupPolicyTransformer) transformBackupSchedule(
	comp componentItem,
	backupPolicy *dpv1alpha1.BackupPolicy,
) (*dpv1alpha1.BackupSchedule, *dpv1alpha1.BackupSchedule) {
	cluster := r.OrigCluster
	scheduleName := generateBackupScheduleName(cluster.Name, comp.componentName, r.tplIdentifier)
	backupSchedule := &dpv1alpha1.BackupSchedule{}
	if err := r.Client.Get(r.Context, client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      scheduleName,
	}, backupSchedule); client.IgnoreNotFound(err) != nil {
		r.Error(err, "failed to get backup schedule", "backupSchedule", scheduleName)
		return nil, nil
	}

	// build a new backup schedule from the backup policy template.
	if len(backupSchedule.Name) == 0 {
		return nil, r.buildBackupSchedule(comp, scheduleName, backupPolicy)
	}

	old := backupSchedule.DeepCopy()
	r.syncBackupSchedule(backupSchedule)
	return old, backupSchedule
}

func (r *clusterBackupPolicyTransformer) setDefaultEncryptionConfig(backupPolicy *dpv1alpha1.BackupPolicy) {
	secretKeyRefJSON := viper.GetString(constant.CfgKeyDPBackupEncryptionSecretKeyRef)
	if secretKeyRefJSON == "" {
		return
	}
	secretKeyRef := &corev1.SecretKeySelector{}
	err := json.Unmarshal([]byte(secretKeyRefJSON), secretKeyRef)
	if err != nil {
		r.Error(err, "failed to unmarshal secretKeyRef", "json", secretKeyRefJSON)
		return
	}
	if secretKeyRef.Name == "" || secretKeyRef.Key == "" {
		return
	}
	algorithm := viper.GetString(constant.CfgKeyDPBackupEncryptionAlgorithm)
	if algorithm == "" {
		algorithm = dpv1alpha1.DefaultEncryptionAlgorithm
	}
	backupPolicy.Spec.EncryptionConfig = &dpv1alpha1.EncryptionConfig{
		Algorithm:              algorithm,
		PassPhraseSecretKeyRef: secretKeyRef,
	}
}

func (r *clusterBackupPolicyTransformer) buildBackupSchedule(
	comp componentItem,
	name string,
	backupPolicy *dpv1alpha1.BackupPolicy) *dpv1alpha1.BackupSchedule {
	if len(r.backupPolicy.Schedules) == 0 {
		return nil
	}
	cluster := r.OrigCluster
	backupSchedule := &dpv1alpha1.BackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   cluster.Namespace,
			Labels:      r.buildLabels(comp, backupPolicy),
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
			RetentionPeriod: s.RetentionPeriod,
		})
	}
	backupSchedule.Spec.Schedules = schedules
	return backupSchedule
}

func (r *clusterBackupPolicyTransformer) syncBackupSchedule(backupSchedule *dpv1alpha1.BackupSchedule) {
	scheduleMethodMap := map[string]struct{}{}
	for _, s := range backupSchedule.Spec.Schedules {
		scheduleMethodMap[s.BackupMethod] = struct{}{}
	}
	mergeMap(backupSchedule.Annotations, r.buildAnnotations())
	// update backupSchedule annotation to reconcile it.
	backupSchedule.Annotations[constant.ReconcileAnnotationKey] = r.Cluster.ResourceVersion
	// sync the newly added schedule policies.
	for _, s := range r.backupPolicy.Schedules {
		if _, ok := scheduleMethodMap[s.BackupMethod]; ok {
			continue
		}
		backupSchedule.Spec.Schedules = append(backupSchedule.Spec.Schedules, dpv1alpha1.SchedulePolicy{
			BackupMethod:    s.BackupMethod,
			CronExpression:  s.CronExpression,
			Enabled:         s.Enabled,
			RetentionPeriod: s.RetentionPeriod,
		})
	}
}

// syncBackupPolicy syncs labels and annotations of the backup policy with the cluster changes.
func (r *clusterBackupPolicyTransformer) syncBackupPolicy(comp componentItem, backupPolicy *dpv1alpha1.BackupPolicy) {
	// update labels and annotations of the backup policy.
	if backupPolicy.Annotations == nil {
		backupPolicy.Annotations = map[string]string{}
	}
	if backupPolicy.Labels == nil {
		backupPolicy.Labels = map[string]string{}
	}
	mergeMap(backupPolicy.Annotations, r.buildAnnotations())
	mergeMap(backupPolicy.Labels, r.buildLabels(comp, nil))

	// update backup repo of the backup policy.
	if r.Cluster.Spec.Backup != nil && r.Cluster.Spec.Backup.RepoName != "" {
		backupPolicy.Spec.BackupRepoName = &r.Cluster.Spec.Backup.RepoName
	}
	backupPolicy.Spec.BackoffLimit = r.backupPolicy.BackoffLimit
	r.syncBackupMethods(backupPolicy, comp)
	r.syncBackupPolicyTargetSpec(backupPolicy, comp)
}

func (r *clusterBackupPolicyTransformer) syncRoleLabelSelector(comp componentItem, target *dpv1alpha1.BackupTarget, role, alternateRole string) {
	if len(role) == 0 || target == nil {
		return
	}
	podSelector := target.PodSelector
	if podSelector.LabelSelector == nil || podSelector.LabelSelector.MatchLabels == nil {
		podSelector.LabelSelector = &metav1.LabelSelector{MatchLabels: map[string]string{}}
	}
	if r.getCompReplicas(comp) == 1 {
		delete(podSelector.LabelSelector.MatchLabels, constant.RoleLabelKey)
		if podSelector.FallbackLabelSelector != nil && podSelector.FallbackLabelSelector.MatchLabels != nil {
			delete(podSelector.FallbackLabelSelector.MatchLabels, constant.RoleLabelKey)
		}
	} else {
		podSelector.LabelSelector.MatchLabels[constant.RoleLabelKey] = role
		if len(alternateRole) > 0 {
			if podSelector.FallbackLabelSelector == nil || podSelector.FallbackLabelSelector.MatchLabels == nil {
				podSelector.FallbackLabelSelector = &metav1.LabelSelector{MatchLabels: map[string]string{}}
			}
			podSelector.FallbackLabelSelector.MatchLabels[constant.RoleLabelKey] = alternateRole
		}
	}
}

func (r *clusterBackupPolicyTransformer) getCompReplicas(comp componentItem) int32 {
	its := &workloads.InstanceSet{}
	name := fmt.Sprintf("%s-%s", r.Cluster.Name, comp.fullComponentName)
	if err := r.Client.Get(r.Context, client.ObjectKey{Name: name, Namespace: r.Cluster.Namespace}, its); err != nil {
		return comp.compSpec.Replicas
	}
	return *its.Spec.Replicas
}

// buildBackupPolicy builds a new backup policy by the backup policy template.
func (r *clusterBackupPolicyTransformer) buildBackupPolicy(comp componentItem, backupPolicyName string) *dpv1alpha1.BackupPolicy {
	cluster := r.OrigCluster
	backupPolicy := &dpv1alpha1.BackupPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:        backupPolicyName,
			Namespace:   cluster.Namespace,
			Labels:      r.buildLabels(comp, nil),
			Annotations: r.buildAnnotations(),
		},
	}
	r.syncBackupMethods(backupPolicy, comp)
	bpSpec := backupPolicy.Spec
	// if cluster have backup repo, set backup repo name to backup policy.
	if cluster.Spec.Backup != nil && cluster.Spec.Backup.RepoName != "" {
		bpSpec.BackupRepoName = &cluster.Spec.Backup.RepoName
	}
	bpSpec.PathPrefix = buildBackupPathPrefix(cluster, comp.componentName)
	bpSpec.BackoffLimit = r.backupPolicy.BackoffLimit
	backupPolicy.Spec = bpSpec
	r.setDefaultEncryptionConfig(backupPolicy)
	r.syncBackupPolicyTargetSpec(backupPolicy, comp)
	return backupPolicy
}

// syncBackupMethods syncs the backupMethod of tpl to backupPolicy.
func (r *clusterBackupPolicyTransformer) syncBackupMethods(backupPolicy *dpv1alpha1.BackupPolicy, comp componentItem) {
	var backupMethods []dpv1alpha1.BackupMethod
	oldBackupMethodMap := map[string]dpv1alpha1.BackupMethod{}
	for _, v := range backupPolicy.Spec.BackupMethods {
		oldBackupMethodMap[v.Name] = v
	}
	for _, v := range r.backupPolicy.BackupMethods {
		backupMethod := v.BackupMethod
		if m, ok := oldBackupMethodMap[backupMethod.Name]; ok {
			backupMethod = m
			delete(oldBackupMethodMap, backupMethod.Name)
		} else if v.Target != nil {
			if comp.isSharding {
				backupMethod.Targets = r.buildBackupTargets(backupMethod.Targets, comp)
			} else {
				backupMethod.Target = r.buildBackupTarget(backupMethod.Target, *v.Target, comp)
			}
		}
		mappingEnv := r.doEnvMapping(comp.compSpec, v.EnvMapping)
		backupMethod.Env = dputils.MergeEnv(backupMethod.Env, mappingEnv)
		backupMethods = append(backupMethods, backupMethod)
	}
	for _, v := range oldBackupMethodMap {
		backupMethods = append(backupMethods, v)
	}
	backupPolicy.Spec.BackupMethods = backupMethods
}

func (r *clusterBackupPolicyTransformer) doEnvMapping(comp *appsv1alpha1.ClusterComponentSpec, envMapping []appsv1alpha1.EnvMappingVar) []corev1.EnvVar {
	var env []corev1.EnvVar
	for _, v := range envMapping {
		for _, cm := range v.ValueFrom.ComponentDef {
			if !slices.Contains(cm.Names, comp.ComponentDef) {
				continue
			}
			env = append(env, corev1.EnvVar{
				Name:  v.Key,
				Value: cm.MappingValue,
			})
		}
	}
	return env
}

func (r *clusterBackupPolicyTransformer) syncBackupPolicyTargetSpec(backupPolicy *dpv1alpha1.BackupPolicy, comp componentItem) {
	if comp.isSharding {
		backupPolicy.Spec.Targets = r.buildBackupTargets(backupPolicy.Spec.Targets, comp)
	} else {
		backupPolicy.Spec.Target = r.buildBackupTarget(backupPolicy.Spec.Target, r.backupPolicy.Target, comp)
	}
}

func (r *clusterBackupPolicyTransformer) buildBackupTargets(targets []dpv1alpha1.BackupTarget, comp componentItem) []dpv1alpha1.BackupTarget {
	shardComponents, _ := intctrlutil.ListShardingComponents(r.Context, r.Client, r.Cluster, comp.componentName)
	sourceTargetMap := map[string]*dpv1alpha1.BackupTarget{}
	for i := range targets {
		sourceTargetMap[targets[i].Name] = &targets[i]
	}
	var backupTargets []dpv1alpha1.BackupTarget
	for _, v := range shardComponents {
		// set ClusterComponentSpec name to component name
		comp.fullComponentName = v.Labels[constant.KBAppComponentLabelKey]
		target := r.buildBackupTarget(sourceTargetMap[comp.fullComponentName], r.backupPolicy.Target, comp)
		if target != nil {
			backupTargets = append(backupTargets, *target)
		}
	}
	return backupTargets
}

func (r *clusterBackupPolicyTransformer) buildBackupTarget(
	oldTarget *dpv1alpha1.BackupTarget,
	targetTpl appsv1alpha1.TargetInstance,
	comp componentItem,
) *dpv1alpha1.BackupTarget {
	if oldTarget != nil {
		// if the target already exists, only sync the role by component replicas automatically.
		r.syncRoleLabelSelector(comp, oldTarget, targetTpl.Role, targetTpl.FallbackRole)
		return oldTarget
	}
	clusterName := r.OrigCluster.Name
	if targetTpl.Strategy == "" {
		targetTpl.Strategy = dpv1alpha1.PodSelectionStrategyAny
	}
	target := &dpv1alpha1.BackupTarget{
		PodSelector: &dpv1alpha1.PodSelector{
			Strategy: targetTpl.Strategy,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: r.buildTargetPodLabels(targetTpl.Role, comp),
			},
		},
		// dataprotection will use its dedicated service account if this field is empty.
		ServiceAccountName: "",
	}
	if len(targetTpl.Role) != 0 && len(targetTpl.FallbackRole) != 0 {
		target.PodSelector.FallbackLabelSelector = &metav1.LabelSelector{
			MatchLabels: r.buildTargetPodLabels(targetTpl.FallbackRole, comp),
		}
	}
	if comp.isSharding {
		target.Name = comp.fullComponentName
	}
	// build the target connection credential
	if targetTpl.Account != "" {
		target.ConnectionCredential = &dpv1alpha1.ConnectionCredential{
			SecretName:  constant.GenerateAccountSecretName(clusterName, comp.fullComponentName, targetTpl.Account),
			PasswordKey: constant.AccountPasswdForSecret,
			UsernameKey: constant.AccountNameForSecret,
		}
	}
	return target
}

func (r *clusterBackupPolicyTransformer) mergeClusterBackup(
	comp componentItem,
	backupPolicy *dpv1alpha1.BackupPolicy,
	backupSchedule *dpv1alpha1.BackupSchedule,
) *dpv1alpha1.BackupSchedule {
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
		return backupSchedule
	}

	backup := cluster.Spec.Backup
	method := dputils.GetBackupMethodByName(backup.Method, backupPolicy)
	// the specified backup method should be in the backup policy, if not, record event and return.
	if method == nil {
		r.EventRecorder.Event(r.Cluster, corev1.EventTypeWarning,
			"BackupMethodNotFound", fmt.Sprintf("backup method %s is not found in backup policy", backup.Method))
		return backupSchedule
	}

	// there is no backup schedule created by backup policy template, so we need to
	// create a new backup schedule for cluster backup.
	if backupSchedule == nil {
		backupSchedule = &dpv1alpha1.BackupSchedule{
			ObjectMeta: metav1.ObjectMeta{
				Name:        generateBackupScheduleName(cluster.Name, comp.componentName, r.tplIdentifier),
				Namespace:   cluster.Namespace,
				Labels:      r.buildLabels(comp, backupPolicy),
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
	// If cluster backup method is changed, we need to disable previous backup
	// method, for instance, the method is changed from A to B, we need to
	// disable A and enable B.
	exist := false
	hasSyncPITRMethod := false
	enableAutoBackup := boolptr.IsSetToTrue(backup.Enabled)
	for i, s := range backupSchedule.Spec.Schedules {
		if s.BackupMethod == backup.Method {
			mergeSchedulePolicy(sp, &backupSchedule.Spec.Schedules[i])
			exist = true
			continue
		}

		m := dputils.GetBackupMethodByName(s.BackupMethod, backupPolicy)
		if m == nil {
			continue
		}
		if m.ActionSetName == "" {
			if boolptr.IsSetToTrue(m.SnapshotVolumes) && enableAutoBackup {
				// disable the automatic backup when the specified method is not a volume snapshot for volume-snapshot method
				backupSchedule.Spec.Schedules[i].Enabled = boolptr.False()
			}
			continue
		}

		as := &dpv1alpha1.ActionSet{}
		if err := r.Client.Get(r.Context, client.ObjectKey{Name: m.ActionSetName}, as); err != nil {
			r.Error(err, "failed to get ActionSet for backup.", "ActionSet", as.Name)
			continue
		}
		if as.Spec.BackupType == dpv1alpha1.BackupTypeContinuous && backup.PITREnabled != nil && !hasSyncPITRMethod {
			// auto-sync the first continuous backup for the 'pirtEnable' option.
			backupSchedule.Spec.Schedules[i].Enabled = backup.PITREnabled
			hasSyncPITRMethod = true
		}
		if as.Spec.BackupType == dpv1alpha1.BackupTypeFull && enableAutoBackup {
			// disable the automatic backup for other full backup method
			backupSchedule.Spec.Schedules[i].Enabled = boolptr.False()
		}
	}
	if !exist {
		if sp.CronExpression == "" {
			sp.CronExpression = defaultCronExpression
		}
		backupSchedule.Spec.Schedules = append(backupSchedule.Spec.Schedules, *sp)
	}
	return backupSchedule
}

func (r *clusterBackupPolicyTransformer) getClusterComponentItems() []componentItem {
	matchedCompDef := func(compSpec appsv1alpha1.ClusterComponentSpec) bool {
		// TODO: support to create bp when using cluster topology and componentDef is empty
		if len(compSpec.ComponentDef) > 0 {
			for _, compDef := range r.backupPolicy.ComponentDefs {
				if strings.HasPrefix(compSpec.ComponentDef, compDef) || strings.HasPrefix(compDef, compSpec.ComponentDef) {
					return true
				}
			}
		}
		return false
	}
	var compSpecItems []componentItem
	for i, v := range r.clusterTransformContext.Cluster.Spec.ComponentSpecs {
		if matchedCompDef(v) {
			compSpecItems = append(compSpecItems, componentItem{
				compSpec:          &r.clusterTransformContext.Cluster.Spec.ComponentSpecs[i],
				componentName:     v.Name,
				fullComponentName: v.Name,
			})
		}
	}
	for i, v := range r.clusterTransformContext.Cluster.Spec.ShardingSpecs {
		shardComponents, _ := intctrlutil.ListShardingComponents(r.Context, r.Client, r.Cluster, v.Name)
		if len(shardComponents) == 0 {
			// waiting for sharding component to be created
			continue
		}
		if matchedCompDef(v.Template) {
			compSpecItems = append(compSpecItems, componentItem{
				compSpec:      &r.clusterTransformContext.Cluster.Spec.ShardingSpecs[i].Template,
				componentName: v.Name,
				isSharding:    true,
			})
		}
	}
	return compSpecItems
}

func (r *clusterBackupPolicyTransformer) defaultPolicyAnnotationValue() string {
	if r.tplCount > 1 && r.isDefaultTemplate != trueVal {
		return "false"
	}
	return trueVal
}

func (r *clusterBackupPolicyTransformer) buildAnnotations() map[string]string {
	annotations := map[string]string{
		dptypes.DefaultBackupPolicyAnnotationKey:   r.defaultPolicyAnnotationValue(),
		constant.BackupPolicyTemplateAnnotationKey: r.backupPolicyTpl.Name,
	}
	if r.backupPolicyTpl.Annotations[dptypes.ReconfigureRefAnnotationKey] != "" {
		annotations[dptypes.ReconfigureRefAnnotationKey] = r.backupPolicyTpl.Annotations[dptypes.ReconfigureRefAnnotationKey]
	}
	return annotations
}

func (r *clusterBackupPolicyTransformer) buildLabels(compItem componentItem, policy *dpv1alpha1.BackupPolicy) map[string]string {
	labels := map[string]string{
		constant.AppManagedByLabelKey:        constant.AppName,
		constant.AppInstanceLabelKey:         r.OrigCluster.Name,
		constant.ComponentDefinitionLabelKey: r.compDefName(compItem.compSpec, policy),
	}
	if compItem.isSharding {
		labels[constant.KBAppShardingNameLabelKey] = compItem.componentName
	} else {
		labels[constant.KBAppComponentLabelKey] = compItem.componentName
	}
	return labels
}

func (r *clusterBackupPolicyTransformer) compDefName(comp *appsv1alpha1.ClusterComponentSpec,
	policy *dpv1alpha1.BackupPolicy) string {
	switch {
	case comp != nil:
		return r.compDefNameFromSpec(comp)
	case policy != nil:
		return r.compDefNameFromPolicy(policy)
	default:
		panic("runtime error - unexpected way to get component definition name")
	}
}

func (r *clusterBackupPolicyTransformer) compDefNameFromSpec(comp *appsv1alpha1.ClusterComponentSpec) string {
	return comp.ComponentDef
}

func (r *clusterBackupPolicyTransformer) compDefNameFromPolicy(policy *dpv1alpha1.BackupPolicy) string {
	compDefName := ""
	if policy.Labels != nil {
		compDefName = policy.Labels[constant.ComponentDefinitionLabelKey]
	}
	return compDefName
}

// buildTargetPodLabels builds the target labels for the backup policy that will be
// used to select the target pod.
func (r *clusterBackupPolicyTransformer) buildTargetPodLabels(role string, comp componentItem) map[string]string {
	labels := map[string]string{
		constant.AppInstanceLabelKey:    r.OrigCluster.Name,
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.KBAppComponentLabelKey: comp.fullComponentName,
	}
	// append label to filter specific role of the component.
	if len(role) > 0 && r.getCompReplicas(comp) > 1 {
		// the role only works when the component has multiple replicas.
		labels[constant.RoleLabelKey] = role
	}
	if comp.isSharding {
		labels[constant.KBAppShardingNameLabelKey] = comp.componentName
	}
	return labels
}

// generateBackupPolicyName generates the backup policy name which is created from backup policy template.
func generateBackupPolicyName(clusterName, componentName, identifier string) string {
	if len(identifier) == 0 {
		return fmt.Sprintf("%s-%s-backup-policy", clusterName, componentName)
	}
	return fmt.Sprintf("%s-%s-backup-policy-%s", clusterName, componentName, identifier)
}

// generateBackupScheduleName generates the backup schedule name which is created from backup policy template.
func generateBackupScheduleName(clusterName, componentName, identifier string) string {
	if len(identifier) == 0 {
		return fmt.Sprintf("%s-%s-backup-schedule", clusterName, componentName)
	}
	return fmt.Sprintf("%s-%s-backup-schedule-%s", clusterName, componentName, identifier)
}

func buildBackupPathPrefix(cluster *appsv1alpha1.Cluster, compName string) string {
	return fmt.Sprintf("/%s-%s/%s", cluster.Name, cluster.UID, compName)
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
