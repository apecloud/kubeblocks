/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"context"
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/stretchr/testify/require"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func TestResolveSourceTargetPodNameRequiresExplicitMappingForInstanceTemplate(t *testing.T) {
	target := &dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			PodSelector: &dpv1alpha1.PodSelector{
				LabelSelector: &metav1.LabelSelector{},
				Strategy:      dpv1alpha1.PodSelectionStrategyAll,
			},
		},
		SelectedTargetPods: []string{"source-mysql-tpl-a-1", "source-mysql-tpl-b-1"},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "data-target-mysql-tpl-b-1",
			Labels: map[string]string{
				constant.KBAppPodNameLabelKey:          "target-mysql-tpl-b-1",
				constant.KBAppInstanceTemplateLabelKey: "tpl-b",
			},
			Annotations: map[string]string{},
		},
	}

	sourcePodName, err := resolveSourceTargetPodName(target, pvc)
	require.Error(t, err)
	require.Empty(t, sourcePodName)

	parameters, err := json.Marshal(map[string]string{
		dptypes.SourceTargetPodNameAnnotationKey: "source-mysql-tpl-b-1",
	})
	require.NoError(t, err)
	pvc.Annotations[constant.RestoreParametersAnnotationKey] = string(parameters)
	sourcePodName, err = resolveSourceTargetPodName(target, pvc)
	require.NoError(t, err)
	require.Equal(t, "source-mysql-tpl-b-1", sourcePodName)
}

func TestRebindPVCAndPVWaitsUntilPopulatePVCIsBound(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "default",
			Name:            "data-target-0",
			UID:             "target-pvc-uid",
			ResourceVersion: "1",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			DataSourceRef: &corev1.TypedObjectReference{Name: "backup"},
		},
	}
	populatePVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "populate",
		},
	}
	reconciler := &VolumePopulatorReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).Build()}
	reqCtx := intctrlutil.RequestCtx{Ctx: context.Background()}

	rebound, err := reconciler.rebindPVCAndPV(reqCtx, populatePVC, pvc)
	require.NoError(t, err)
	require.False(t, rebound)

	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "pv"}}
	reconciler.Client = fake.NewClientBuilder().WithScheme(scheme).WithObjects(pv).Build()
	populatePVC.Spec.VolumeName = pv.Name

	rebound, err = reconciler.rebindPVCAndPV(reqCtx, populatePVC, pvc)
	require.NoError(t, err)
	require.True(t, rebound)

	patchedPV := &corev1.PersistentVolume{}
	require.NoError(t, reconciler.Client.Get(reqCtx.Ctx, client.ObjectKey{Name: pv.Name}, patchedPV))
	require.NotNil(t, patchedPV.Spec.ClaimRef)
	require.Equal(t, pvc.Name, patchedPV.Spec.ClaimRef.Name)
	require.Equal(t, pvc.UID, patchedPV.Spec.ClaimRef.UID)
}

func TestRestoreSystemAccountSecretsUsesShardingSecretName(t *testing.T) {
	require.Equal(t, "cluster-shard-admin", systemAccountSecretName(systemAccountSecretScopeSharding, "cluster", "shard", "admin"))
	require.Equal(t, constant.GenerateAccountSecretName("cluster", "mysql", "admin"),
		systemAccountSecretName(systemAccountSecretScopeComponent, "cluster", "mysql", "admin"))
}

func TestRestoreSystemAccountSecretsRestoresComponentAndShardingSecrets(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))

	encryptor := intctrlutil.NewEncryptor("")
	componentPassword, err := encryptor.Encrypt([]byte("component-password"))
	require.NoError(t, err)
	shardingPassword, err := encryptor.Encrypt([]byte("sharding-password"))
	require.NoError(t, err)
	accounts, err := json.Marshal(map[string]map[string]string{
		"mysql": {
			"admin": componentPassword,
		},
		"shard": {
			"root": shardingPassword,
		},
	})
	require.NoError(t, err)
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "backup",
			Annotations: map[string]string{
				constant.EncryptedSystemAccountsAnnotationKey: string(accounts),
			},
		},
	}
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "cluster",
			UID:       types.UID("cluster-uid"),
		},
	}
	component := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      constant.GenerateClusterComponentName("cluster", "mysql"),
			UID:       types.UID("component-uid"),
		},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "data-target-0",
			Labels: map[string]string{
				constant.AppInstanceLabelKey:       "cluster",
				constant.KBAppComponentLabelKey:    "mysql",
				constant.KBAppShardingNameLabelKey: "shard",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			DataSourceRef: &corev1.TypedObjectReference{Name: backup.Name},
		},
	}
	reconciler := &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(backup, cluster, component).Build(),
		Scheme: scheme,
	}

	err = reconciler.restoreSystemAccountSecrets(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup.Namespace)
	require.NoError(t, err)

	componentSecret := &corev1.Secret{}
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKey{
		Namespace: "default",
		Name:      constant.GenerateAccountSecretName("cluster", "mysql", "admin"),
	}, componentSecret))
	require.Equal(t, []byte("component-password"), componentSecret.Data[constant.AccountPasswdForSecret])
	require.Len(t, componentSecret.OwnerReferences, 1)
	require.Equal(t, "Component", componentSecret.OwnerReferences[0].Kind)
	require.Equal(t, component.Name, componentSecret.OwnerReferences[0].Name)

	shardingSecret := &corev1.Secret{}
	require.NoError(t, reconciler.Client.Get(context.Background(), client.ObjectKey{
		Namespace: "default",
		Name:      "cluster-shard-root",
	}, shardingSecret))
	require.Equal(t, []byte("sharding-password"), shardingSecret.Data[constant.AccountPasswdForSecret])
	require.Len(t, shardingSecret.OwnerReferences, 1)
	require.Equal(t, "Cluster", shardingSecret.OwnerReferences[0].Kind)
	require.Equal(t, cluster.Name, shardingSecret.OwnerReferences[0].Name)
}

func TestRestoreSystemAccountSecretsReturnsFatalForInvalidAccountsPayload(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "backup",
			Annotations: map[string]string{
				constant.EncryptedSystemAccountsAnnotationKey: "{",
			},
		},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "data-target-0",
			Labels: map[string]string{
				constant.AppInstanceLabelKey:    "cluster",
				constant.KBAppComponentLabelKey: "mysql",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			DataSourceRef: &corev1.TypedObjectReference{Name: backup.Name},
		},
	}
	reconciler := &VolumePopulatorReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(backup).Build()}

	err := reconciler.restoreSystemAccountSecrets(intctrlutil.RequestCtx{Ctx: context.Background()}, pvc, backup.Namespace)
	require.Error(t, err)
	require.True(t, intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal))
}
