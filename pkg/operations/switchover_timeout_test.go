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

package operations

import (
	"context"
	"testing"
	"time"

	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLongSwitchoverCallerBudget(t *testing.T) {
	for _, tt := range []struct {
		name             string
		action, ops      int32
		elapsed          time.Duration
		missing, wantErr bool
	}{
		{name: "no Ops deadline", action: 300},
		{name: "adequate Ops runway", action: 300, ops: 600},
		{name: "short Ops rejected", action: 300, ops: 60, wantErr: true},
		{name: "elapsed time counted", action: 300, ops: 600, elapsed: 400 * time.Second, wantErr: true},
		{name: "missing start rejected", action: 300, ops: 600, missing: true, wantErr: true},
		{name: "legacy action unchanged", action: 60, ops: 60},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ops := &opsv1alpha1.OpsRequest{}
			ops.Spec.TimeoutSeconds = &tt.ops
			if !tt.missing {
				ops.Status.StartTimestamp = metav1.NewTime(time.Now().Add(-tt.elapsed))
			}
			err := checkLongSwitchoverBudget(context.Background(), ops, tt.action)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if checkLongSwitchoverBudget(ctx, &opsv1alpha1.OpsRequest{}, 300) == nil {
		t.Fatal("short caller deadline accepted")
	}
}
