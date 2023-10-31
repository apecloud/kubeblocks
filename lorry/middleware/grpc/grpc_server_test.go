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

package grpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	health "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/apecloud/kubeblocks/lorry/binding"
	"github.com/apecloud/kubeblocks/lorry/middleware/probe"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestNewServer(t *testing.T) {
	server := NewGRPCServer()
	assert.NotNil(t, server.logger)
	assert.Error(t, server.Watch(nil, nil))
}

func TestCheck(t *testing.T) {
	// set up the host
	s := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			_, _ = w.Write([]byte("leader"))
		}),
	)
	addr := s.Listener.Addr().String()
	index := strings.LastIndex(addr, ":")
	portStr := addr[index+1:]

	// set up the environment
	viper.Set("KB_RSM_ACTION_SVC_LIST", "["+portStr+"]")
	viper.Set("KB_RSM_ROLE_UPDATE_MECHANISM", "ReadinessProbeEventUpdate")

	assert.Nil(t, probe.RegisterBuiltin(""))
	server := NewGRPCServer()
	check, err := server.Check(context.Background(), nil)

	assert.Error(t, err)
	assert.Equal(t, health.HealthCheckResponse_NOT_SERVING, check.Status)

	// set up the expected answer
	result := binding.OpsResult{}
	result["event"] = "Success"
	result["operation"] = "checkRole"
	result["originalRole"] = ""
	result["role"] = "leader"
	ans, _ := json.Marshal(result)

	assert.Equal(t, string(ans), err.Error())
}
