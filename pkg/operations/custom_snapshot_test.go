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

package operations

import (
	"context"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestSnapshottedCustomOpsDefinitionNotFoundIsFatal(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := opsv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	opsRequest := &opsv1alpha1.OpsRequest{Spec: opsv1alpha1.OpsRequestSpec{
		Type: opsv1alpha1.CustomType,
		SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{CustomOps: &opsv1alpha1.CustomOps{
			OpsDefinitionName: "missing-managed-definition",
			ExecutionSnapshot: &opsv1alpha1.CustomOpsExecutionSnapshot{
				OpsDefinitionUID:        "missing-uid",
				OpsDefinitionGeneration: 1,
				OpsDefinitionSpecHash:   strings.Repeat("a", 64),
				TargetSnapshotHash:      strings.Repeat("b", 64),
			},
		}},
	}}
	err := initOpsDefAndValidate(intctrlutil.RequestCtx{Ctx: context.Background()}, cli, &OpsResource{
		Reader: cli, OpsRequest: opsRequest, Cluster: &appsv1.Cluster{},
	})
	if err == nil || !intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
		t.Fatalf("error=%v, want fatal NotFound for snapshotted OpsDefinition", err)
	}
}
