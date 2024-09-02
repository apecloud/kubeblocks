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
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
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

// backupPolicyBuilder transforms the backup policy template to the data protection backup policy and backup schedule.
type clusterBackupPolicyTransformer struct {
	*clusterTransformContext
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
	transformBackupPolicy := func(bpBuilder *backupPolicyBuilder) *dpv1alpha1.BackupPolicy {
		// build the data protection backup policy from the template.
		oldBackupPolicy, newBackupPolicy := bpBuilder.transformBackupPolicy()
		if newBackupPolicy == nil {
			return nil
		}
		if oldBackupPolicy == nil {
			graphCli.Create(dag, newBackupPolicy)
		} else {
			graphCli.Patch(dag, oldBackupPolicy, newBackupPolicy)
		}
		return newBackupPolicy
	}

	transformBackupSchedule := func(bpBuilder *backupPolicyBuilder, backupPolicy *dpv1alpha1.BackupPolicy) {
		// if backup policy is nil, it means that the backup policy template
		// is invalid, backup schedule depends on backup policy, so we do
		// not need to transform backup schedule.
		if backupPolicy == nil {
			return
		}
		if bpBuilder.isHScaleTPL {
			r.V(1).Info("Skip creating backup schedule for the h-scale backup policy template", "template", bpBuilder.backupPolicyTPL.Name)
			return
		}
		// build the data protection backup schedule from the template.
		oldBackupSchedule, newBackupSchedule := bpBuilder.transformBackupSchedule(backupPolicy)
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
		newBackupSchedule = bpBuilder.mergeClusterBackup(backupPolicy, newBackupSchedule)
		if newBackupSchedule == nil {
			return
		}
		if oldBackupSchedule == nil {
			graphCli.Create(dag, newBackupSchedule)
		} else {
			graphCli.Patch(dag, oldBackupSchedule, newBackupSchedule)
		}
		graphCli.DependOn(dag, backupPolicy, newBackupSchedule)
		comps := graphCli.FindAll(dag, &appsv1.Component{})
		graphCli.DependOn(dag, backupPolicy, comps...)
	}

	transformBackupPolicyAndSchedule := func(compSpec *appsv1.ClusterComponentSpec, componentName, bptName string, isSharding, isHScaleTPL bool) error {
		bpt := &dpv1alpha1.BackupPolicyTemplate{}
		if err := r.Client.Get(ctx.GetContext(), client.ObjectKey{Name: bptName}, bpt); err != nil {
			return err
		}
		bpBuilder := newBackupPolicyBuilder(r, compSpec, bpt, componentName, isSharding, isHScaleTPL)
		policy := transformBackupPolicy(bpBuilder)
		// only merge the first backupSchedule for the cluster backup.
		transformBackupSchedule(bpBuilder, policy)
		return nil
	}

	transformComponentBackupPolicy := func(compSpec *appsv1.ClusterComponentSpec, componentName string, isSharding bool) error {
		compDef := r.ComponentDefs[compSpec.ComponentDef]
		if compDef == nil {
			return nil
		}
		if compDef.Spec.BackupPolicyTemplateName != "" {
			if err := transformBackupPolicyAndSchedule(compSpec, componentName, compDef.Spec.BackupPolicyTemplateName, isSharding, false); err != nil {
				return err
			}
		}
		hScaleBPTName := compDef.Annotations[constant.HorizontalScaleBackupPolicyTemplateKey]
		if hScaleBPTName != "" {
			if err := transformBackupPolicyAndSchedule(compSpec, componentName, hScaleBPTName, isSharding, true); err != nil {
				return err
			}
		}
		return nil
	}

	for i := range r.Cluster.Spec.ComponentSpecs {
		compSpec := &r.Cluster.Spec.ComponentSpecs[i]
		if err := transformComponentBackupPolicy(compSpec, compSpec.Name, false); err != nil {
			return err
		}
	}
	for i := range r.Cluster.Spec.ShardingSpecs {
		shardingSpec := r.Cluster.Spec.ShardingSpecs[i]
		if err := transformComponentBackupPolicy(&shardingSpec.Template, shardingSpec.Name, true); err != nil {
			return err
		}
	}
	return nil
}

type backupPolicyBuilder struct {
	context.Context
	Client client.Reader
	record.EventRecorder
	logr.Logger
	Cluster         *appsv1.Cluster
	isHScaleTPL     bool
	backupPolicyTPL *dpv1alpha1.BackupPolicyTemplate
	compSpec        *appsv1.ClusterComponentSpec
	componentName   string
	isSharding      bool
}

func newBackupPolicyBuilder(r *clusterBackupPolicyTransformer,
	compSpec *appsv1.ClusterComponentSpec,
	backupPolicyTPL *dpv1alpha1.BackupPolicyTemplate,
	componentName string,
	isSharding,
	isHScaleTPL bool) *backupPolicyBuilder {
	return &backupPolicyBuilder{
		Context:         r.Context,
		Client:          r.Client,
		EventRecorder:   r.EventRecorder,
		Logger:          r.Logger,
		Cluster:         r.Cluster,
		compSpec:        compSpec,
		backupPolicyTPL: backupPolicyTPL,
		componentName:   componentName,
		isHScaleTPL:     isHScaleTPL,
		isSharding:      isSharding,
	}
}

// transformBackupPolicy transforms backup policy template to backup policy.
func (r *backupPolicyBuilder) transformBackupPolicy() (*dpv1alpha1.BackupPolicy, *dpv1alpha1.BackupPolicy) {
	backupPolicyName := generateBackupPolicyName(r.Cluster.Name, r.componentName, r.isHScaleTPL)
	backupPolicy := &dpv1alpha1.BackupPolicy{}
	if err := r.Client.Get(r.Context, client.ObjectKey{
		Namespace: r.Cluster.Namespace,
		Name:      backupPolicyName,
	}, backupPolicy); client.IgnoreNotFound(err) != nil {
		r.Error(err, "failed to get backup policy", "backupPolicy", backupPolicyName)
		return nil, nil
	}

	if len(backupPolicy.Name) == 0 {
		// build a new backup policy by the backup policy template.
		return nil, r.buildBackupPolicy(backupPolicyName)
	}

	// sync the existing backup policy with the cluster changes
	old := backupPolicy.DeepCopy()
	r.syncBackupPolicy(backupPolicy)
	return old, backupPolicy
}

func (r *backupPolicyBuilder) transformBackupSchedule(
	backupPolicy *dpv1alpha1.BackupPolicy,
) (*dpv1alpha1.BackupSchedule, *dpv1alpha1.BackupSchedule) {
	scheduleName := generateBackupScheduleName(r.Cluster.Name, r.componentName)
	backupSchedule := &dpv1alpha1.BackupSchedule{}
	if err := r.Client.Get(r.Context, client.ObjectKey{
		Namespace: r.Cluster.Namespace,
		Name:      scheduleName,
	}, backupSchedule); client.IgnoreNotFound(err) != nil {
		r.Error(err, "failed to get backup schedule", "backupSchedule", scheduleName)
		return nil, nil
	}

	// build a new backup schedule from the backup policy template.
	if len(backupSchedule.Name) == 0 {
		return nil, r.buildBackupSchedule(scheduleName, backupPolicy)
	}

	old := backupSchedule.DeepCopy()
	r.syncBackupSchedule(backupSchedule)
	return old, backupSchedule
}

func (r *backupPolicyBuilder) setDefaultEncryptionConfig(backupPolicy *dpv1alpha1.BackupPolicy) {
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

func (r *backupPolicyBuilder) buildBackupSchedule(
	name string,
	backupPolicy *dpv1alpha1.BackupPolicy) *dpv1alpha1.BackupSchedule {
	if len(r.backupPolicyTPL.Spec.Schedules) == 0 {
		return nil
	}
	backupSchedule := &dpv1alpha1.BackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   r.Cluster.Namespace,
			Labels:      r.buildLabels(),
			Annotations: r.buildAnnotations(),
		},
		Spec: dpv1alpha1.BackupScheduleSpec{
			BackupPolicyName: backupPolicy.Name,
		},
	}

	var schedules []dpv1alpha1.SchedulePolicy
	for _, s := range r.backupPolicyTPL.Spec.Schedules {
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

func (r *backupPolicyBuilder) syncBackupSchedule(backupSchedule *dpv1alpha1.BackupSchedule) {
	scheduleMethodMap := map[string]struct{}{}
	for _, s := range backupSchedule.Spec.Schedules {
		scheduleMethodMap[s.BackupMethod] = struct{}{}
	}
	mergeMap(backupSchedule.Annotations, r.buildAnnotations())
	// update backupSchedule annotation to reconcile it.
	backupSchedule.Annotations[constant.ReconcileAnnotationKey] = r.Cluster.ResourceVersion
	// sync the newly added schedule policies.
	for _, s := range r.backupPolicyTPL.Spec.Schedules {
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
func (r *backupPolicyBuilder) syncBackupPolicy(backupPolicy *dpv1alpha1.BackupPolicy) {
	// update labels and annotations of the backup policy.
	if backupPolicy.Annotations == nil {
		backupPolicy.Annotations = map[string]string{}
	}
	if backupPolicy.Labels == nil {
		backupPolicy.Labels = map[string]string{}
	}
	mergeMap(backupPolicy.Annotations, r.buildAnnotations())
	mergeMap(backupPolicy.Labels, r.buildLabels())

	// update backup repo of the backup policy.
	if r.Cluster.Spec.Backup != nil && r.Cluster.Spec.Backup.RepoName != "" {
		backupPolicy.Spec.BackupRepoName = &r.Cluster.Spec.Backup.RepoName
	}
	backupPolicy.Spec.BackoffLimit = r.backupPolicyTPL.Spec.BackoffLimit
	r.syncBackupMethods(backupPolicy)
	r.syncBackupPolicyTargetSpec(backupPolicy)
}

func (r *backupPolicyBuilder) syncRoleLabelSelector(target *dpv1alpha1.BackupTarget, role, alternateRole, fullCompName string) {
	if len(role) == 0 || target == nil {
		return
	}
	podSelector := target.PodSelector
	if podSelector.LabelSelector == nil || podSelector.LabelSelector.MatchLabels == nil {
		podSelector.LabelSelector = &metav1.LabelSelector{MatchLabels: map[string]string{}}
	}
	if r.getCompReplicas(fullCompName) == 1 {
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

func (r *backupPolicyBuilder) getCompReplicas(fullCompName string) int32 {
	its := &workloads.InstanceSet{}
	name := fmt.Sprintf("%s-%s", r.Cluster.Name, fullCompName)
	if err := r.Client.Get(r.Context, client.ObjectKey{Name: name, Namespace: r.Cluster.Namespace}, its); err != nil {
		return r.compSpec.Replicas
	}
	return *its.Spec.Replicas
}

// buildBackupPolicy builds a new backup policy by the backup policy template.
func (r *backupPolicyBuilder) buildBackupPolicy(backupPolicyName string) *dpv1alpha1.BackupPolicy {
	backupPolicy := &dpv1alpha1.BackupPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:        backupPolicyName,
			Namespace:   r.Cluster.Namespace,
			Labels:      r.buildLabels(),
			Annotations: r.buildAnnotations(),
		},
	}
	r.syncBackupMethods(backupPolicy)
	bpSpec := backupPolicy.Spec
	// if cluster have backup repo, set backup repo name to backup policy.
	if r.Cluster.Spec.Backup != nil && r.Cluster.Spec.Backup.RepoName != "" {
		bpSpec.BackupRepoName = &r.Cluster.Spec.Backup.RepoName
	}
	bpSpec.PathPrefix = buildBackupPathPrefix(r.Cluster, r.componentName)
	bpSpec.BackoffLimit = r.backupPolicyTPL.Spec.BackoffLimit
	backupPolicy.Spec = bpSpec
	r.setDefaultEncryptionConfig(backupPolicy)
	r.syncBackupPolicyTargetSpec(backupPolicy)
	return backupPolicy
}

// syncBackupMethods syncs the backupMethod of tpl to backupPolicy.
func (r *backupPolicyBuilder) syncBackupMethods(backupPolicy *dpv1alpha1.BackupPolicy) {
	var backupMethods []dpv1alpha1.BackupMethod
	oldBackupMethodMap := map[string]dpv1alpha1.BackupMethod{}
	for _, v := range backupPolicy.Spec.BackupMethods {
		oldBackupMethodMap[v.Name] = v
	}
	for _, backupMethodTPL := range r.backupPolicyTPL.Spec.BackupMethods {
		backupMethod := dpv1alpha1.BackupMethod{
			Name:            backupMethodTPL.Name,
			ActionSetName:   backupMethodTPL.ActionSetName,
			SnapshotVolumes: backupMethodTPL.SnapshotVolumes,
			TargetVolumes:   backupMethodTPL.TargetVolumes,
			RuntimeSettings: backupMethodTPL.RuntimeSettings,
		}
		if m, ok := oldBackupMethodMap[backupMethodTPL.Name]; ok {
			backupMethod = m
			delete(oldBackupMethodMap, backupMethod.Name)
		} else if backupMethodTPL.Target != nil {
			if r.isSharding {
				backupMethod.Targets = r.buildBackupTargets(backupMethod.Targets)
			} else {
				backupMethod.Target = r.buildBackupTarget(backupMethod.Target, *backupMethodTPL.Target, r.componentName)
			}
		}
		backupMethod.Env = r.resolveBackupMethodEnv(r.compSpec, backupMethodTPL.Env)
		backupMethods = append(backupMethods, backupMethod)
	}
	for _, v := range oldBackupMethodMap {
		backupMethods = append(backupMethods, v)
	}
	backupPolicy.Spec.BackupMethods = backupMethods
}

func (r *backupPolicyBuilder) resolveBackupMethodEnv(compSpec *appsv1.ClusterComponentSpec, envs []dpv1alpha1.EnvVar) []corev1.EnvVar {
	var env []corev1.EnvVar
	for _, v := range envs {
		if v.Value != nil {
			env = append(env, corev1.EnvVar{Name: v.Name, Value: *v.Value})
			continue
		}
		if v.ValueFrom != nil {
			for _, versionMapping := range v.ValueFrom.VersionMapping {
				if !r.matchMappingName(versionMapping.ServiceVersions, compSpec.ServiceVersion) {
					continue
				}
				env = append(env, corev1.EnvVar{Name: v.Name, Value: versionMapping.MappedValue})
			}
		}
	}
	return env
}

func (r *backupPolicyBuilder) matchMappingName(names []string, target string) bool {
	for _, name := range names {
		if component.CompDefMatched(target, name) {
			return true
		}
	}
	return false
}

func (r *backupPolicyBuilder) syncBackupPolicyTargetSpec(backupPolicy *dpv1alpha1.BackupPolicy) {
	if r.isSharding {
		backupPolicy.Spec.Targets = r.buildBackupTargets(backupPolicy.Spec.Targets)
	} else {
		backupPolicy.Spec.Target = r.buildBackupTarget(backupPolicy.Spec.Target, r.backupPolicyTPL.Spec.Target, r.componentName)
	}
}

func (r *backupPolicyBuilder) buildBackupTargets(targets []dpv1alpha1.BackupTarget) []dpv1alpha1.BackupTarget {
	shardComponents, _ := intctrlutil.ListShardingComponents(r.Context, r.Client, r.Cluster, r.componentName)
	sourceTargetMap := map[string]*dpv1alpha1.BackupTarget{}
	for i := range targets {
		sourceTargetMap[targets[i].Name] = &targets[i]
	}
	var backupTargets []dpv1alpha1.BackupTarget
	for _, v := range shardComponents {
		fullComponentName := v.Labels[constant.KBAppComponentLabelKey]
		target := r.buildBackupTarget(sourceTargetMap[fullComponentName], r.backupPolicyTPL.Spec.Target, fullComponentName)
		if target != nil {
			backupTargets = append(backupTargets, *target)
		}
	}
	return backupTargets
}

func (r *backupPolicyBuilder) buildBackupTarget(
	oldTarget *dpv1alpha1.BackupTarget,
	targetTpl dpv1alpha1.TargetInstance,
	fullCompName string,
) *dpv1alpha1.BackupTarget {
	if oldTarget != nil {
		// if the target already exists, only sync the role by component replicas automatically.
		r.syncRoleLabelSelector(oldTarget, targetTpl.Role, targetTpl.FallbackRole, fullCompName)
		return oldTarget
	}
	clusterName := r.Cluster.Name
	if targetTpl.Strategy == "" {
		targetTpl.Strategy = dpv1alpha1.PodSelectionStrategyAny
	}
	target := &dpv1alpha1.BackupTarget{
		PodSelector: &dpv1alpha1.PodSelector{
			Strategy: targetTpl.Strategy,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: r.buildTargetPodLabels(targetTpl.Role, fullCompName),
			},
		},
		// dataprotection will use its dedicated service account if this field is empty.
		ServiceAccountName: "",
		ContainerPort:      targetTpl.ContainerPort,
	}
	if len(targetTpl.Role) != 0 && len(targetTpl.FallbackRole) != 0 {
		target.PodSelector.FallbackLabelSelector = &metav1.LabelSelector{
			MatchLabels: r.buildTargetPodLabels(targetTpl.FallbackRole, fullCompName),
		}
	}
	target.Name = fullCompName
	// build the target connection credential
	if targetTpl.Account != "" {
		target.ConnectionCredential = &dpv1alpha1.ConnectionCredential{
			SecretName:  constant.GenerateAccountSecretName(clusterName, fullCompName, targetTpl.Account),
			PasswordKey: constant.AccountPasswdForSecret,
			UsernameKey: constant.AccountNameForSecret,
		}
	}
	return target
}

func (r *backupPolicyBuilder) mergeClusterBackup(
	backupPolicy *dpv1alpha1.BackupPolicy,
	backupSchedule *dpv1alpha1.BackupSchedule,
) *dpv1alpha1.BackupSchedule {
	backupEnabled := func() bool {
		return r.Cluster.Spec.Backup != nil && boolValue(r.Cluster.Spec.Backup.Enabled)
	}
	if backupPolicy == nil || r.Cluster.Spec.Backup == nil {
		// backup policy is nil, can not enable cluster backup, so record event and return.
		if backupEnabled() {
			r.EventRecorder.Event(r.Cluster, corev1.EventTypeWarning,
				"BackupPolicyNotFound", "backup policy is nil, can not enable cluster backup")
		}
		return backupSchedule
	}

	backup := r.Cluster.Spec.Backup
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
				Name:        generateBackupScheduleName(r.Cluster.Name, r.componentName),
				Namespace:   r.Cluster.Namespace,
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

func (r *backupPolicyBuilder) defaultPolicyAnnotationValue() string {
	if r.isHScaleTPL {
		return "false"
	}
	return trueVal
}

func (r *backupPolicyBuilder) buildAnnotations() map[string]string {
	annotations := map[string]string{
		dptypes.DefaultBackupPolicyAnnotationKey:   r.defaultPolicyAnnotationValue(),
		constant.BackupPolicyTemplateAnnotationKey: r.backupPolicyTPL.Name,
	}
	if r.backupPolicyTPL.Annotations[dptypes.ReconfigureRefAnnotationKey] != "" {
		annotations[dptypes.ReconfigureRefAnnotationKey] = r.backupPolicyTPL.Annotations[dptypes.ReconfigureRefAnnotationKey]
	}
	return annotations
}

func (r *backupPolicyBuilder) buildLabels() map[string]string {
	labels := map[string]string{
		constant.AppManagedByLabelKey:        constant.AppName,
		constant.AppInstanceLabelKey:         r.Cluster.Name,
		constant.ComponentDefinitionLabelKey: r.compSpec.ComponentDef,
	}
	if r.isSharding {
		labels[constant.KBAppShardingNameLabelKey] = r.componentName
	} else {
		labels[constant.KBAppComponentLabelKey] = r.componentName
	}
	return labels
}

// buildTargetPodLabels builds the target labels for the backup policy that will be
// used to select the target pod.
func (r *backupPolicyBuilder) buildTargetPodLabels(role string, fullCompName string) map[string]string {
	labels := map[string]string{
		constant.AppInstanceLabelKey:    r.Cluster.Name,
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.KBAppComponentLabelKey: fullCompName,
	}
	// append label to filter specific role of the component.
	if len(role) > 0 && r.getCompReplicas(fullCompName) > 1 {
		// the role only works when the component has multiple replicas.
		labels[constant.RoleLabelKey] = role
	}
	if r.isSharding {
		labels[constant.KBAppShardingNameLabelKey] = r.componentName
	}
	return labels
}

// generateBackupPolicyName generates the backup policy name which is created from backup policy template.
func generateBackupPolicyName(clusterName, componentName string, isHScaleTPL bool) string {
	if isHScaleTPL {
		return fmt.Sprintf("%s-%s-backup-policy-hscale", clusterName, componentName)
	}
	return fmt.Sprintf("%s-%s-backup-policy", clusterName, componentName)
}

// generateBackupScheduleName generates the backup schedule name which is created from backup policy template.
func generateBackupScheduleName(clusterName, componentName string) string {
	return fmt.Sprintf("%s-%s-backup-schedule", clusterName, componentName)
}

func buildBackupPathPrefix(cluster *appsv1.Cluster, compName string) string {
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
