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
			Phase: BackupCompleted,
			Manifests: &ManifestsStatus{
				BackupLog: &BackupLogStatus{
					StartTime: &now,
					StopTime:  &now,
				},
			},
		},
	}
	backupLogfile := Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-logfile"},
		Spec:       BackupSpec{BackupType: BackupTypeLogFile},
		Status: BackupStatus{
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
}
