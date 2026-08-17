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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

var _ = Describe("cluster sharding shared system account password contract", func() {
	const (
		namespace   = "default"
		clusterName = "cluster"
		sharding    = "sharding"
		compDefName = "compdef"
		accountName = "root"
	)

	newContext := func(account appsv1.SystemAccount, objects ...client.Object) (*clusterTransformContext, *appsv1.ClusterSharding) {
		scheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(appsv1.AddToScheme(scheme)).To(Succeed())
		Expect(dpv1alpha1.AddToScheme(scheme)).To(Succeed())
		compDef := &appsv1.ComponentDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: compDefName},
			Spec: appsv1.ComponentDefinitionSpec{
				SystemAccounts: []appsv1.SystemAccount{account},
			},
		}
		return &clusterTransformContext{
				Context: context.Background(),
				Client: fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(objects...).
					Build(),
				Cluster: &appsv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: clusterName},
				},
				componentDefs: map[string]*appsv1.ComponentDefinition{compDefName: compDef},
			}, &appsv1.ClusterSharding{
				Name: sharding,
				Template: appsv1.ClusterComponentSpec{
					ComponentDef: compDefName,
				},
			}
	}

	It("generates a password from the presence-aware configuration", func() {
		transCtx, shardingSpec := newContext(appsv1.SystemAccount{
			Name: accountName,
			PasswordConfig: &appsv1.PasswordConfig{
				Length:    16,
				NumDigits: ptr.To[int32](0),
			},
		})

		secret, err := (&clusterShardingAccountTransformer{}).
			newSystemAccountSecret(transCtx, shardingSpec, accountName)
		Expect(err).NotTo(HaveOccurred())
		Expect(secret.Data[constant.AccountPasswdForSecret]).To(HaveLen(16))
		Expect(secret.Immutable).To(BeNil())
		Expect(secret.Labels).To(HaveKeyWithValue(constant.SystemAccountLabelKey, accountName))
		Expect(secret.Annotations[constant.SecretRevisionAnnotationKey]).NotTo(BeEmpty())
	})

	It("keeps built-in labels authoritative over shared account metadata", func() {
		transCtx, shardingSpec := newContext(appsv1.SystemAccount{Name: accountName})
		shardingSpec.Template.Labels = map[string]string{
			"precedence":                       "dynamic",
			constant.AppManagedByLabelKey:      "dynamic-managed-by",
			constant.AppInstanceLabelKey:       "dynamic-cluster",
			constant.KBAppShardingNameLabelKey: "dynamic-sharding",
			constant.SystemAccountLabelKey:     "dynamic-account",
		}
		transCtx.componentDefs[compDefName].Spec.Labels = map[string]string{
			"precedence":                       "static",
			constant.AppManagedByLabelKey:      "static-managed-by",
			constant.AppInstanceLabelKey:       "static-cluster",
			constant.KBAppShardingNameLabelKey: "static-sharding",
			constant.SystemAccountLabelKey:     "static-account",
		}

		secret, err := (&clusterShardingAccountTransformer{}).
			newAccountSecretWithPassword(transCtx, shardingSpec, accountName, []byte("password"), "managed-revision")
		Expect(err).NotTo(HaveOccurred())
		Expect(secret.Labels).To(HaveKeyWithValue("precedence", "dynamic"))
		Expect(secret.Labels).To(HaveKeyWithValue(constant.AppManagedByLabelKey, constant.AppName))
		Expect(secret.Labels).To(HaveKeyWithValue(constant.AppInstanceLabelKey, clusterName))
		Expect(secret.Labels).To(HaveKeyWithValue(constant.KBAppShardingNameLabelKey, sharding))
		Expect(secret.Labels).To(HaveKeyWithValue(constant.SystemAccountLabelKey, accountName))
		Expect(secret.Annotations).To(HaveKeyWithValue(constant.SecretRevisionAnnotationKey, "managed-revision"))
	})

	It("recognizes a created shared account Secret on the next reconcile", func() {
		transCtx, shardingSpec := newContext(appsv1.SystemAccount{Name: accountName})
		shardingSpec.Template.Labels = map[string]string{
			constant.AppInstanceLabelKey:       "user-cluster",
			constant.KBAppShardingNameLabelKey: "user-sharding",
			constant.SystemAccountLabelKey:     "user-account",
		}
		transCtx.componentDefs[compDefName].Spec.Labels = map[string]string{
			constant.AppInstanceLabelKey:       "definition-cluster",
			constant.KBAppShardingNameLabelKey: "definition-sharding",
			constant.SystemAccountLabelKey:     "definition-account",
		}

		created, err := (&clusterShardingAccountTransformer{}).
			newAccountSecretWithPassword(transCtx, shardingSpec, accountName, []byte("password"), "managed-revision")
		Expect(err).NotTo(HaveOccurred())
		Expect(transCtx.Client.(client.Client).Create(context.Background(), created)).To(Succeed())
		found, err := (&clusterShardingAccountTransformer{}).
			getSystemAccountSecret(transCtx, shardingSpec, accountName)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).NotTo(BeNil())
		Expect(found.Name).To(Equal(created.Name))
	})

	It("keeps the deprecated generation policy compatible", func() {
		transCtx, shardingSpec := newContext(appsv1.SystemAccount{
			Name: accountName,
			PasswordGenerationPolicy: appsv1.PasswordConfig{
				Length:    12,
				NumDigits: ptr.To[int32](2),
			},
		})

		secret, err := (&clusterShardingAccountTransformer{}).
			newSystemAccountSecret(transCtx, shardingSpec, accountName)
		Expect(err).NotTo(HaveOccurred())
		Expect(secret.Data[constant.AccountPasswdForSecret]).To(HaveLen(12))
	})

	It("creates a passwordless account when neither configuration is supplied", func() {
		transCtx, shardingSpec := newContext(appsv1.SystemAccount{Name: accountName})

		secret, err := (&clusterShardingAccountTransformer{}).
			newSystemAccountSecret(transCtx, shardingSpec, accountName)
		Expect(err).NotTo(HaveOccurred())
		Expect(secret.Data).To(HaveKey(constant.AccountPasswdForSecret))
		Expect(secret.Data[constant.AccountPasswdForSecret]).To(BeEmpty())
	})

	It("preserves an empty password restored from backup", func() {
		encryptedPassword, err := intctrlutil.NewEncryptor(viper.GetString(constant.CfgKeyDPEncryptionKey)).Encrypt(nil)
		Expect(err).NotTo(HaveOccurred())
		backup := &dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{
			Name:      "backup",
			Namespace: namespace,
			Annotations: map[string]string{
				constant.EncryptedSystemAccountsAnnotationKey: fmt.Sprintf(`{"%s":{"%s":"%s"}}`, sharding, accountName, encryptedPassword),
			},
		}}
		transCtx, shardingSpec := newContext(appsv1.SystemAccount{
			Name: accountName,
			PasswordConfig: &appsv1.PasswordConfig{
				Length: 16,
			},
		}, backup)
		transCtx.Cluster.Annotations = map[string]string{
			constant.RestoreFromBackupAnnotationKey: `{"sharding":{"name":"backup","namespace":"default"}}`,
		}

		secret, err := (&clusterShardingAccountTransformer{}).
			newSystemAccountSecret(transCtx, shardingSpec, accountName)
		Expect(err).NotTo(HaveOccurred())
		Expect(secret.Data[constant.AccountPasswdForSecret]).To(BeEmpty())
	})

	It("preserves an empty password from a same-namespace secretRef", func() {
		referenced := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "passwordless"},
			Data:       map[string][]byte{constant.AccountPasswdForSecret: {}},
		}
		transCtx, shardingSpec := newContext(appsv1.SystemAccount{Name: accountName}, referenced)
		shardingSpec.Template.SystemAccounts = []appsv1.ComponentSystemAccount{{
			Name: accountName,
			SecretRef: &appsv1.ProvisionSecretRef{
				Name: referenced.Name,
			},
		}}

		secret, err := (&clusterShardingAccountTransformer{}).
			newSystemAccountSecret(transCtx, shardingSpec, accountName)
		Expect(err).NotTo(HaveOccurred())
		Expect(secret.Data[constant.AccountPasswdForSecret]).To(BeEmpty())
	})

	It("rejects a cross-namespace secretRef", func() {
		transCtx, shardingSpec := newContext(appsv1.SystemAccount{Name: accountName})
		shardingSpec.Template.SystemAccounts = []appsv1.ComponentSystemAccount{{
			Name: accountName,
			SecretRef: &appsv1.ProvisionSecretRef{
				Name:      "password",
				Namespace: "other",
			},
		}}

		_, err := (&clusterShardingAccountTransformer{}).
			newSystemAccountSecret(transCtx, shardingSpec, accountName)
		Expect(err).To(MatchError("cross-namespace secretRef is not supported for shared sharding system accounts"))
	})

	It("returns a missing source Secret error when the revision is empty", func() {
		transCtx, shardingSpec := newContext(appsv1.SystemAccount{Name: accountName})
		shardingSpec.Template.SystemAccounts = []appsv1.ComponentSystemAccount{{
			Name:      accountName,
			SecretRef: &appsv1.ProvisionSecretRef{Name: "missing-source"},
		}}

		_, err := (&clusterShardingAccountTransformer{}).
			newSystemAccountSecret(transCtx, shardingSpec, accountName)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
		Expect(intctrlutil.IsDelayedRequeueError(err)).To(BeFalse())
	})

	It("returns a missing source Secret error even when a revision is set", func() {
		transCtx, shardingSpec := newContext(appsv1.SystemAccount{Name: accountName})
		shardingSpec.Template.SystemAccounts = []appsv1.ComponentSystemAccount{{
			Name:              accountName,
			SecretRef:         &appsv1.ProvisionSecretRef{Name: "missing-source"},
			SecretRefRevision: "opaque-revision",
		}}

		_, err := (&clusterShardingAccountTransformer{}).
			newSystemAccountSecret(transCtx, shardingSpec, accountName)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
		Expect(intctrlutil.IsDelayedRequeueError(err)).To(BeFalse())
	})

	It("uses the opaque field without requiring a source Secret annotation", func() {
		for _, annotations := range []map[string]string{
			nil,
			{constant.SecretRevisionAnnotationKey: "unrelated-revision"},
		} {
			source := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "source-account"},
				Data:       map[string][]byte{constant.AccountPasswdForSecret: []byte("new-password")},
			}
			source.Annotations = annotations
			transCtx, shardingSpec := newContext(appsv1.SystemAccount{Name: accountName}, source)
			shardingSpec.Template.SystemAccounts = []appsv1.ComponentSystemAccount{{
				Name:              accountName,
				SecretRef:         &appsv1.ProvisionSecretRef{Name: source.Name},
				SecretRefRevision: "opaque-revision",
			}}

			secret, err := (&clusterShardingAccountTransformer{}).
				newSystemAccountSecret(transCtx, shardingSpec, accountName)
			Expect(err).NotTo(HaveOccurred())
			managedRevision := sourceSecretRevision(source, constant.AccountPasswdForSecret)
			Expect(secret.Annotations[constant.SecretRevisionAnnotationKey]).To(Equal(managedRevision))
			Expect(managedRevision).NotTo(Equal("opaque-revision"))
			Expect(secret.Data[constant.AccountPasswdForSecret]).To(Equal([]byte("new-password")))
		}
	})

	It("accepts a legacy managed shared account secret", func() {
		transCtx, shardingSpec := newContext(appsv1.SystemAccount{Name: accountName})
		existing := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      shardingAccountSecretName(clusterName, sharding, accountName),
			Labels: constant.GetClusterLabels(clusterName, map[string]string{
				constant.KBAppShardingNameLabelKey: sharding,
			}),
		}}
		Expect(transCtx.Client.(client.Client).Create(context.Background(), existing)).To(Succeed())

		secret, err := (&clusterShardingAccountTransformer{}).
			getSystemAccountSecret(transCtx, shardingSpec, accountName)
		Expect(err).NotTo(HaveOccurred())
		Expect(secret).NotTo(BeNil())
		Expect(secret.Name).To(Equal(existing.Name))
	})

	It("rejects a same-name shared account secret without managed labels", func() {
		transCtx, shardingSpec := newContext(appsv1.SystemAccount{Name: accountName})
		foreign := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      shardingAccountSecretName(clusterName, sharding, accountName),
		}}
		Expect(transCtx.Client.(client.Client).Create(context.Background(), foreign)).To(Succeed())

		secret, err := (&clusterShardingAccountTransformer{}).
			getSystemAccountSecret(transCtx, shardingSpec, accountName)
		Expect(secret).To(BeNil())
		Expect(err).To(MatchError(ContainSubstring("is not managed by sharding")))
	})

	setShardingComps := func(transCtx *clusterTransformContext, shardingSpec *appsv1.ClusterSharding) {
		shard0 := shardingSpec.Template.DeepCopy()
		shard0.Name = "shard-0"
		shard1 := shardingSpec.Template.DeepCopy()
		shard1.Name = "shard-1"
		comps := []*appsv1.ClusterComponentSpec{shard0, shard1}
		transCtx.shardingComps = map[string][]*appsv1.ClusterComponentSpec{
			sharding: comps,
		}
		transCtx.shardingCompsWithTpl = map[string]map[string][]*appsv1.ClusterComponentSpec{
			sharding: {"": comps},
		}
	}

	expectBuiltShardsUseManagedSecret := func(transCtx *clusterTransformContext,
		shardingSpec *appsv1.ClusterSharding, revision string) {
		comps, err := (&clusterShardingHandler{}).buildComps(transCtx, shardingSpec, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(comps).To(HaveLen(2))
		for _, comp := range comps {
			Expect(comp.Spec.SystemAccounts).To(HaveLen(1))
			Expect(comp.Spec.SystemAccounts[0].SecretRef.Name).
				To(Equal(shardingAccountSecretName(clusterName, sharding, accountName)))
			Expect(comp.Spec.SystemAccounts[0].SecretRefRevision).To(Equal(revision))
		}
	}

	type sourceSecretReconcileOptions struct {
		legacyManaged bool
	}
	newSourceSecretReconcile := func(sourcePassword, managedPassword []byte, immutable bool,
		options ...sourceSecretReconcileOptions) (
		*clusterShardingAccountTransformer, *clusterTransformContext, model.GraphClient, *graph.DAG,
		*appsv1.ClusterSharding, *corev1.Secret) {
		option := sourceSecretReconcileOptions{}
		if len(options) > 0 {
			option = options[0]
		}
		transCtx, shardingSpec := newContext(appsv1.SystemAccount{Name: accountName})
		shardingSpec.Template.SystemAccounts = []appsv1.ComponentSystemAccount{{
			Name: accountName,
			SecretRef: &appsv1.ProvisionSecretRef{
				Name: "source-account",
			},
		}}
		source := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "source-account",
			},
			Data: map[string][]byte{constant.AccountPasswdForSecret: sourcePassword},
		}
		managed := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      shardingAccountSecretName(clusterName, sharding, accountName),
			},
			Data: map[string][]byte{
				constant.AccountNameForSecret:   []byte(accountName),
				constant.AccountPasswdForSecret: managedPassword,
			},
		}
		managed.Labels = constant.GetClusterLabels(clusterName, map[string]string{
			constant.KBAppShardingNameLabelKey: sharding,
		})
		if !option.legacyManaged {
			managed.Annotations = map[string]string{constant.SecretRevisionAnnotationKey: "managed-revision"}
		}
		if immutable {
			managed.Immutable = ptr.To(true)
		}
		writer := transCtx.Client.(client.Client)
		Expect(writer.Create(context.Background(), source)).To(Succeed())
		Expect(writer.Create(context.Background(), managed)).To(Succeed())
		graphCli := model.NewGraphClient(transCtx.Client)
		transCtx.Client = graphCli
		transCtx.shardings = []*appsv1.ClusterSharding{shardingSpec}
		setShardingComps(transCtx, shardingSpec)
		dag := graph.NewDAG()
		graphCli.Root(dag, transCtx.Cluster, transCtx.Cluster, model.ActionStatusPtr())
		return &clusterShardingAccountTransformer{}, transCtx, graphCli, dag, shardingSpec, managed
	}

	It("derives the managed revision independently from the opaque source revision", func() {
		transformer, transCtx, graphCli, dag, shardingSpec, managed :=
			newSourceSecretReconcile([]byte("new-password"), []byte("old-password"), false)
		shardingSpec.Template.SystemAccounts[0].SecretRefRevision = "opaque-revision"
		(&clusterNormalizationTransformer{}).writeBackCompNShardingSpecs(transCtx)

		Expect(transformer.reconcileShardingAccount(transCtx, graphCli, dag, shardingSpec, accountName)).To(Succeed())
		updated := graphCli.FindMatchedVertex(dag, managed).(*model.ObjectVertex).Obj.(*corev1.Secret)
		managedRevision := updated.Annotations[constant.SecretRevisionAnnotationKey]
		_, source, passwordKey, err := transformer.getPasswordSource(
			transCtx, &appsv1.ProvisionSecretRef{Name: "source-account"})
		Expect(err).NotTo(HaveOccurred())
		Expect(managedRevision).To(Equal(sourceSecretRevision(source, passwordKey)))
		Expect(managedRevision).NotTo(Equal("opaque-revision"))
		Expect(shardingSpec.Template.SystemAccounts[0].SecretRef.Name).To(Equal("source-account"))
		Expect(shardingSpec.Template.SystemAccounts[0].SecretRefRevision).To(Equal("opaque-revision"))
		persisted := transCtx.Cluster.Spec.Shardings[0].Template.SystemAccounts[0]
		Expect(persisted.SecretRef.Name).To(Equal("source-account"))
		Expect(persisted.SecretRefRevision).To(Equal("opaque-revision"))
		for _, comp := range transCtx.shardingComps[sharding] {
			Expect(comp.SystemAccounts[0].SecretRefRevision).To(Equal(managedRevision))
		}
		expectBuiltShardsUseManagedSecret(transCtx, shardingSpec, managedRevision)
	})

	It("initializes a legacy managed Secret revision", func() {
		transformer, transCtx, graphCli, dag, shardingSpec, managed :=
			newSourceSecretReconcile([]byte("same-password"), []byte("same-password"), false,
				sourceSecretReconcileOptions{legacyManaged: true})

		Expect(transformer.reconcileShardingAccount(transCtx, graphCli, dag, shardingSpec, accountName)).To(Succeed())
		updated := graphCli.FindMatchedVertex(dag, managed).(*model.ObjectVertex).Obj.(*corev1.Secret)
		revision := updated.Annotations[constant.SecretRevisionAnnotationKey]
		Expect(revision).NotTo(BeEmpty())
		Expect(updated.Data).To(HaveKeyWithValue(constant.AccountPasswdForSecret, []byte("same-password")))
		expectBuiltShardsUseManagedSecret(transCtx, shardingSpec, revision)
	})

	It("keeps an immutable shared account secret when the source password is unchanged", func() {
		transformer, transCtx, graphCli, dag, shardingSpec, managed :=
			newSourceSecretReconcile([]byte("same-password"), []byte("same-password"), true)

		Expect(transformer.reconcileShardingAccount(transCtx, graphCli, dag, shardingSpec, accountName)).To(Succeed())
		Expect(graphCli.IsAction(dag, managed, model.ActionUpdatePtr())).To(BeTrue())
		Expect(graphCli.IsAction(dag, managed, model.ActionDeletePtr())).To(BeFalse())
		updated := graphCli.FindMatchedVertex(dag, managed).(*model.ObjectVertex).Obj.(*corev1.Secret)
		Expect(updated.Data).To(HaveKeyWithValue(constant.AccountPasswdForSecret, []byte("same-password")))
		Expect(updated.Annotations[constant.SecretRevisionAnnotationKey]).NotTo(BeEmpty())
	})

	It("preserves generated shared account revisions, including legacy absence", func() {
		for _, revision := range []string{"", "stable-revision"} {
			transCtx, shardingSpec := newContext(appsv1.SystemAccount{Name: accountName})
			managed := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      shardingAccountSecretName(clusterName, sharding, accountName),
				Labels: constant.GetClusterLabels(clusterName, map[string]string{
					constant.KBAppShardingNameLabelKey: sharding,
				}),
			}}
			if revision != "" {
				managed.Annotations = map[string]string{constant.SecretRevisionAnnotationKey: revision}
			}
			Expect(transCtx.Client.(client.Client).Create(context.Background(), managed)).To(Succeed())
			graphCli := model.NewGraphClient(transCtx.Client)
			transCtx.Client = graphCli
			transCtx.shardings = []*appsv1.ClusterSharding{shardingSpec}
			transCtx.shardingComps = map[string][]*appsv1.ClusterComponentSpec{
				sharding: {{Name: "shard-0"}},
			}
			dag := graph.NewDAG()
			graphCli.Root(dag, transCtx.Cluster, transCtx.Cluster, model.ActionStatusPtr())

			Expect((&clusterShardingAccountTransformer{}).
				reconcileShardingAccount(transCtx, graphCli, dag, shardingSpec, accountName)).To(Succeed())
			Expect(graphCli.FindMatchedVertex(dag, managed)).To(BeNil())
			Expect(transCtx.shardingComps[sharding][0].SystemAccounts[0].SecretRefRevision).
				To(Equal(revision))
		}
	})

	It("updates a mutable shared account secret when the source password rotates", func() {
		transformer, transCtx, graphCli, dag, shardingSpec, managed :=
			newSourceSecretReconcile([]byte("new-password"), []byte("old-password"), false)

		Expect(transformer.reconcileShardingAccount(transCtx, graphCli, dag, shardingSpec, accountName)).To(Succeed())
		Expect(graphCli.IsAction(dag, managed, model.ActionUpdatePtr())).To(BeTrue())
		vertex := graphCli.FindMatchedVertex(dag, managed).(*model.ObjectVertex)
		updated := vertex.Obj.(*corev1.Secret)
		Expect(updated.Data).To(HaveKeyWithValue(constant.AccountPasswdForSecret, []byte("new-password")))
		for _, comp := range transCtx.shardingComps[sharding] {
			Expect(comp.SystemAccounts[0].SecretRef.Name).To(Equal(managed.Name))
			Expect(comp.SystemAccounts[0].SecretRefRevision).NotTo(BeEmpty())
			Expect(comp.SystemAccounts[0].SecretRefRevision).To(Equal(
				updated.Annotations[constant.SecretRevisionAnnotationKey]))
		}
	})

	It("uses a source-derived revision without password material", func() {
		transformer, transCtx, graphCli, dag, shardingSpec, managed :=
			newSourceSecretReconcile([]byte("new-password"), []byte("old-password"), false)
		Expect(transformer.reconcileShardingAccount(transCtx, graphCli, dag, shardingSpec, accountName)).To(Succeed())
		updated := graphCli.FindMatchedVertex(dag, managed).(*model.ObjectVertex).Obj.(*corev1.Secret)
		revision := updated.Annotations[constant.SecretRevisionAnnotationKey]
		Expect(revision).NotTo(BeEmpty())
		Expect(revision).NotTo(ContainSubstring("new-password"))
		Expect(revision).To(Equal(transCtx.shardingComps[sharding][0].SystemAccounts[0].SecretRefRevision))
	})

	It("preserves an empty password when the shared source secret rotates to empty", func() {
		transformer, transCtx, graphCli, dag, shardingSpec, managed :=
			newSourceSecretReconcile([]byte{}, []byte("old-password"), false)

		Expect(transformer.reconcileShardingAccount(transCtx, graphCli, dag, shardingSpec, accountName)).To(Succeed())
		vertex := graphCli.FindMatchedVertex(dag, managed).(*model.ObjectVertex)
		updated := vertex.Obj.(*corev1.Secret)
		Expect(updated.Data).To(HaveKey(constant.AccountPasswdForSecret))
		Expect(updated.Data[constant.AccountPasswdForSecret]).To(BeEmpty())
	})

	It("rejects an overlong rotated shared source password without updating the managed secret", func() {
		transformer, transCtx, graphCli, dag, shardingSpec, managed := newSourceSecretReconcile(
			[]byte("12345678901234567890123456789012345678901234567890123456789012345"),
			[]byte("old-password"), false)

		err := transformer.reconcileShardingAccount(transCtx, graphCli, dag, shardingSpec, accountName)
		Expect(err).To(MatchError("password length exceeds 64 bytes"))
		Expect(graphCli.FindMatchedVertex(dag, managed)).To(BeNil())
	})

	It("deletes a stale immutable shared account secret and requests recreation", func() {
		transformer, transCtx, graphCli, dag, shardingSpec, managed :=
			newSourceSecretReconcile([]byte("new-password"), []byte("old-password"), true)

		err := transformer.reconcileShardingAccount(transCtx, graphCli, dag, shardingSpec, accountName)
		Expect(intctrlutil.IsRequeueError(err)).To(BeTrue())
		Expect(intctrlutil.IsDelayedRequeueError(err)).To(BeFalse())
		Expect(graphCli.IsAction(dag, managed, model.ActionDeletePtr())).To(BeTrue())
	})
})
