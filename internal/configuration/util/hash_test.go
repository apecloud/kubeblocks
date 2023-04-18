/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
