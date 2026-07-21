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

package v1

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRetryPolicyJSONCompatibility(t *testing.T) {
	var legacy RetryPolicy
	if err := json.Unmarshal([]byte(`{"retryInterval":1000000000}`), &legacy); err != nil {
		t.Fatalf("unmarshal legacy policy: %v", err)
	}
	if legacy.RetryInterval != time.Second {
		t.Fatalf("legacy retry interval = %v, want %v", legacy.RetryInterval, time.Second)
	}
	if legacy.RetryIntervalSeconds != nil {
		t.Fatalf("legacy seconds field = %v, want nil", *legacy.RetryIntervalSeconds)
	}

	seconds := int64(1)
	policy := RetryPolicy{RetryInterval: time.Second, RetryIntervalSeconds: &seconds}
	data, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("marshal policy: %v", err)
	}
	const want = `{"retryInterval":1000000000,"retryIntervalSeconds":1}`
	if string(data) != want {
		t.Fatalf("policy JSON = %s, want %s", data, want)
	}
}
