/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package handlers

import "testing"

func TestMaxInt64(t *testing.T) {
	result := MaxInt64(10, 20)
	expected := int64(20)
	if result != expected {
		t.Errorf("MaxInt64(10, 20) = %d; expected %d", result, expected)
	}

	result = MaxInt64(50, 30)
	expected = int64(50)
	if result != expected {
		t.Errorf("MaxInt64(50, 30) = %d; expected %d", result, expected)
	}

	result = MaxInt64(-10, -5)
	expected = int64(-5)
	if result != expected {
		t.Errorf("MaxInt64(-10, -5) = %d; expected %d", result, expected)
	}
}

func TestGetIndex(t *testing.T) {
	tests := []struct {
		name       string
		memberName string
		wantIndex  int
		wantErr    bool
	}{
		{
			name:       "Valid member name",
			memberName: "pvc1-pod-0",
			wantIndex:  0,
			wantErr:    false,
		},
		{
			name:       "Invalid member name",
			memberName: "pvc1-pod",
			wantIndex:  0,
			wantErr:    true,
		},
		// Add more test cases here if needed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIndex, err := GetIndex(tt.memberName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetIndex() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotIndex != tt.wantIndex {
				t.Errorf("GetIndex() = %v, want %v", gotIndex, tt.wantIndex)
			}
		})
	}
}
