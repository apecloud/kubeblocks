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
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	pkgcomponent "github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var _ = Describe("component system account API contract", func() {
	It("preserves an omitted password configuration through the typed API", func() {
		compDef := testapps.NewComponentDefinitionFactory("system-account-wire-contract").
			SetDefaultSpec().
			GetObject()
		compDef.Spec.SystemAccounts = []appsv1.SystemAccount{
			{Name: "passwordless"},
		}

		Expect(k8sClient.Create(ctx, compDef)).To(Succeed())
		DeferCleanup(func() {
			Expect(ctrlclient.IgnoreNotFound(k8sClient.Delete(ctx, compDef))).To(Succeed())
		})

		stored := &unstructured.Unstructured{}
		stored.SetGroupVersionKind(appsv1.GroupVersion.WithKind("ComponentDefinition"))
		Expect(k8sClient.Get(ctx, ctrlclient.ObjectKeyFromObject(compDef), stored)).To(Succeed())

		accounts, found, err := unstructured.NestedSlice(stored.Object, "spec", "systemAccounts")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(accounts).To(HaveLen(1))
		Expect(accounts[0]).To(HaveKey("name"))
		Expect(accounts[0]).NotTo(HaveKey("passwordConfig"))
	})

	It("preserves an explicit zero digit count from a typed API request", func() {
		compDef := testapps.NewComponentDefinitionFactory("system-account-zero-digits").
			SetDefaultSpec().
			GetObject()
		compDef.Spec.SystemAccounts = []appsv1.SystemAccount{
			{
				Name: "zero-digits",
				PasswordConfig: &appsv1.PasswordConfig{
					Length:     8,
					NumDigits:  ptr.To[int32](0),
					LetterCase: appsv1.LowerCases,
				},
			},
		}

		Expect(k8sClient.Create(ctx, compDef)).To(Succeed())
		DeferCleanup(func() {
			Expect(ctrlclient.IgnoreNotFound(k8sClient.Delete(ctx, compDef))).To(Succeed())
		})

		stored := &unstructured.Unstructured{}
		stored.SetGroupVersionKind(appsv1.GroupVersion.WithKind("ComponentDefinition"))
		Expect(k8sClient.Get(ctx, ctrlclient.ObjectKeyFromObject(compDef), stored)).To(Succeed())

		accounts, found, err := unstructured.NestedSlice(stored.Object, "spec", "systemAccounts")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(accounts).To(HaveLen(1))

		passwordConfig, found, err := unstructured.NestedMap(
			accounts[0].(map[string]interface{}), "passwordConfig")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(passwordConfig).To(HaveKeyWithValue("numDigits", BeEquivalentTo(0)))
	})
})

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

func TestBuildAccountSecretRevisionContract(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	referenced := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "shared-account"},
		Data:       map[string][]byte{constant.AccountPasswdForSecret: []byte("password")},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(referenced).Build()
	transCtx := &componentTransformContext{
		Context: context.Background(),
		Client:  fakeClient,
		Component: &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "demo-comp"},
		},
		SynthesizeComponent: &pkgcomponent.SynthesizedComponent{
			Namespace: "default", ClusterName: "demo", Name: "comp",
		},
	}
	account := synthesizedSystemAccount{
		SystemAccount:     appsv1.SystemAccount{Name: "default"},
		SecretRef:         &appsv1.ProvisionSecretRef{Name: referenced.Name},
		SecretRefRevision: "new-revision",
	}

	secret, err := (&componentAccountTransformer{}).buildAccountSecret(transCtx, account)
	if err != nil {
		t.Fatalf("a user-managed Secret without a revision annotation should be read directly: %v", err)
	}
	if got := string(secret.Data[constant.AccountPasswdForSecret]); got != "password" {
		t.Fatalf("expected referenced password, got %q", got)
	}

	referenced.Annotations = map[string]string{constant.SecretRevisionAnnotationKey: "old-revision"}
	if err = fakeClient.Update(context.Background(), referenced); err != nil {
		t.Fatalf("update referenced Secret revision: %v", err)
	}
	if _, err = (&componentAccountTransformer{}).buildAccountSecret(transCtx, account); !intctrlutil.IsDelayedRequeueError(err) {
		t.Fatalf("a managed Secret with a mismatched revision should wait, got %v", err)
	}

	referenced.Annotations[constant.SecretRevisionAnnotationKey] = "new-revision"
	if err = fakeClient.Update(context.Background(), referenced); err != nil {
		t.Fatalf("update referenced Secret revision: %v", err)
	}
	if _, err = (&componentAccountTransformer{}).buildAccountSecret(transCtx, account); err != nil {
		t.Fatalf("a managed Secret with a matching revision should be read: %v", err)
	}

	if err = fakeClient.Delete(context.Background(), referenced); err != nil {
		t.Fatalf("delete referenced Secret: %v", err)
	}
	if _, err = (&componentAccountTransformer{}).buildAccountSecret(transCtx, account); !intctrlutil.IsDelayedRequeueError(err) {
		t.Fatalf("a missing referenced Secret with a revision should wait, got %v", err)
	}
}

func TestBuildAccountSecretRejectsOverlongReferencedPassword(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	referenced := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "overlong-password"},
		Data:       map[string][]byte{constant.AccountPasswdForSecret: []byte(strings.Repeat("a", 65))},
	}
	transCtx := &componentTransformContext{
		Context: context.Background(),
		Client:  fake.NewClientBuilder().WithScheme(scheme).WithObjects(referenced).Build(),
		Component: &appsv1.Component{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "demo-comp"},
		},
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

	_, err := (&componentAccountTransformer{}).buildAccountSecret(transCtx, account)
	if err == nil || err.Error() != "password length exceeds 64 bytes" {
		t.Fatalf("expected password length error, got %v", err)
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
