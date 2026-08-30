/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
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
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestHorizontalScalingCreateRestoreFailsFastWithoutBackupMethod(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	for _, addToScheme := range []func(*runtime.Scheme) error{
		corev1.AddToScheme, appsv1.AddToScheme, dpv1alpha1.AddToScheme, opsv1alpha1.AddToScheme,
	} {
		if err := addToScheme(scheme); err != nil {
			t.Fatalf("add scheme: %v", err)
		}
	}

	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{
		Name: "cluster", Namespace: "default", UID: types.UID("cluster1"),
	}}
	opsRequest := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{
		Name: "scale-out-from-backup", Namespace: "default", UID: types.UID("opsreq1"),
	}}
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "default"},
		Status:     dpv1alpha1.BackupStatus{Phase: dpv1alpha1.BackupPhaseCompleted},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	restoreMGR := plan.NewRestoreManager(ctx, cli, cluster, scheme, map[string]string{
		constant.OpsRequestNameLabelKey: opsRequest.Name,
	}, 1, 3)

	err := horizontalScalingOpsHandler{}.createRestore(intctrlutil.RequestCtx{Ctx: ctx}, cli,
		&OpsResource{Cluster: cluster, OpsRequest: opsRequest},
		&component.SynthesizedComponent{Name: "mysql"}, restoreMGR,
		&appsv1.ClusterComponentSpec{Name: "mysql"}, backup, "")
	if err == nil {
		t.Fatal("expected fatal error for completed backup without backup method")
	}
	if !intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
		t.Fatalf("expected fatal error, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "status.backupMethod") {
		t.Fatalf("unexpected error: %v", err)
	}

	restores := &dpv1alpha1.RestoreList{}
	if err := cli.List(ctx, restores, client.InNamespace("default")); err != nil {
		t.Fatalf("list restores: %v", err)
	}
	if len(restores.Items) != 0 {
		t.Fatalf("expected no restore to be created, got %d", len(restores.Items))
	}
}
