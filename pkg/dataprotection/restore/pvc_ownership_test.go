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

package restore

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type pvcCreateRaceClient struct {
	client.Client
	patches, updates int
}

func (c *pvcCreateRaceClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	existing := obj.DeepCopyObject().(client.Object)
	existing.SetAnnotations(nil)
	if err := c.Client.Create(ctx, existing, opts...); err != nil {
		return err
	}
	return apierrors.NewAlreadyExists(schema.GroupResource{Resource: "persistentvolumeclaims"}, obj.GetName())
}
func (c *pvcCreateRaceClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	c.patches++
	return c.Client.Patch(ctx, obj, patch, opts...)
}
func (c *pvcCreateRaceClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	c.updates++
	return c.Client.Update(ctx, obj, opts...)
}

func TestRestorePVCMarkerDoesNotClaimExistingData(t *testing.T) {
	for _, mode := range []string{"new", "existing", "already exists race"} {
		t.Run(mode, func(t *testing.T) {
			scheme := runtime.NewScheme()
			if err := corev1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			metadata := metav1.ObjectMeta{Name: "data-target-db-0", Namespace: "default",
				Annotations: map[string]string{constant.RestorePVCComponentUIDAnnotationKey: "component-uid"}}
			var objects []client.Object
			if mode == "existing" {
				objects = append(objects, &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: metadata.Name, Namespace: metadata.Namespace}})
			}
			baseClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
			var cli client.Client = baseClient
			var race *pvcCreateRaceClient
			if mode == "already exists race" {
				race = &pvcCreateRaceClient{Client: baseClient}
				cli = race
			}
			manager := &RestoreManager{Restore: &dpv1alpha1.Restore{ObjectMeta: metav1.ObjectMeta{Name: "restore", Namespace: metadata.Namespace}}, Recorder: record.NewFakeRecorder(8)}
			reqCtx := intctrlutil.RequestCtx{Ctx: context.Background(), Req: ctrl.Request{NamespacedName: client.ObjectKey{Namespace: metadata.Namespace}}}
			if err := manager.createPVCIfNotExist(reqCtx, cli, metadata, corev1.PersistentVolumeClaimSpec{}); err != nil {
				t.Fatal(err)
			}
			fetched := &corev1.PersistentVolumeClaim{}
			if err := cli.Get(context.Background(), client.ObjectKey{Name: metadata.Name, Namespace: metadata.Namespace}, fetched); err != nil {
				t.Fatal(err)
			}
			expected := ""
			if mode == "new" {
				expected = "component-uid"
			}
			if fetched.Annotations[constant.RestorePVCComponentUIDAnnotationKey] != expected {
				t.Fatalf("marker=%q, want %q", fetched.Annotations[constant.RestorePVCComponentUIDAnnotationKey], expected)
			}
			if len(fetched.OwnerReferences) != 0 {
				t.Fatal("PVC must remain available for InstanceSet adoption")
			}
			if race != nil && (race.patches != 0 || race.updates != 0) {
				t.Fatal("AlreadyExists must not update or patch another PVC")
			}
		})
	}
}
