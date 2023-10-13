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

package probe

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	viper "github.com/apecloud/kubeblocks/internal/viperx"
	"github.com/apecloud/kubeblocks/lorry/binding"
	"github.com/apecloud/kubeblocks/lorry/component"
	"github.com/apecloud/kubeblocks/lorry/util"
)

func TestGetRouter(t *testing.T) {
	t.Run("Use Custom to get role", func(t *testing.T) {
		s := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				_, _ = w.Write([]byte("leader"))
			}),
		)
		defer s.Close()

		addr := s.Listener.Addr().String()
		index := strings.LastIndex(addr, ":")
		portStr := addr[index+1:]
		viper.Set("KB_RSM_ACTION_SVC_LIST", "["+portStr+"]")

		err := mockRegister()
		assert.Nil(t, err)

		request := httptest.NewRequest("GET", "/v1.0/bindings/custom?operation=getRole", nil)
		recorder := httptest.NewRecorder()
		middleware := SetMiddleware(GetRouter())
		middleware(recorder, request)

		result := binding.OpsResult{}
		err = json.Unmarshal(recorder.Body.Bytes(), &result)
		assert.Nil(t, err)
		assert.Equal(t, util.OperationSuccess, result["event"])
		assert.Equal(t, "leader", result["role"])
	})
}

func emptyConfig() map[string]component.Properties {
	viper.Set("KB_POD_NAME", "test-pod-0")

	m := make(map[string]component.Properties)
	mysqlMap := component.Properties{}
	mysqlMap["url"] = "root:@tcp(127.0.0.1:3306)/mysql?multiStatements=true"

	m["mysql"] = mysqlMap

	m["redis"] = component.Properties{}

	pgMap := component.Properties{}
	pgMap["url"] = "user=postgres password=docker host=localhost port=5432 dbname=postgres pool_min_conns=1 pool_max_conns=10"
	m["postgres"] = pgMap

	etcdMap := component.Properties{}
	etcdMap["endpoint"] = "127.0.0.1:2379"
	m["etcd"] = etcdMap

	mongoMap := component.Properties{}
	mongoMap["host"] = "127.0.0.1:27017"
	mongoMap["params"] = "?directConnection=true"
	m["mongodb"] = mongoMap

	return m
}

func mockRegister() error {
	component.Name2Property = emptyConfig()
	return RegisterBuiltin("custom")
}
