/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project.

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

package cluster

import (
	"testing"

	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

func TestClusterGraphSchemeRegistersOpsRequest(t *testing.T) {
	key, err := model.GetGVKName(&opsv1alpha1.OpsRequest{})
	if err != nil {
		t.Fatalf("OpsRequest must be registered in the cluster graph scheme: %v", err)
	}
	want := opsv1alpha1.GroupVersion.WithKind("OpsRequest")
	if key.GroupVersionKind != want {
		t.Fatalf("unexpected OpsRequest GVK: got %s, want %s", key.GroupVersionKind, want)
	}
}
