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

package v1

import (
	"testing"
	"time"
)

func TestRetentionPeriodToDuration(t *testing.T) {
	tests := []struct {
		name    string
		period  RetentionPeriod
		want    time.Duration
		wantErr bool
	}{
		{
			name:   "empty period",
			period: "",
			want:   0,
		},
		{
			name:   "combined units",
			period: "1y2mo3w4d5h6m",
			want:   (365*24*time.Hour + 2*30*24*time.Hour + 3*7*24*time.Hour + 4*24*time.Hour + 5*time.Hour + 6*time.Minute),
		},
		{
			name:    "negative days",
			period:  "-1d",
			wantErr: true,
		},
		{
			name:    "negative unit in combined period",
			period:  "1d-2h",
			wantErr: true,
		},
		{
			name:    "unit without number",
			period:  "1dh",
			wantErr: true,
		},
		{
			name:    "number without unit",
			period:  "1d2",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.period.ToDuration()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %s, got %s", tt.want, got)
			}
		})
	}
}
