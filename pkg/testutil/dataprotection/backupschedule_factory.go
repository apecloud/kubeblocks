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

package dataprotection

import (
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

type BackupScheduleFactory struct {
	testapps.BaseFactory[dpv1alpha1.BackupSchedule, *dpv1alpha1.BackupSchedule, BackupScheduleFactory]
}

func NewBackupScheduleFactory(namespace, name string) *BackupScheduleFactory {
	f := &BackupScheduleFactory{}
	f.Init(namespace, name, &dpv1alpha1.BackupSchedule{}, f)
	f.Get().Spec.Schedules = []dpv1alpha1.SchedulePolicy{}
	return f
}

func (f *BackupScheduleFactory) SetBackupPolicyName(backupPolicyName string) *BackupScheduleFactory {
	f.Get().Spec.BackupPolicyName = backupPolicyName
	return f
}

func (f *BackupScheduleFactory) SetStartingDeadlineMinutes(minutes int64) *BackupScheduleFactory {
	f.Get().Spec.StartingDeadlineMinutes = &minutes
	return f
}

func (f *BackupScheduleFactory) AddSchedulePolicy(schedulePolicy dpv1alpha1.SchedulePolicy) *BackupScheduleFactory {
	f.Get().Spec.Schedules = append(f.Get().Spec.Schedules, schedulePolicy)
	return f
}

func (f *BackupScheduleFactory) SetSchedules(schedules []dpv1alpha1.SchedulePolicy) *BackupScheduleFactory {
	f.Get().Spec.Schedules = schedules
	return f
}
