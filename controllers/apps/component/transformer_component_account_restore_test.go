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
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	pkgcomponent "github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestBuildAccountSecretPreservesEmptyRestorePassword(t *testing.T) {
	const (
		componentName = "comp"
		accountName   = "root"
	)
	encryptedPassword, err := intctrlutil.NewEncryptor(viper.GetString(constant.CfgKeyDPEncryptionKey)).Encrypt(nil)
	if err != nil {
		t.Fatalf("encrypt empty password: %v", err)
	}
	backup := &dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{
		Name:      "backup",
		Namespace: "default",
		Annotations: map[string]string{
			constant.EncryptedSystemAccountsAnnotationKey: fmt.Sprintf(`{"%s":{"%s":"%s"}}`, componentName, accountName, encryptedPassword),
		},
	}}
	scheme := runtime.NewScheme()
	if err := dpv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add backup scheme: %v", err)
	}
	transCtx := &componentTransformContext{
		Context: context.Background(),
		Client:  fake.NewClientBuilder().WithScheme(scheme).WithObjects(backup).Build(),
		Component: &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-comp",
			Namespace: "default",
		}},
		SynthesizeComponent: &pkgcomponent.SynthesizedComponent{
			Namespace:   "default",
			ClusterName: "cluster",
			Name:        componentName,
			Annotations: map[string]string{
				constant.RestoreFromBackupAnnotationKey: `{"comp":{"name":"backup","namespace":"default"}}`,
			},
		},
	}
	account := synthesizedSystemAccount{SystemAccount: appsv1.SystemAccount{
		Name:        accountName,
		InitAccount: true,
		PasswordConfig: &appsv1.PasswordConfig{
			Length: 16,
		},
	}}

	secret, err := (&componentAccountTransformer{}).buildAccountSecret(transCtx, account)
	if err != nil {
		t.Fatalf("build account secret: %v", err)
	}
	if password := secret.Data[constant.AccountPasswdForSecret]; len(password) != 0 {
		t.Fatalf("expected restored empty password, got %d bytes", len(password))
	}
}
