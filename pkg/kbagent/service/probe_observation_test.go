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

package service

import (
	"testing"
)

func TestProbeEventObservationVersionUsesSampleTime(t *testing.T) {
	now := int64(100)
	runner := &probeRunner{nowMicros: func() int64 { return now }}
	first := runner.buildEvent("db-0", "roleProbe", 0, []byte("primary"), "")
	if first.ObservationVersion != 100 {
		t.Fatalf("first sample version = %d, want 100", first.ObservationVersion)
	}
	now = 200
	retry := runner.buildEvent("db-0", "roleProbe", 0, []byte("primary"), "")
	if retry.ObservationVersion != first.ObservationVersion {
		t.Fatalf("same sample changed version from %d to %d", first.ObservationVersion, retry.ObservationVersion)
	}
	now = 300
	changed := runner.buildEvent("db-0", "roleProbe", 0, []byte("secondary"), "")
	if changed.ObservationVersion != 300 {
		t.Fatalf("changed sample version = %d, want 300", changed.ObservationVersion)
	}
	failure := runner.buildEvent("db-0", "roleProbe", 1, nil, "failed")
	if failure.ObservationVersion != changed.ObservationVersion {
		t.Fatalf("failure changed version from %d to %d", changed.ObservationVersion, failure.ObservationVersion)
	}
}
