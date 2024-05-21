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

package plugin

import (
	"fmt"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
)

func TestStripSecrets(t *testing.T) {
	secretName := "secret-abc"
	secretValue := "123"

	getRoleReq := &GetRoleRequest{
		DbInfo: &DBInfo{
			Fqdn:          "fqdn",
			Port:          123,
			AdminUser:     "admin",
			AdminPassword: "123",
		},
	}

	type testcase struct {
		original, stripped interface{}
	}

	cases := []testcase{
		{nil, "null"},
		{1, "1"},
		{"hello world", `"hello world"`},
		{true, "true"},
		{false, "false"},
		{&GetRoleRequest{}, `{}`},
		{getRoleReq, `{"db_info":{"admin_password":"***stripped***","admin_user":"admin","}`},
	}

	// Message from revised spec as received by a sidecar based on the current spec.
	// The XXX_unrecognized field contains secrets and must not get logged.
	unknownFields := &GetRoleRequest{}
	data, err := proto.Marshal(getRoleReq)
	if assert.NoError(t, err, "marshall future message") &&
		assert.NoError(t, proto.Unmarshal(data, unknownFields), "unmarshal with unknown fields") {
		cases = append(cases, testcase{unknownFields,
			`{"capacity_range":{"required_bytes":1024},"name":"foo","secrets":"***stripped***","volume_capabilities":[{"AccessType":{"Mount":{"fs_type":"ext4"}}},{"AccessType":null}],"volume_content_source":{"Type":{"Volume":{"volume_id":"abc"}}}}`,
		})
	}

	for _, c := range cases {
		before := fmt.Sprint(c.original)
		var stripped fmt.Stringer
		stripped = StripSecrets(c.original)
		if assert.Equal(t, c.stripped, stripped.String(), "unexpected result for fmt s of %s", c.original) {
			if assert.Equal(t, c.stripped, fmt.Sprintf("%v", stripped), "unexpected result for fmt v of %s", c.original) {
				assert.Equal(t, c.stripped, fmt.Sprintf("%+v", stripped), "unexpected result for fmt +v of %s", c.original)
			}
		}
		assert.Equal(t, before, fmt.Sprint(c.original), "original value modified")
	}

	// The secret is hidden because StripSecrets is a struct referencing it.
	dump := fmt.Sprintf("%#v", StripSecrets(createVolume))
	assert.NotContains(t, dump, secretName)
	assert.NotContains(t, dump, secretValue)
}
