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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

type MockBackupFactory struct {
	testapps.BaseFactory[dpv1alpha1.Backup, *dpv1alpha1.Backup, MockBackupFactory]
}

func NewBackupFactory(namespace, name string) *MockBackupFactory {
	f := &MockBackupFactory{}
	f.Init(namespace, name,
		&dpv1alpha1.Backup{
			Spec: dpv1alpha1.BackupSpec{},
		}, f)
	return f
}

func (f *MockBackupFactory) SetBackupPolicyName(backupPolicyName string) *MockBackupFactory {
	f.Get().Spec.BackupPolicyName = backupPolicyName
	return f
}

func (f *MockBackupFactory) SetBackupMethod(backupMethod string) *MockBackupFactory {
	f.Get().Spec.BackupMethod = backupMethod
	return f
}

func (f *MockBackupFactory) SetLabels(labels map[string]string) *MockBackupFactory {
	f.Get().SetLabels(labels)
	return f
}

func (f *MockBackupFactory) SetBackupTimeRange(startTime, stopTime time.Time) *MockBackupFactory {
	tr := f.Get().Status.TimeRange
	if tr == nil {
		tr = &dpv1alpha1.BackupTimeRange{}
	}
	tr.Start = &metav1.Time{Time: startTime}
	tr.End = &metav1.Time{Time: stopTime}
	f.Get().Status.TimeRange = tr
	return f
}
