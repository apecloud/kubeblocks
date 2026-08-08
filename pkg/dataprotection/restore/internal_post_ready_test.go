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

package restore

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func validInternalPostReadyRestore(component *appsv1.Component) *dpv1alpha1.Restore {
	return &dpv1alpha1.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: component.Namespace,
			Name:      PostReadyRestoreName(component.UID),
			Labels: map[string]string{
				DataProtectionInternalPostReadyLabelKey: DataProtectionInternalPostReadyLabelValue,
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: appsv1.APIVersion,
				Kind:       appsv1.ComponentKind,
				Name:       component.Name,
				UID:        component.UID,
				Controller: ptr.To(true),
			}},
		},
	}
}

func TestInternalPostReadyRestoreComponentOwnerClassification(t *testing.T) {
	component := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "cluster-mysql",
			UID:       types.UID("component-uid"),
		},
	}

	t.Run("ordinary external restore", func(t *testing.T) {
		ref, internal, err := InternalPostReadyRestoreComponentOwner(&dpv1alpha1.Restore{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "user-restore"},
		})
		require.NoError(t, err)
		require.False(t, internal)
		require.Nil(t, ref)
	})

	t.Run("valid internal restore", func(t *testing.T) {
		ref, internal, err := InternalPostReadyRestoreComponentOwner(validInternalPostReadyRestore(component))
		require.NoError(t, err)
		require.True(t, internal)
		require.NotNil(t, ref)
		require.Equal(t, component.Name, ref.Name)
		require.Equal(t, component.UID, ref.UID)
	})

	t.Run("marker on non-deterministic name", func(t *testing.T) {
		restore := validInternalPostReadyRestore(component)
		restore.Name = "user-restore"

		ref, internal, err := InternalPostReadyRestoreComponentOwner(restore)
		require.Error(t, err)
		require.True(t, internal)
		require.True(t, intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal), err.Error())
		require.Nil(t, ref)
	})
}

func TestValidateInternalPostReadyRestoreComponentRejectsExpectedComponentMismatch(t *testing.T) {
	component := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "cluster-mysql",
			UID:       types.UID("component-uid"),
		},
	}
	tests := []struct {
		name   string
		mutate func(*appsv1.Component)
	}{
		{
			name: "namespace",
			mutate: func(actual *appsv1.Component) {
				actual.Namespace = "other"
			},
		},
		{
			name: "name",
			mutate: func(actual *appsv1.Component) {
				actual.Name = "other"
			},
		},
		{
			name: "uid",
			mutate: func(actual *appsv1.Component) {
				actual.UID = "other"
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := component.DeepCopy()
			tt.mutate(actual)

			err := ValidateInternalPostReadyRestoreComponent(
				validInternalPostReadyRestore(component),
				actual,
			)

			require.Error(t, err)
			require.True(t, intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal), err.Error())
		})
	}
}

func TestValidateInternalPostReadyRestoreComponentRejectsNilInputs(t *testing.T) {
	component := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "cluster-mysql",
			UID:       types.UID("component-uid"),
		},
	}

	err := ValidateInternalPostReadyRestoreComponent(nil, component)
	require.Error(t, err)
	require.True(t, intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal), err.Error())

	err = ValidateInternalPostReadyRestoreComponent(validInternalPostReadyRestore(component), nil)
	require.Error(t, err)
	require.True(t, intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal), err.Error())
}
