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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apecloud/kubeblocks/lorry/binding"
	"github.com/apecloud/kubeblocks/lorry/component"
	. "github.com/apecloud/kubeblocks/lorry/util"
	"github.com/apecloud/kubeblocks/pkg/common"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestInit(t *testing.T) {
	hs, s := setUpHost("leader", t)
	defer s.Close()

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
			response, err := hs.Invoke(context.TODO(), &binding.ProbeRequest{
				Data:      []byte(tc.input),
				Metadata:  tc.metadata,
				Operation: OperationKind(tc.operation),
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

func TestGlobalRoleSnapshot(t *testing.T) {
	var lines []string
	for i := 0; i < 3; i++ {
		podName := "pod-" + strconv.Itoa(i)
		var role string
		if i == 0 {
			role = "leader"
		} else {
			role = "follower"
		}
		lines = append(lines, fmt.Sprintf("%d,%s,%s", 1, podName, role))
	}
	join := strings.Join(lines, "\n")
	hs, s := setUpHost(join, t)
	defer s.Close()

	tests := map[string]struct {
		input     string
		operation string
		metadata  map[string]string
		path      string
		err       string
	}{
		"get": {
			input:     join,
			operation: "getRole",
			metadata:  nil,
			path:      "/",
			err:       "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			response := binding.ProbeResponse{}
			info, err := hs.GetRole(context.TODO(), &binding.ProbeRequest{
				Operation: OperationKind(tc.operation),
			}, &response)
			require.NoError(t, err)
			snapshot := &common.GlobalRoleSnapshot{}
			assert.NoError(t, json.Unmarshal([]byte(info), snapshot))
			assert.Equal(t, 3, len(snapshot.PodRoleNamePairs))
			assert.Equal(t, "1", snapshot.Version)
		})
	}

}

func setUpHost(respContent string, t *testing.T) (*HTTPCustom, *httptest.Server) {
	s := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			_, _ = w.Write([]byte(respContent))
		}),
	)
	addr := s.Listener.Addr().String()
	index := strings.LastIndex(addr, ":")
	portStr := addr[index+1:]
	viper.Set("KB_RSM_ACTION_SVC_LIST", "["+portStr+"]")
	hs := NewHTTPCustom()
	metadata := make(component.Properties)
	err := hs.Init(metadata)
	require.NoError(t, err)
	return hs, s
}
