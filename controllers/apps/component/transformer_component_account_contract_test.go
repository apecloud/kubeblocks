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

package component

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	pkgcomponent "github.com/apecloud/kubeblocks/pkg/controller/component"
)

func TestBuildAccountSecretPreservesEmptyReferencedPassword(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	referenced := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "passwordless"},
		Data:       map[string][]byte{constant.AccountPasswdForSecret: {}},
	}
	component := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "demo-comp"},
	}
	transCtx := &componentTransformContext{
		Context:   context.Background(),
		Client:    fake.NewClientBuilder().WithScheme(scheme).WithObjects(referenced).Build(),
		Component: component,
		SynthesizeComponent: &pkgcomponent.SynthesizedComponent{
			Namespace:   "default",
			ClusterName: "demo",
			Name:        "comp",
		},
	}
	account := synthesizedSystemAccount{
		SystemAccount: appsv1.SystemAccount{Name: "default"},
		SecretRef: &appsv1.ProvisionSecretRef{
			Name: referenced.Name,
		},
	}

	secret, err := (&componentAccountTransformer{}).buildAccountSecret(transCtx, account)
	if err != nil {
		t.Fatalf("build account secret: %v", err)
	}
	password, ok := secret.Data[constant.AccountPasswdForSecret]
	if !ok {
		t.Fatal("managed account secret is missing the password key")
	}
	if len(password) != 0 {
		t.Fatalf("expected referenced empty password to be preserved, got %d bytes", len(password))
	}
}

func TestBuildAccountSecretWithoutGenerationConfigurationIsPasswordless(t *testing.T) {
	component := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "demo-comp"},
	}
	transCtx := &componentTransformContext{
		Context:   context.Background(),
		Component: component,
		SynthesizeComponent: &pkgcomponent.SynthesizedComponent{
			Namespace:   "default",
			ClusterName: "demo",
			Name:        "comp",
		},
	}

	secret, err := (&componentAccountTransformer{}).buildAccountSecret(transCtx, synthesizedSystemAccount{
		SystemAccount: appsv1.SystemAccount{Name: "default"},
	})
	if err != nil {
		t.Fatalf("build account secret: %v", err)
	}
	password, ok := secret.Data[constant.AccountPasswdForSecret]
	if !ok {
		t.Fatal("managed account secret is missing the password key")
	}
	if len(password) != 0 {
		t.Fatalf("expected passwordless account, got %d password bytes", len(password))
	}
}
