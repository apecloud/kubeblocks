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

package dataprotection

import (
	"context"
	"errors"
	"testing"
	"time"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	testclock "k8s.io/utils/clock/testing"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGCProtectsUnexpiredDependencyChain(t *testing.T) {
	now := time.Date(2026, 10, 25, 19, 0, 0, 0, time.UTC)
	backup := func(name, parent, base string, offset time.Duration) *dpv1alpha1.Backup {
		b := &dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "retention", Labels: map[string]string{dptypes.BackupPolicyLabelKey: "policy"}}}
		b.Spec.BackupPolicyName = "policy"
		b.Status.Phase = dpv1alpha1.BackupPhaseCompleted
		b.Status.ParentBackupName = parent
		b.Status.BaseBackupName = base
		expiry := metav1.NewTime(now.Add(offset))
		b.Status.Expiration = &expiry
		return b
	}
	cases := []struct {
		name     string
		children func() []*dpv1alpha1.Backup
		keep     bool
	}{
		{"unexpired incremental", func() []*dpv1alpha1.Backup { return []*dpv1alpha1.Backup{backup("child", "full", "full", time.Hour)} }, true},
		{"unexpired grandchild retains expired intermediate", func() []*dpv1alpha1.Backup {
			return []*dpv1alpha1.Backup{backup("child", "full", "", -time.Hour), backup("grandchild", "child", "", time.Hour)}
		}, true},
		{"base reference without parent", func() []*dpv1alpha1.Backup { return []*dpv1alpha1.Backup{backup("child", "", "full", time.Hour)} }, true},
		{"indefinitely retained child", func() []*dpv1alpha1.Backup {
			b := backup("child", "full", "", 0)
			b.Status.Expiration = nil
			return []*dpv1alpha1.Backup{b}
		}, true},
		{"pending explicit parent", func() []*dpv1alpha1.Backup {
			b := backup("child", "", "", 0)
			b.Spec.ParentBackupName = "full"
			b.Status.Expiration = nil
			b.Status.Phase = ""
			return []*dpv1alpha1.Backup{b}
		}, true},
		{"all expired", func() []*dpv1alpha1.Backup {
			return []*dpv1alpha1.Backup{backup("child", "full", "full", -time.Hour), backup("grandchild", "child", "full", -time.Minute)}
		}, false},
		{"expiry equal to now", func() []*dpv1alpha1.Backup { return []*dpv1alpha1.Backup{backup("child", "full", "full", 0)} }, false},
		{"unrelated future backup", func() []*dpv1alpha1.Backup {
			return []*dpv1alpha1.Backup{backup("other", "other-full", "other-full", time.Hour)}
		}, false},
		{"same name in another namespace", func() []*dpv1alpha1.Backup {
			b := backup("child", "full", "full", time.Hour)
			b.Namespace = "other"
			return []*dpv1alpha1.Backup{b}
		}, false},
		{"expired cycle terminates", func() []*dpv1alpha1.Backup {
			return []*dpv1alpha1.Backup{backup("child", "full", "grandchild", -time.Hour), backup("grandchild", "child", "full", -time.Hour)}
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			if err := dpv1alpha1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			root := backup("full", "", "", -2*time.Hour)
			objs := []client.Object{root}
			for _, b := range tc.children() {
				objs = append(objs, b)
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
			r := &GCReconciler{Client: cli, Recorder: record.NewFakeRecorder(10), clock: testclock.NewFakeClock(now)}
			key := client.ObjectKeyFromObject(root)
			if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: key}); err != nil {
				t.Fatal(err)
			}
			err := cli.Get(context.Background(), key, &dpv1alpha1.Backup{})
			if tc.keep && err != nil {
				t.Fatalf("dependency root must remain: %v", err)
			}
			if tc.name == "unexpired incremental" {
				r.clock.(*testclock.FakeClock).Step(2 * time.Hour)
				if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: key}); err != nil {
					t.Fatal(err)
				}
				if err := cli.Get(context.Background(), key, &dpv1alpha1.Backup{}); !apierrors.IsNotFound(err) {
					t.Fatalf("root must become collectible once child expires: %v", err)
				}
			}

			if !tc.keep && !apierrors.IsNotFound(err) {
				t.Fatalf("expired unneeded root should be deleted, got %v", err)
			}
		})
	}
}

type failingGCClient struct {
	client.Client
	getErr, listErr error
}

func (c failingGCClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if c.getErr != nil {
		return c.getErr
	}
	return c.Client.Get(ctx, key, obj, opts...)
}
func (c failingGCClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if c.listErr != nil {
		return c.listErr
	}
	return c.Client.List(ctx, list, opts...)
}
func TestGCReadErrorsNeverDelete(t *testing.T) {
	for _, stage := range []string{"get", "list"} {
		t.Run(stage, func(t *testing.T) {
			now := time.Now()
			expiry := metav1.NewTime(now.Add(-time.Hour))
			scheme := runtime.NewScheme()
			if err := dpv1alpha1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			b := &dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "full", Namespace: "retention"}}
			b.Status.Expiration = &expiry
			b.Status.Phase = dpv1alpha1.BackupPhaseCompleted
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(b).Build()
			broken := failingGCClient{Client: cli}
			want := errors.New("temporary API failure")
			if stage == "get" {
				broken.getErr = want
			} else {
				broken.listErr = want
			}
			r := &GCReconciler{Client: broken, Recorder: record.NewFakeRecorder(10), clock: testclock.NewFakeClock(now)}
			if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(b)}); !errors.Is(err, want) {
				t.Fatalf("expected retryable read error, got %v", err)
			}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(b), &dpv1alpha1.Backup{}); err != nil {
				t.Fatalf("read failure deleted backup: %v", err)
			}
		})
	}
}
