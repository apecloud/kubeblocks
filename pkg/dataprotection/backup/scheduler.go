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

package backup

import (
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dperrors "github.com/apecloud/kubeblocks/pkg/dataprotection/errors"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	dputils "github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils/boolptr"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type Scheduler struct {
	intctrlutil.RequestCtx
	Client               client.Client
	Scheme               *k8sruntime.Scheme
	BackupSchedule       *dpv1alpha1.BackupSchedule
	BackupPolicy         *dpv1alpha1.BackupPolicy
	WorkerServiceAccount string
}

func (s *Scheduler) Schedule() error {
	if err := s.validate(); err != nil {
		return err
	}

	for i := range s.BackupSchedule.Spec.Schedules {
		if err := s.handleSchedulePolicy(i); err != nil {
			return err
		}
	}
	return nil
}

// validate validates the backup schedule.
func (s *Scheduler) validate() error {
	methodInBackupPolicy := func(name string) *dpv1alpha1.BackupMethod {
		for i, method := range s.BackupPolicy.Spec.BackupMethods {
			if method.Name == name {
				return &s.BackupPolicy.Spec.BackupMethods[i]
			}
		}
		return nil
	}

	// validate schedule names
	if err := dputils.ValidateScheduleNames(s.BackupSchedule.Spec.Schedules); err != nil {
		return err
	}

	for _, sp := range s.BackupSchedule.Spec.Schedules {
		method := methodInBackupPolicy(sp.BackupMethod)
		if method == nil {
			// backup method name is not in backup policy
			return fmt.Errorf("backup method %s is not in backup policy %s/%s",
				sp.BackupMethod, s.BackupPolicy.Namespace, s.BackupPolicy.Name)
		}
		// validate schedule parameters
		if len(method.ActionSetName) > 0 && len(sp.Parameters) > 0 {
			actionSet, err := dputils.GetActionSetByName(s.RequestCtx, s.Client, method.ActionSetName)
			if err != nil {
				return err
			}
			if err := dputils.ValidateParameters(actionSet, sp.Parameters, true); err != nil {
				return fmt.Errorf("fails to validate parameters of backupMethod %s: %v", sp.BackupMethod, err)
			}
		}
	}

	return nil
}

func (s *Scheduler) handleSchedulePolicy(index int) error {
	schedulePolicy := &s.BackupSchedule.Spec.Schedules[index]

	for _, method := range s.BackupPolicy.Spec.BackupMethods {
		if method.Name == schedulePolicy.BackupMethod && !boolptr.IsSetToTrue(method.SnapshotVolumes) {
			actionSet, err := dputils.GetActionSetByName(s.RequestCtx, s.Client, method.ActionSetName)
			if err != nil {
				return err
			}
			if actionSet.Spec.BackupType == dpv1alpha1.BackupTypeContinuous {
				if err = s.reconfigure(schedulePolicy); err != nil {
					return err
				}
				var targetSelectorLabels map[string]string
				if method.Target != nil {
					targetSelectorLabels = method.Target.PodSelector.MatchLabels
				} else if s.BackupPolicy.Spec.Target != nil {
					targetSelectorLabels = s.BackupPolicy.Spec.Target.PodSelector.MatchLabels
				}
				return s.reconcileForContinuous(schedulePolicy, targetSelectorLabels)
			}
		}
	}

	// create/delete/patch cronjob workload
	return s.reconcileCronJob(schedulePolicy)
}

// buildCronJob builds cronjob from backup schedule.
func (s *Scheduler) buildCronJob(schedulePolicy *dpv1alpha1.SchedulePolicy, cronJobName string) (*batchv1.CronJob, error) {
	var (
		successfulJobsHistoryLimit int32 = 0
		failedJobsHistoryLimit     int32 = 1
	)

	if cronJobName == "" {
		cronJobName = GenerateCRNameByScheduleNameAndMethod(s.BackupSchedule, schedulePolicy.BackupMethod, schedulePolicy.Name)
	}

	podSpec, err := s.buildPodSpec(schedulePolicy)
	if err != nil {
		return nil, err
	}

	cronjob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cronJobName,
			Namespace: s.BackupSchedule.Namespace,
			Labels:    map[string]string{},
		},
		Spec: batchv1.CronJobSpec{
			SuccessfulJobsHistoryLimit: &successfulJobsHistoryLimit,
			FailedJobsHistoryLimit:     &failedJobsHistoryLimit,
			ConcurrencyPolicy:          batchv1.ForbidConcurrent,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					BackoffLimit: s.BackupPolicy.Spec.BackoffLimit,
					Template: corev1.PodTemplateSpec{
						Spec: *podSpec,
					},
				},
			},
		},
	}

	timeZone, cronExpression := BuildCronJobSchedule(schedulePolicy.CronExpression)
	if timeZone != nil {
		cronjob.Spec.Schedule = schedulePolicy.CronExpression
		cronjob.Spec.TimeZone = timeZone
	} else {
		cronjob.Spec.Schedule = cronExpression
	}

	controllerutil.AddFinalizer(cronjob, dptypes.DataProtectionFinalizerName)
	// set labels
	for k, v := range s.BackupSchedule.Labels {
		cronjob.Labels[k] = v
	}
	cronjob.Labels[dptypes.BackupScheduleLabelKey] = s.BackupSchedule.Name
	cronjob.Labels[dptypes.BackupMethodLabelKey] = schedulePolicy.BackupMethod
	cronjob.Labels[constant.AppManagedByLabelKey] = dptypes.AppName
	return cronjob, nil
}

func (s *Scheduler) buildPodSpec(schedulePolicy *dpv1alpha1.SchedulePolicy) (*corev1.PodSpec, error) {
	// TODO(ldm): add backup deletionPolicy
	parameters, err := BuildParametersManifest(schedulePolicy.Parameters)
	if err != nil {
		return nil, err
	}
	checkCommand, err := s.buildCheckCommand(schedulePolicy)
	if err != nil {
		return nil, err
	}
	createBackupCmd := fmt.Sprintf(`%s
kubectl create -f - <<EOF
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: Backup
metadata:
  labels:
    dataprotection.kubeblocks.io/autobackup: "true"
    dataprotection.kubeblocks.io/backup-schedule: "%s"
  name: %s
  namespace: %s
spec:
  backupPolicyName: %s
  backupMethod: %s
  retentionPeriod: %s%s
EOF
`, checkCommand, s.BackupSchedule.Name, s.generateBackupName(schedulePolicy), s.BackupSchedule.Namespace,
		s.BackupPolicy.Name, schedulePolicy.BackupMethod,
		schedulePolicy.RetentionPeriod, parameters)

	container := corev1.Container{
		Name:            "backup-schedule",
		Image:           viper.GetString(constant.KBToolsImage),
		ImagePullPolicy: corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy)),
		Command:         []string{"sh", "-c"},
		Args:            []string{createBackupCmd},
	}
	intctrlutil.InjectZeroResourcesLimitsIfEmpty(&container)

	podSpec := &corev1.PodSpec{
		ServiceAccountName: s.WorkerServiceAccount,
		RestartPolicy:      corev1.RestartPolicyNever,
		Containers:         []corev1.Container{container},
	}
	if err := dputils.AddTolerations(podSpec); err != nil {
		return nil, err
	}
	return podSpec, nil
}

// reconcileCronJob will create/delete/patch cronjob according to cronExpression and policy changes.
func (s *Scheduler) reconcileCronJob(schedulePolicy *dpv1alpha1.SchedulePolicy) error {
	// get cronjob from labels
	cronJob := &batchv1.CronJob{}
	cronJobList := &batchv1.CronJobList{}
	if err := s.Client.List(s.Ctx, cronJobList,
		client.InNamespace(s.BackupSchedule.Namespace),
		client.MatchingLabels{
			dptypes.BackupScheduleLabelKey: s.BackupSchedule.Name,
			dptypes.BackupMethodLabelKey:   schedulePolicy.BackupMethod,
		},
	); err != nil {
		return err
	} else if len(cronJobList.Items) > 0 {
		// the schedulePolicy name can be empty
		targetCronJobName := GenerateCRNameByScheduleNameAndMethod(s.BackupSchedule, schedulePolicy.BackupMethod, schedulePolicy.Name)
		for i, item := range cronJobList.Items {
			if item.Name == targetCronJobName {
				cronJob = &cronJobList.Items[i]
				break
			}
		}
	}

	// schedule is disabled, delete cronjob if exists
	if !boolptr.IsSetToTrue(schedulePolicy.Enabled) {
		if len(cronJob.Name) != 0 {
			// delete the old cronjob.
			if err := dputils.RemoveDataProtectionFinalizer(s.Ctx, s.Client, cronJob); err != nil {
				return err
			}
			return s.Client.Delete(s.Ctx, cronJob)
		}
		// if cronjob does not exist, return
		return nil
	}

	cronjobProto, err := s.buildCronJob(schedulePolicy, cronJob.Name)
	if err != nil {
		return err
	}

	if s.BackupSchedule.Spec.StartingDeadlineMinutes != nil {
		startingDeadlineSeconds := *s.BackupSchedule.Spec.StartingDeadlineMinutes * 60
		cronjobProto.Spec.StartingDeadlineSeconds = &startingDeadlineSeconds
	}

	if len(cronJob.Name) == 0 {
		// if no cronjob, create it.
		return s.Client.Create(s.Ctx, cronjobProto)
	}

	if reflect.DeepEqual(cronJob.Spec, cronjobProto.Spec) &&
		reflect.DeepEqual(cronJob.Labels, cronjobProto.Labels) &&
		reflect.DeepEqual(cronJob.Annotations, cronjobProto.Annotations) {
		return nil
	}

	// sync the cronjob with the current backup policy configuration.
	patch := client.MergeFrom(cronJob.DeepCopy())
	cronJob.Spec = cronjobProto.Spec
	cronJob.Labels = cronjobProto.Labels
	cronJob.Annotations = cronjobProto.Annotations
	return s.Client.Patch(s.Ctx, cronJob, patch)
}

func (s *Scheduler) generateBackupName(schedulePolicy *dpv1alpha1.SchedulePolicy) string {
	var backupNamePrefix string
	targets := dputils.GetBackupTargets(s.BackupPolicy, dputils.GetBackupMethodByName(schedulePolicy.BackupMethod, s.BackupPolicy))
	if len(targets) > 0 {
		// if cluster name can be found in target labels, use it as backup name prefix
		backupNamePrefix = targets[0].PodSelector.MatchLabels[constant.AppInstanceLabelKey]

	}
	// if cluster name can not be found, use backup schedule name as backup name prefix
	if backupNamePrefix == "" {
		backupNamePrefix = s.BackupSchedule.Name
	}
	// use schedule name to distinguish different schedule policies
	name := schedulePolicy.GetScheduleName()
	if len(name) > 0 {
		backupNamePrefix = fmt.Sprintf("%s-%s", backupNamePrefix, name)
	}
	return backupNamePrefix + "-$(date -u +'%Y%m%d%H%M%S')"
}

func (s *Scheduler) getGenerateContinuousBackup(schedulePolicy *dpv1alpha1.SchedulePolicy) (*dpv1alpha1.Backup, error) {
	backup := &dpv1alpha1.Backup{}
	backupName := GenerateCRNameByBackupSchedule(s.BackupSchedule, schedulePolicy.BackupMethod)
	exists, err := intctrlutil.CheckResourceExists(s.Ctx, s.Client, client.ObjectKey{Name: backupName,
		Namespace: s.BackupSchedule.Namespace}, backup)
	if err != nil {
		return nil, err
	}
	if exists {
		return backup, nil
	}
	// if no backup found, check if existing legacy backup.
	backupName = GenerateLegacyCRNameByBackupSchedule(s.BackupSchedule, schedulePolicy.BackupMethod)
	if _, err = intctrlutil.CheckResourceExists(s.Ctx, s.Client, client.ObjectKey{Name: backupName,
		Namespace: s.BackupSchedule.Namespace}, backup); err != nil {
		return nil, err
	}
	return backup, nil
}

func (s *Scheduler) reconcileForContinuous(schedulePolicy *dpv1alpha1.SchedulePolicy,
	targetSelectorLabels map[string]string) error {
	backup, err := s.getGenerateContinuousBackup(schedulePolicy)
	if err != nil {
		return err
	}
	patch := client.MergeFrom(backup.DeepCopy())
	if backup.Labels == nil {
		backup.Labels = map[string]string{}
	}
	for k, v := range targetSelectorLabels {
		backup.Labels[k] = v
	}
	backup.Labels[constant.AppManagedByLabelKey] = dptypes.AppName
	backup.Labels[dptypes.BackupScheduleLabelKey] = s.BackupSchedule.Name
	backup.Labels[dptypes.BackupTypeLabelKey] = string(dpv1alpha1.BackupTypeContinuous)
	backup.Labels[dptypes.AutoBackupLabelKey] = "true"
	if backup.Name == "" {
		if boolptr.IsSetToFalse(schedulePolicy.Enabled) {
			return nil
		}
		backup.Name = GenerateCRNameByBackupSchedule(s.BackupSchedule, schedulePolicy.BackupMethod)
		backup.Namespace = s.BackupSchedule.Namespace
		backup.Spec.BackupMethod = schedulePolicy.BackupMethod
		backup.Spec.BackupPolicyName = s.BackupSchedule.Spec.BackupPolicyName
		backup.Spec.RetentionPeriod = schedulePolicy.RetentionPeriod
		return intctrlutil.IgnoreIsAlreadyExists(s.Client.Create(s.Ctx, backup))
	}

	// notice to reconcile backup CR
	if boolptr.IsSetToTrue(schedulePolicy.Enabled) && backup.Status.Phase == dpv1alpha1.BackupPhaseCompleted {
		// if schedule is enabled and backup already is Completed, update phase to running
		backup.Status.Phase = dpv1alpha1.BackupPhaseRunning
		backup.Status.FailureReason = ""
		return s.Client.Status().Patch(s.Ctx, backup, patch)
	}
	if backup.Annotations == nil {
		backup.Annotations = map[string]string{}
	}
	backup.Spec.RetentionPeriod = schedulePolicy.RetentionPeriod
	backup.Annotations[constant.ReconcileAnnotationKey] = s.BackupSchedule.ResourceVersion
	return s.Client.Patch(s.Ctx, backup, patch)
}

type backupReconfigureRef struct {
	Name    string         `json:"name"`
	Key     string         `json:"key"`
	Enable  parameterPairs `json:"enable,omitempty"`
	Disable parameterPairs `json:"disable,omitempty"`
}

type parameterPairs map[string][]opsv1alpha1.ParameterPair

// @Deprecated remove it in next release, only compatible with old release.
func (s *Scheduler) convertLastAppliedConfigs(continuousMethod string) {
	if _, ok := s.BackupSchedule.Annotations[dptypes.LastAppliedConfigsAnnotationKey]; ok {
		return
	}
	lastAppliedConfig := s.BackupSchedule.Annotations[constant.LastAppliedConfigAnnotationKey]
	if lastAppliedConfig == "" {
		return
	}
	lastAppliedConfigMap := map[string]string{}
	lastAppliedConfigMap[continuousMethod] = lastAppliedConfig
	str, _ := json.Marshal(lastAppliedConfigMap)
	s.BackupSchedule.Annotations[dptypes.LastAppliedConfigsAnnotationKey] = string(str)
}

func (s *Scheduler) getLastAppliedConfigsMap() (map[string]string, error) {
	lastAppliedConfigAnno := s.BackupSchedule.Annotations[dptypes.LastAppliedConfigsAnnotationKey]
	if lastAppliedConfigAnno == "" {
		return map[string]string{}, nil
	}
	resMap := map[string]string{}
	if err := json.Unmarshal([]byte(lastAppliedConfigAnno), &resMap); err != nil {
		return nil, err
	}
	return resMap, nil
}

func (s *Scheduler) reconfigure(schedulePolicy *dpv1alpha1.SchedulePolicy) error {
	reCfgRef := s.BackupSchedule.Annotations[dptypes.ReconfigureRefAnnotationKey]
	if reCfgRef == "" {
		return nil
	}
	// convert deprecated "lastAppliedConfig "to "lastAppliedConfigs"
	s.convertLastAppliedConfigs(schedulePolicy.BackupMethod)
	configRef := backupReconfigureRef{}
	if err := json.Unmarshal([]byte(reCfgRef), &configRef); err != nil {
		return err
	}
	enable := boolptr.IsSetToTrue(schedulePolicy.Enabled)
	if s.BackupSchedule.Annotations[dptypes.LastAppliedConfigsAnnotationKey] == "" && !enable {
		// disable in the first policy created, no need reconfigure because default configs had been set.
		return nil
	}
	configParameters := configRef.Disable
	if enable {
		configParameters = configRef.Enable
	}
	if configParameters == nil {
		return nil
	}
	parameters := configParameters[schedulePolicy.BackupMethod]
	if len(parameters) == 0 {
		// skip reconfigure if not found parameters.
		return nil
	}
	lastAppliedConfigsMap, err := s.getLastAppliedConfigsMap()
	if err != nil {
		return err
	}
	updateParameterPairsBytes, _ := json.Marshal(parameters)
	updateParameterPairs := string(updateParameterPairsBytes)
	if updateParameterPairs == lastAppliedConfigsMap[schedulePolicy.BackupMethod] {
		// reconcile the config job if finished
		return s.reconcileReconfigure(s.BackupSchedule)
	}
	targets := dputils.GetBackupTargets(s.BackupPolicy, dputils.GetBackupMethodByName(schedulePolicy.BackupMethod, s.BackupPolicy))
	if len(targets) == 0 {
		return intctrlutil.NewFatalError(fmt.Sprintf(`spec.target and spec.targets can not be empty in backupPOlicy "%s"`, s.BackupPolicy.Name))
	}
	targetPodSelector := targets[0].PodSelector
	clusterName := targetPodSelector.MatchLabels[constant.AppInstanceLabelKey]
	cluster := &appsv1.Cluster{}
	if err := s.Client.Get(s.Ctx, client.ObjectKey{Name: clusterName, Namespace: s.BackupSchedule.Namespace}, cluster); err != nil {
		return err
	}
	if !slices.Contains(appsv1.GetReconfiguringRunningPhases(), cluster.Status.Phase) {
		return intctrlutil.NewErrorf(intctrlutil.ErrorTypeRequeue, "requeue to waiting for the cluster %s to be available.", clusterName)
	}
	ops := opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: s.BackupSchedule.Name + "-",
			Namespace:    s.BackupSchedule.Namespace,
			Labels: map[string]string{
				dptypes.BackupScheduleLabelKey: s.BackupSchedule.Name,
			},
		},
		Spec: opsv1alpha1.OpsRequestSpec{
			Type:        opsv1alpha1.ReconfiguringType,
			ClusterName: clusterName,
			SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{
				Reconfigures: []opsv1alpha1.Reconfigure{
					{
						ComponentOps: opsv1alpha1.ComponentOps{
							ComponentName: targetPodSelector.MatchLabels[constant.KBAppComponentLabelKey],
						},
						Configurations: []opsv1alpha1.ConfigurationItem{
							{
								Name: configRef.Name,
								Keys: []opsv1alpha1.ParameterConfig{
									{
										Key:        configRef.Key,
										Parameters: parameters,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	if err := s.Client.Create(s.Ctx, &ops); err != nil {
		return err
	}
	s.Recorder.Eventf(s.BackupSchedule, corev1.EventTypeNormal, "Reconfiguring", "update config %s", updateParameterPairs)
	patch := client.MergeFrom(s.BackupSchedule.DeepCopy())
	if s.BackupSchedule.Annotations == nil {
		s.BackupSchedule.Annotations = map[string]string{}
	}
	lastAppliedConfigsMap[schedulePolicy.BackupMethod] = updateParameterPairs
	updateParameterPairsBytes, _ = json.Marshal(lastAppliedConfigsMap)
	s.BackupSchedule.Annotations[dptypes.LastAppliedConfigsAnnotationKey] = string(updateParameterPairsBytes)
	delete(s.BackupSchedule.Annotations, constant.LastAppliedConfigAnnotationKey)
	if err := s.Client.Patch(s.Ctx, s.BackupSchedule, patch); err != nil {
		return err
	}
	return intctrlutil.NewErrorf(intctrlutil.ErrorTypeRequeue, "requeue to waiting for ops %s finished.", ops.Name)
}

func (s *Scheduler) reconcileReconfigure(backupSchedule *dpv1alpha1.BackupSchedule) error {
	opsList := opsv1alpha1.OpsRequestList{}
	if err := s.Client.List(s.Ctx, &opsList,
		client.InNamespace(backupSchedule.Namespace),
		client.MatchingLabels{dptypes.BackupScheduleLabelKey: backupSchedule.Name}); err != nil {
		return err
	}
	if len(opsList.Items) > 0 {
		sort.Slice(opsList.Items, func(i, j int) bool {
			return opsList.Items[j].CreationTimestamp.Before(&opsList.Items[i].CreationTimestamp)
		})
		latestOps := opsList.Items[0]
		if latestOps.Status.Phase == opsv1alpha1.OpsFailedPhase {
			return intctrlutil.NewErrorf(dperrors.ErrorTypeReconfigureFailed, "ops failed %s", latestOps.Name)
		} else if latestOps.Status.Phase != opsv1alpha1.OpsSucceedPhase {
			return intctrlutil.NewErrorf(intctrlutil.ErrorTypeRequeue, "waiting for ops %s finished.", latestOps.Name)
		}
	}
	return nil
}

func (s *Scheduler) buildCheckCommand(schedulePolicy *dpv1alpha1.SchedulePolicy) (string, error) {
	backupMethod := dputils.GetBackupMethodByName(schedulePolicy.BackupMethod, s.BackupPolicy)
	actionSet, err := dputils.GetActionSetByName(s.RequestCtx, s.Client, backupMethod.ActionSetName)
	if err != nil {
		return "", err
	}
	// command is used by incremental backup
	if backupType := dputils.GetBackupType(actionSet, backupMethod.SnapshotVolumes); backupType != dpv1alpha1.BackupTypeIncremental {
		return "", nil
	}
	// filter completed full backup, if there is no completed full backup, exit.
	labelMap := map[string]string{
		dptypes.BackupPolicyLabelKey: s.BackupSchedule.Spec.BackupPolicyName,
		dptypes.BackupTypeLabelKey:   string(dpv1alpha1.BackupTypeFull),
	}
	labelSlice := []string{}
	for k, v := range labelMap {
		labelSlice = append(labelSlice, fmt.Sprintf("%s=%s", k, v))
	}
	checkCommand := fmt.Sprintf(`
repoName=$(kubectl -n "%s" get backuppolicies.dataprotection.kubeblocks.io "%s" -o jsonpath={.spec.backupRepoName})
if [ -z "$repoName" ]; then
	defaultRepos=$(kubectl get backuprepos.dataprotection.kubeblocks.io -o jsonpath='{range .items[?(@.metadata.annotations.dataprotection\.kubeblocks\.io/is-default-repo=="true")]}{.metadata.name}{"\t"}{end}')
	if [ -z "$defaultRepos" ]; then
		echo "No default backupRepo found. Exiting."
		exit 0
	elif [ $(echo $defaultRepos | wc -w) -ne 1 ]; then
		echo "Multiple default backupRepo found. Exiting."
		exit 0
	fi
	repoName=$(echo $defaultRepos | awk '{print $1}')
fi
repoLabel=",dataprotection.kubeblocks.io/backup-repo-name=$repoName"
count=$(kubectl get backups.dataprotection.kubeblocks.io -n %s --selector=%s$repoLabel -o jsonpath='{range .items[?(@.spec.backupMethod=="%s")]}{.status.phase}{"\n"}{end}' | grep "Completed" | wc -l)
if [ "$count" -eq 0 ]; then
    echo "No completed full backups found. Exiting."
    exit 0
fi
`,
		s.BackupSchedule.Namespace,
		s.BackupSchedule.Spec.BackupPolicyName,
		s.BackupSchedule.Namespace,
		strings.Join(labelSlice, ","),
		backupMethod.CompatibleMethod,
	)
	return checkCommand, nil
}
