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

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestEnsureWorkerServiceAccountPreservesExistingImagePullSecrets(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := rbacv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add rbac scheme: %v", err)
	}

	const (
		namespace = "default"
		saName    = "worker-sa"
		roleName  = "worker-role"
	)
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.SetDefault(dptypes.CfgKeyWorkerServiceAccountName, saName)
	viper.SetDefault(dptypes.CfgKeyWorkerClusterRoleName, roleName)
	viper.SetDefault(dptypes.CfgKeyWorkerServiceAccountAnnotations, `{"role-arn":"arn:xxx:xxx"}`)

	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      saName,
			},
			ImagePullSecrets: []corev1.LocalObjectReference{{Name: "user-pull-secret"}},
		}).
		Build()

	gotName, err := EnsureWorkerServiceAccount(intctrlutil.RequestCtx{Ctx: context.Background()}, cli, namespace, nil)
	if err != nil {
		t.Fatalf("ensure worker service account: %v", err)
	}
	if gotName != saName {
		t.Fatalf("expected service account %s, got %s", saName, gotName)
	}

	current := &corev1.ServiceAccount{}
	if err := cli.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: saName}, current); err != nil {
		t.Fatalf("get service account: %v", err)
	}
	if len(current.ImagePullSecrets) != 1 || current.ImagePullSecrets[0].Name != "user-pull-secret" {
		t.Fatalf("expected user imagePullSecrets to be preserved, got %#v", current.ImagePullSecrets)
	}
	if current.Annotations["role-arn"] != "arn:xxx:xxx" {
		t.Fatalf("expected dataprotection annotation to be merged, got %#v", current.Annotations)
	}
}
