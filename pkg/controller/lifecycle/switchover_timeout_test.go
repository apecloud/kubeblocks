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

package lifecycle

import (
	"context"
	"testing"
	"time"
)

func TestLongSwitchoverTransportBudget(t *testing.T) {
	for _, tt := range []struct {
		name    string
		seconds int32
		targets int
		want    time.Duration
		reject  bool
	}{
		{"long", 300, 1, 310 * time.Second, false},
		{"bounded", 900, 1, 310 * time.Second, false},
		{"negative bounded", -1, 1, 310 * time.Second, false},
		{"short unchanged", 60, 2, 0, false},
		{"multiple execution pods", 300, 2, 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel, err := longSwitchoverCallContext(context.Background(), "switchover", tt.seconds, tt.targets)
			if (err != nil) != tt.reject {
				t.Fatalf("error=%v", err)
			}
			if err != nil {
				return
			}
			defer cancel()
			deadline, ok := ctx.Deadline()
			if tt.want == 0 {
				if ok {
					t.Fatal("changed legacy deadline")
				}
				return
			}
			remaining := time.Until(deadline)
			if !ok || remaining > tt.want || remaining < tt.want-time.Second {
				t.Fatalf("remaining=%v", remaining)
			}
		})
	}
	parent, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if _, _, err := longSwitchoverCallContext(parent, "switchover", 300, 1); err == nil {
		t.Fatal("short parent accepted")
	}
	ctx, cancel, err := longSwitchoverCallContext(parent, "roleProbe", 300, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	if ctx != parent {
		t.Fatal("changed unrelated action")
	}
}
