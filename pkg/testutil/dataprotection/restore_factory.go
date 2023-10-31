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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

type MockRestoreFactory struct {
	testapps.BaseFactory[dpv1alpha1.Restore, *dpv1alpha1.Restore, MockRestoreFactory]
}

func NewRestoreactory(namespace, name string) *MockRestoreFactory {
	f := &MockRestoreFactory{}
	f.Init(namespace, name,
		&dpv1alpha1.Restore{
			Spec: dpv1alpha1.RestoreSpec{},
		}, f)
	return f
}

func (f *MockRestoreFactory) SetBackup(name, namespace string) *MockRestoreFactory {
	f.Get().Spec.Backup = dpv1alpha1.BackupRef{
		Name:      name,
		Namespace: namespace,
	}
	return f
}

func (f *MockRestoreFactory) SetRestoreTime(restoreTime string) *MockRestoreFactory {
	f.Get().Spec.RestoreTime = restoreTime
	return f
}

func (f *MockRestoreFactory) SetLabels(labels map[string]string) *MockRestoreFactory {
	f.Get().SetLabels(labels)
	return f
}

func (f *MockRestoreFactory) AddEnv(env corev1.EnvVar) *MockRestoreFactory {
	f.Get().Spec.Env = append(f.Get().Spec.Env, env)
	return f
}

func (f *MockRestoreFactory) initPrepareDataConfig() {
	prepareDataConfig := f.Get().Spec.PrepareDataConfig
	if prepareDataConfig == nil {
		f.Get().Spec.PrepareDataConfig = &dpv1alpha1.PrepareDataConfig{
			VolumeClaimRestorePolicy: dpv1alpha1.VolumeClaimRestorePolicyParallel,
		}
	}
}

func (f *MockRestoreFactory) initReadyConfig() {
	ReadyConfig := f.Get().Spec.ReadyConfig
	if ReadyConfig == nil {
		f.Get().Spec.ReadyConfig = &dpv1alpha1.ReadyConfig{}
	}
}

func (f *MockRestoreFactory) buildRestoreVolumeClaim(name, volumeSource, mountPath, storageClass string) dpv1alpha1.RestoreVolumeClaim {
	return dpv1alpha1.RestoreVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				constant.AppManagedByLabelKey: "restore",
			},
		},
		VolumeConfig: dpv1alpha1.VolumeConfig{
			VolumeSource: volumeSource,
			MountPath:    mountPath,
		},
		VolumeClaimSpec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &storageClass,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("20Gi"),
				},
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		},
	}
}

func (f *MockRestoreFactory) SetVolumeClaimRestorePolicy(policy dpv1alpha1.VolumeClaimRestorePolicy) *MockRestoreFactory {
	f.initPrepareDataConfig()
	f.Get().Spec.PrepareDataConfig.VolumeClaimRestorePolicy = policy
	return f
}

func (f *MockRestoreFactory) SetSchedulingSpec(schedulingSpec dpv1alpha1.SchedulingSpec) *MockRestoreFactory {
	f.initPrepareDataConfig()
	f.Get().Spec.PrepareDataConfig.SchedulingSpec = schedulingSpec
	return f
}

func (f *MockRestoreFactory) SetDataSourceRef(volumeSource, mountPath string) *MockRestoreFactory {
	f.initPrepareDataConfig()
	f.Get().Spec.PrepareDataConfig.DataSourceRef = &dpv1alpha1.VolumeConfig{
		VolumeSource: volumeSource,
		MountPath:    mountPath,
	}
	return f
}

func (f *MockRestoreFactory) SetVolumeClaimsTemplate(templateName, volumeSource, mountPath, storageClass string, replicas, startingIndex int32) *MockRestoreFactory {
	f.initPrepareDataConfig()
	f.Get().Spec.PrepareDataConfig.RestoreVolumeClaimsTemplate = &dpv1alpha1.RestoreVolumeClaimsTemplate{
		Replicas:      replicas,
		StartingIndex: startingIndex,
		Templates: []dpv1alpha1.RestoreVolumeClaim{
			f.buildRestoreVolumeClaim(templateName, volumeSource, mountPath, storageClass),
		},
	}
	return f
}

func (f *MockRestoreFactory) AddVolumeClaim(claimName, volumeSource, mountPath, storageClass string) *MockRestoreFactory {
	f.initPrepareDataConfig()
	f.Get().Spec.PrepareDataConfig.RestoreVolumeClaims = append(f.Get().Spec.PrepareDataConfig.RestoreVolumeClaims,
		f.buildRestoreVolumeClaim(claimName, volumeSource, mountPath, storageClass))
	return f
}

func (f *MockRestoreFactory) SetConnectCredential(secretName string) *MockRestoreFactory {
	f.initReadyConfig()
	f.Get().Spec.ReadyConfig.ConnectionCredential = &dpv1alpha1.ConnectionCredential{
		SecretName:  secretName,
		UsernameKey: "username",
		PasswordKey: "password",
	}
	return f
}

func (f *MockRestoreFactory) SetJobActionConfig(matchLabels map[string]string) *MockRestoreFactory {
	f.initReadyConfig()
	f.Get().Spec.ReadyConfig.JobAction = &dpv1alpha1.JobAction{
		Target: dpv1alpha1.JobActionTarget{
			PodSelector: metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      DataVolumeName,
					MountPath: DataVolumeMountPath,
				},
			},
		},
	}
	return f
}

func (f *MockRestoreFactory) SetExecActionConfig(matchLabels map[string]string) *MockRestoreFactory {
	f.initReadyConfig()
	f.Get().Spec.ReadyConfig.ExecAction = &dpv1alpha1.ExecAction{
		Target: dpv1alpha1.ExecActionTarget{
			PodSelector: metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
		},
	}
	return f
}
