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

package component

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestConfigCleanupReadsConsumersWhenScopedMirrorIsAbsent(t *testing.T) {
	for _, consumer := range []string{"Pod", "Instance"} {
		t.Run(consumer, func(t *testing.T) {
			comp, synth, scheme := instanceConfigFixture(t)
			authority := managedInstanceConfig(comp, "retired")
			authority.Annotations = map[string]string{instanceConfigMapPlacementsAnnotationKey: "previous-provider"}
			spec := corev1.PodSpec{Volumes: []corev1.Volume{{Name: "config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: authority.Name}}}}}}
			meta := metav1.ObjectMeta{Name: comp.Name + "-0", Namespace: comp.Namespace, Labels: constant.GetCompLabels("db", "mysql")}
			var object client.Object = &corev1.Pod{ObjectMeta: meta, Spec: spec}
			if consumer == "Instance" {
				object = &workloads.Instance{ObjectMeta: meta, Spec: workloads.InstanceSpec{Template: corev1.PodTemplateSpec{Spec: spec}}}
			}
			control := fake.NewClientBuilder().WithScheme(scheme).WithObjects(authority).Build()
			provider := fake.NewClientBuilder().WithScheme(scheme).WithObjects(object).Build()
			cli := multicluster.NewClient(control, map[string]client.Client{"previous-provider": provider})
			transCtx := &componentTransformContext{Context: context.Background(), Component: comp, SynthesizeComponent: synth}
			if err := collectInstanceConfigMaps(transCtx, cli, nil, []string{"previous-provider"}); !intctrlutil.IsDelayedRequeueError(err) {
				t.Fatalf("expected reference wait: %v", err)
			}
			if err := control.Get(context.Background(), client.ObjectKeyFromObject(authority), &corev1.ConfigMap{}); err != nil {
				t.Fatalf("scoped NotFound hid a consumer: %v", err)
			}
		})
	}
}

func TestConfigCleanupRemovesMirrorBeforeAuthority(t *testing.T) {
	comp, synth, scheme := instanceConfigFixture(t)
	authority := managedInstanceConfig(comp, "retired")
	authority.Annotations = map[string]string{instanceConfigMapPlacementsAnnotationKey: "old-provider"}
	mirror := authority.DeepCopy()
	mirror.OwnerReferences = nil
	mirror.UID = "mirror-uid"
	control := fake.NewClientBuilder().WithScheme(scheme).WithObjects(authority).Build()
	provider := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mirror).Build()
	cli := multicluster.NewClient(control, map[string]client.Client{"old-provider": provider})
	transCtx := &componentTransformContext{Context: context.Background(), Component: comp, SynthesizeComponent: synth}
	for attempt := 0; attempt < 2; attempt++ {
		if err := collectInstanceConfigMaps(transCtx, cli, nil, []string{"old-provider"}); !intctrlutil.IsDelayedRequeueError(err) {
			t.Fatalf("expected cleanup confirmation: %v", err)
		}
		err := control.Get(context.Background(), client.ObjectKeyFromObject(authority), &corev1.ConfigMap{})
		if attempt == 0 && err != nil {
			t.Fatalf("authority removed before mirror absence confirmed: %v", err)
		}
		if attempt == 1 && !apierrors.IsNotFound(err) {
			t.Fatalf("authority remains after mirror absence: %v", err)
		}
	}
	if err := provider.Get(context.Background(), client.ObjectKeyFromObject(mirror), &corev1.ConfigMap{}); !apierrors.IsNotFound(err) {
		t.Fatalf("mirror remains: %v", err)
	}
}

func TestConfigCleanupDefersUnknownOrUnavailableProvider(t *testing.T) {
	for _, scope := range []string{"unknown", "unavailable"} {
		t.Run(scope, func(t *testing.T) {
			comp, synth, scheme := instanceConfigFixture(t)
			authority := managedInstanceConfig(comp, "retired")
			authority.Annotations = map[string]string{instanceConfigMapPlacementsAnnotationKey: scope}
			control := fake.NewClientBuilder().WithScheme(scheme).WithObjects(authority).Build()
			workers := map[string]client.Client{"other": fake.NewClientBuilder().WithScheme(scheme).Build()}
			if scope == "unavailable" {
				workers[scope] = interceptor.NewClient(fake.NewClientBuilder().WithScheme(scheme).Build(), interceptor.Funcs{Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
					return errors.New("provider unavailable")
				}})
			}
			cli := multicluster.NewClient(control, workers)
			transCtx := &componentTransformContext{Context: context.Background(), Component: comp, SynthesizeComponent: synth}
			if err := collectInstanceConfigMaps(transCtx, cli, nil, []string{"other", "unavailable"}); !intctrlutil.IsDelayedRequeueError(err) {
				t.Fatalf("expected incomplete-scope wait: %v", err)
			}
			if err := control.Get(context.Background(), client.ObjectKeyFromObject(authority), &corev1.ConfigMap{}); err != nil {
				t.Fatalf("incomplete scope deleted authority: %v", err)
			}
		})
	}
}

func TestDeletingConfigCleanupWaitsForAllProviderRuntimes(t *testing.T) {
	comp, _, scheme := instanceConfigFixture(t)
	authority := managedInstanceConfig(comp, "retired")
	authority.Annotations = map[string]string{instanceConfigMapPlacementsAnnotationKey: "old-provider"}
	mirror := authority.DeepCopy()
	mirror.OwnerReferences = nil
	mirror.UID = "mirror-uid"
	instance := &workloads.Instance{ObjectMeta: metav1.ObjectMeta{Name: comp.Name + "-0", Namespace: comp.Namespace, Labels: constant.GetCompLabels("db", "mysql")}}
	control := fake.NewClientBuilder().WithScheme(scheme).WithObjects(authority).Build()
	provider := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mirror, instance).Build()
	cli := multicluster.NewClient(control, map[string]client.Client{"old-provider": provider})
	transCtx := &componentTransformContext{Context: context.Background(), Component: comp}
	labels := constant.GetCompLabels("db", "mysql")
	if err := collectDeletingInstanceConfigMaps(transCtx, cli, labels, []string{"old-provider"}); !intctrlutil.IsRequeueError(err) {
		t.Fatalf("expected runtime wait: %v", err)
	}
	if err := provider.Get(context.Background(), client.ObjectKeyFromObject(mirror), &corev1.ConfigMap{}); err != nil {
		t.Fatalf("provider runtime lost config: %v", err)
	}
	if err := provider.Delete(context.Background(), instance); err != nil {
		t.Fatal(err)
	}
	if err := collectDeletingInstanceConfigMaps(transCtx, cli, labels, []string{"old-provider"}); !intctrlutil.IsRequeueError(err) {
		t.Fatalf("expected mirror confirmation wait: %v", err)
	}
	if err := collectDeletingInstanceConfigMaps(transCtx, cli, labels, []string{"old-provider"}); err != nil {
		t.Fatal(err)
	}
	if err := control.Get(context.Background(), client.ObjectKeyFromObject(authority), &corev1.ConfigMap{}); err != nil {
		t.Fatalf("whole-delete helper bypassed source deletion policy: %v", err)
	}
}

func TestConfigCleanupRetainsRecordsAfterProviderRemoval(t *testing.T) {
	for _, deleting := range []bool{false, true} {
		t.Run(map[bool]string{false: "retired config", true: "component deletion"}[deleting], func(t *testing.T) {
			comp, synth, scheme := instanceConfigFixture(t)
			comp.Spec.EnableInstanceAPI = ptr.To(true)
			synth.EnableInstanceAPI = ptr.To(true)
			authority := managedInstanceConfig(comp, "retired")
			authority.Finalizers = []string{constant.DBComponentFinalizerName}
			authority.Annotations = map[string]string{instanceConfigMapPlacementsAnnotationKey: "old-provider"}
			mirror := authority.DeepCopy()
			mirror.OwnerReferences = nil
			mirror.Finalizers = nil
			mirror.UID = "mirror-uid"
			control := fake.NewClientBuilder().WithScheme(scheme).WithObjects(authority).Build()
			provider := fake.NewClientBuilder().WithScheme(scheme).WithObjects(mirror).Build()
			transCtx := &componentTransformContext{Context: context.Background(), Component: comp, SynthesizeComponent: synth}
			collect := func(cli client.Client, providers []string) error {
				if deleting {
					return collectDeletingInstanceConfigMaps(transCtx, cli, constant.GetCompLabels("db", "mysql"), providers)
				}
				return collectInstanceConfigMaps(transCtx, cli, nil, providers)
			}
			// Without a multi-cluster manager the production controller receives this
			// plain control client, which ignores provider routing options.
			for attempt := 0; attempt < 2; attempt++ {
				if err := collect(control, nil); err == nil {
					t.Fatal("missing provider configuration allowed cleanup")
				}
				current := &corev1.ConfigMap{}
				if err := control.Get(context.Background(), client.ObjectKeyFromObject(authority), current); err != nil {
					t.Fatalf("lost authority while provider was absent: %v", err)
				}
				if len(current.Finalizers) != 1 || current.Annotations[instanceConfigMapPlacementsAnnotationKey] != "old-provider" {
					t.Fatal("cleanup changed the authority record while its provider was absent")
				}
			}
			configured := multicluster.NewClient(control, map[string]client.Client{"old-provider": provider})
			if err := collect(configured, []string{"old-provider"}); err == nil {
				t.Fatal("mirror deletion did not request confirmation")
			}
			if err := provider.Get(context.Background(), client.ObjectKeyFromObject(mirror), &corev1.ConfigMap{}); !apierrors.IsNotFound(err) {
				t.Fatalf("restored provider mirror was not deleted: %v", err)
			}
			if err := control.Get(context.Background(), client.ObjectKeyFromObject(authority), &corev1.ConfigMap{}); err != nil {
				t.Fatalf("authority removed before mirror absence was confirmed: %v", err)
			}
			if deleting {
				if err := collect(configured, []string{"old-provider"}); err != nil {
					t.Fatalf("whole-deletion cleanup did not resume: %v", err)
				}
				if err := control.Get(context.Background(), client.ObjectKeyFromObject(authority), &corev1.ConfigMap{}); err != nil {
					t.Fatalf("whole-deletion cleanup bypassed authority policy: %v", err)
				}
				return
			}
			if err := collect(configured, []string{"old-provider"}); !intctrlutil.IsDelayedRequeueError(err) {
				t.Fatalf("expected finalizer retry: %v", err)
			}
			current := &corev1.ConfigMap{}
			if err := control.Get(context.Background(), client.ObjectKeyFromObject(authority), current); err != nil || len(current.Finalizers) != 0 {
				t.Fatalf("source finalizer cleanup did not persist: %v", err)
			}
			if err := collect(configured, []string{"old-provider"}); !intctrlutil.IsDelayedRequeueError(err) {
				t.Fatalf("expected guarded-delete confirmation: %v", err)
			}
			if err := control.Get(context.Background(), client.ObjectKeyFromObject(authority), &corev1.ConfigMap{}); !apierrors.IsNotFound(err) {
				t.Fatalf("authority remains after mirror cleanup: %v", err)
			}
			if err := collect(configured, []string{"old-provider"}); err != nil {
				t.Fatalf("cleanup did not converge: %v", err)
			}
		})
	}
}

func TestConfigCleanupAllowsConfiguredControlProviderAlias(t *testing.T) {
	for _, deleting := range []bool{false, true} {
		t.Run(map[bool]string{false: "retired config", true: "component deletion"}[deleting], func(t *testing.T) {
			comp, synth, scheme := instanceConfigFixture(t)
			comp.Spec.EnableInstanceAPI = ptr.To(true)
			synth.EnableInstanceAPI = ptr.To(true)
			authority := managedInstanceConfig(comp, "retired")
			authority.Finalizers = []string{constant.DBComponentFinalizerName}
			authority.Annotations = map[string]string{instanceConfigMapPlacementsAnnotationKey: "control-alias"}
			control := fake.NewClientBuilder().WithScheme(scheme).WithObjects(authority).Build()
			cli := multicluster.NewClient(control, map[string]client.Client{"control-alias": control})
			transCtx := &componentTransformContext{Context: context.Background(), Component: comp, SynthesizeComponent: synth}
			collect := func() error {
				if deleting {
					return collectDeletingInstanceConfigMaps(transCtx, cli, constant.GetCompLabels("db", "mysql"), []string{"control-alias"})
				}
				return collectInstanceConfigMaps(transCtx, cli, nil, []string{"control-alias"})
			}
			if deleting {
				if err := collect(); err != nil {
					t.Fatalf("configured control alias blocked whole deletion: %v", err)
				}
				if err := control.Get(context.Background(), client.ObjectKeyFromObject(authority), &corev1.ConfigMap{}); err != nil {
					t.Fatalf("whole deletion bypassed authority policy: %v", err)
				}
				return
			}
			for attempt := 0; attempt < 2; attempt++ {
				if err := collect(); !intctrlutil.IsDelayedRequeueError(err) {
					t.Fatalf("configured alias blocked cleanup: %v", err)
				}
			}
			if err := control.Get(context.Background(), client.ObjectKeyFromObject(authority), &corev1.ConfigMap{}); !apierrors.IsNotFound(err) {
				t.Fatalf("authority remains for supported control alias: %v", err)
			}
			if err := collect(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
