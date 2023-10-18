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
	"k8s.io/apimachinery/pkg/api/resource"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

type MockBackupRepoFactory struct {
	testapps.BaseFactory[dpv1alpha1.BackupRepo, *dpv1alpha1.BackupRepo, MockBackupRepoFactory]
}

func NewBackupRepoFactory(namespace, name string) *MockBackupRepoFactory {
	f := &MockBackupRepoFactory{}
	f.Init(namespace, name,
		&dpv1alpha1.BackupRepo{
			Spec: dpv1alpha1.BackupRepoSpec{
				VolumeCapacity:  resource.MustParse("100Gi"),
				PVReclaimPolicy: "Retain",
			},
		}, f)
	return f
}

func (factory *MockBackupRepoFactory) SetStorageProviderRef(providerName string) *MockBackupRepoFactory {
	factory.Get().Spec.StorageProviderRef = providerName
	return factory
}

func (factory *MockBackupRepoFactory) SetVolumeCapacity(amount string) *MockBackupRepoFactory {
	factory.Get().Spec.VolumeCapacity = resource.MustParse(amount)
	return factory
}

func (factory *MockBackupRepoFactory) SetPVReclaimPolicy(policy string) *MockBackupRepoFactory {
	factory.Get().Spec.PVReclaimPolicy = corev1.PersistentVolumeReclaimPolicy(policy)
	return factory
}

func (factory *MockBackupRepoFactory) SetConfig(config map[string]string) *MockBackupRepoFactory {
	factory.Get().Spec.Config = config
	return factory
}

func (factory *MockBackupRepoFactory) SetCredential(ref *corev1.SecretReference) *MockBackupRepoFactory {
	factory.Get().Spec.Credential = ref
	return factory
}

func (factory *MockBackupRepoFactory) SetAsDefaultRepo(v bool) *MockBackupRepoFactory {
	if v {
		obj := factory.Get()
		if obj.Annotations == nil {
			obj.Annotations = map[string]string{}
		}
		obj.Annotations[dptypes.DefaultBackupRepoAnnotationKey] = "true"
	} else {
		delete(factory.Get().Annotations, dptypes.DefaultBackupRepoAnnotationKey)
	}
	return factory
}
