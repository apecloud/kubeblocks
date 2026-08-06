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

package dataprotection

import (
	"context"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func newBackupReferenceGrant(name, backupNamespace, restoreNamespace, backupName string) *gatewayv1beta1.ReferenceGrant {
	var grantedBackupName *gatewayv1beta1.ObjectName
	if backupName != "" {
		name := gatewayv1beta1.ObjectName(backupName)
		grantedBackupName = &name
	}
	return &gatewayv1beta1.ReferenceGrant{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: backupNamespace},
		Spec: gatewayv1beta1.ReferenceGrantSpec{
			From: []gatewayv1beta1.ReferenceGrantFrom{{
				Group:     gatewayv1beta1.Group(dpv1alpha1.GroupVersion.Group),
				Kind:      gatewayv1beta1.Kind("Restore"),
				Namespace: gatewayv1beta1.Namespace(restoreNamespace),
			}},
			To: []gatewayv1beta1.ReferenceGrantTo{{
				Group: gatewayv1beta1.Group(dpv1alpha1.GroupVersion.Group),
				Kind:  gatewayv1beta1.Kind("Backup"),
				Name:  grantedBackupName,
			}},
		},
	}
}

func TestCheckBackupRepoForRestoreAuthorizesBeforeBackupRead(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := dpv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := gatewayv1beta1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	restore := &dpv1alpha1.Restore{
		ObjectMeta: metav1.ObjectMeta{Name: "restore", Namespace: "target"},
		Spec: dpv1alpha1.RestoreSpec{
			Backup: dpv1alpha1.BackupRef{Name: "backup", Namespace: "source"},
		},
	}

	_, err := CheckBackupRepoForRestore(intctrlutil.RequestCtx{Ctx: context.Background()}, cli, restore)
	if err == nil || !strings.Contains(err.Error(), "ReferenceGrant") {
		t.Fatalf("expected ReferenceGrant error before Backup lookup, got %v", err)
	}
	if strings.Contains(err.Error(), "backups.dataprotection.kubeblocks.io") {
		t.Fatalf("Backup was read before authorization: %v", err)
	}
}
