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

package custom

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	s := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			_, _ = w.Write([]byte("leader"))
		}),
	)
	defer s.Close()

	addr := s.Listener.Addr().String()
	index := strings.LastIndex(addr, ":")
	portStr := addr[index+1:]
	viper.Set("KB_CONSENSUS_SET_ACTION_SVC_LIST", "["+portStr+"]")
	m := bindings.Metadata{}
	hs := NewHTTPCustom(logger.NewLogger("test"))
	err := hs.Init(m)
	require.NoError(t, err)

	tests := map[string]struct {
		input     string
		operation string
		metadata  map[string]string
		path      string
		err       string
	}{
		"get": {
			input:     `{"event":"Success","operation":"checkRole","originalRole":"","role":"leader"}`,
			operation: "checkRole",
			metadata:  nil,
			path:      "/",
			err:       "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			response, err := hs.Invoke(context.TODO(), &bindings.InvokeRequest{
				Data:      []byte(tc.input),
				Metadata:  tc.metadata,
				Operation: bindings.OperationKind(tc.operation),
			})
			if tc.err == "" {
				require.NoError(t, err)
				assert.Equal(t, strings.ToUpper(tc.input), strings.ToUpper(string(response.Data)))
			} else {
				require.Error(t, err)
				assert.Equal(t, tc.err, err.Error())
			}
		})
	}
}
