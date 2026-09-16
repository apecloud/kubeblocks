/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd
This file is part of KubeBlocks project
This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.
You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/

package component

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestCopyAndMergeITSRestartAnnotation(t *testing.T) {
	const older = "2026-09-01T00:00:00Z"
	const newer = "2026-09-02T00:00:00Z"
	for _, tt := range []struct {
		name, running, desired, want string
	}{
		{"preserve config restart against older ops", newer, older, newer},
		{"accept newer ops restart", older, newer, newer},
		{"equal timestamp", newer, newer, newer},
		{"compare timezone offsets", "2026-09-02T01:00:00+08:00", "2026-09-01T18:00:00Z", "2026-09-01T18:00:00Z"},
		{"preserve equal instant representation", newer, "2026-09-02T08:00:00+08:00", newer},
		{"compare fractional seconds", "2026-09-02T00:00:00.5Z", newer, "2026-09-02T00:00:00.5Z"},
		{"first restart", "", newer, newer},
		{"no incoming restart", newer, "", newer},
		{"replace invalid running value", "invalid", newer, newer},
		{"preserve opaque incoming value behavior", newer, "force-restart", "force-restart"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			running := &workloads.InstanceSet{Spec: workloads.InstanceSetSpec{
				Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"custom": "old", "retained": "value"},
				}},
			}}
			desired := running.DeepCopy()
			desired.Spec.Template.Annotations = map[string]string{"custom": "new"}
			if tt.running != "" {
				running.Spec.Template.Annotations[constant.RestartAnnotationKey] = tt.running
			}
			if tt.desired != "" {
				desired.Spec.Template.Annotations[constant.RestartAnnotationKey] = tt.desired
			}
			before := running.DeepCopy()
			merged := copyAndMergeITS(running, desired)
			if merged == nil {
				t.Fatal("expected the custom annotation update")
			}
			want := map[string]string{"custom": "new", "retained": "value", constant.RestartAnnotationKey: tt.want}
			if !reflect.DeepEqual(merged.Spec.Template.Annotations, want) {
				t.Fatalf("annotations = %v, want %v", merged.Spec.Template.Annotations, want)
			}
			if !reflect.DeepEqual(running, before) {
				t.Fatal("mutated the running InstanceSet")
			}
			if again := copyAndMergeITS(merged, desired); again != nil {
				t.Fatalf("repeated reconciliation changed the InstanceSet: %v", again.Spec.Template.Annotations)
			}
		})
	}
}
