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

package component

import (
	"testing"

	"k8s.io/utils/ptr"
)

func TestIsReplicaProvisioningOpen(t *testing.T) {
	tests := []struct {
		name         string
		dataLoaded   *bool
		memberJoined *bool
		want         bool
	}{
		// nil means "not applicable or record closed" — NOT unknown.
		{name: "both nil (closed/not applicable)", dataLoaded: nil, memberJoined: nil, want: false},
		{name: "dataLoaded false, memberJoined nil", dataLoaded: ptr.To(false), memberJoined: nil, want: true},
		{name: "dataLoaded nil, memberJoined false", dataLoaded: nil, memberJoined: ptr.To(false), want: true},
		{name: "both true (completed)", dataLoaded: ptr.To(true), memberJoined: ptr.To(true), want: false},
		{name: "dataLoaded false, memberJoined true", dataLoaded: ptr.To(false), memberJoined: ptr.To(true), want: true},
		{name: "dataLoaded true, memberJoined false", dataLoaded: ptr.To(true), memberJoined: ptr.To(false), want: true},
		{name: "both false", dataLoaded: ptr.To(false), memberJoined: ptr.To(false), want: true},
		{name: "dataLoaded true, memberJoined nil", dataLoaded: ptr.To(true), memberJoined: nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := ReplicaStatus{
				Name:         "pod-0",
				DataLoaded:   tt.dataLoaded,
				MemberJoined: tt.memberJoined,
			}
			if got := IsReplicaProvisioningOpen(s); got != tt.want {
				t.Fatalf("IsReplicaProvisioningOpen(dataLoaded=%v, memberJoined=%v) = %v, want %v",
					tt.dataLoaded, tt.memberJoined, got, tt.want)
			}
		})
	}
}
