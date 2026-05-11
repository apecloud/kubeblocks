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
