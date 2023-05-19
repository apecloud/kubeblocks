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

package mysql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/components-contrib/metadata"
	"github.com/spf13/viper"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/dapr/kit/logger"
	"github.com/stretchr/testify/assert"

	. "github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	. "github.com/apecloud/kubeblocks/cmd/probe/util"
)

const (
	urlWithPort   = "root:@tcp(127.0.0.1:3306)/mysql?multiStatements=true"
	urlWithNoPort = "root:@tcp(127.0.0.1)/mysql?multiStatements=true"
)

// Test case for Init() function
func TestInit(t *testing.T) {
	// Set up relevant viper config variables
	viper.Set("KB_SERVICE_USER", "testuser")
	viper.Set("KB_SERVICE_PASSWORD", "testpassword")

	mysqlOps, _, _ := mockDatabase(t)
	mysqlOps.Metadata.Properties["url"] = urlWithPort
	// Call the function being tested
	err := mysqlOps.Init(mysqlOps.Metadata)
	if err != nil {
		t.Errorf("Error during Init(): %s", err)
	}

	// Verify that the object is in the expected state after initialization
	assert.Equal(t, "mysql", mysqlOps.DBType)
	assert.NotNil(t, mysqlOps.InitIfNeed)
	assert.NotNil(t, mysqlOps.GetRole)
	assert.Equal(t, 3306, mysqlOps.DBPort)
	assert.NotNil(t, mysqlOps.OperationMap[GetRoleOperation])
	assert.NotNil(t, mysqlOps.OperationMap[CheckStatusOperation])
	assert.NotNil(t, mysqlOps.OperationMap[GetLagOperation])
	assert.NotNil(t, mysqlOps.OperationMap[ExecOperation])
	assert.NotNil(t, mysqlOps.OperationMap[QueryOperation])

	assert.NotNil(t, mysqlOps.OperationMap[ListUsersOp])
	assert.NotNil(t, mysqlOps.OperationMap[CreateUserOp])
	assert.NotNil(t, mysqlOps.OperationMap[DeleteUserOp])
	assert.NotNil(t, mysqlOps.OperationMap[DescribeUserOp])
	assert.NotNil(t, mysqlOps.OperationMap[GrantUserRoleOp])
	assert.NotNil(t, mysqlOps.OperationMap[RevokeUserRoleOp])
	// Clear out previously set viper variables
	viper.Reset()
}

func TestInitDelay(t *testing.T) {
	// Initialize a new instance of MysqlOperations.
	mysqlOps, _, _ := mockDatabase(t)
	// mysqlOps.initIfNeed()
	t.Run("Invalid url", func(t *testing.T) {
		mysqlOps.db = nil
		mysqlOps.initIfNeed()
		mysqlOps.Metadata.Properties["url"] = "invalid_url"
		err := mysqlOps.InitDelay()
		if err == nil {
			t.Errorf("Expected error but got none")
		}
	})

	t.Run("Invalid listen", func(t *testing.T) {
		mysqlOps.db = nil
		mysqlOps.Metadata.Properties["url"] = urlWithPort
		mysqlOps.Metadata.Properties[maxIdleConnsKey] = "100"
		mysqlOps.Metadata.Properties[connMaxIdleTimeKey] = "100ms"
		err := mysqlOps.InitDelay()
		if err == nil {
			t.Errorf("Expected error but got none")
		}
	})

	t.Run("Invalid pem", func(t *testing.T) {
		mysqlOps.db = nil
		mysqlOps.Metadata.Properties[pemPathKey] = "invalid.pem"
		mysqlOps.Metadata.Properties["url"] = urlWithPort
		err := mysqlOps.InitDelay()
		if err == nil {
			t.Errorf("Expected error but got none")
		}
	})
}

func TestGetRunningPort(t *testing.T) {
	mysqlOps, _, _ := mockDatabase(t)

	t.Run("Get port from url", func(t *testing.T) {
		mysqlOps.Metadata.Properties["url"] = urlWithPort
		port := mysqlOps.GetRunningPort()
		assert.Equal(t, 3306, port)
	})

	t.Run("Get default port if url has no port", func(t *testing.T) {
		mysqlOps.Metadata.Properties["url"] = urlWithNoPort
		port := mysqlOps.GetRunningPort()
		assert.Equal(t, defaultDBPort, port)
	})
}

func TestGetRole(t *testing.T) {
	mysqlOps, mock, _ := mockDatabase(t)

	t.Run("GetRole succeed", func(t *testing.T) {
		col1 := sqlmock.NewColumn("CURRENT_LEADER").OfType("VARCHAR", "")
		col2 := sqlmock.NewColumn("ROLE").OfType("VARCHAR", "")
		col3 := sqlmock.NewColumn("SERVER_ID").OfType("INT", 0)
		rows := sqlmock.NewRowsWithColumnDefinition(col1, col2, col3).AddRow("wesql-main-1.wesql-main-headless:13306", "Follower", 1)
		mock.ExpectQuery("select .* from information_schema.wesql_cluster_local").WillReturnRows(rows)

		role, err := mysqlOps.GetRole(context.Background(), &bindings.InvokeRequest{}, &bindings.InvokeResponse{})
		assert.Nil(t, err)
		assert.Equal(t, "Follower", role)
	})

	t.Run("GetRole fails", func(t *testing.T) {
		mock.ExpectQuery("select .* from information_schema.wesql_cluster_local").WillReturnError(errors.New("no record"))

		role, err := mysqlOps.GetRole(context.Background(), &bindings.InvokeRequest{}, &bindings.InvokeResponse{})
		assert.Equal(t, "", role)
		assert.NotNil(t, err)
	})
}

func TestGetLagOps(t *testing.T) {
	mysqlOps, mock, _ := mockDatabase(t)
	req := &bindings.InvokeRequest{Metadata: map[string]string{}}

	t.Run("GetLagOps succeed", func(t *testing.T) {
		col1 := sqlmock.NewColumn("CURRENT_LEADER").OfType("VARCHAR", "")
		col2 := sqlmock.NewColumn("ROLE").OfType("VARCHAR", "")
		col3 := sqlmock.NewColumn("SERVER_ID").OfType("INT", 0)
		rows := sqlmock.NewRowsWithColumnDefinition(col1, col2, col3).AddRow("wesql-main-1.wesql-main-headless:13306", "Follower", 1)
		getRoleRows := sqlmock.NewRowsWithColumnDefinition(col1, col2, col3).AddRow("wesql-main-1.wesql-main-headless:13306", "Follower", 1)
		if mysqlOps.OriRole == "" {
			mock.ExpectQuery("select .* from information_schema.wesql_cluster_local").WillReturnRows(getRoleRows)
		}
		mock.ExpectQuery("show slave status").WillReturnRows(rows)

		result, err := mysqlOps.GetLagOps(context.Background(), req, &bindings.InvokeResponse{})
		assert.NoError(t, err)

		// Assert that the event and message are correct
		event, ok := result["event"]
		assert.True(t, ok)
		assert.Equal(t, OperationSuccess, event)
	})
}

func TestQueryOps(t *testing.T) {
	mysqlOps, mock, _ := mockDatabase(t)
	req := &bindings.InvokeRequest{Metadata: map[string]string{}}
	req.Metadata["sql"] = "select .* from information_schema.wesql_cluster_local"

	t.Run("QueryOps succeed", func(t *testing.T) {
		col1 := sqlmock.NewColumn("CURRENT_LEADER").OfType("VARCHAR", "")
		col2 := sqlmock.NewColumn("ROLE").OfType("VARCHAR", "")
		col3 := sqlmock.NewColumn("SERVER_ID").OfType("INT", 0)
		rows := sqlmock.NewRowsWithColumnDefinition(col1, col2, col3).AddRow("wesql-main-1.wesql-main-headless:13306", "Follower", 1)
		mock.ExpectQuery("select .* from information_schema.wesql_cluster_local").WillReturnRows(rows)

		result, err := mysqlOps.QueryOps(context.Background(), req, &bindings.InvokeResponse{})
		assert.NoError(t, err)

		// Assert that the event and message are correct
		event, ok := result["event"]
		assert.True(t, ok)
		assert.Equal(t, OperationSuccess, event)

		message, ok := result["message"]
		assert.True(t, ok)
		t.Logf("query message: %s", message)
	})

	t.Run("QueryOps fails", func(t *testing.T) {
		mock.ExpectQuery("select .* from information_schema.wesql_cluster_local").WillReturnError(errors.New("no record"))

		result, err := mysqlOps.QueryOps(context.Background(), req, &bindings.InvokeResponse{})
		assert.NoError(t, err)

		// Assert that the event and message are correct
		event, ok := result["event"]
		assert.True(t, ok)
		assert.Equal(t, OperationFailed, event)

		message, ok := result["message"]
		assert.True(t, ok)
		t.Logf("query message: %s", message)
	})
}

func TestExecOps(t *testing.T) {
	mysqlOps, mock, _ := mockDatabase(t)
	req := &bindings.InvokeRequest{Metadata: map[string]string{}}
	req.Metadata["sql"] = "INSERT INTO foo (id, v1, ts) VALUES (1, 'test-1', '2021-01-22')"

	t.Run("ExecOps succeed", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO foo \\(id, v1, ts\\) VALUES \\(.*\\)").WillReturnResult(sqlmock.NewResult(1, 1))

		result, err := mysqlOps.ExecOps(context.Background(), req, &bindings.InvokeResponse{})
		assert.NoError(t, err)

		// Assert that the event and message are correct
		event, ok := result["event"]
		assert.True(t, ok)
		assert.Equal(t, OperationSuccess, event)

		count, ok := result["count"]
		assert.True(t, ok)
		assert.Equal(t, int64(1), count.(int64))
	})

	t.Run("ExecOps fails", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO foo \\(id, v1, ts\\) VALUES \\(.*\\)").WillReturnError(errors.New("insert error"))

		result, err := mysqlOps.ExecOps(context.Background(), req, &bindings.InvokeResponse{})
		assert.NoError(t, err)

		// Assert that the event and message are correct
		event, ok := result["event"]
		assert.True(t, ok)
		assert.Equal(t, OperationFailed, event)

		message, ok := result["message"]
		assert.True(t, ok)
		t.Logf("exec error message: %s", message)
	})
}

func TestCheckStatusOps(t *testing.T) {
	ctx := context.Background()
	req := &bindings.InvokeRequest{}
	resp := &bindings.InvokeResponse{Metadata: map[string]string{}}
	mysqlOps, mock, _ := mockDatabase(t)

	t.Run("Check follower", func(t *testing.T) {
		mysqlOps.OriRole = "follower"
		col1 := sqlmock.NewColumn("id").OfType("BIGINT", 1)
		col2 := sqlmock.NewColumn("type").OfType("BIGINT", 1)
		col3 := sqlmock.NewColumn("check_ts").OfType("TIME", time.Now())
		rows := sqlmock.NewRowsWithColumnDefinition(col1, col2, col3).
			AddRow(1, 1, time.Now())

		roSQL := fmt.Sprintf(`select check_ts from kb_health_check where type=%d limit 1;`, CheckStatusType)
		mock.ExpectQuery(roSQL).WillReturnRows(rows)
		// Call CheckStatusOps
		result, err := mysqlOps.CheckStatusOps(ctx, req, resp)
		assert.NoError(t, err)

		// Assert that the event and message are correct
		event, ok := result["event"]
		assert.True(t, ok)
		assert.Equal(t, OperationSuccess, event)

		message, ok := result["message"]
		assert.True(t, ok)
		t.Logf("check status message: %s", message)
	})

	t.Run("Check leader", func(t *testing.T) {
		mysqlOps.OriRole = "leader"
		rwSQL := fmt.Sprintf(`begin;
	create table if not exists kb_health_check(type int, check_ts bigint, primary key(type));
	insert into kb_health_check values(%d, now()) on duplicate key update check_ts = now();
	commit;
	select check_ts from kb_health_check where type=%d limit 1;`, CheckStatusType, CheckStatusType)
		mock.ExpectExec(regexp.QuoteMeta(rwSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
		// Call CheckStatusOps
		result, err := mysqlOps.CheckStatusOps(ctx, req, resp)
		assert.NoError(t, err)

		// Assert that the event and message are correct
		event, ok := result["event"]
		assert.True(t, ok)
		assert.Equal(t, OperationSuccess, event)

		message, ok := result["message"]
		assert.True(t, ok)
		t.Logf("check status message: %s", message)
	})

	t.Run("Role not configured", func(t *testing.T) {
		mysqlOps.OriRole = "leader1"
		// Call CheckStatusOps
		result, err := mysqlOps.CheckStatusOps(ctx, req, resp)
		assert.NoError(t, err)

		// Assert that the event and message are correct
		event, ok := result["event"]
		assert.True(t, ok)
		assert.Equal(t, OperationSuccess, event)

		message, ok := result["message"]
		assert.True(t, ok)
		assert.True(t, strings.HasPrefix(message.(string), "unknown access mode for role"))
		t.Logf("check status message: %s", message)
	})

	t.Run("Check failed", func(t *testing.T) {
		mysqlOps.OriRole = "leader"
		rwSQL := fmt.Sprintf(`begin;
	create table if not exists kb_health_check(type int, check_ts bigint, primary key(type));
	insert into kb_health_check values(%d, now()) on duplicate key update check_ts = now();
	commit;
	select check_ts from kb_health_check where type=%d limit 1;`, CheckStatusType, CheckStatusType)
		mock.ExpectExec(regexp.QuoteMeta(rwSQL)).WillReturnError(errors.New("insert error"))
		// Call CheckStatusOps
		result, err := mysqlOps.CheckStatusOps(ctx, req, resp)
		assert.NoError(t, err)

		// Assert that the event and message are correct
		event, ok := result["event"]
		assert.True(t, ok)
		assert.Equal(t, OperationFailed, event)

		message, ok := result["message"]
		assert.True(t, ok)
		t.Logf("check status message: %s", message)
	})
}

func TestQuery(t *testing.T) {
	mysqlOps, mock, _ := mockDatabase(t)

	t.Run("no dbType provided", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "value", "timestamp"}).
			AddRow(1, "value-1", time.Now()).
			AddRow(2, "value-2", time.Now().Add(1000)).
			AddRow(3, "value-3", time.Now().Add(2000))

		mock.ExpectQuery("SELECT \\* FROM foo WHERE id < 4").WillReturnRows(rows)
		ret, err := mysqlOps.query(context.Background(), `SELECT * FROM foo WHERE id < 4`)
		assert.Nil(t, err)
		t.Logf("query result: %s", ret)
		assert.Contains(t, string(ret), "\"id\":\"1")
		var result []interface{}
		err = json.Unmarshal(ret, &result)
		assert.Nil(t, err)
		assert.Equal(t, 3, len(result))
	})

	t.Run("dbType provided", func(t *testing.T) {
		col1 := sqlmock.NewColumn("id").OfType("BIGINT", 1)
		col2 := sqlmock.NewColumn("value").OfType("FLOAT", 1.0)
		col3 := sqlmock.NewColumn("timestamp").OfType("TIME", time.Now())
		rows := sqlmock.NewRowsWithColumnDefinition(col1, col2, col3).
			AddRow(1, 1.1, time.Now()).
			AddRow(2, 2.2, time.Now().Add(1000)).
			AddRow(3, 3.3, time.Now().Add(2000))
		mock.ExpectQuery("SELECT \\* FROM foo WHERE id < 4").WillReturnRows(rows)
		ret, err := mysqlOps.query(context.Background(), "SELECT * FROM foo WHERE id < 4")
		assert.Nil(t, err)
		t.Logf("query result: %s", ret)

		// verify number
		assert.Contains(t, string(ret), "\"id\":1")
		assert.Contains(t, string(ret), "\"value\":2.2")

		var result []interface{}
		err = json.Unmarshal(ret, &result)
		assert.Nil(t, err)
		assert.Equal(t, 3, len(result))

		// verify timestamp
		ts, ok := result[0].(map[string]interface{})["timestamp"].(string)
		assert.True(t, ok)
		var tt time.Time
		tt, err = time.Parse(time.RFC3339, ts)
		assert.Nil(t, err)
		t.Logf("time stamp is: %v", tt)
	})
}

func TestExec(t *testing.T) {
	mysqlOps, mock, _ := mockDatabase(t)
	mock.ExpectExec("INSERT INTO foo \\(id, v1, ts\\) VALUES \\(.*\\)").WillReturnResult(sqlmock.NewResult(1, 1))
	i, err := mysqlOps.exec(context.Background(), "INSERT INTO foo (id, v1, ts) VALUES (1, 'test-1', '2021-01-22')")
	assert.Equal(t, int64(1), i)
	assert.Nil(t, err)
}

func TestMySQLAccounts(t *testing.T) {
	ctx := context.Background()
	resp := &bindings.InvokeResponse{Metadata: map[string]string{}}
	mysqlOps, mock, _ := mockDatabase(t)

	const (
		userName = "turning"
		password = "red"
		roleName = "readOnly"
	)
	t.Run("Create account", func(t *testing.T) {
		var err error
		var result OpsResult

		req := &bindings.InvokeRequest{}
		req.Operation = CreateUserOp
		req.Metadata = map[string]string{}

		result, err = mysqlOps.createUserOps(ctx, req, resp)
		assert.Nil(t, err)
		assert.Equal(t, RespEveFail, result[RespTypEve])
		assert.Equal(t, ErrNoUserName.Error(), result[RespTypMsg])

		req.Metadata["userName"] = userName
		result, err = mysqlOps.createUserOps(ctx, req, resp)
		assert.Nil(t, err)
		assert.Equal(t, RespEveFail, result[RespTypEve])
		assert.Equal(t, ErrNoPassword.Error(), result[RespTypMsg])

		req.Metadata["password"] = password

		createUserCmd := fmt.Sprintf("CREATE USER '%s'@'%%' IDENTIFIED BY '%s';", req.Metadata["userName"], req.Metadata["password"])
		mock.ExpectExec(createUserCmd).WillReturnResult(sqlmock.NewResult(1, 1))
		result, err = mysqlOps.createUserOps(ctx, req, resp)
		assert.Nil(t, err)
		assert.Equal(t, RespEveSucc, result[RespTypEve], result[RespTypMsg])
	})

	t.Run("Delete account", func(t *testing.T) {
		var err error
		var result OpsResult

		req := &bindings.InvokeRequest{}
		req.Operation = CreateUserOp
		req.Metadata = map[string]string{}

		result, err = mysqlOps.deleteUserOps(ctx, req, resp)
		assert.Nil(t, err)
		assert.Equal(t, RespEveFail, result[RespTypEve])
		assert.Equal(t, ErrNoUserName.Error(), result[RespTypMsg])

		req.Metadata["userName"] = userName
		deleteUserCmd := fmt.Sprintf("DROP USER IF EXISTS '%s'@'%%';", req.Metadata["userName"])
		mock.ExpectExec(deleteUserCmd).WillReturnResult(sqlmock.NewResult(1, 1))

		result, err = mysqlOps.deleteUserOps(ctx, req, resp)
		assert.Nil(t, err)
		assert.Equal(t, RespEveSucc, result[RespTypEve], result[RespTypMsg])
	})
	t.Run("Describe account", func(t *testing.T) {
		var err error
		var result OpsResult

		req := &bindings.InvokeRequest{}
		req.Operation = CreateUserOp
		req.Metadata = map[string]string{}

		col1 := sqlmock.NewColumn("Grants for "+userName+"@%").OfType("STRING", "turning")
		rows := sqlmock.NewRowsWithColumnDefinition(col1).AddRow(readOnlyRPriv)

		result, err = mysqlOps.describeUserOps(ctx, req, resp)
		assert.Nil(t, err)
		assert.Equal(t, RespEveFail, result[RespTypEve])
		assert.Equal(t, ErrNoUserName.Error(), result[RespTypMsg])

		req.Metadata["userName"] = userName

		showGrantTpl := "SHOW GRANTS FOR '%s'@'%%';"
		descUserCmd := fmt.Sprintf(showGrantTpl, req.Metadata["userName"])
		mock.ExpectQuery(descUserCmd).WillReturnRows(rows)

		result, err = mysqlOps.describeUserOps(ctx, req, resp)
		assert.Nil(t, err)
		assert.Equal(t, RespEveSucc, result[RespTypEve])

		data := result[RespTypMsg].(string)
		users := []UserInfo{}
		err = json.Unmarshal([]byte(data), &users)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(users))
		assert.Equal(t, userName, users[0].UserName)
		assert.NotEmpty(t, users[0].RoleName)
		assert.True(t, ReadOnlyRole.EqualTo(users[0].RoleName))
	})

	t.Run("List accounts", func(t *testing.T) {
		var err error
		var result OpsResult

		req := &bindings.InvokeRequest{}
		req.Operation = CreateUserOp
		req.Metadata = map[string]string{}

		col1 := sqlmock.NewColumn("userName").OfType("STRING", "turning")
		col2 := sqlmock.NewColumn("expired").OfType("STRING", "T")

		rows := sqlmock.NewRowsWithColumnDefinition(col1, col2).
			AddRow(userName, "T").AddRow("testuser", "F")

		listUserCmd := "SELECT user AS userName, CASE password_expired WHEN 'N' THEN 'F' ELSE 'T' END as expired FROM mysql.user WHERE host = '%' and user <> 'root' and user not like 'kb%';"
		mock.ExpectQuery(regexp.QuoteMeta(listUserCmd)).WillReturnRows(rows)

		result, err = mysqlOps.listUsersOps(ctx, req, resp)
		assert.Nil(t, err)
		assert.Equal(t, RespEveSucc, result[RespTypEve], result[RespTypMsg])
		data := result[RespTypMsg].(string)
		users := []UserInfo{}
		err = json.Unmarshal([]byte(data), &users)
		assert.Nil(t, err)
		assert.Equal(t, 2, len(users))
		assert.Equal(t, userName, users[0].UserName)
	})

	t.Run("Grant Roles", func(t *testing.T) {
		var err error
		var result OpsResult

		req := &bindings.InvokeRequest{}
		req.Operation = CreateUserOp
		req.Metadata = map[string]string{}

		result, err = mysqlOps.grantUserRoleOps(ctx, req, resp)
		assert.Nil(t, err)
		assert.Equal(t, RespEveFail, result[RespTypEve])
		assert.Equal(t, ErrNoUserName.Error(), result[RespTypMsg])

		req.Metadata["userName"] = userName
		result, err = mysqlOps.grantUserRoleOps(ctx, req, resp)
		assert.Nil(t, err)
		assert.Equal(t, RespEveFail, result[RespTypEve])
		assert.Equal(t, ErrNoRoleName.Error(), result[RespTypMsg])

		req.Metadata["roleName"] = roleName
		roleDesc, err := mysqlOps.role2Priv(req.Metadata["roleName"])
		assert.Nil(t, err)
		grantRoleCmd := fmt.Sprintf("GRANT %s TO '%s'@'%%';", roleDesc, req.Metadata["userName"])

		mock.ExpectExec(grantRoleCmd).WillReturnResult(sqlmock.NewResult(1, 1))
		result, err = mysqlOps.grantUserRoleOps(ctx, req, resp)
		assert.Nil(t, err)
		assert.Equal(t, RespEveSucc, result[RespTypEve], result[RespTypMsg])
	})

	t.Run("Revoke Roles", func(t *testing.T) {
		var err error
		var result OpsResult

		req := &bindings.InvokeRequest{}
		req.Operation = CreateUserOp
		req.Metadata = map[string]string{}

		result, err = mysqlOps.revokeUserRoleOps(ctx, req, resp)
		assert.Nil(t, err)
		assert.Equal(t, RespEveFail, result[RespTypEve])
		assert.Equal(t, ErrNoUserName.Error(), result[RespTypMsg])

		req.Metadata["userName"] = userName
		result, err = mysqlOps.revokeUserRoleOps(ctx, req, resp)
		assert.Nil(t, err)
		assert.Equal(t, RespEveFail, result[RespTypEve])
		assert.Equal(t, ErrNoRoleName.Error(), result[RespTypMsg])

		req.Metadata["roleName"] = roleName
		roleDesc, err := mysqlOps.role2Priv(req.Metadata["roleName"])
		assert.Nil(t, err)
		revokeRoleCmd := fmt.Sprintf("REVOKE %s FROM '%s'@'%%';", roleDesc, req.Metadata["userName"])

		mock.ExpectExec(revokeRoleCmd).WillReturnResult(sqlmock.NewResult(1, 1))
		result, err = mysqlOps.revokeUserRoleOps(ctx, req, resp)
		assert.Nil(t, err)
		assert.Equal(t, RespEveSucc, result[RespTypEve], result[RespTypMsg])
	})
	t.Run("List System Accounts", func(t *testing.T) {
		var err error
		var result OpsResult

		req := &bindings.InvokeRequest{}
		req.Operation = CreateUserOp
		req.Metadata = map[string]string{}

		col1 := sqlmock.NewColumn("userName").OfType("STRING", "turning")

		rows := sqlmock.NewRowsWithColumnDefinition(col1).
			AddRow("kbadmin")

		stmt := "SELECT user AS userName FROM mysql.user WHERE host = '%' and user like 'kb%';"
		mock.ExpectQuery(regexp.QuoteMeta(stmt)).WillReturnRows(rows)

		result, err = mysqlOps.listSystemAccountsOps(ctx, req, resp)
		assert.Nil(t, err)
		assert.Equal(t, RespEveSucc, result[RespTypEve], result[RespTypMsg])
		data := result[RespTypMsg].(string)
		users := []string{}
		err = json.Unmarshal([]byte(data), &users)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(users))
		assert.Equal(t, "kbadmin", users[0])
	})
}
func mockDatabase(t *testing.T) (*MysqlOperations, sqlmock.Sqlmock, error) {
	viper.SetDefault("KB_SERVICE_ROLES", "{\"follower\":\"Readonly\",\"leader\":\"ReadWrite\"}")
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	metadata := bindings.Metadata{
		Base: metadata.Base{
			Properties: map[string]string{},
		},
	}
	mysqlOps := NewMysql(logger.NewLogger("test")).(*MysqlOperations)
	_ = mysqlOps.Init(metadata)
	mysqlOps.db = db

	return mysqlOps, mock, err
}
