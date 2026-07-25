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

package util

import (
	"context"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestManagedJobDefaultedSpecHashIsStable(t *testing.T) {
	testEnv := &envtest.Environment{}
	cfg, err := testEnv.Start()
	if err != nil {
		t.Fatalf("start envtest: %v", err)
	}
	defer func() {
		if err := testEnv.Stop(); err != nil {
			t.Errorf("stop envtest: %v", err)
		}
	}()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := batchv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add batch scheme: %v", err)
	}
	cli, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	const selectorKey = "operations.kubeblocks.io/managed-job-task"
	const selectorValue = "y7i4xtj2rw4s3w4usf7p2auvq6uoqkysobdjzwzjurjohp3mb4sa"
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "managed-job-hash", Namespace: "default"},
		Spec: batchv1.JobSpec{
			ManualSelector: ptr.To(true),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
				selectorKey: selectorValue,
			}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{selectorKey: selectorValue}},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name: "runner", Image: "busybox:1.36", Command: []string{"sh", "-c", "true"},
					}},
				},
			},
		},
	}

	ctx := context.Background()
	if err := cli.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}); err != nil &&
		!apierrors.IsAlreadyExists(err) {
		t.Fatalf("create test namespace: %v", err)
	}
	dryRunOne := job.DeepCopy()
	if err := cli.Create(ctx, dryRunOne, client.DryRunAll); err != nil {
		t.Fatalf("first dry-run create: %v", err)
	}
	dryRunTwo := job.DeepCopy()
	if err := cli.Create(ctx, dryRunTwo, client.DryRunAll); err != nil {
		t.Fatalf("second dry-run create: %v", err)
	}
	live := job.DeepCopy()
	if err := cli.Create(ctx, live); err != nil {
		t.Fatalf("real create: %v", err)
	}

	hashOne, err := ManagedJobSpecHash(dryRunOne.Spec)
	if err != nil {
		t.Fatalf("hash first dry-run: %v", err)
	}
	hashTwo, err := ManagedJobSpecHash(dryRunTwo.Spec)
	if err != nil {
		t.Fatalf("hash second dry-run: %v", err)
	}
	liveHash, err := ManagedJobSpecHash(live.Spec)
	if err != nil {
		t.Fatalf("hash live Job: %v", err)
	}
	if hashOne != hashTwo || hashOne != liveHash {
		t.Fatalf("defaulted Job spec hash is not stable: dry-run1=%s dry-run2=%s live=%s", hashOne, hashTwo, liveHash)
	}

	mutations := map[string]func(*batchv1.JobSpec){
		"command": func(spec *batchv1.JobSpec) {
			spec.Template.Spec.Containers[0].Command = []string{"sh", "-c", "false"}
		},
		"env": func(spec *batchv1.JobSpec) {
			spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{{Name: "TOKEN", Value: "changed"}}
		},
		"image": func(spec *batchv1.JobSpec) {
			spec.Template.Spec.Containers[0].Image = "busybox:1.35"
		},
		"selector": func(spec *batchv1.JobSpec) {
			spec.Selector.MatchLabels[selectorKey] = "changed"
		},
		"template label": func(spec *batchv1.JobSpec) {
			spec.Template.Labels[selectorKey] = "changed"
		},
	}
	for name, mutate := range mutations {
		changed := live.Spec.DeepCopy()
		mutate(changed)
		changedHash, err := ManagedJobSpecHash(*changed)
		if err != nil {
			t.Fatalf("hash %s mutation: %v", name, err)
		}
		if changedHash == liveHash {
			t.Errorf("%s mutation did not change managed Job spec hash", name)
		}
	}
}
