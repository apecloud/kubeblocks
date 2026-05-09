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

package plan

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

func TestGetRestoreObjectMetaShortensName(t *testing.T) {
	manager := &RestoreManager{
		Cluster: &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      strings.Repeat("restore-cluster-", 4),
				Namespace: "default",
				UID:       types.UID("12345678-1234-1234-1234-1234567890ab"),
			},
		},
	}
	synthesized := &component.SynthesizedComponent{
		Name: strings.Repeat("component-", 4),
	}

	meta := manager.GetRestoreObjectMeta(synthesized, dpv1alpha1.PrepareData, "template")
	if len(meta.Name) > constant.KubeNameMaxLength {
		t.Fatalf("restore name length = %d, want <= %d: %s", len(meta.Name), constant.KubeNameMaxLength, meta.Name)
	}
}
