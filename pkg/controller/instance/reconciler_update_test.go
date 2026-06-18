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

package instance

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestUpdateReconcilerPatchesMetadataOnlyInPlaceUpdate(t *testing.T) {
	origSupportResize := intctrlutil.SupportResizeSubResource
	intctrlutil.SupportResizeSubResource = func() (bool, error) { return false, nil }
	defer func() { intctrlutil.SupportResizeSubResource = origSupportResize }()

	inst := builder.NewInstanceBuilder(testNamespace, "mysql-0").
		SetUID(types.UID("12345678-1234-1234-1234-1234567890ab")).
		SetContainers([]corev1.Container{{Name: "mysql", Image: "mysql:8.0"}}).
		SetPodUpdatePolicy(kbappsv1.PreferInPlacePodUpdatePolicyType).
		SetConfigs([]workloads.ConfigTemplate{{
			Name:       "mysql-conf",
			ConfigHash: ptr.To("old-hash"),
		}}).
		GetObject()

	revision, err := buildInstancePodRevision(&inst.Spec.Template, inst)
	if err != nil {
		t.Fatalf("buildInstancePodRevision() error = %v", err)
	}
	inst.Status.UpdateRevision = revision

	pod, err := buildInstancePod(inst, revision)
	if err != nil {
		t.Fatalf("buildInstancePod() error = %v", err)
	}
	pod.Status.Phase = corev1.PodRunning
	pod.Status.Conditions = []corev1.PodCondition{{
		Type:               corev1.PodReady,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
	}}
	oldConfigHashAnnotation := pod.Annotations[constant.CMInsConfigurationHashLabelKey]

	inst.Spec.Configs[0].ConfigHash = ptr.To("new-hash")
	tree := newTestTree(inst)
	if err = tree.Add(pod); err != nil {
		t.Fatalf("tree.Add() error = %v", err)
	}

	res, err := NewUpdateReconciler().Reconcile(tree)
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if res != kubebuilderx.Continue {
		t.Fatalf("Reconcile() result = %v, want %v", res, kubebuilderx.Continue)
	}

	obj, options, err := tree.GetWithOption(pod)
	if err != nil {
		t.Fatalf("tree.GetWithOption() error = %v", err)
	}
	updatedPod, ok := obj.(*corev1.Pod)
	if !ok {
		t.Fatalf("expected updated Pod, got %T", obj)
	}
	if !options.Patch {
		t.Fatal("expected metadata-only pod update to be committed with Patch")
	}
	if updatedPod.Annotations[constant.CMInsConfigurationHashLabelKey] == oldConfigHashAnnotation {
		t.Fatalf("expected config-hash annotation to be updated, got %q", updatedPod.Annotations[constant.CMInsConfigurationHashLabelKey])
	}
	configs, err := configsFromPod(updatedPod)
	if err != nil {
		t.Fatalf("configsFromPod() error = %v", err)
	}
	if len(configs) != 1 || configs[0].Name != "mysql-conf" || ptr.Deref(configs[0].ConfigHash, "") != "new-hash" {
		t.Fatalf("unexpected updated configs: %#v", configs)
	}
}
