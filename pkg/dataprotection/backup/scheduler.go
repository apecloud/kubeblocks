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

package backup

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	dputils "github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils/boolptr"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type Scheduler struct {
	intctrlutil.RequestCtx
	Client         client.Client
	Scheme         *k8sruntime.Scheme
	BackupSchedule *dpv1alpha1.BackupSchedule
	BackupPolicy   *dpv1alpha1.BackupPolicy
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
	methodInBackupPolicy := func(name string) bool {
		for _, method := range s.BackupPolicy.Spec.BackupMethods {
			if method.Name == name {
				return true
			}
		}
		return false
	}

	for _, sp := range s.BackupSchedule.Spec.Schedules {
		if methodInBackupPolicy(sp.BackupMethod) {
			continue
		}
		// backup method name is not in backup policy
		return fmt.Errorf("backup method %s is not in backup policy %s/%s",
			sp.BackupMethod, s.BackupPolicy.Namespace, s.BackupPolicy.Name)
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
				// ignore continuous backup
				return nil
			}
		}
	}

	// create/delete/patch cronjob workload
	return s.reconcileCronJob(schedulePolicy)
}

// buildCronJob builds cronjob from backup schedule.
func (s *Scheduler) buildCronJob(
	schedulePolicy *dpv1alpha1.SchedulePolicy,
	cronJobName string) (*batchv1.CronJob, error) {
	var (
		successfulJobsHistoryLimit int32 = 0
		failedJobsHistoryLimit     int32 = 1
	)

	if cronJobName == "" {
		cronJobName = GenerateCRNameByBackupSchedule(s.BackupSchedule, schedulePolicy.BackupMethod)
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
	createBackupCmd := fmt.Sprintf(`
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
  retentionPeriod: %s
EOF
`, s.BackupSchedule.Name, s.generateBackupName(), s.BackupSchedule.Namespace,
		s.BackupPolicy.Name, schedulePolicy.BackupMethod,
		schedulePolicy.RetentionPeriod)

	container := corev1.Container{
		Name:            "backup-schedule",
		Image:           viper.GetString(constant.KBToolsImage),
		ImagePullPolicy: corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy)),
		Command:         []string{"sh", "-c"},
		Args:            []string{createBackupCmd},
	}
	intctrlutil.InjectZeroResourcesLimitsIfEmpty(&container)

	podSpec := &corev1.PodSpec{
		ServiceAccountName: s.BackupPolicy.Spec.Target.ServiceAccountName,
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
		cronJob = &cronJobList.Items[0]
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
		// if no cron expression, return
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

	// sync the cronjob with the current backup policy configuration.
	patch := client.MergeFrom(cronJob.DeepCopy())
	cronJob.Spec.StartingDeadlineSeconds = cronjobProto.Spec.StartingDeadlineSeconds
	cronJob.Spec.JobTemplate.Spec.BackoffLimit = s.BackupPolicy.Spec.BackoffLimit
	cronJob.Spec.JobTemplate.Spec.Template = cronjobProto.Spec.JobTemplate.Spec.Template
	cronJob.Spec.Schedule = cronjobProto.Spec.Schedule
	return s.Client.Patch(s.Ctx, cronJob, patch)
}

func (s *Scheduler) generateBackupName() string {
	target := s.BackupPolicy.Spec.Target

	// if cluster name can be found in target labels, use it as backup name prefix
	backupNamePrefix := target.PodSelector.MatchLabels[constant.AppInstanceLabelKey]

	// if cluster name can not be found, use backup schedule name as backup name prefix
	if backupNamePrefix == "" {
		backupNamePrefix = s.BackupSchedule.Name
	}
	return backupNamePrefix + "-$(date -u +'%Y%m%d%H%M%S')"
}
