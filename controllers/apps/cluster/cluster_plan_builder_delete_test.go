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

package cluster

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func TestClusterPlanDeleteKeepsReplacementWithDifferentUID(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	replacement := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "system-account",
		UID:       "replacement-uid",
	}}
	oldUID := types.UID("old-uid")
	var observedUID *types.UID
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(replacement).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(ctx context.Context, cli client.WithWatch, obj client.Object,
				opts ...client.DeleteOption) error {
				deleteOptions := &client.DeleteOptions{}
				deleteOptions.ApplyOptions(opts)
				if deleteOptions.Preconditions == nil || deleteOptions.Preconditions.UID == nil {
					return fmt.Errorf("delete has no UID precondition")
				}
				uid := *deleteOptions.Preconditions.UID
				observedUID = &uid
				current := &corev1.ConfigMap{}
				if err := cli.Get(ctx, client.ObjectKeyFromObject(obj), current); err != nil {
					return err
				}
				if current.UID != uid {
					return apierrors.NewConflict(schema.GroupResource{Resource: "configmaps"},
						obj.GetName(), fmt.Errorf("UID precondition %s does not match %s", uid, current.UID))
				}
				return cli.Delete(ctx, obj, opts...)
			},
		}).
		Build()
	builder := &clusterPlanBuilder{
		cli: cli,
		transCtx: &clusterTransformContext{
			Context: context.Background(),
			Logger:  logr.Discard(),
		},
	}
	stale := replacement.DeepCopy()
	stale.UID = oldUID
	err := builder.defaultWalkFunc(&model.ObjectVertex{
		Obj:                 stale,
		Action:              model.ActionDeletePtr(),
		DeletePreconditions: &metav1.Preconditions{UID: &oldUID},
	})
	if !apierrors.IsConflict(err) {
		t.Fatalf("expected UID conflict, got %v", err)
	}
	if observedUID == nil || *observedUID != oldUID {
		t.Fatalf("delete UID precondition = %v, want %s", observedUID, oldUID)
	}
	current := &corev1.ConfigMap{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(replacement), current); err != nil {
		t.Fatalf("replacement was deleted: %v", err)
	}
	if current.UID != replacement.UID {
		t.Fatalf("replacement UID = %s, want %s", current.UID, replacement.UID)
	}
}
