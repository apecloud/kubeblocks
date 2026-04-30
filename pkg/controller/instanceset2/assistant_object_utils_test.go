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

package instanceset2

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

func TestObjectReferenceToObject(t *testing.T) {
	tests := []struct {
		name    string
		ref     corev1.ObjectReference
		wantNs  string
		wantNm  string
		wantErr bool
	}{
		{
			name:   "ConfigMap",
			ref:    corev1.ObjectReference{Kind: "ConfigMap", Namespace: "ns", Name: "cm1"},
			wantNs: "ns",
			wantNm: "cm1",
		},
		{
			name:   "Secret",
			ref:    corev1.ObjectReference{Kind: "Secret", Namespace: "ns", Name: "sec1"},
			wantNs: "ns",
			wantNm: "sec1",
		},
		{
			name:   "ServiceAccount",
			ref:    corev1.ObjectReference{Kind: "ServiceAccount", Namespace: "ns", Name: "sa1"},
			wantNs: "ns",
			wantNm: "sa1",
		},
		{
			name:   "Service",
			ref:    corev1.ObjectReference{Kind: "Service", Namespace: "ns", Name: "svc1"},
			wantNs: "ns",
			wantNm: "svc1",
		},
		{
			name:   "Role",
			ref:    corev1.ObjectReference{Kind: "Role", Namespace: "ns", Name: "role1"},
			wantNs: "ns",
			wantNm: "role1",
		},
		{
			name:   "RoleBinding",
			ref:    corev1.ObjectReference{Kind: "RoleBinding", Namespace: "ns", Name: "rb1"},
			wantNs: "ns",
			wantNm: "rb1",
		},
		{
			name:    "unknown kind errors",
			ref:     corev1.ObjectReference{Kind: "Deployment", Namespace: "ns", Name: "d1"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, err := objectReferenceToObject(tt.ref)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantNs, obj.GetNamespace())
			assert.Equal(t, tt.wantNm, obj.GetName())
		})
	}
}

func TestInstanceAssistantObject_Service(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns", Name: "svc1",
			Labels:      map[string]string{"l": "v"},
			Annotations: map[string]string{"a": "v"},
		},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: "10.0.0.1",
		},
	}
	result := instanceAssistantObject(svc)
	require.NotNil(t, result.Service)
	assert.Equal(t, "svc1", result.Service.Name)
	assert.Equal(t, corev1.ServiceTypeClusterIP, result.Service.Spec.Type)
	assert.Nil(t, result.ConfigMap)
	assert.Nil(t, result.Secret)
}

func TestInstanceAssistantObject_ConfigMap(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cm1"},
		Data:       map[string]string{"key": "val"},
	}
	result := instanceAssistantObject(cm)
	require.NotNil(t, result.ConfigMap)
	assert.Equal(t, "cm1", result.ConfigMap.Name)
	assert.Equal(t, "val", result.ConfigMap.Data["key"])
}

func TestInstanceAssistantObject_Secret(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "sec1"},
		Data:       map[string][]byte{"key": []byte("val")},
	}
	result := instanceAssistantObject(secret)
	require.NotNil(t, result.Secret)
	assert.Equal(t, "sec1", result.Secret.Name)
}

func TestInstanceAssistantObject_ServiceAccount(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "sa1"},
	}
	result := instanceAssistantObject(sa)
	require.NotNil(t, result.ServiceAccount)
	assert.Equal(t, "sa1", result.ServiceAccount.Name)
}

func TestInstanceAssistantObject_Role(t *testing.T) {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "role1"},
		Rules: []rbacv1.PolicyRule{
			{Verbs: []string{"get"}, Resources: []string{"pods"}},
		},
	}
	result := instanceAssistantObject(role)
	require.NotNil(t, result.Role)
	assert.Equal(t, "role1", result.Role.Name)
	assert.Len(t, result.Role.Rules, 1)
}

func TestInstanceAssistantObject_RoleBinding(t *testing.T) {
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "rb1"},
		Subjects: []rbacv1.Subject{
			{Kind: "ServiceAccount", Name: "sa1"},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "role1",
		},
	}
	result := instanceAssistantObject(rb)
	require.NotNil(t, result.RoleBinding)
	assert.Equal(t, "rb1", result.RoleBinding.Name)
	assert.Len(t, result.RoleBinding.Subjects, 1)
	assert.Equal(t, "role1", result.RoleBinding.RoleRef.Name)
}

func TestObjectKind(t *testing.T) {
	assert.Equal(t, "ConfigMap", objectKind(&corev1.ConfigMap{}))
	assert.Equal(t, "Secret", objectKind(&corev1.Secret{}))
	assert.Equal(t, "Service", objectKind(&corev1.Service{}))
	assert.Equal(t, "ServiceAccount", objectKind(&corev1.ServiceAccount{}))
	assert.Equal(t, "Role", objectKind(&rbacv1.Role{}))
	assert.Equal(t, "RoleBinding", objectKind(&rbacv1.RoleBinding{}))
}

func TestShouldCloneInstanceAssistantObjects(t *testing.T) {
	t.Run("no multicluster annotation", func(t *testing.T) {
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
		}
		assert.False(t, shouldCloneInstanceAssistantObjects(its))
	})

	t.Run("with multicluster annotation", func(t *testing.T) {
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
				Annotations: map[string]string{
					constant.KBAppMultiClusterPlacementKey: "cluster-a,cluster-b",
				},
			},
		}
		assert.True(t, shouldCloneInstanceAssistantObjects(its))
	})
}

func TestCloneInstanceAssistantObjects(t *testing.T) {
	t.Run("empty assistant objects", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
		}
		tree.SetRoot(its)

		objs, err := cloneInstanceAssistantObjects(tree, its)
		require.NoError(t, err)
		assert.Empty(t, objs)
	})

	t.Run("clone from tree", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns"},
			Spec: workloads.InstanceSetSpec{
				InstanceAssistantObjects: []corev1.ObjectReference{
					{Kind: "ConfigMap", Namespace: "ns", Name: "cm1"},
				},
			},
		}
		tree.SetRoot(its)

		// Add the configmap to the tree
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cm1"},
			Data:       map[string]string{"key": "val"},
		}
		require.NoError(t, tree.Add(cm))

		objs, err := cloneInstanceAssistantObjects(tree, its)
		require.NoError(t, err)
		assert.Len(t, objs, 1)
		assert.NotNil(t, objs[0].ConfigMap)
	})
}

func TestCloneInstanceAssistantObject(t *testing.T) {
	t.Run("object found in tree", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		tree.SetRoot(&workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
		})
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "sec1"},
		}
		require.NoError(t, tree.Add(secret))

		ref := corev1.ObjectReference{Kind: "Secret", Namespace: "ns", Name: "sec1"}
		obj, err := cloneInstanceAssistantObject(tree, ref)
		require.NoError(t, err)
		assert.NotNil(t, obj)
		assert.Equal(t, "sec1", obj.GetName())
	})

	t.Run("object not in tree returns nil", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		tree.SetRoot(&workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
		})

		ref := corev1.ObjectReference{Kind: "ConfigMap", Namespace: "ns", Name: "missing"}
		obj, err := cloneInstanceAssistantObject(tree, ref)
		require.NoError(t, err)
		assert.Nil(t, obj)
	})
}

func newFakeReader(t *testing.T, objs ...client.Object) client.Reader {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, rbacv1.SchemeBuilder.AddToScheme(scheme))
	require.NoError(t, workloads.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func TestLoadInstanceAssistantObject(t *testing.T) {
	t.Run("object exists", func(t *testing.T) {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cm1"},
			Data:       map[string]string{"key": "val"},
		}
		reader := newFakeReader(t, cm)
		ref := corev1.ObjectReference{Kind: "ConfigMap", Namespace: "ns", Name: "cm1"}

		obj, err := loadInstanceAssistantObject(context.Background(), reader, ref)
		require.NoError(t, err)
		require.NotNil(t, obj)
		assert.Equal(t, "cm1", obj.GetName())
	})

	t.Run("object not found returns nil", func(t *testing.T) {
		reader := newFakeReader(t)
		ref := corev1.ObjectReference{Kind: "Secret", Namespace: "ns", Name: "missing"}

		obj, err := loadInstanceAssistantObject(context.Background(), reader, ref)
		require.NoError(t, err)
		assert.Nil(t, obj)
	})

	t.Run("unknown kind returns error", func(t *testing.T) {
		reader := newFakeReader(t)
		ref := corev1.ObjectReference{Kind: "Deployment", Namespace: "ns", Name: "dep1"}

		_, err := loadInstanceAssistantObject(context.Background(), reader, ref)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown assistant object")
	})
}

func TestLoadInstanceAssistantObjects(t *testing.T) {
	t.Run("nil root returns nil", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		reader := newFakeReader(t)
		err := loadInstanceAssistantObjects(context.Background(), reader, tree)
		assert.NoError(t, err)
	})

	t.Run("deleting root returns nil", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		now := metav1.Now()
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test", DeletionTimestamp: &now, Finalizers: []string{"test"},
			},
		}
		tree.SetRoot(its)
		reader := newFakeReader(t)
		err := loadInstanceAssistantObjects(context.Background(), reader, tree)
		assert.NoError(t, err)
	})

	t.Run("non-multicluster returns nil without loading", func(t *testing.T) {
		tree := kubebuilderx.NewObjectTree()
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns"},
			Spec: workloads.InstanceSetSpec{
				InstanceAssistantObjects: []corev1.ObjectReference{
					{Kind: "ConfigMap", Namespace: "ns", Name: "cm1"},
				},
			},
		}
		tree.SetRoot(its)
		reader := newFakeReader(t)
		err := loadInstanceAssistantObjects(context.Background(), reader, tree)
		assert.NoError(t, err)
		// No objects should have been added to tree
		assert.Empty(t, tree.List(&corev1.ConfigMap{}))
	})

	t.Run("multicluster loads assistant objects into tree", func(t *testing.T) {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cm1"},
			Data:       map[string]string{"key": "val"},
		}
		reader := newFakeReader(t, cm)

		tree := kubebuilderx.NewObjectTree()
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "ns",
				Annotations: map[string]string{
					constant.KBAppMultiClusterPlacementKey: "cluster-a",
				},
			},
			Spec: workloads.InstanceSetSpec{
				InstanceAssistantObjects: []corev1.ObjectReference{
					{Kind: "ConfigMap", Namespace: "ns", Name: "cm1"},
				},
			},
		}
		tree.SetRoot(its)

		err := loadInstanceAssistantObjects(context.Background(), reader, tree)
		require.NoError(t, err)
		cmList := tree.List(&corev1.ConfigMap{})
		assert.Len(t, cmList, 1)
	})

	t.Run("multicluster object not found is skipped", func(t *testing.T) {
		reader := newFakeReader(t) // no objects exist

		tree := kubebuilderx.NewObjectTree()
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "ns",
				Annotations: map[string]string{
					constant.KBAppMultiClusterPlacementKey: "cluster-a",
				},
			},
			Spec: workloads.InstanceSetSpec{
				InstanceAssistantObjects: []corev1.ObjectReference{
					{Kind: "ConfigMap", Namespace: "ns", Name: "missing"},
				},
			},
		}
		tree.SetRoot(its)

		err := loadInstanceAssistantObjects(context.Background(), reader, tree)
		require.NoError(t, err)
		assert.Empty(t, tree.List(&corev1.ConfigMap{}))
	})

	t.Run("multicluster unknown kind returns error", func(t *testing.T) {
		reader := newFakeReader(t)

		tree := kubebuilderx.NewObjectTree()
		its := &workloads.InstanceSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "ns",
				Annotations: map[string]string{
					constant.KBAppMultiClusterPlacementKey: "cluster-a",
				},
			},
			Spec: workloads.InstanceSetSpec{
				InstanceAssistantObjects: []corev1.ObjectReference{
					{Kind: "Deployment", Namespace: "ns", Name: "dep1"},
				},
			},
		}
		tree.SetRoot(its)

		err := loadInstanceAssistantObjects(context.Background(), reader, tree)
		assert.Error(t, err)
	})
}

