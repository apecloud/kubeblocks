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

package action

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ref "k8s.io/client-go/tools/reference"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

// StatefulSetAction is an action that creates or updates the StatefulSet of Continuous backup.
type StatefulSetAction struct {
	// Name is the Name of the action.
	Name string

	Backup *dpv1alpha1.Backup

	// ObjectMeta is the metadata of the volume snapshot.
	ObjectMeta metav1.ObjectMeta
	Replicas   *int32

	PodSpec *corev1.PodSpec

	ActionSet *dpv1alpha1.ActionSet
}

func (s *StatefulSetAction) GetName() string {
	return s.Name
}

func (s *StatefulSetAction) Type() dpv1alpha1.ActionType {
	return dpv1alpha1.ActionTypeStatefulSet
}

func (s *StatefulSetAction) Execute(ctx ActionContext) (actionStatus *dpv1alpha1.ActionStatus, err error) {
	defer func() {
		if err != nil {
			err = intctrlutil.NewErrorf(intctrlutil.ErrorTypeRequeue, err.Error())
		}
	}()
	sts := &appsv1.StatefulSet{}
	exists, err := intctrlutil.CheckResourceExists(ctx.Ctx, ctx.Client, client.ObjectKey{
		Namespace: s.ObjectMeta.Namespace,
		Name:      s.ObjectMeta.Name,
	}, sts)
	if err != nil {
		return nil, err
	}
	// inject continuous env
	_ = s.injectContinuousEnvForPodSpec(ctx, s.PodSpec)
	s.PodSpec.RestartPolicy = corev1.RestartPolicyAlways
	// if not exists, create the statefulSet
	if !exists {
		if err = s.createStatefulSet(ctx, s.PodSpec); err != nil {
			return nil, err
		}
		return &dpv1alpha1.ActionStatus{
			Name:           s.Name,
			Phase:          dpv1alpha1.ActionPhaseRunning,
			ActionType:     s.Type(),
			StartTimestamp: &metav1.Time{Time: time.Now()},
		}, nil
	}
	sts.Spec.Replicas = s.Replicas
	sts.Spec.Template.Spec = *s.PodSpec

	// update the statefulSet
	if err = ctx.Client.Update(ctx.Ctx, sts); err != nil {
		return nil, err
	}
	actionStatus = &dpv1alpha1.ActionStatus{
		Name:              s.Name,
		Phase:             dpv1alpha1.ActionPhaseRunning,
		AvailableReplicas: &sts.Status.AvailableReplicas,
		ActionType:        s.Type(),
	}
	s.Backup.Status.Phase = dpv1alpha1.BackupPhaseRunning
	actionStatus.ObjectRef, _ = ref.GetReference(ctx.Scheme, sts)
	if s.stsIsFailed(ctx) {
		actionStatus.Phase = dpv1alpha1.ActionPhaseFailed
		actionStatus.FailureReason = fmt.Sprintf("pod %s-0 is not running", sts.Name)
	}
	return actionStatus, nil
}

func (s *StatefulSetAction) createStatefulSet(ctx ActionContext, podSpec *corev1.PodSpec) error {
	sts := &appsv1.StatefulSet{
		ObjectMeta: s.ObjectMeta,
		Spec: appsv1.StatefulSetSpec{
			Replicas: s.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: s.ObjectMeta.Labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: s.ObjectMeta.Labels,
				},
				Spec: *podSpec,
			},
		},
	}
	controllerutil.AddFinalizer(sts, dptypes.DataProtectionFinalizerName)
	if err := controllerutil.SetControllerReference(s.Backup, sts, ctx.Scheme); err != nil {
		return err
	}
	return ctx.Client.Create(ctx.Ctx, sts)
}

func (s *StatefulSetAction) injectContinuousEnvForPodSpec(ctx ActionContext, podSpec *corev1.PodSpec) error {
	backupSchedule := &dpv1alpha1.BackupSchedule{}
	if err := ctx.Client.Get(ctx.Ctx, client.ObjectKey{Name: s.Backup.Labels[dptypes.BackupScheduleLabelKey],
		Namespace: s.Backup.Namespace}, backupSchedule); err != nil {
		return client.IgnoreNotFound(err)
	}
	var schedulePolicy *dpv1alpha1.SchedulePolicy
	for _, v := range backupSchedule.Spec.Schedules {
		if v.BackupMethod == s.Backup.Spec.BackupMethod {
			schedulePolicy = &v
			break
		}
	}
	if schedulePolicy == nil {
		return nil
	}
	duration, err := s.Backup.Spec.RetentionPeriod.ToDuration()
	if err != nil {
		return err
	}
	envs := []corev1.EnvVar{
		{
			Name:  dptypes.DPArchiveInterval,
			Value: s.getIntervalSeconds(schedulePolicy.CronExpression),
		},
	}
	if duration.Seconds() != float64(0) {
		envs = append(envs, corev1.EnvVar{
			Name:  dptypes.DPContinuousTTLSeconds,
			Value: strconv.FormatInt(int64(math.Floor(duration.Seconds())), 10),
		})
	}
	podSpec.Containers[0].Env = append(podSpec.Containers[0].Env, envs...)
	return nil
}

func (s *StatefulSetAction) getIntervalSeconds(cronExpression string) string {
	// move time zone field
	if strings.HasPrefix(cronExpression, "TZ=") || strings.HasPrefix(cronExpression, "CRON_TZ=") {
		i := strings.Index(cronExpression, " ")
		cronExpression = strings.TrimSpace(cronExpression[i:])
	}
	var interval = "60"
	// skip the macro syntax
	if strings.HasPrefix(cronExpression, "@") {
		return interval + "s"
	}
	fields := strings.Fields(cronExpression)
loop:
	for i, v := range fields {
		switch i {
		case 0:
			if strings.HasPrefix(v, "*/") {
				m, _ := strconv.Atoi(strings.ReplaceAll(v, "*/", ""))
				interval = strconv.Itoa(m * 60)
				break loop
			}
		case 1:
			if strings.HasPrefix(v, "*/") {
				m, _ := strconv.Atoi(strings.ReplaceAll(v, "*/", ""))
				interval = strconv.Itoa(m * 60 * 60)
				break loop
			}
		default:
			break loop
		}
	}
	return interval + "s"
}

func (s *StatefulSetAction) stsIsFailed(ctx ActionContext) bool {
	pod := &corev1.Pod{}
	if err := ctx.Client.Get(ctx.Ctx, client.ObjectKey{Name: s.ObjectMeta.Name + "-0",
		Namespace: s.ObjectMeta.Namespace}, pod); err != nil {
		return false
	}
	isFailed, isTimeout, _ := intctrlutil.IsPodFailedAndTimedOut(pod)
	return isFailed && isTimeout
}
