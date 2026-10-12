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

package cluster

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestSharedShardingAccountPreservesEmptyRestorePassword(t *testing.T) {
	const (
		shardingName = "sharding"
		accountName  = "root"
		compDefName  = "compdef"
	)
	encryptedPassword, err := intctrlutil.NewEncryptor(viper.GetString(constant.CfgKeyDPEncryptionKey)).Encrypt(nil)
	if err != nil {
		t.Fatalf("encrypt empty password: %v", err)
	}
	backup := &dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{
		Name:      "backup",
		Namespace: "default",
		Annotations: map[string]string{
			constant.EncryptedSystemAccountsAnnotationKey: fmt.Sprintf(`{"%s":{"%s":"%s"}}`, shardingName, accountName, encryptedPassword),
		},
	}}
	scheme := runtime.NewScheme()
	if err := dpv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add backup scheme: %v", err)
	}
	compDef := &appsv1.ComponentDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: compDefName},
		Spec: appsv1.ComponentDefinitionSpec{SystemAccounts: []appsv1.SystemAccount{{
			Name: accountName,
			PasswordGenerationPolicy: appsv1.PasswordConfig{
				Length: 16,
			},
		}}},
	}
	transCtx := &clusterTransformContext{
		Context: context.Background(),
		Client:  fake.NewClientBuilder().WithScheme(scheme).WithObjects(backup).Build(),
		Cluster: &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster",
			Namespace: "default",
			Annotations: map[string]string{
				constant.RestoreFromBackupAnnotationKey: `{"sharding":{"name":"backup","namespace":"default"}}`,
			},
		}},
		componentDefs: map[string]*appsv1.ComponentDefinition{compDefName: compDef},
	}
	sharding := &appsv1.ClusterSharding{
		Name: shardingName,
		Template: appsv1.ClusterComponentSpec{
			ComponentDef: compDefName,
		},
	}

	secret, err := (&clusterShardingAccountTransformer{}).newSystemAccountSecret(transCtx, sharding, accountName)
	if err != nil {
		t.Fatalf("build shared sharding account secret: %v", err)
	}
	if password := secret.Data[constant.AccountPasswdForSecret]; len(password) != 0 {
		t.Fatalf("expected restored empty password, got %d bytes", len(password))
	}
}

const (
	testShardingClusterName  = "cluster"
	testShardingNamespace    = "default"
	testShardingName         = "sharding1"
	testShardingDefName      = "sharding-def"
	testShardingCompDefName  = "compdef"
	testShardingAccountName  = "root"
	testShardingAccountName2 = "admin"
)

func testManagedShardingSecretName(account string) string {
	return shardingAccountSecretName(testShardingClusterName, testShardingName, account)
}

func newShardingAccountTransformContext(t *testing.T, objs ...client.Object) *clusterTransformContext {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1 scheme: %v", err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add appsv1 scheme: %v", err)
	}
	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{
		Name:      testShardingClusterName,
		Namespace: testShardingNamespace,
	}}
	return &clusterTransformContext{
		Context:       context.Background(),
		Client:        model.NewGraphClient(fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()),
		Logger:        logr.Discard(),
		Cluster:       cluster,
		OrigCluster:   cluster.DeepCopy(),
		componentDefs: map[string]*appsv1.ComponentDefinition{},
		shardingDefs:  map[string]*appsv1.ShardingDefinition{},
		shardingComps: map[string][]*appsv1.ClusterComponentSpec{},
	}
}

func newShardingAccountCompDef(name string) *appsv1.ComponentDefinition {
	return &appsv1.ComponentDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: appsv1.ComponentDefinitionSpec{
			SystemAccounts: []appsv1.SystemAccount{
				{
					Name:                     testShardingAccountName,
					PasswordGenerationPolicy: appsv1.PasswordConfig{Length: 16},
				},
				{
					Name:                     testShardingAccountName2,
					PasswordGenerationPolicy: appsv1.PasswordConfig{Length: 16},
				},
			},
		},
	}
}

// secretNamesInDAG returns the sorted names of the secrets created into the DAG.
func secretNamesInDAG(graphCli model.GraphClient, dag *graph.DAG) []string {
	names := make([]string, 0)
	for _, obj := range graphCli.FindAll(dag, &corev1.Secret{}) {
		names = append(names, obj.GetName())
	}
	sort.Strings(names)
	return names
}

func sortedStrings(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	return out
}

// accountSecretRefNames maps account name to the name of its secretRef, empty string if not set.
func accountSecretRefNames(accounts []appsv1.ComponentSystemAccount) map[string]string {
	result := make(map[string]string, len(accounts))
	for i := range accounts {
		name := ""
		if accounts[i].SecretRef != nil {
			name = accounts[i].SecretRef.Name
		}
		result[accounts[i].Name] = name
	}
	return result
}

func TestSystemAccountSecretRefSpecified(t *testing.T) {
	userSecretRef := func() *appsv1.ProvisionSecretRef {
		return &appsv1.ProvisionSecretRef{Name: "user-secret", Namespace: testShardingNamespace}
	}

	tests := []struct {
		name        string
		accounts    []appsv1.ComponentSystemAccount
		accountName string
		want        bool
	}{
		{
			name:        "account not defined in the template",
			accounts:    nil,
			accountName: testShardingAccountName,
			want:        false,
		},
		{
			name:        "account defined without secretRef",
			accounts:    []appsv1.ComponentSystemAccount{{Name: testShardingAccountName}},
			accountName: testShardingAccountName,
			want:        false,
		},
		{
			name: "account defined with secretRef",
			accounts: []appsv1.ComponentSystemAccount{
				{Name: testShardingAccountName, SecretRef: userSecretRef()},
			},
			accountName: testShardingAccountName,
			want:        true,
		},
		{
			name: "secretRef belongs to another account",
			accounts: []appsv1.ComponentSystemAccount{
				{Name: testShardingAccountName2, SecretRef: userSecretRef()},
			},
			accountName: testShardingAccountName,
			want:        false,
		},
		{
			name: "matched account among multiple accounts",
			accounts: []appsv1.ComponentSystemAccount{
				{Name: testShardingAccountName2},
				{Name: testShardingAccountName, SecretRef: userSecretRef()},
			},
			accountName: testShardingAccountName,
			want:        true,
		},
		{
			// documents the current behavior: a non-nil but empty secretRef is treated as
			// specified, so the account will be skipped.
			name: "empty secretRef object",
			accounts: []appsv1.ComponentSystemAccount{
				{Name: testShardingAccountName, SecretRef: &appsv1.ProvisionSecretRef{}},
			},
			accountName: testShardingAccountName,
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharding := &appsv1.ClusterSharding{
				Name: testShardingName,
				Template: appsv1.ClusterComponentSpec{
					ComponentDef:   testShardingCompDefName,
					SystemAccounts: tt.accounts,
				},
			}
			if got := systemAccountSecretRefSpecified(sharding, tt.accountName); got != tt.want {
				t.Errorf("systemAccountSecretRefSpecified() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReconcileShardingAccounts(t *testing.T) {
	userSecretRef := func() *appsv1.ProvisionSecretRef {
		return &appsv1.ProvisionSecretRef{Name: "user-secret", Namespace: testShardingNamespace}
	}
	shared := func(name string) appsv1.ShardingSystemAccount {
		return appsv1.ShardingSystemAccount{Name: name, Shared: ptr.To(true)}
	}
	notShared := func(name string) appsv1.ShardingSystemAccount {
		return appsv1.ShardingSystemAccount{Name: name, Shared: ptr.To(false)}
	}

	tests := []struct {
		name             string
		shardingAccounts []appsv1.ShardingSystemAccount  // shardingDefinition.spec.systemAccounts
		templateAccounts []appsv1.ComponentSystemAccount // sharding.template.systemAccounts
		preObjects       []client.Object
		// wantCreated is the set of secrets expected to be created into the DAG.
		wantCreated []string
		// wantTemplateRefs maps account name to the expected secretRef name in sharding.template.
		wantTemplateRefs map[string]string
		// wantCompRefs maps account name to the expected secretRef name appended to the sharding components.
		wantCompRefs map[string]string
	}{
		{
			// the main scenario of the change: the user specified a secretRef in the sharding
			// template, so the shared account must be left untouched.
			name:             "shared account with user defined secretRef is skipped",
			shardingAccounts: []appsv1.ShardingSystemAccount{shared(testShardingAccountName)},
			templateAccounts: []appsv1.ComponentSystemAccount{
				{Name: testShardingAccountName, SecretRef: userSecretRef()},
			},
			wantCreated:      []string{},
			wantTemplateRefs: map[string]string{testShardingAccountName: "user-secret"},
			wantCompRefs:     map[string]string{},
		},
		{
			name:             "shared account without secretRef is reconciled",
			shardingAccounts: []appsv1.ShardingSystemAccount{shared(testShardingAccountName)},
			templateAccounts: []appsv1.ComponentSystemAccount{{Name: testShardingAccountName}},
			wantCreated:      []string{testManagedShardingSecretName(testShardingAccountName)},
			wantTemplateRefs: map[string]string{testShardingAccountName: testManagedShardingSecretName(testShardingAccountName)},
			wantCompRefs:     map[string]string{testShardingAccountName: testManagedShardingSecretName(testShardingAccountName)},
		},
		{
			name:             "shared account not defined in the template is reconciled",
			shardingAccounts: []appsv1.ShardingSystemAccount{shared(testShardingAccountName)},
			templateAccounts: nil,
			wantCreated:      []string{testManagedShardingSecretName(testShardingAccountName)},
			wantTemplateRefs: map[string]string{testShardingAccountName: testManagedShardingSecretName(testShardingAccountName)},
			wantCompRefs:     map[string]string{testShardingAccountName: testManagedShardingSecretName(testShardingAccountName)},
		},
		{
			name:             "non shared account is ignored",
			shardingAccounts: []appsv1.ShardingSystemAccount{notShared(testShardingAccountName)},
			templateAccounts: nil,
			wantCreated:      []string{},
			wantTemplateRefs: map[string]string{},
			wantCompRefs:     map[string]string{},
		},
		{
			name:             "account without explicit shared flag is ignored",
			shardingAccounts: []appsv1.ShardingSystemAccount{{Name: testShardingAccountName}},
			templateAccounts: nil,
			wantCreated:      []string{},
			wantTemplateRefs: map[string]string{},
			wantCompRefs:     map[string]string{},
		},
		{
			name: "only accounts without secretRef are reconciled",
			shardingAccounts: []appsv1.ShardingSystemAccount{
				shared(testShardingAccountName),
				shared(testShardingAccountName2),
			},
			templateAccounts: []appsv1.ComponentSystemAccount{
				{Name: testShardingAccountName, SecretRef: userSecretRef()},
				{Name: testShardingAccountName2},
			},
			wantCreated: []string{testManagedShardingSecretName(testShardingAccountName2)},
			wantTemplateRefs: map[string]string{
				testShardingAccountName:  "user-secret",
				testShardingAccountName2: testManagedShardingSecretName(testShardingAccountName2),
			},
			wantCompRefs: map[string]string{
				testShardingAccountName2: testManagedShardingSecretName(testShardingAccountName2),
			},
		},
		{
			name:             "existing managed secret is not recreated",
			shardingAccounts: []appsv1.ShardingSystemAccount{shared(testShardingAccountName)},
			templateAccounts: nil,
			preObjects: []client.Object{&corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name:      testManagedShardingSecretName(testShardingAccountName),
				Namespace: testShardingNamespace,
			}}},
			wantCreated:      []string{},
			wantTemplateRefs: map[string]string{testShardingAccountName: testManagedShardingSecretName(testShardingAccountName)},
			wantCompRefs:     map[string]string{testShardingAccountName: testManagedShardingSecretName(testShardingAccountName)},
		},
		{
			// NOTE: this documents the current behavior of systemAccountSecretRefSpecified, which
			// cannot tell a user-specified secretRef from the one written back by rewriteSystemAccount
			// during a previous reconcile. If the check is tightened to exclude the KB-managed secret
			// name, this expectation should be updated to the "reconciled" one.
			name:             "template secretRef pointing to the managed secret name is skipped",
			shardingAccounts: []appsv1.ShardingSystemAccount{shared(testShardingAccountName)},
			templateAccounts: []appsv1.ComponentSystemAccount{{
				Name: testShardingAccountName,
				SecretRef: &appsv1.ProvisionSecretRef{
					Name:      testManagedShardingSecretName(testShardingAccountName),
					Namespace: testShardingNamespace,
				},
			}},
			wantCreated:      []string{},
			wantTemplateRefs: map[string]string{testShardingAccountName: testManagedShardingSecretName(testShardingAccountName)},
			wantCompRefs:     map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transCtx := newShardingAccountTransformContext(t, tt.preObjects...)
			transCtx.shardings = []*appsv1.ClusterSharding{{
				Name:        testShardingName,
				ShardingDef: testShardingDefName,
				Template: appsv1.ClusterComponentSpec{
					ComponentDef:   testShardingCompDefName,
					SystemAccounts: tt.templateAccounts,
				},
			}}
			transCtx.shardingDefs[testShardingDefName] = &appsv1.ShardingDefinition{
				ObjectMeta: metav1.ObjectMeta{Name: testShardingDefName},
				Spec:       appsv1.ShardingDefinitionSpec{SystemAccounts: tt.shardingAccounts},
			}
			transCtx.componentDefs[testShardingCompDefName] = newShardingAccountCompDef(testShardingCompDefName)
			transCtx.shardingComps[testShardingName] = []*appsv1.ClusterComponentSpec{{
				Name:         testShardingName,
				ComponentDef: testShardingCompDefName,
			}}

			graphCli := transCtx.Client.(model.GraphClient)
			dag := graph.NewDAG()
			graphCli.Root(dag, transCtx.OrigCluster, transCtx.Cluster, model.ActionStatusPtr())

			transformer := &clusterShardingAccountTransformer{}
			if err := transformer.reconcileShardingAccounts(transCtx, graphCli, dag); err != nil {
				t.Fatalf("reconcileShardingAccounts() error = %v", err)
			}

			if got := secretNamesInDAG(graphCli, dag); !reflect.DeepEqual(got, sortedStrings(tt.wantCreated)) {
				t.Errorf("secrets created into the DAG = %v, want %v", got, sortedStrings(tt.wantCreated))
			}
			if got := accountSecretRefNames(transCtx.shardings[0].Template.SystemAccounts); !reflect.DeepEqual(got, tt.wantTemplateRefs) {
				t.Errorf("sharding template account secretRefs = %v, want %v", got, tt.wantTemplateRefs)
			}
			if got := accountSecretRefNames(transCtx.shardingComps[testShardingName][0].SystemAccounts); !reflect.DeepEqual(got, tt.wantCompRefs) {
				t.Errorf("sharding component account secretRefs = %v, want %v", got, tt.wantCompRefs)
			}
		})
	}
}
