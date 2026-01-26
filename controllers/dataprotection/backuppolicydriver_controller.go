/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package dataprotection

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	dputils "github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils/boolptr"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	defaultCronExpression             = "0 18 * * *"
	disableSyncFromTemplateAnnotation = "dataprotection.kubeblocks.io/disable-sync-from-template"
)

// BackupPolicyDriverReconciler reconciles a BackupPolicy object
type BackupPolicyDriverReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=clusters/status,verbs=get

// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicies,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backuppolicies/status,verbs=get
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backupschedules,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=dataprotection.kubeblocks.io,resources=backupschedules/status,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the backuppolicy closer to the desired state.
func (r *BackupPolicyDriverReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqCtx := intctrlutil.RequestCtx{
		Ctx:      ctx,
		Req:      req,
		Log:      log.FromContext(ctx).WithValues("cluster", req.NamespacedName),
		Recorder: r.Recorder,
	}

	cluster := &appsv1.Cluster{}
	if err := r.Client.Get(reqCtx.Ctx, reqCtx.Req.NamespacedName, cluster); err != nil {
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	if model.IsObjectDeleting(cluster) {
		return intctrlutil.Reconciled()
	}
	crdAPIVersion := cluster.GetAnnotations()[constant.CRDAPIVersionAnnotationKey]
	if !intctrlutil.IsAPIVersionSupported(crdAPIVersion) {
		return intctrlutil.Reconciled()
	}
	if err := r.reconcile(reqCtx, cluster); err != nil {
		if apierrors.IsConflict(err) {
			return intctrlutil.Requeue(reqCtx.Log, err.Error())
		}
		r.Recorder.Eventf(cluster, corev1.EventTypeWarning, "ReconcileBackupPolicyFail", "failed to reconcile: %v", err)
		return intctrlutil.CheckedRequeueWithError(err, reqCtx.Log, "")
	}
	return intctrlutil.Reconciled()
}

func (r *BackupPolicyDriverReconciler) reconcile(reqCtx intctrlutil.RequestCtx, cluster *appsv1.Cluster) error {
	for i := range cluster.Spec.ComponentSpecs {
		compSpec := &cluster.Spec.ComponentSpecs[i]
		if err := r.transformComponentBackupPolicyAndSchedule(reqCtx, cluster, compSpec, compSpec.Name, false); err != nil {
			return err
		}
	}
	for i := range cluster.Spec.Shardings {
		spec := cluster.Spec.Shardings[i]
		if err := r.transformComponentBackupPolicyAndSchedule(reqCtx, cluster, &spec.Template, spec.Name, true); err != nil {
			return err
		}
	}
	return nil
}

func (r *BackupPolicyDriverReconciler) getBackupPolicyTemplate(reqCtx intctrlutil.RequestCtx, componentDef string) (*dpv1alpha1.BackupPolicyTemplate, error) {
	bptList := &dpv1alpha1.BackupPolicyTemplateList{}
	if err := r.Client.List(reqCtx.Ctx, bptList, client.MatchingLabels{
		componentDef: componentDef,
	}); err != nil {
		return nil, err
	}
	if len(bptList.Items) > 0 {
		return &bptList.Items[0], nil
	}
	return nil, nil
}

func (r *BackupPolicyDriverReconciler) transformComponentBackupPolicyAndSchedule(reqCtx intctrlutil.RequestCtx,
	cluster *appsv1.Cluster,
	compSpec *appsv1.ClusterComponentSpec,
	specName string,
	isSharding bool) error {
	if len(compSpec.ComponentDef) == 0 {
		return nil
	}
	bpt, err := r.getBackupPolicyTemplate(reqCtx, compSpec.ComponentDef)
	if err != nil {
		return err
	}
	if bpt == nil {
		return nil
	}
	bpAndScheduleBuilder := newBackupPolicyAndScheduleBuilder(reqCtx, r.Client, r.Scheme,
		cluster, compSpec, bpt, specName, isSharding)
	bp, err := bpAndScheduleBuilder.transformBackupPolicy()
	if err != nil {
		return err
	}
	return bpAndScheduleBuilder.transformBackupSchedule(bp)
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupPolicyDriverReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return intctrlutil.NewControllerManagedBy(mgr).
		For(&appsv1.Cluster{}).
		Owns(&dpv1alpha1.BackupPolicy{}).
		Owns(&dpv1alpha1.BackupSchedule{}).
		Complete(r)
}

type backupPolicyAndScheduleBuilder struct {
	context.Context
	Client client.Client
	record.EventRecorder
	logr.Logger
	schema          *runtime.Scheme
	Cluster         *appsv1.Cluster
	backupPolicyTPL *dpv1alpha1.BackupPolicyTemplate
	compSpec        *appsv1.ClusterComponentSpec
	componentName   string
	isSharding      bool
}

func newBackupPolicyAndScheduleBuilder(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	schema *runtime.Scheme,
	cluster *appsv1.Cluster,
	compSpec *appsv1.ClusterComponentSpec,
	backupPolicyTPL *dpv1alpha1.BackupPolicyTemplate,
	componentName string,
	isSharding bool) *backupPolicyAndScheduleBuilder {
	return &backupPolicyAndScheduleBuilder{
		Context:         reqCtx.Ctx,
		Client:          cli,
		EventRecorder:   reqCtx.Recorder,
		schema:          schema,
		Logger:          reqCtx.Log,
		Cluster:         cluster,
		compSpec:        compSpec,
		backupPolicyTPL: backupPolicyTPL,
		componentName:   componentName,
		isSharding:      isSharding,
	}
}

// transformBackupPolicy transforms backup policy template to backup policy.
func (r *backupPolicyAndScheduleBuilder) transformBackupPolicy() (*dpv1alpha1.BackupPolicy, error) {
	backupPolicyName := generateBackupPolicyName(r.Cluster.Name, r.componentName)
	backupPolicy := &dpv1alpha1.BackupPolicy{}
	if err := r.Client.Get(r.Context, client.ObjectKey{
		Namespace: r.Cluster.Namespace,
		Name:      backupPolicyName,
	}, backupPolicy); client.IgnoreNotFound(err) != nil {
		r.Error(err, "failed to get backup policy", "backupPolicy", backupPolicyName)
		return nil, err
	}

	if len(backupPolicy.Name) == 0 {
		// create a new backup policy by the backup policy template.
		backupPolicy = &dpv1alpha1.BackupPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:        backupPolicyName,
				Namespace:   r.Cluster.Namespace,
				Labels:      r.buildLabels(),
				Annotations: r.buildAnnotations(),
			},
		}
		if err := r.buildBackupPolicy(backupPolicy); err != nil {
			return nil, err
		}
		if err := controllerutil.SetControllerReference(r.Cluster, backupPolicy, r.schema); err != nil {
			return nil, err
		}
		return backupPolicy, r.Client.Create(r.Context, backupPolicy)
	}
	if err := r.buildBackupPolicy(backupPolicy); err != nil {
		return nil, err
	}
	return backupPolicy, r.Client.Update(r.Context, backupPolicy)
}

func (r *backupPolicyAndScheduleBuilder) transformBackupSchedule(bp *dpv1alpha1.BackupPolicy) error {
	if bp == nil {
		return nil
	}
	scheduleName := generateBackupScheduleName(r.Cluster.Name, r.componentName)
	backupSchedule := &dpv1alpha1.BackupSchedule{}
	if err := r.Client.Get(r.Context, client.ObjectKey{
		Namespace: r.Cluster.Namespace,
		Name:      scheduleName,
	}, backupSchedule); client.IgnoreNotFound(err) != nil {
		r.Error(err, "failed to get backup schedule", "backupSchedule", scheduleName)
		return err
	}

	// build a new backup schedule from the backup policy template.
	if len(backupSchedule.Name) == 0 {
		backupSchedule = r.buildBackupSchedule(scheduleName, bp)
		if backupSchedule == nil {
			return nil
		}
		if err := controllerutil.SetControllerReference(r.Cluster, backupSchedule, r.schema); err != nil {
			return err
		}
		r.mergeClusterBackup(bp, backupSchedule)
		return r.Client.Create(r.Context, backupSchedule)
	}
	r.syncBackupSchedule(backupSchedule)
	r.mergeClusterBackup(bp, backupSchedule)
	return r.Client.Update(r.Context, backupSchedule)
}

func (r *backupPolicyAndScheduleBuilder) buildBackupSchedule(
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
		name = s.GetScheduleName()
		schedules = append(schedules, dpv1alpha1.SchedulePolicy{
			BackupMethod:    s.BackupMethod,
			CronExpression:  s.CronExpression,
			Enabled:         s.Enabled,
			RetentionPeriod: s.RetentionPeriod,
			Name:            name,
			Parameters:      s.Parameters,
		})
	}
	backupSchedule.Spec.Schedules = schedules
	return backupSchedule
}

func (r *backupPolicyAndScheduleBuilder) syncBackupSchedule(backupSchedule *dpv1alpha1.BackupSchedule) {
	scheduleNameMap := map[string]struct{}{}
	for i := range backupSchedule.Spec.Schedules {
		s := &backupSchedule.Spec.Schedules[i]
		if len(s.Name) == 0 {
			// assign to backupMethod if name is empty.
			s.Name = s.BackupMethod
		}
		scheduleNameMap[s.Name] = struct{}{}
	}
	intctrlutil.MergeMetadataMapInplace(r.buildAnnotations(), &backupSchedule.Annotations)
	intctrlutil.MergeMetadataMapInplace(r.buildLabels(), &backupSchedule.Labels)
	// update backupSchedule annotation to reconcile it.
	backupSchedule.Annotations[constant.ReconcileAnnotationKey] = r.Cluster.ResourceVersion
	// sync the newly added schedule policies.
	for _, s := range r.backupPolicyTPL.Spec.Schedules {
		name := s.GetScheduleName()
		if _, ok := scheduleNameMap[name]; ok {
			continue
		}
		backupSchedule.Spec.Schedules = append(backupSchedule.Spec.Schedules, dpv1alpha1.SchedulePolicy{
			BackupMethod:    s.BackupMethod,
			CronExpression:  s.CronExpression,
			Enabled:         s.Enabled,
			RetentionPeriod: s.RetentionPeriod,
			Name:            name,
			Parameters:      s.Parameters,
		})
	}
}

// buildBackupPolicy builds a new backup policy by the backup policy template.
func (r *backupPolicyAndScheduleBuilder) buildBackupPolicy(backupPolicy *dpv1alpha1.BackupPolicy) error {
	bpSpec := &backupPolicy.Spec
	// if cluster have backup repo, set backup repo name to backup policy.
	if r.Cluster.Spec.Backup != nil && r.Cluster.Spec.Backup.RepoName != "" {
		bpSpec.BackupRepoName = &r.Cluster.Spec.Backup.RepoName
	}
	bpSpec.PathPrefix = buildBackupPathPrefix(r.Cluster, r.componentName)

	r.setDefaultEncryptionConfig(backupPolicy)

	intctrlutil.MergeMetadataMapInplace(r.buildAnnotations(), &backupPolicy.Annotations)
	intctrlutil.MergeMetadataMapInplace(r.buildLabels(), &backupPolicy.Labels)

	if err := r.buildBackupMethods(backupPolicy); err != nil {
		return err
	}

	if needSyncFromTemplate(backupPolicy) {
		bpSpec.BackoffLimit = r.backupPolicyTPL.Spec.BackoffLimit
		bpSpec.RetentionPolicy = r.backupPolicyTPL.Spec.RetentionPolicy
		if r.isSharding {
			targets, err := r.buildBackupTargets(backupPolicy.Spec.Targets)
			if err != nil {
				return err
			}
			bpSpec.Targets = targets
		} else {
			bpSpec.Target = r.buildBackupTarget(backupPolicy.Spec.Target, r.backupPolicyTPL.Spec.Target, r.componentName)
		}
	}
	return nil
}

func (r *backupPolicyAndScheduleBuilder) setDefaultEncryptionConfig(backupPolicy *dpv1alpha1.BackupPolicy) {
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

func (r *backupPolicyAndScheduleBuilder) syncRoleLabelSelectorWhenReplicaChanges(target *dpv1alpha1.BackupTarget, role, alternateRole, compName string) {
	if len(role) == 0 || target == nil {
		return
	}
	podSelector := target.PodSelector
	if podSelector.LabelSelector == nil || podSelector.LabelSelector.MatchLabels == nil {
		podSelector.LabelSelector = &metav1.LabelSelector{MatchLabels: map[string]string{}}
	}
	if r.compSpec.Replicas == 1 {
		delete(podSelector.LabelSelector.MatchLabels, constant.RoleLabelKey)
		if podSelector.FallbackLabelSelector != nil && podSelector.FallbackLabelSelector.MatchLabels != nil {
			delete(podSelector.FallbackLabelSelector.MatchLabels, constant.RoleLabelKey)
		}
	} else if podSelector.LabelSelector.MatchLabels[constant.RoleLabelKey] == "" {
		podSelector.LabelSelector.MatchLabels[constant.RoleLabelKey] = role
		if len(alternateRole) > 0 {
			if podSelector.FallbackLabelSelector == nil || podSelector.FallbackLabelSelector.MatchLabels == nil {
				podSelector.FallbackLabelSelector = &metav1.LabelSelector{MatchLabels: map[string]string{}}
			}
			podSelector.FallbackLabelSelector.MatchLabels[constant.RoleLabelKey] = alternateRole
		}
	}
}

// buildBackupMethods build the backupMethod of tpl to backupPolicy.
func (r *backupPolicyAndScheduleBuilder) buildBackupMethods(backupPolicy *dpv1alpha1.BackupPolicy) error {
	var backupMethods []dpv1alpha1.BackupMethod
	oldBackupMethodMap := map[string]dpv1alpha1.BackupMethod{}
	for _, v := range backupPolicy.Spec.BackupMethods {
		oldBackupMethodMap[v.Name] = v
	}
	for _, backupMethodTPL := range r.backupPolicyTPL.Spec.BackupMethods {
		if oldMethod, ok := oldBackupMethodMap[backupMethodTPL.Name]; ok && !needSyncFromTemplate(backupPolicy) {
			backupMethods = append(backupMethods, oldMethod)
			continue
		}
		backupMethod := dpv1alpha1.BackupMethod{
			Name:             backupMethodTPL.Name,
			CompatibleMethod: backupMethodTPL.CompatibleMethod,
			ActionSetName:    backupMethodTPL.ActionSetName,
			SnapshotVolumes:  backupMethodTPL.SnapshotVolumes,
			TargetVolumes:    backupMethodTPL.TargetVolumes,
			RuntimeSettings:  backupMethodTPL.RuntimeSettings,
		}
		if backupMethodTPL.Target != nil {
			if r.isSharding {
				targets, err := r.buildBackupTargets(backupMethod.Targets)
				if err != nil {
					return err
				}
				backupMethod.Targets = targets
			} else {
				backupMethod.Target = r.buildBackupTarget(backupMethod.Target, *backupMethodTPL.Target, r.componentName)
			}
		}
		backupMethod.Env = r.resolveBackupMethodEnv(r.compSpec, backupMethodTPL.Env)
		backupMethods = append(backupMethods, backupMethod)
	}
	backupPolicy.Spec.BackupMethods = backupMethods
	return nil
}

func (r *backupPolicyAndScheduleBuilder) resolveBackupMethodEnv(compSpec *appsv1.ClusterComponentSpec, envs []dpv1alpha1.EnvVar) []corev1.EnvVar {
	var env []corev1.EnvVar
	for _, v := range envs {
		if v.Value != nil {
			env = append(env, corev1.EnvVar{Name: v.Name, Value: *v.Value})
			continue
		}
		if v.ValueFrom != nil {
			mappedValue := findBestMatchingValue(v.ValueFrom.VersionMapping, compSpec.ServiceVersion)
			if mappedValue != "" {
				env = append(env, corev1.EnvVar{Name: v.Name, Value: mappedValue})
			}
		}
	}
	return env
}

// findBestMatchingValue finds the best matching value for the given service version.
// It prefers exact matches first, then falls back to prefix/regex matches.
func findBestMatchingValue(versionMappings []dpv1alpha1.VersionMapping, serviceVersion string) string {
	// First pass: look for exact match
	for _, versionMapping := range versionMappings {
		for _, v := range versionMapping.ServiceVersions {
			if v == serviceVersion {
				return versionMapping.MappedValue
			}
		}
	}
	// Second pass: look for prefix/regex match
	for _, versionMapping := range versionMappings {
		for _, v := range versionMapping.ServiceVersions {
			if component.PrefixOrRegexMatched(serviceVersion, v) {
				return versionMapping.MappedValue
			}
		}
	}
	return ""
}

func (r *backupPolicyAndScheduleBuilder) buildBackupTargets(targets []dpv1alpha1.BackupTarget) ([]dpv1alpha1.BackupTarget, error) {
	shardComponents, err := intctrlutil.ListShardingComponents(r.Context, r.Client, r.Cluster, r.componentName)
	if err != nil {
		return nil, err
	}
	if len(shardComponents) == 0 {
		return nil, fmt.Errorf("sharding components %s not found", r.componentName)
	}

	sourceTargetMap := map[string]*dpv1alpha1.BackupTarget{}
	for i := range targets {
		sourceTargetMap[targets[i].Name] = &targets[i]
	}
	var backupTargets []dpv1alpha1.BackupTarget
	for _, v := range shardComponents {
		compName := v.Labels[constant.KBAppComponentLabelKey]
		target := r.buildBackupTarget(sourceTargetMap[compName], r.backupPolicyTPL.Spec.Target, compName)
		if target != nil {
			backupTargets = append(backupTargets, *target)
		}
	}
	return backupTargets, nil
}

func (r *backupPolicyAndScheduleBuilder) buildBackupTarget(
	oldTarget *dpv1alpha1.BackupTarget,
	targetTpl dpv1alpha1.TargetInstance,
	compName string,
) *dpv1alpha1.BackupTarget {
	if oldTarget != nil {
		// if the target already exists, only sync the role by component replicas automatically.
		r.syncRoleLabelSelectorWhenReplicaChanges(oldTarget, targetTpl.Role, targetTpl.FallbackRole, compName)
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
				MatchLabels: r.buildTargetPodLabels(targetTpl.Role, compName),
			},
			UseParentSelectedPods: targetTpl.UseParentSelectedPods,
		},
		// dataprotection will use its dedicated service account if this field is empty.
		ServiceAccountName: "",
		ContainerPort:      targetTpl.ContainerPort,
	}
	if len(targetTpl.Role) != 0 && len(targetTpl.FallbackRole) != 0 {
		target.PodSelector.FallbackLabelSelector = &metav1.LabelSelector{
			MatchLabels: r.buildTargetPodLabels(targetTpl.FallbackRole, compName),
		}
	}
	if r.isSharding {
		target.Name = compName
	}
	// build the target connection credential
	if targetTpl.Account != "" {
		target.ConnectionCredential = &dpv1alpha1.ConnectionCredential{
			SecretName:  constant.GenerateAccountSecretName(clusterName, compName, targetTpl.Account),
			PasswordKey: constant.AccountPasswdForSecret,
			UsernameKey: constant.AccountNameForSecret,
		}
	}
	return target
}

func (r *backupPolicyAndScheduleBuilder) mergeClusterBackup(
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
	hasSyncIncMethod := false
	enableAutoBackup := boolptr.IsSetToTrue(backup.Enabled)
	for i := range backupSchedule.Spec.Schedules {
		s := &backupSchedule.Spec.Schedules[i]
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
		switch as.Spec.BackupType {
		case dpv1alpha1.BackupTypeContinuous:
			if backup.PITREnabled == nil {
				continue
			}
			if boolptr.IsSetToFalse(backup.PITREnabled) || hasSyncPITRMethod ||
				(len(backup.ContinuousMethod) > 0 && backup.ContinuousMethod != s.BackupMethod) {
				s.Enabled = boolptr.False()
				continue
			}
			// auto-sync the first or specified continuous backup for the 'pirtEnable' option.
			s.Enabled = backup.PITREnabled
			if backup.RetentionPeriod.String() != "" {
				s.RetentionPeriod = backup.RetentionPeriod
			}
			hasSyncPITRMethod = true
		case dpv1alpha1.BackupTypeIncremental:
			if len(backup.Method) == 0 || m.CompatibleMethod != backup.Method {
				// disable other incremental backup schedules
				s.Enabled = boolptr.False()
			} else if backup.IncrementalBackupEnabled != nil && !hasSyncIncMethod {
				// auto-sync the first compatible incremental backup for the 'incrementalBackupEnabled' option.
				mergeSchedulePolicy(&dpv1alpha1.SchedulePolicy{
					Enabled:         backup.IncrementalBackupEnabled,
					RetentionPeriod: backup.RetentionPeriod,
					CronExpression:  backup.IncrementalCronExpression,
				}, s)
				hasSyncIncMethod = true
			}
		case dpv1alpha1.BackupTypeFull:
			if enableAutoBackup {
				// disable the automatic backup for other full backup method
				s.Enabled = boolptr.False()
			}
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

func (r *backupPolicyAndScheduleBuilder) buildAnnotations() map[string]string {
	annotations := map[string]string{
		dptypes.DefaultBackupPolicyAnnotationKey:   "true",
		constant.BackupPolicyTemplateAnnotationKey: r.backupPolicyTPL.Name,
	}
	if r.backupPolicyTPL.Annotations[dptypes.ReconfigureRefAnnotationKey] != "" {
		annotations[dptypes.ReconfigureRefAnnotationKey] = r.backupPolicyTPL.Annotations[dptypes.ReconfigureRefAnnotationKey]
	}
	return annotations
}

func (r *backupPolicyAndScheduleBuilder) buildLabels() map[string]string {
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
func (r *backupPolicyAndScheduleBuilder) buildTargetPodLabels(role string, fullCompName string) map[string]string {
	labels := map[string]string{
		constant.AppInstanceLabelKey:    r.Cluster.Name,
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.KBAppComponentLabelKey: fullCompName,
	}
	// append label to filter specific role of the component.
	if len(role) > 0 && r.compSpec.Replicas > 1 {
		// the role only works when the component has multiple replicas.
		labels[constant.RoleLabelKey] = role
	}
	if r.isSharding {
		labels[constant.KBAppShardingNameLabelKey] = r.componentName
	}
	return labels
}

// generateBackupPolicyName generates the backup policy name which is created from backup policy template.
func generateBackupPolicyName(clusterName, componentName string) string {
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

func boolValue(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

func needSyncFromTemplate(bp *dpv1alpha1.BackupPolicy) bool {
	return bp.Annotations[disableSyncFromTemplateAnnotation] != "true"
}
