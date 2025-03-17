/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package util

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestReflect(t *testing.T) {
	var list client.ObjectList
	sts := appsv1.StatefulSet{}
	sts.SetName("hello")
	list = &appsv1.StatefulSetList{Items: []appsv1.StatefulSet{sts}}
	v := reflect.ValueOf(list).Elem().FieldByName("Items")
	if v.Kind() != reflect.Slice {
		t.Error("not slice")
	}
	c := v.Len()
	objects := make([]client.Object, c)
	for i := 0; i < c; i++ {
		var st = v.Index(i).Addr().Interface()
		objects[i] = st.(client.Object)
	}
	for _, e := range objects {
		fmt.Println(e)
	}

	var o client.Object = &sts
	ptr := reflect.ValueOf(o)
	v = ptr.Elem().FieldByName("Spec")
	fmt.Println(v)
}

func TestIsOwnedByInstanceSet(t *testing.T) {
	its := &workloads.InstanceSet{}
	assert.False(t, IsOwnedByInstanceSet(its))

	its.OwnerReferences = []metav1.OwnerReference{
		{
			Kind:       workloads.InstanceSetKind,
			Controller: pointer.Bool(true),
		},
	}
	assert.True(t, IsOwnedByInstanceSet(its))

	its.OwnerReferences = []metav1.OwnerReference{
		{
			Kind:       reflect.TypeOf(kbappsv1.Cluster{}).Name(),
			Controller: pointer.Bool(true),
		},
	}
	assert.False(t, IsOwnedByInstanceSet(its))
}

func TestGetRestoreSystemAccountPassword(t *testing.T) {
	encryptor := intctrlutil.NewEncryptor(viper.GetString(constant.CfgKeyDPEncryptionKey))
	encryptedPwd, _ := encryptor.Encrypt([]byte("test-password"))

	tests := []struct {
		name        string
		annotations map[string]string
		backup      *dpv1alpha1.Backup
		component   string
		account     string
		want        string
	}{
		{
			name: "normal case",
			annotations: map[string]string{
				constant.RestoreFromBackupAnnotationKey: `{"comp1":{"name":"backup1","namespace":"default"}}`,
			},
			backup: &dpv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "backup1",
					Namespace: "default",
					Annotations: map[string]string{
						constant.EncryptedSystemAccountsAnnotationKey: fmt.Sprintf(`{"comp1":{"account1":"%s"}}`, encryptedPwd),
					},
				},
			},
			component: "comp1",
			account:   "account1",
			want:      "test-password",
		},
		{
			name: "empty restore annotation",
			annotations: map[string]string{
				constant.RestoreFromBackupAnnotationKey: "",
			},
			backup:    &dpv1alpha1.Backup{},
			component: "comp1",
			account:   "account1",
			want:      "",
		},
		{
			name: "invalid restore annotation json",
			annotations: map[string]string{
				constant.RestoreFromBackupAnnotationKey: "invalid-json",
			},
			backup:    &dpv1alpha1.Backup{},
			component: "comp1",
			account:   "account1",
			want:      "",
		},
		{
			name: "component not found in restore annotation",
			annotations: map[string]string{
				constant.RestoreFromBackupAnnotationKey: `{"other-comp":{"name":"backup1","namespace":"default"}}`,
			},
			backup:    &dpv1alpha1.Backup{},
			component: "comp1",
			account:   "account1",
			want:      "",
		},
		{
			name: "backup not found",
			annotations: map[string]string{
				constant.RestoreFromBackupAnnotationKey: `{"comp1":{"name":"non-existent","namespace":"default"}}`,
			},
			backup:    nil,
			component: "comp1",
			account:   "account1",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = dpv1alpha1.AddToScheme(scheme)
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects()
			if tt.backup != nil {
				cli = cli.WithObjects(tt.backup)
			}
			got := GetRestoreSystemAccountPassword(context.Background(), cli.Build(),
				tt.annotations, tt.component, tt.account)
			assert.Equal(t, tt.want, got)
		})
	}
}
