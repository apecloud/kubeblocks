/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package v1alpha1

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidate(t *testing.T) {
	g := NewGomegaWithT(t)

	backupPolicy := &BackupPolicy{Spec: BackupPolicySpec{Snapshot: &SnapshotPolicy{}}}
	backupSpec := &BackupSpec{BackupType: BackupTypeSnapshot}
	g.Expect(backupSpec.Validate(backupPolicy)).Should(Succeed())

	backupPolicy = &BackupPolicy{}
	backupSpec = &BackupSpec{BackupType: BackupTypeSnapshot}
	g.Expect(backupSpec.Validate(backupPolicy)).Should(HaveOccurred())

	backupSpec = &BackupSpec{BackupType: BackupTypeDataFile}
	g.Expect(backupSpec.Validate(backupPolicy)).Should(HaveOccurred())

	backupSpec = &BackupSpec{BackupType: BackupTypeLogFile}
	g.Expect(backupSpec.Validate(backupPolicy)).Should(HaveOccurred())
}

func TestGetRecoverableTimeRange(t *testing.T) {
	g := NewGomegaWithT(t)

	// test empty backups
	emptyBackups := []Backup{}
	g.Expect(GetRecoverableTimeRange(emptyBackups)).Should(BeEmpty())

	now := metav1.Now()
	backupSnapshot := Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-snapshot"},
		Spec:       BackupSpec{BackupType: BackupTypeSnapshot},
		Status: BackupStatus{
			Phase: BackupPhaseCompleted,
			Manifests: &ManifestsStatus{
				BackupLog: &BackupLogStatus{
					StartTime: &now,
					StopTime:  &now,
				},
			},
		},
	}
	noStartTimeSnapshotBackup := Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-snapshot1"},
		Spec:       BackupSpec{BackupType: BackupTypeSnapshot},
		Status: BackupStatus{
			Phase: BackupPhaseCompleted,
			Manifests: &ManifestsStatus{
				BackupLog: &BackupLogStatus{
					StopTime: &now,
				},
			},
		},
	}
	noTimeSnapshotBackup := Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-snapshot2"},
		Spec:       BackupSpec{BackupType: BackupTypeSnapshot},
		Status: BackupStatus{
			Phase: BackupPhaseCompleted,
		},
	}
	stopTimeGTLogFileStopTimeBaseBackup := Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-snapshot2"},
		Spec:       BackupSpec{BackupType: BackupTypeSnapshot},
		Status: BackupStatus{
			Phase: BackupPhaseCompleted,
			Manifests: &ManifestsStatus{
				BackupLog: &BackupLogStatus{
					StartTime: &now,
					StopTime:  &metav1.Time{Time: now.Add(time.Minute * 61)},
				},
			},
		},
	}
	backupLogfile := Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-logfile"},
		Spec:       BackupSpec{BackupType: BackupTypeLogFile},
		Status: BackupStatus{
			Phase: BackupPhaseCompleted,
			Manifests: &ManifestsStatus{
				BackupLog: &BackupLogStatus{
					StartTime: &now,
					StopTime:  &metav1.Time{Time: now.Add(time.Hour)},
				},
			},
		},
	}

	backups := []Backup{backupSnapshot, backupLogfile, {}}
	g.Expect(GetRecoverableTimeRange(backups)).ShouldNot(BeEmpty())

	backups = []Backup{noStartTimeSnapshotBackup, backupLogfile, {}}
	g.Expect(GetRecoverableTimeRange(backups)).ShouldNot(BeEmpty())

	backups = []Backup{backupLogfile, noTimeSnapshotBackup}
	g.Expect(GetRecoverableTimeRange(backups)).Should(BeEmpty())

	backups = []Backup{backupLogfile, stopTimeGTLogFileStopTimeBaseBackup}
	g.Expect(GetRecoverableTimeRange(backups)).Should(BeEmpty())
}

func TestGetStartTime(t *testing.T) {
	startTimestamp := metav1.Now()
	backTimeRangeStartTime := metav1.Time{Time: time.Now().Add(10 * time.Second)}
	backup := Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "backup1"},
		Spec:       BackupSpec{BackupType: BackupTypeSnapshot},
		Status: BackupStatus{
			Phase:          BackupPhaseCompleted,
			StartTimestamp: &startTimestamp,
			Manifests: &ManifestsStatus{
				BackupLog: &BackupLogStatus{
					StartTime: &backTimeRangeStartTime,
				},
			},
		},
	}
	g := NewGomegaWithT(t)
	g.Expect(backup.Status.GetStartTime().Second()).Should(Equal(backTimeRangeStartTime.Second()))

	backup.Status.Manifests.BackupLog.StartTime = nil
	g.Expect(backup.Status.GetStartTime().Second()).Should(Equal(startTimestamp.Second()))
}

func TestGetStopTime(t *testing.T) {
	stopTimestamp := metav1.Now()
	backTimeRangeStopTime := metav1.Time{Time: time.Now().Add(10 * time.Second)}
	backup := Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "backup1"},
		Spec:       BackupSpec{BackupType: BackupTypeSnapshot},
		Status: BackupStatus{
			Phase:               BackupPhaseCompleted,
			CompletionTimestamp: &stopTimestamp,
			Manifests: &ManifestsStatus{
				BackupLog: &BackupLogStatus{
					StopTime: &backTimeRangeStopTime,
				},
			},
		},
	}
	g := NewGomegaWithT(t)
	g.Expect(backup.Status.GetStopTime().Second()).Should(Equal(backTimeRangeStopTime.Second()))

	backup.Status.Manifests.BackupLog.StopTime = nil
	g.Expect(backup.Status.GetStopTime().Second()).Should(Equal(stopTimestamp.Second()))
}
