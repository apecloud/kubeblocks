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

package instanceset2

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
)

func TestValidationRejectsNodeSelectorOnce(t *testing.T) {
	tree := kubebuilderx.NewObjectTree()
	tree.SetRoot(&workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-its",
			Namespace: "default",
			Annotations: map[string]string{
				constant.NodeSelectorOnceAnnotationKey: `{"test-its-0":"node-a"}`,
			},
		},
		Spec: workloads.InstanceSetSpec{
			Replicas:          ptr.To[int32](1),
			EnableInstanceAPI: ptr.To(true),
		},
	})

	_, err := NewValidationReconciler().Reconcile(tree)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), constant.NodeSelectorOnceAnnotationKey) {
		t.Fatalf("expected error to mention %s, got: %v", constant.NodeSelectorOnceAnnotationKey, err)
	}
}

func TestValidationRejectsInvalidTemplateConfigs(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*workloads.InstanceSet)
	}{
		{name: "unknown config", mutate: func(its *workloads.InstanceSet) {
			its.Spec.Instances[0].Configs[0].Name = "missing"
		}},
		{name: "missing volume on inactive template", mutate: func(its *workloads.InstanceSet) {
			its.Spec.Instances[0].Replicas = ptr.To[int32](0)
			its.Spec.Instances[0].Configs[0].Volume.VolumeName = "missing"
		}},
		{name: "inherited PVC replaces bound volume", mutate: func(its *workloads.InstanceSet) {
			its.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "config"}}}
		}},
		{name: "global config has parent-only binding", mutate: func(its *workloads.InstanceSet) {
			its.Spec.Configs[0].Volume = &workloads.ConfigMapVolumeBinding{VolumeName: "config", ConfigMapName: "other"}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			its, tree, _ := newITS2ConfigFixture(t, false)
			tt.mutate(its)
			if _, err := NewValidationReconciler().Reconcile(tree); err == nil {
				t.Fatal("invalid template configuration was accepted")
			}
			if len(tree.List(&workloads.Instance{})) != 0 {
				t.Fatal("invalid configuration created Instances")
			}
		})
	}
}
