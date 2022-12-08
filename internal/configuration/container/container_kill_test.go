/*
Copyright ApeCloud Inc.

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

package container

import (
	"reflect"
	"testing"
)

func TestNewContainerKiller(t *testing.T) {
	type args struct {
		criType    CRIType
		socketPath string
	}
	tests := []struct {
		name    string
		args    args
		want    ContainerKiller
		wantErr bool
	}{{
		name: "test1",
		args: args{
			criType: "xxxx",
		},
		wantErr: true,
	}, {
		name: "test2",
		args: args{
			criType:    ContainerdType,
			socketPath: "for_test",
		},
		wantErr: false,
		want: &containerdContainer{
			socketPath: formatSocketPath("for_test"),
		},
	}, {
		name: "test3",
		args: args{
			criType: DockerType,
		},
		wantErr: false,
		want:    &dockerContainer{},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewContainerKiller(tt.args.criType, tt.args.socketPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewContainerKiller() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewContainerKiller() got = %v, want %v", got, tt.want)
			}
		})
	}
}
