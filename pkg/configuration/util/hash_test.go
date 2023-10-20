/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package util

import "testing"

func TestComputeHash(t *testing.T) {
	type args struct {
		object interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{{
		"test1",
		args{
			map[string]string{
				"abdc": "bcde",
			},
		},
		"58c7f7c8b5",
		false,
	}, {
		"empty_test",
		args{
			map[string]string{},
		},
		"5894b84845",
		false,
	}, {
		"nil_test",
		args{
			nil,
		},
		"cd856cb98",
		false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ComputeHash(tt.args.object)
			if (err != nil) != tt.wantErr {
				t.Errorf("ComputeHash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ComputeHash() got = %v, want %v", got, tt.want)
			}
		})
	}
}
