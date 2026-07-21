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

package proto

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRetryPolicyIntervalCompatibility(t *testing.T) {
	seconds := int64(1)
	zero := int64(0)
	maxSeconds := int64(1<<63 - 1)
	maxDuration := time.Duration(1<<63 - 1)

	tests := []struct {
		name            string
		legacy          time.Duration
		seconds         *int64
		want            time.Duration
		wantWireSeconds *int64
	}{
		{name: "legacy duration keeps nanoseconds", legacy: time.Second, want: time.Second},
		{name: "seconds field", seconds: &seconds, want: time.Second, wantWireSeconds: &seconds},
		{name: "seconds field wins", legacy: time.Nanosecond, seconds: &seconds, want: time.Second, wantWireSeconds: &seconds},
		{name: "explicit zero seconds wins", legacy: time.Second, seconds: &zero, want: 0, wantWireSeconds: &zero},
		{name: "seconds overflow saturates", seconds: &maxSeconds, want: maxDuration, wantWireSeconds: &maxSeconds},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := NewRetryPolicy(2, tt.legacy, tt.seconds)
			if got := policy.Interval(); got != tt.want {
				t.Fatalf("Interval() = %v, want %v", got, tt.want)
			}
			if policy.RetryInterval != tt.want {
				t.Fatalf("legacy wire interval = %v, want %v", policy.RetryInterval, tt.want)
			}
			if tt.wantWireSeconds == nil {
				if policy.RetryIntervalSeconds != nil {
					t.Fatalf("wire seconds = %v, want nil", *policy.RetryIntervalSeconds)
				}
			} else if policy.RetryIntervalSeconds == nil || *policy.RetryIntervalSeconds != *tt.wantWireSeconds {
				t.Fatalf("wire seconds = %v, want %v", policy.RetryIntervalSeconds, *tt.wantWireSeconds)
			}
		})
	}
}

func TestRetryPolicyNewWriterOldReader(t *testing.T) {
	seconds := int64(1)
	policy := NewRetryPolicy(2, 0, &seconds)
	data, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("marshal retry policy: %v", err)
	}

	var legacy struct {
		MaxRetries    int           `json:"maxRetries,omitempty"`
		RetryInterval time.Duration `json:"retryInterval,omitempty"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		t.Fatalf("unmarshal retry policy with legacy reader: %v", err)
	}
	if legacy.MaxRetries != 2 || legacy.RetryInterval != time.Second {
		t.Fatalf("legacy reader got maxRetries=%d retryInterval=%v", legacy.MaxRetries, legacy.RetryInterval)
	}
}
