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

package component

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	compctrl "github.com/apecloud/kubeblocks/pkg/controller/component"
)

var _ = Describe("component account transformer", func() {
	It("copies the system account provisioned annotation from secretRef source", func() {
		const (
			namespace   = "default"
			clusterName = "cluster"
			compName    = "comp"
			accountName = "admin"
			sourceName  = "source-account"
			passwordKey = "admin-password"
		)

		sourceSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      sourceName,
				Annotations: map[string]string{
					constant.SystemAccountProvisionedAnnotationKey: "true",
				},
			},
			Data: map[string][]byte{
				passwordKey: []byte("source-password"),
			},
		}
		transCtx := &componentTransformContext{
			Context: context.Background(),
			Client: &appsutil.MockReader{
				Objects: []client.Object{sourceSecret},
			},
			Component: &appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      compName,
					UID:       types.UID("component-uid"),
				},
			},
			SynthesizeComponent: &compctrl.SynthesizedComponent{
				Namespace:   namespace,
				ClusterName: clusterName,
				Name:        compName,
			},
		}

		secret, err := (&componentAccountTransformer{}).buildAccountSecret(transCtx, synthesizedSystemAccount{
			SystemAccount: appsv1.SystemAccount{
				Name: accountName,
			},
			SecretRef: &appsv1.ProvisionSecretRef{
				Name:      sourceName,
				Namespace: namespace,
				Password:  passwordKey,
			},
		})

		Expect(err).ShouldNot(HaveOccurred())
		Expect(secret.Name).Should(Equal(constant.GenerateAccountSecretName(clusterName, compName, accountName)))
		Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountNameForSecret, []byte(accountName)))
		Expect(secret.Data).Should(HaveKeyWithValue(constant.AccountPasswdForSecret, []byte("source-password")))
		Expect(secret.Annotations).Should(HaveKeyWithValue(constant.SystemAccountProvisionedAnnotationKey, "true"))
	})
})
