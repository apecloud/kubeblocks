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

package systemaccount

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestReadLegacyRestoreOperationStateUsesExactInstanceParentChain(t *testing.T) {
	fixture := newLegacyOperationFixture(t)

	state, err := ReadRestoreOperationState(context.Background(), fixture.reader(), fixture.operation)

	require.NoError(t, err)
	require.Equal(t, RestoreOperationActive, state)
}

func TestReadLegacyRestoreOperationStateExcludesTerminatingParticipant(t *testing.T) {
	fixture := newLegacyOperationFixture(t)
	now := metav1.Now()
	fixture.pvc.DeletionTimestamp = &now
	fixture.pvc.Finalizers = []string{"test.kubeblocks.io/hold"}

	state, err := ReadRestoreOperationState(context.Background(), fixture.reader(), fixture.operation)

	require.NoError(t, err)
	require.Equal(t, RestoreOperationGone, state)
}

func TestReadLegacyRestoreOperationStatePropagatesAuthorityReadError(t *testing.T) {
	fixture := newLegacyOperationFixture(t)
	expected := errors.New("strong read unavailable")
	reader := fake.NewClientBuilder().
		WithScheme(fixture.scheme).
		WithObjects(fixture.objects()...).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, cli client.WithWatch, key client.ObjectKey,
				obj client.Object, opts ...client.GetOption) error {
				if _, ok := obj.(*workloads.InstanceSet); ok {
					return expected
				}
				return cli.Get(ctx, key, obj, opts...)
			},
		}).
		Build()

	state, err := ReadRestoreOperationState(context.Background(), reader, fixture.operation)

	require.ErrorIs(t, err, expected)
	require.Empty(t, state)
}

type legacyOperationFixture struct {
	scheme      *runtime.Scheme
	cluster     *appsv1.Cluster
	component   *appsv1.Component
	instanceSet *workloads.InstanceSet
	instance    *workloads.Instance
	pvc         *corev1.PersistentVolumeClaim
	operation   RestoreOperationIdentity
}

func newLegacyOperationFixture(t *testing.T) *legacyOperationFixture {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, workloads.AddToScheme(scheme))

	cluster := &appsv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.GroupVersion.String(),
			Kind:       appsv1.ClusterKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "cluster",
			UID:       types.UID("cluster-uid"),
		},
	}
	component := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "cluster-mysql",
		UID:       types.UID("component-uid"),
	}}
	instanceSet := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "cluster-mysql",
		UID:       types.UID("instanceset-uid"),
	}}
	instance := &workloads.Instance{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "cluster-mysql-0",
		UID:       types.UID("instance-uid"),
	}}
	apiGroup := "dataprotection.kubeblocks.io"
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "data-cluster-mysql-0",
			UID:       types.UID("pvc-uid"),
			Labels: map[string]string{
				constant.AppInstanceLabelKey: cluster.Name,
			},
			Annotations: map[string]string{
				constant.RestorePITRAnnotationKey: "2026-07-23T00:00:00Z",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			DataSourceRef: &corev1.TypedObjectReference{
				APIGroup:  &apiGroup,
				Kind:      "Backup",
				Name:      "backup",
				Namespace: ptrTo("default"),
			},
		},
	}
	require.NoError(t, controllerutil.SetControllerReference(cluster, component, scheme))
	require.NoError(t, controllerutil.SetControllerReference(component, instanceSet, scheme))
	require.NoError(t, controllerutil.SetControllerReference(instanceSet, instance, scheme))
	require.NoError(t, controllerutil.SetControllerReference(instance, pvc, scheme))

	return &legacyOperationFixture{
		scheme:      scheme,
		cluster:     cluster,
		component:   component,
		instanceSet: instanceSet,
		instance:    instance,
		pvc:         pvc,
		operation: RestoreOperationIdentity{
			Protocol: RestoreProtocolV2,
			Profile:  RestoreProfileLegacyPVCGroup,
			Root: ObjectIdentity{
				APIVersion: appsv1.GroupVersion.String(),
				Kind:       appsv1.ClusterKind,
				Namespace:  cluster.Namespace,
				Name:       cluster.Name,
				UID:        cluster.UID,
			},
			Source: SourceIdentity{
				APIGroup:  apiGroup,
				Kind:      "Backup",
				Namespace: "default",
				Name:      "backup",
			},
			PITR: "2026-07-23T00:00:00Z",
		},
	}
}

func (f *legacyOperationFixture) objects() []client.Object {
	return []client.Object{f.cluster, f.component, f.instanceSet, f.instance, f.pvc}
}

func (f *legacyOperationFixture) reader() client.Reader {
	return fake.NewClientBuilder().WithScheme(f.scheme).WithObjects(f.objects()...).Build()
}

func ptrTo[T any](value T) *T {
	return &value
}
