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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stretchr/testify/require"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
)

type recordingAccountProvisionLifecycle struct {
	lifecycle.Lifecycle
	called    bool
	statement string
	username  string
	password  string
}

func (r *recordingAccountProvisionLifecycle) AccountProvision(_ context.Context, _ client.Reader,
	_ *lifecycle.Options, statement, username, password string) error {
	r.called = true
	r.statement = statement
	r.username = username
	r.password = password
	return nil
}

func TestProvisionPasswordlessAccount(t *testing.T) {
	transformer := &componentAccountProvisionTransformer{}
	transCtx := &componentTransformContext{Context: context.Background()}
	lfa := &recordingAccountProvisionLifecycle{}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "account"},
		Data: map[string][]byte{
			constant.AccountNameForSecret:   []byte("root"),
			constant.AccountPasswdForSecret: {},
		},
	}

	require.NoError(t, transformer.provision(transCtx, lfa, "ALTER USER", secret))
	require.True(t, lfa.called)
	require.Equal(t, "ALTER USER", lfa.statement)
	require.Equal(t, "root", lfa.username)
	require.Empty(t, lfa.password)
}

func TestProvisionRejectsMissingCredentialFields(t *testing.T) {
	transformer := &componentAccountProvisionTransformer{}
	transCtx := &componentTransformContext{Context: context.Background()}

	for name, data := range map[string]map[string][]byte{
		"missing username": {constant.AccountPasswdForSecret: {}},
		"missing password": {constant.AccountNameForSecret: []byte("root")},
	} {
		t.Run(name, func(t *testing.T) {
			lfa := &recordingAccountProvisionLifecycle{}
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "account"}, Data: data}
			require.Error(t, transformer.provision(transCtx, lfa, "ALTER USER", secret))
			require.False(t, lfa.called)
		})
	}
}
