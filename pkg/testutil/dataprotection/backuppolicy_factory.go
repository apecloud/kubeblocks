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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

type MockBackupPolicyFactory struct {
	testapps.BaseFactory[dpv1alpha1.BackupPolicy, *dpv1alpha1.BackupPolicy, MockBackupPolicyFactory]
}

func NewBackupPolicyFactory(namespace, name string) *MockBackupPolicyFactory {
	f := &MockBackupPolicyFactory{}
	f.Init(namespace, name, &dpv1alpha1.BackupPolicy{}, f)
	return f
}

func (f *MockBackupPolicyFactory) SetBackupRepoName(backupRepoName string) *MockBackupPolicyFactory {
	if backupRepoName == "" {
		f.Get().Spec.BackupRepoName = nil
	} else {
		f.Get().Spec.BackupRepoName = &backupRepoName
	}
	return f
}

func (f *MockBackupPolicyFactory) SetPathPrefix(pathPrefix string) *MockBackupPolicyFactory {
	f.Get().Spec.PathPrefix = pathPrefix
	return f
}

func (f *MockBackupPolicyFactory) SetBackoffLimit(backoffLimit int32) *MockBackupPolicyFactory {
	f.Get().Spec.BackoffLimit = &backoffLimit
	return f
}

func (f *MockBackupPolicyFactory) AddBackupMethod(name string,
	snapshotVolumes bool, actionSetName string) *MockBackupPolicyFactory {
	f.Get().Spec.BackupMethods = append(f.Get().Spec.BackupMethods,
		dpv1alpha1.BackupMethod{
			Name:            name,
			SnapshotVolumes: &snapshotVolumes,
			ActionSetName:   actionSetName,
			TargetVolumes:   &dpv1alpha1.TargetVolumeInfo{},
		})
	return f
}

func (f *MockBackupPolicyFactory) SetBackupMethodVolumes(names []string) *MockBackupPolicyFactory {
	f.Get().Spec.BackupMethods[len(f.Get().Spec.BackupMethods)-1].TargetVolumes.Volumes = names
	return f
}

func (f *MockBackupPolicyFactory) SetBackupMethodVolumeMounts(keyAndValues ...string) *MockBackupPolicyFactory {
	var volumeMounts []corev1.VolumeMount
	for k, v := range testapps.WithMap(keyAndValues...) {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      k,
			MountPath: v,
		})
	}
	f.Get().Spec.BackupMethods[len(f.Get().Spec.BackupMethods)-1].TargetVolumes.VolumeMounts = volumeMounts
	return f
}

func (f *MockBackupPolicyFactory) SetTarget(keyAndValues ...string) *MockBackupPolicyFactory {
	f.Get().Spec.Target = &dpv1alpha1.BackupTarget{
		PodSelector: &dpv1alpha1.PodSelector{
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: testapps.WithMap(keyAndValues...),
			},
		},
	}
	return f
}

func (f *MockBackupPolicyFactory) SetTargetConnectionCredential(secretName string) *MockBackupPolicyFactory {
	f.Get().Spec.Target.ConnectionCredential = &dpv1alpha1.ConnectionCredential{
		SecretName:  secretName,
		UsernameKey: "username",
		PasswordKey: "password",
	}
	return f
}
