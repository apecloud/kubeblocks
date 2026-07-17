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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestMountPreCheckRetriesBoundRepoPVLookup(t *testing.T) {
	oldAffinity := viper.GetString(constant.CfgKeyCtrlrMgrAffinity)
	oldNodeSelector := viper.GetString(constant.CfgKeyCtrlrMgrNodeSelector)
	defer func() {
		viper.Set(constant.CfgKeyCtrlrMgrAffinity, oldAffinity)
		viper.Set(constant.CfgKeyCtrlrMgrNodeSelector, oldNodeSelector)
	}()
	viper.Set(constant.CfgKeyCtrlrMgrAffinity, "")
	viper.Set(constant.CfgKeyCtrlrMgrNodeSelector, `{"kubernetes.io/hostname":"node6"}`)

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))

	repo := &dpv1alpha1.BackupRepo{ObjectMeta: metav1.ObjectMeta{
		Name: "repo", UID: types.UID("repo-uid-12345678"),
	}}
	reconCtx := &reconcileContext{
		RequestCtx: intctrlutil.RequestCtx{Ctx: context.Background()},
		repo:       repo,
		digest:     "placement-digest",
	}
	pvcKey := client.ObjectKey{Name: reconCtx.preCheckResourceName(), Namespace: "kb-system"}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: pvcKey.Name, Namespace: pvcKey.Namespace,
			Annotations: map[string]string{dataProtectionBackupRepoDigestAnnotationKey: reconCtx.getDigest()},
		},
		Spec:   corev1.PersistentVolumeClaimSpec{VolumeName: "repo-pv"},
		Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pvc).Build()
	reconciler := &BackupRepoReconciler{Client: cli, Scheme: scheme}
	originalStatus := repo.Status.DeepCopy()

	job, _, err := reconciler.runPreCheckJobForMounting(reconCtx, pvcKey.Namespace, "worker")
	require.Error(t, err)
	assert.True(t, intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeRequeue))
	assert.Nil(t, job)
	assert.Equal(t, *originalStatus, repo.Status)
	assert.Error(t, cli.Get(context.Background(), pvcKey, &batchv1.Job{}))

	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: pvc.Spec.VolumeName},
		Spec: corev1.PersistentVolumeSpec{NodeAffinity: &corev1.VolumeNodeAffinity{Required: &corev1.NodeSelector{
			NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{{
				Key: corev1.LabelHostname, Operator: corev1.NodeSelectorOpIn, Values: []string{"node3"},
			}}}},
		}}},
	}
	require.NoError(t, cli.Create(context.Background(), pv))
	job, _, err = reconciler.runPreCheckJobForMounting(reconCtx, pvcKey.Namespace, "worker")
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Nil(t, job.Spec.Template.Spec.NodeSelector)
	assert.NoError(t, cli.Get(context.Background(), pvcKey, &batchv1.Job{}))
}
