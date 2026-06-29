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

package operations

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlcomp "github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestHorizontalScalingCreateRestoreReturnsFatalWhenNoRestoreBuilt(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	for _, addToScheme := range []func(*runtime.Scheme) error{
		corev1.AddToScheme,
		appsv1.AddToScheme,
		dpv1alpha1.AddToScheme,
		opsv1alpha1.AddToScheme,
	} {
		if err := addToScheme(scheme); err != nil {
			t.Fatalf("add scheme: %v", err)
		}
	}

	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tidb",
			Namespace: "default",
			UID:       types.UID("cluster1"),
		},
	}
	opsRequest := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "scale-out-from-backup",
			Namespace: "default",
			UID:       types.UID("opsreq1"),
		},
	}
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "br-full",
			Namespace: "default",
		},
		Status: dpv1alpha1.BackupStatus{
			Phase: dpv1alpha1.BackupPhaseCompleted,
			BackupMethod: &dpv1alpha1.BackupMethod{
				Name: "br",
			},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, opsRequest, backup).Build()
	opsRes := &OpsResource{
		Cluster:    cluster,
		OpsRequest: opsRequest,
	}
	synthesizedComponent := &intctrlcomp.SynthesizedComponent{
		Name: "tikv",
		VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{{
			ObjectMeta: metav1.ObjectMeta{Name: "data"},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			},
		}},
	}
	restoreMGR := plan.NewRestoreManager(ctx, cli, cluster, scheme, map[string]string{
		constant.OpsRequestNameLabelKey: opsRequest.Name,
	}, 1, 3)

	err := horizontalScalingOpsHandler{}.createRestore(intctrlutil.RequestCtx{Ctx: ctx}, cli, opsRes,
		synthesizedComponent, restoreMGR, &appsv1.ClusterComponentSpec{Name: "tikv"}, backup, "")
	if err == nil {
		t.Fatal("expected fatal error when backup method cannot build prepareData restore")
	}
	if !intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
		t.Fatalf("expected fatal error, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "has no target volumes matching component") {
		t.Fatalf("unexpected error: %v", err)
	}

	restoreList := &dpv1alpha1.RestoreList{}
	if err := cli.List(ctx, restoreList, client.InNamespace("default")); err != nil {
		t.Fatalf("list restores: %v", err)
	}
	if len(restoreList.Items) != 0 {
		t.Fatalf("expected no restore to be created, got %d", len(restoreList.Items))
	}
}
