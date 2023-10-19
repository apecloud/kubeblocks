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

package openapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRuntime(t *testing.T) {
	type args struct {
		cueString string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{{
		name: "empty",
		args: args{
			cueString: ``,
		},
	}, {
		name: "normal test",
		args: args{
			cueString: `
                #MysqlParameter: {
                    //  mysql server param: a set of name/value pairs.
                    mysqld: {
                        // SectionName is extract section name
                		SectionName: string	
                	}
                }
                `,
		},
	}, {
		name: "failed",
		args: args{
			cueString: `
			    #List: {
			    	value: _
			    	next: #List
			    }
			    a: b: #List
			`,
		},
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewRuntime(tt.args.cueString)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRuntime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				assert.NotNil(t, got)
				assert.NotNil(t, got.Context())
				assert.Nil(t, got.Underlying().Err())
			}
		})
	}
}
