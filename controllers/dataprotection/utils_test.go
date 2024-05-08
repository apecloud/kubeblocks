/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("test EnsureWorkerServiceAccount", func() {
	cleanEnv := func() {
		By("clean resources")

		// delete rest mocked objects
		inNS := client.InNamespace(testCtx.DefaultNamespace)
		ml := client.HasLabels{testCtx.TestObjLabelKey}

		serviceAccountSignature := func(_ corev1.ServiceAccount, _ *corev1.ServiceAccount, _ corev1.ServiceAccountList, _ *corev1.ServiceAccountList) {
		}
		roleBindingSignature := func(_ rbacv1.RoleBinding, _ *rbacv1.RoleBinding, _ rbacv1.RoleBindingList, _ *rbacv1.RoleBindingList) {
		}

		// namespaced
		testapps.ClearResources(&testCtx, serviceAccountSignature, inNS, ml)
		testapps.ClearResources(&testCtx, roleBindingSignature, inNS, ml)
	}

	const (
		defaultWorkerServiceAccountName        = "sa-name"
		defaultExecWorkerServiceAccountName    = "exec-sa-name"
		defaultWorkerServiceAccountAnnotations = `{"role-arn": "arn:xxx:xxx"}`
		defaultWorkerClusterRoleName           = "worker-role"
	)

	var (
		saKey types.NamespacedName
		rbKey types.NamespacedName
	)

	BeforeEach(func() {
		cleanEnv()
		viper.SetDefault(dptypes.CfgKeyWorkerServiceAccountName, defaultWorkerServiceAccountName)
		viper.SetDefault(dptypes.CfgKeyExecWorkerServiceAccountName, defaultExecWorkerServiceAccountName)
		viper.SetDefault(dptypes.CfgKeyWorkerServiceAccountAnnotations, defaultWorkerServiceAccountAnnotations)
		viper.SetDefault(dptypes.CfgKeyWorkerClusterRoleName, defaultWorkerClusterRoleName)

		saKey = types.NamespacedName{
			Name:      defaultWorkerServiceAccountName,
			Namespace: testCtx.DefaultNamespace,
		}
		rbKey = types.NamespacedName{
			Name:      defaultWorkerServiceAccountName + "-rolebinding",
			Namespace: testCtx.DefaultNamespace,
		}
	})

	AfterEach(func() {
		cleanEnv()
	})

	It("should create a service account if not exists", func() {
		Eventually(testapps.CheckObjExists(&testCtx, saKey, &corev1.ServiceAccount{}, false)).Should(Succeed())
		Eventually(testapps.CheckObjExists(&testCtx, rbKey, &rbacv1.RoleBinding{}, false)).Should(Succeed())

		reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
		saName, err := EnsureWorkerServiceAccount(reqCtx, testCtx.Cli, testCtx.DefaultNamespace, nil)
		Expect(err).To(BeNil())
		Expect(saName).To(Equal(defaultWorkerServiceAccountName))

		// become available
		Eventually(testapps.CheckObjExists(&testCtx, saKey, &corev1.ServiceAccount{}, true)).Should(Succeed())
		Eventually(testapps.CheckObjExists(&testCtx, rbKey, &rbacv1.RoleBinding{}, true)).Should(Succeed())

		Eventually(testapps.CheckObj(&testCtx, saKey, func(g Gomega, sa *corev1.ServiceAccount) {
			g.Expect(sa.Annotations).To(Equal(map[string]string{
				"role-arn": "arn:xxx:xxx",
			}))
		})).Should(Succeed())

		Eventually(testapps.CheckObj(&testCtx, rbKey, func(g Gomega, rb *rbacv1.RoleBinding) {
			g.Expect(rb.Subjects).To(HaveLen(1))
			g.Expect(rb.Subjects[0]).To(Equal(rbacv1.Subject{
				Kind:      "ServiceAccount",
				APIGroup:  "",
				Name:      defaultWorkerServiceAccountName,
				Namespace: testCtx.DefaultNamespace,
			}))
			g.Expect(rb.RoleRef).To(Equal(rbacv1.RoleRef{
				Kind:     "ClusterRole",
				APIGroup: "rbac.authorization.k8s.io",
				Name:     defaultWorkerClusterRoleName,
			}))
		})).Should(Succeed())
	})

	It("should update ServiceAccount's annotation if it's changed", func() {
		reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
		saName, err := EnsureWorkerServiceAccount(reqCtx, testCtx.Cli, testCtx.DefaultNamespace, nil)
		Expect(err).To(BeNil())
		Expect(saName).To(Equal(defaultWorkerServiceAccountName))

		By("updating annotation env")
		viper.SetDefault(dptypes.CfgKeyWorkerServiceAccountAnnotations, `{"role-arn":"arn:changed","newfield":"newvalue"}`)
		By("calling EnsureWorkerServiceAccount() again")
		saName, err = EnsureWorkerServiceAccount(reqCtx, testCtx.Cli, testCtx.DefaultNamespace, nil)
		Expect(err).To(BeNil())
		Expect(saName).To(Equal(defaultWorkerServiceAccountName))

		Eventually(testapps.CheckObj(&testCtx, saKey, func(g Gomega, sa *corev1.ServiceAccount) {
			g.Expect(sa.Annotations).To(Equal(map[string]string{
				"role-arn": "arn:changed",
				"newfield": "newvalue",
			}))
		})).Should(Succeed())
	})

	Context("testing invalid argument", func() {
		updateDefaultEnv := func(key string, value string) func() {
			old := viper.GetString(key)
			viper.SetDefault(key, value)
			return func() {
				viper.SetDefault(key, old)
			}
		}

		It("should return error if namespace is empty", func() {
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			_, err := EnsureWorkerServiceAccount(reqCtx, testCtx.Cli, "", nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("namespace is empty"))
		})

		It("should return error if CfgKeyWorkerServiceAccountName is empty", func() {
			defer updateDefaultEnv(dptypes.CfgKeyWorkerServiceAccountName, "")()
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			_, err := EnsureWorkerServiceAccount(reqCtx, testCtx.Cli, testCtx.DefaultNamespace, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("worker service account name is empty"))
		})

		It("should return error if CfgKeyWorkerServiceAccountAnnotations is invalid json", func() {
			defer updateDefaultEnv(dptypes.CfgKeyWorkerServiceAccountAnnotations, `{"bad": "json`)()
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			_, err := EnsureWorkerServiceAccount(reqCtx, testCtx.Cli, testCtx.DefaultNamespace, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to unmarshal worker service account annotations"))
		})

		It("should return error if CfgKeyWorkerClusterRoleName is empty", func() {
			defer updateDefaultEnv(dptypes.CfgKeyWorkerClusterRoleName, "")()
			reqCtx := intctrlutil.RequestCtx{Ctx: testCtx.Ctx}
			_, err := EnsureWorkerServiceAccount(reqCtx, testCtx.Cli, testCtx.DefaultNamespace, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("worker cluster role name is empty"))
		})
	})
})
