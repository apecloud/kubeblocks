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

package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/components-contrib/metadata"
	"github.com/dapr/kit/logger"
	"github.com/stretchr/testify/assert"

	. "github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	. "github.com/apecloud/kubeblocks/cmd/probe/util"
)

const (
	testTableDDL = `CREATE TABLE IF NOT EXISTS foo (
		id bigint NOT NULL,
		v1 character varying(50) NOT NULL,
		ts TIMESTAMP)`
	testInsert = "INSERT INTO foo (id, v1, ts) VALUES (%d, 'test-%d', '%v')"
	testDelete = "DELETE FROM foo"
	testUpdate = "UPDATE foo SET ts = '%v' WHERE id = %d"
	testSelect = "SELECT * FROM foo WHERE id < 3"
)

func TestOperations(t *testing.T) {
	pgOps := NewPostgres(logger.NewLogger("test")).(*PostgresOperations)
	metadata := bindings.Metadata{
		Base: metadata.Base{
			Properties: map[string]string{},
		},
	}
	metadata.Properties["url"] = "user=postgres password=docker host=localhost port=5432 dbname=postgres pool_min_conns=1 pool_max_conns=10"
	_ = pgOps.Init(metadata)
	assert.Equal(t, "postgres", pgOps.DBType)
	assert.NotNil(t, pgOps.InitIfNeed)
	assert.NotNil(t, pgOps.GetRole)
	assert.Equal(t, 5432, pgOps.DBPort)
	assert.NotNil(t, pgOps.OperationMap[GetRoleOperation])
	assert.NotNil(t, pgOps.OperationMap[ExecOperation])
	assert.NotNil(t, pgOps.OperationMap[QueryOperation])
	assert.NotNil(t, pgOps.OperationMap[CheckStatusOperation])

	assert.NotNil(t, pgOps.OperationMap[ListUsersOp])
	assert.NotNil(t, pgOps.OperationMap[CreateUserOp])
	assert.NotNil(t, pgOps.OperationMap[DeleteUserOp])
	assert.NotNil(t, pgOps.OperationMap[DescribeUserOp])
	assert.NotNil(t, pgOps.OperationMap[GrantUserRoleOp])
	assert.NotNil(t, pgOps.OperationMap[RevokeUserRoleOp])

}

// SETUP TESTS
// 1. `createdb daprtest`
// 2. `createuser daprtest`
// 3. `psql=# grant all privileges on database daprtest to daprtest;``
// 4. `export POSTGRES_TEST_CONN_URL="postgres://daprtest@localhost:5432/daprtest"``
// 5. `go test -v -count=1 ./bindings/postgres -run ^TestPostgresIntegration`

func TestPostgresIntegration(t *testing.T) {
	url := os.Getenv("POSTGRES_TEST_CONN_URL")
	if url == "" {
		t.SkipNow()
	}

	// live DB test
	b := NewPostgres(logger.NewLogger("test")).(*PostgresOperations)
	m := bindings.Metadata{Base: metadata.Base{Properties: map[string]string{connectionURLKey: url}}}
	if err := b.Init(m); err != nil {
		t.Fatal(err)
	}
	assert.True(t, b.InitIfNeed())
	_ = b.InitDelay()
	assert.False(t, b.InitIfNeed())

	// create table
	req := &bindings.InvokeRequest{
		Operation: ExecOperation,
		Metadata:  map[string]string{commandSQLKey: testTableDDL},
	}
	ctx := context.TODO()
	t.Run("Prepare Data", func(t *testing.T) {
		res, err := b.Invoke(ctx, req)
		assertResponse(t, res, err, "Success")
	})

	t.Run("Invoke checkRole", func(t *testing.T) {
		req.Operation = "checkRole"
		res, err := b.Invoke(ctx, req)
		assertResponse(t, res, err, "Success")
	})

	t.Run("Invoke getRole", func(t *testing.T) {
		req.Operation = "getRole"
		res, err := b.Invoke(ctx, req)
		assertResponse(t, res, err, "Success")
	})

	t.Run("Invoke checkStatus", func(t *testing.T) {
		req.Operation = "checkStatus"
		res, err := b.Invoke(ctx, req)
		assertResponse(t, res, err, "Success")
	})

	t.Run("Invoke create table", func(t *testing.T) {
		res, err := b.Invoke(ctx, req)
		assertResponse(t, res, err, "Success")
	})

	t.Run("Invoke delete", func(t *testing.T) {
		req.Metadata[commandSQLKey] = testDelete
		res, err := b.Invoke(ctx, req)
		assertResponse(t, res, err, "Success")
	})

	t.Run("Invoke exec with no sql", func(t *testing.T) {
		req.Operation = ExecOperation
		req.Metadata[commandSQLKey] = ""
		res, err := b.Invoke(ctx, req)
		assertResponse(t, res, err, "Failed")
	})

	t.Run("Invoke exec with invalid sql", func(t *testing.T) {
		req.Operation = ExecOperation
		req.Metadata[commandSQLKey] = "invalid sql"
		res, err := b.Invoke(ctx, req)
		assertResponse(t, res, err, "Failed")
	})

	t.Run("Invoke insert", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			req.Metadata[commandSQLKey] = fmt.Sprintf(testInsert, i, i, time.Now().Format(time.RFC3339))
			res, err := b.Invoke(ctx, req)
			assertResponse(t, res, err, "Success")
		}
	})

	t.Run("Invoke update", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			req.Metadata[commandSQLKey] = fmt.Sprintf(testUpdate, time.Now().Format(time.RFC3339), i)
			res, err := b.Invoke(ctx, req)
			assertResponse(t, res, err, "Success")
		}
	})

	t.Run("Invoke select", func(t *testing.T) {
		req.Operation = QueryOperation
		req.Metadata[commandSQLKey] = testSelect
		res, err := b.Invoke(ctx, req)
		assertResponse(t, res, err, "Success")
	})

	t.Run("Invoke query with no sql", func(t *testing.T) {
		req.Metadata[commandSQLKey] = ""
		res, err := b.Invoke(ctx, req)
		assertResponse(t, res, err, "Failed")
	})

	t.Run("Invoke query with invalid sql", func(t *testing.T) {
		req.Metadata[commandSQLKey] = "invalid sql"
		res, err := b.Invoke(ctx, req)
		assertResponse(t, res, err, "Failed")
	})

	t.Run("Invoke delete", func(t *testing.T) {
		req.Operation = ExecOperation
		req.Metadata[commandSQLKey] = testDelete
		req.Data = nil
		res, err := b.Invoke(ctx, req)
		assertResponse(t, res, err, "Success")
	})
}

// SETUP TESTS, run as `postgres` to manage accounts
// 1. export PGUSER=potgres
// 2. export PGPASSWORD=<your-pg-password>
// 4. export POSTGRES_TEST_CONN_URL="postgres://${PGUSER}:${PGPASSWORD}@localhost:5432/postgres"
// 5. `go test -v -count=1 ./cmd/probe/internal/binding/postgres -run ^TestPostgresIntegrationAccounts`
func TestPostgresIntegrationAccounts(t *testing.T) {
	url := os.Getenv("POSTGRES_TEST_CONN_URL")
	if url == "" {
		t.SkipNow()
	}

	// live DB test
	b := NewPostgres(logger.NewLogger("test")).(*PostgresOperations)
	m := bindings.Metadata{Base: metadata.Base{Properties: map[string]string{connectionURLKey: url}}}
	if err := b.Init(m); err != nil {
		t.Fatal(err)
	}
	assert.True(t, b.InitIfNeed())
	_ = b.InitDelay()
	assert.False(t, b.InitIfNeed())

	ctx := context.TODO()

	t.Run("Account management", func(t *testing.T) {
		const (
			userName = "fakeuser"
			password = "fakePassword"
			roleName = "readonly"
		)
		// delete user to clean up
		req := &bindings.InvokeRequest{
			Operation: DeleteUserOp,
			Metadata:  map[string]string{},
		}
		res, err := b.Invoke(ctx, req)
		assertResponse(t, res, err, RespEveFail)
		req.Metadata["userName"] = userName
		res, err = b.Invoke(ctx, req)
		assertResponse(t, res, err, RespEveSucc)

		// create user
		req = &bindings.InvokeRequest{
			Operation: CreateUserOp,
			Metadata:  map[string]string{},
		}
		req.Metadata["userName"] = userName
		res, err = b.Invoke(ctx, req)
		assertResponse(t, res, err, RespEveFail)
		req.Metadata["password"] = password
		res, err = b.Invoke(ctx, req)
		assertResponse(t, res, err, RespEveSucc)

		// describe user
		req = &bindings.InvokeRequest{
			Operation: DescribeUserOp,
			Metadata:  map[string]string{},
		}
		res, err = b.Invoke(ctx, req)
		assertResponse(t, res, err, RespEveFail)
		req.Metadata["userName"] = userName
		res, err = b.Invoke(ctx, req)
		assertResponse(t, res, err, RespEveSucc)

		// list users
		req = &bindings.InvokeRequest{
			Operation: ListUsersOp,
			Metadata:  map[string]string{},
		}
		res, err = b.Invoke(ctx, req)
		assertResponse(t, res, err, RespEveSucc)

		// list system users
		req = &bindings.InvokeRequest{
			Operation: ListSystemAccountsOp,
			Metadata:  map[string]string{},
		}
		res, err = b.Invoke(ctx, req)
		assertResponse(t, res, err, RespEveSucc)

		// grant role
		req = &bindings.InvokeRequest{
			Operation: GrantUserRoleOp,
			Metadata:  map[string]string{},
		}
		res, err = b.Invoke(ctx, req)
		assertResponse(t, res, err, RespEveFail)

		req.Metadata["userName"] = userName
		res, err = b.Invoke(ctx, req)
		assertResponse(t, res, err, RespEveFail)

		req.Metadata["roleName"] = "fakerole"
		res, err = b.Invoke(ctx, req)
		assertResponse(t, res, err, RespEveFail)

		for _, roleType := range []RoleType{ReadOnlyRole, ReadWriteRole, SuperUserRole} {
			roleStr := (string)(roleType)
			req.Metadata["roleName"] = roleStr
			res, err = b.Invoke(ctx, req)
			assertResponse(t, res, err, RespEveSucc)

			req.Metadata["roleName"] = strings.ToUpper(roleStr)
			res, err = b.Invoke(ctx, req)
			assertResponse(t, res, err, RespEveSucc)
		}

		// revoke role
		req = &bindings.InvokeRequest{
			Operation: RevokeUserRoleOp,
			Metadata:  map[string]string{},
		}
		res, err = b.Invoke(ctx, req)
		assertResponse(t, res, err, RespEveFail)

		req.Metadata["userName"] = userName
		res, err = b.Invoke(ctx, req)
		assertResponse(t, res, err, RespEveFail)

		req.Metadata["roleName"] = "fakerole"
		res, err = b.Invoke(ctx, req)
		assertResponse(t, res, err, RespEveFail)

		for _, roleType := range []RoleType{ReadOnlyRole, ReadWriteRole, SuperUserRole} {
			roleStr := (string)(roleType)
			req.Metadata["roleName"] = roleStr
			res, err = b.Invoke(ctx, req)
			assertResponse(t, res, err, RespEveSucc)

			req.Metadata["roleName"] = strings.ToUpper(roleStr)
			res, err = b.Invoke(ctx, req)
			assertResponse(t, res, err, RespEveSucc)
		}
		// delete user
		req = &bindings.InvokeRequest{
			Operation: DeleteUserOp,
			Metadata:  map[string]string{},
		}
		res, err = b.Invoke(ctx, req)
		assertResponse(t, res, err, RespEveFail)
		req.Metadata["userName"] = userName
		res, err = b.Invoke(ctx, req)
		assertResponse(t, res, err, RespEveSucc)
	})
}

func assertResponse(t *testing.T, res *bindings.InvokeResponse, err error, event string) {
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.NotNil(t, res.Metadata)
	opsRes := OpsResult{}
	err = json.Unmarshal(res.Data, &opsRes)
	assert.NoError(t, err)
	t.Logf("ops result: %v", opsRes)
	assert.True(t, strings.Contains(opsRes["event"].(string), event))
}
