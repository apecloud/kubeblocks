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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
)

const (
	urlWithPort   = "root:@tcp(127.0.0.1:3306)/mysql?multiStatements=true"
	urlWithNoPort = "root:@tcp(127.0.0.1)/mysql?multiStatements=true"
)

// Test case for Init() function
var _ = Describe("MySQL DBManager", func() {
	// Set up relevant viper config variables
	viper.Set("KB_SERVICE_USER", "testuser")
	viper.Set("KB_SERVICE_PASSWORD", "testpassword")
	Context("new db manager", func() {
		It("with rigth configurations", func() {
			properties := engines.Properties{
				"url": urlWithPort,
			}
			dbManger, err := NewManager(properties)
			Expect(err).Should(Succeed())
			Expect(dbManger).ShouldNot(BeNil())
		})

		It("with wrong configurations", func() {
			properties := engines.Properties{
				"url": "wrong-url-format",
			}
			dbManger, err := NewManager(properties)
			Expect(err).Should(HaveOccurred())
			Expect(dbManger).Should(BeNil())
		})
	})
})

// func TestGetRole(t *testing.T) {
// 	mysqlOps, mock, _ := mockDatabase(t)
//
// 	t.Run("GetRole succeed", func(t *testing.T) {
// 		col1 := sqlmock.NewColumn("CURRENT_LEADER").OfType("VARCHAR", "")
// 		col2 := sqlmock.NewColumn("ROLE").OfType("VARCHAR", "")
// 		col3 := sqlmock.NewColumn("SERVER_ID").OfType("INT", 0)
// 		rows := sqlmock.NewRowsWithColumnDefinition(col1, col2, col3).AddRow("wesql-main-1.wesql-main-headless:13306", "Follower", 1)
// 		mock.ExpectQuery("select .* from information_schema.wesql_cluster_local").WillReturnRows(rows)
//
// 		role, err := mysqlOps.GetRole(context.Background(), &ProbeRequest{}, &ProbeResponse{})
// 		assert.Nil(t, err)
// 		assert.Equal(t, "Follower", role)
// 	})
//
// 	t.Run("GetRole fails", func(t *testing.T) {
// 		mock.ExpectQuery("select .* from information_schema.wesql_cluster_local").WillReturnError(errors.New("no record"))
//
// 		role, err := mysqlOps.GetRole(context.Background(), &ProbeRequest{}, &ProbeResponse{})
// 		assert.Equal(t, "", role)
// 		assert.NotNil(t, err)
// 	})
// }
//
// func TestGetLagOps(t *testing.T) {
// 	mysqlOps, mock, _ := mockDatabase(t)
// 	req := &ProbeRequest{}
//
// 	t.Run("GetLagOps succeed", func(t *testing.T) {
// 		col1 := sqlmock.NewColumn("CURRENT_LEADER").OfType("VARCHAR", "")
// 		col2 := sqlmock.NewColumn("ROLE").OfType("VARCHAR", "")
// 		col3 := sqlmock.NewColumn("SERVER_ID").OfType("INT", 0)
// 		rows := sqlmock.NewRowsWithColumnDefinition(col1, col2, col3).AddRow("wesql-main-1.wesql-main-headless:13306", "Follower", 1)
// 		getRoleRows := sqlmock.NewRowsWithColumnDefinition(col1, col2, col3).AddRow("wesql-main-1.wesql-main-headless:13306", "Follower", 1)
// 		if mysqlOps.OriRole == "" {
// 			mock.ExpectQuery("select .* from information_schema.wesql_cluster_local").WillReturnRows(getRoleRows)
// 		}
// 		mock.ExpectQuery("show slave status").WillReturnRows(rows)
//
// 		result, err := mysqlOps.GetLagOps(context.Background(), req, &ProbeResponse{})
// 		assert.NoError(t, err)
//
// 		// Assert that the event and message are correct
// 		event, ok := result["event"]
// 		assert.True(t, ok)
// 		assert.Equal(t, util.OperationSuccess, event)
// 	})
// }
//
// func TestQueryOps(t *testing.T) {
// 	mysqlOps, mock, _ := mockDatabase(t)
// 	req := &ProbeRequest{Metadata: map[string]string{}}
// 	req.Metadata["sql"] = "select .* from information_schema.wesql_cluster_local"
//
// 	t.Run("QueryOps succeed", func(t *testing.T) {
// 		col1 := sqlmock.NewColumn("CURRENT_LEADER").OfType("VARCHAR", "")
// 		col2 := sqlmock.NewColumn("ROLE").OfType("VARCHAR", "")
// 		col3 := sqlmock.NewColumn("SERVER_ID").OfType("INT", 0)
// 		rows := sqlmock.NewRowsWithColumnDefinition(col1, col2, col3).AddRow("wesql-main-1.wesql-main-headless:13306", "Follower", 1)
// 		mock.ExpectQuery("select .* from information_schema.wesql_cluster_local").WillReturnRows(rows)
//
// 		result, err := mysqlOps.QueryOps(context.Background(), req, &ProbeResponse{})
// 		assert.NoError(t, err)
//
// 		// Assert that the event and message are correct
// 		event, ok := result["event"]
// 		assert.True(t, ok)
// 		assert.Equal(t, util.OperationSuccess, event)
//
// 		message, ok := result["message"]
// 		assert.True(t, ok)
// 		t.Logf("query message: %s", message)
// 	})
//
// 	t.Run("QueryOps fails", func(t *testing.T) {
// 		mock.ExpectQuery("select .* from information_schema.wesql_cluster_local").WillReturnError(errors.New("no record"))
//
// 		result, err := mysqlOps.QueryOps(context.Background(), req, &ProbeResponse{})
// 		assert.NoError(t, err)
//
// 		// Assert that the event and message are correct
// 		event, ok := result["event"]
// 		assert.True(t, ok)
// 		assert.Equal(t, util.OperationFailed, event)
//
// 		message, ok := result["message"]
// 		assert.True(t, ok)
// 		t.Logf("query message: %s", message)
// 	})
// }
//
// func TestExecOps(t *testing.T) {
// 	mysqlOps, mock, _ := mockDatabase(t)
// 	req := &ProbeRequest{Metadata: map[string]string{}}
// 	req.Metadata["sql"] = "INSERT INTO foo (id, v1, ts) VALUES (1, 'test-1', '2021-01-22')"
//
// 	t.Run("ExecOps succeed", func(t *testing.T) {
// 		mock.ExpectExec("INSERT INTO foo \\(id, v1, ts\\) VALUES \\(.*\\)").WillReturnResult(sqlmock.NewResult(1, 1))
//
// 		result, err := mysqlOps.ExecOps(context.Background(), req, &ProbeResponse{})
// 		assert.NoError(t, err)
//
// 		// Assert that the event and message are correct
// 		event, ok := result["event"]
// 		assert.True(t, ok)
// 		assert.Equal(t, util.OperationSuccess, event)
//
// 		count, ok := result["count"]
// 		assert.True(t, ok)
// 		assert.Equal(t, int64(1), count.(int64))
// 	})
//
// 	t.Run("ExecOps fails", func(t *testing.T) {
// 		mock.ExpectExec("INSERT INTO foo \\(id, v1, ts\\) VALUES \\(.*\\)").WillReturnError(errors.New("insert error"))
//
// 		result, err := mysqlOps.ExecOps(context.Background(), req, &ProbeResponse{})
// 		assert.NoError(t, err)
//
// 		// Assert that the event and message are correct
// 		event, ok := result["event"]
// 		assert.True(t, ok)
// 		assert.Equal(t, util.OperationFailed, event)
//
// 		message, ok := result["message"]
// 		assert.True(t, ok)
// 		t.Logf("exec error message: %s", message)
// 	})
// }
//
// func TestCheckStatusOps(t *testing.T) {
// 	ctx := context.Background()
// 	req := &ProbeRequest{}
// 	resp := &ProbeResponse{Metadata: map[string]string{}}
// 	mysqlOps, mock, _ := mockDatabase(t)
//
// 	t.Run("Check follower", func(t *testing.T) {
// 		mysqlOps.OriRole = "follower"
// 		col1 := sqlmock.NewColumn("id").OfType("BIGINT", 1)
// 		col2 := sqlmock.NewColumn("type").OfType("BIGINT", 1)
// 		col3 := sqlmock.NewColumn("check_ts").OfType("TIME", time.Now())
// 		rows := sqlmock.NewRowsWithColumnDefinition(col1, col2, col3).
// 			AddRow(1, 1, time.Now())
//
// 		roSQL := fmt.Sprintf(`select check_ts from kb_health_check where type=%d limit 1;`, component.CheckStatusType)
// 		mock.ExpectQuery(roSQL).WillReturnRows(rows)
// 		// Call CheckStatusOps
// 		result, err := mysqlOps.CheckStatusOps(ctx, req, resp)
// 		assert.NoError(t, err)
//
// 		// Assert that the event and message are correct
// 		event, ok := result["event"]
// 		assert.True(t, ok)
// 		assert.Equal(t, util.OperationSuccess, event)
//
// 		message, ok := result["message"]
// 		assert.True(t, ok)
// 		t.Logf("check status message: %s", message)
// 	})
//
// 	t.Run("Check leader", func(t *testing.T) {
// 		mysqlOps.OriRole = "leader"
// 		rwSQL := fmt.Sprintf(`begin;
// 	create table if not exists kb_health_check(type int, check_ts bigint, primary key(type));
// 	insert into kb_health_check values(%d, now()) on duplicate key update check_ts = now();
// 	commit;
// 	select check_ts from kb_health_check where type=%d limit 1;`, component.CheckStatusType, component.CheckStatusType)
// 		mock.ExpectExec(regexp.QuoteMeta(rwSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
// 		// Call CheckStatusOps
// 		result, err := mysqlOps.CheckStatusOps(ctx, req, resp)
// 		assert.NoError(t, err)
//
// 		// Assert that the event and message are correct
// 		event, ok := result["event"]
// 		assert.True(t, ok)
// 		assert.Equal(t, util.OperationSuccess, event)
//
// 		message, ok := result["message"]
// 		assert.True(t, ok)
// 		t.Logf("check status message: %s", message)
// 	})
//
// 	t.Run("Role not configured", func(t *testing.T) {
// 		mysqlOps.OriRole = "leader1"
// 		// Call CheckStatusOps
// 		result, err := mysqlOps.CheckStatusOps(ctx, req, resp)
// 		assert.NoError(t, err)
//
// 		// Assert that the event and message are correct
// 		event, ok := result["event"]
// 		assert.True(t, ok)
// 		assert.Equal(t, util.OperationSuccess, event)
//
// 		message, ok := result["message"]
// 		assert.True(t, ok)
// 		assert.True(t, strings.HasPrefix(message.(string), "unknown access mode for role"))
// 		t.Logf("check status message: %s", message)
// 	})
//
// 	t.Run("Check failed", func(t *testing.T) {
// 		mysqlOps.OriRole = "leader"
// 		rwSQL := fmt.Sprintf(`begin;
// 	create table if not exists kb_health_check(type int, check_ts bigint, primary key(type));
// 	insert into kb_health_check values(%d, now()) on duplicate key update check_ts = now();
// 	commit;
// 	select check_ts from kb_health_check where type=%d limit 1;`, component.CheckStatusType, component.CheckStatusType)
// 		mock.ExpectExec(regexp.QuoteMeta(rwSQL)).WillReturnError(errors.New("insert error"))
// 		// Call CheckStatusOps
// 		result, err := mysqlOps.CheckStatusOps(ctx, req, resp)
// 		assert.NoError(t, err)
//
// 		// Assert that the event and message are correct
// 		event, ok := result["event"]
// 		assert.True(t, ok)
// 		assert.Equal(t, util.OperationFailed, event)
//
// 		message, ok := result["message"]
// 		assert.True(t, ok)
// 		t.Logf("check status message: %s", message)
// 	})
// }
//
// func TestMySQLAccounts(t *testing.T) {
// 	ctx := context.Background()
// 	resp := &ProbeResponse{}
// 	mysqlOps, mock, _ := mockDatabase(t)
//
// 	const (
// 		userName = "turning"
// 		password = "red"
// 		roleName = "readOnly"
// 	)
// 	t.Run("Create account", func(t *testing.T) {
// 		var err error
// 		var result OpsResult
//
// 		req := &ProbeRequest{}
// 		req.Operation = util.CreateUserOp
// 		req.Metadata = map[string]string{}
//
// 		result, err = mysqlOps.createUserOps(ctx, req, resp)
// 		assert.Nil(t, err)
// 		assert.Equal(t, util.RespEveFail, result[util.RespFieldEvent])
// 		assert.Equal(t, ErrNoUserName.Error(), result[util.RespFieldMessage])
//
// 		req.Metadata["userName"] = userName
// 		result, err = mysqlOps.createUserOps(ctx, req, resp)
// 		assert.Nil(t, err)
// 		assert.Equal(t, util.RespEveFail, result[util.RespFieldEvent])
// 		assert.Equal(t, ErrNoPassword.Error(), result[util.RespFieldMessage])
//
// 		req.Metadata["password"] = password
//
// 		createUserCmd := fmt.Sprintf("CREATE USER '%s'@'%%' IDENTIFIED BY '%s';", req.Metadata["userName"], req.Metadata["password"])
// 		mock.ExpectExec(createUserCmd).WillReturnResult(sqlmock.NewResult(1, 1))
// 		result, err = mysqlOps.createUserOps(ctx, req, resp)
// 		assert.Nil(t, err)
// 		assert.Equal(t, util.RespEveSucc, result[util.RespFieldEvent], result[util.RespFieldMessage])
// 	})
//
// 	t.Run("Delete account", func(t *testing.T) {
// 		var err error
// 		var result OpsResult
//
// 		req := &ProbeRequest{}
// 		req.Operation = util.CreateUserOp
// 		req.Metadata = map[string]string{}
//
// 		result, err = mysqlOps.deleteUserOps(ctx, req, resp)
// 		assert.Nil(t, err)
// 		assert.Equal(t, util.RespEveFail, result[util.RespFieldEvent])
// 		assert.Equal(t, ErrNoUserName.Error(), result[util.RespFieldMessage])
//
// 		req.Metadata["userName"] = userName
// 		deleteUserCmd := fmt.Sprintf("DROP USER IF EXISTS '%s'@'%%';", req.Metadata["userName"])
// 		mock.ExpectExec(deleteUserCmd).WillReturnResult(sqlmock.NewResult(1, 1))
//
// 		result, err = mysqlOps.deleteUserOps(ctx, req, resp)
// 		assert.Nil(t, err)
// 		assert.Equal(t, util.RespEveSucc, result[util.RespFieldEvent], result[util.RespFieldMessage])
// 	})
// 	t.Run("Describe account", func(t *testing.T) {
// 		var err error
// 		var result OpsResult
//
// 		req := &ProbeRequest{}
// 		req.Operation = util.CreateUserOp
// 		req.Metadata = map[string]string{}
//
// 		col1 := sqlmock.NewColumn("Grants for "+userName+"@%").OfType("STRING", "turning")
// 		rows := sqlmock.NewRowsWithColumnDefinition(col1).AddRow(readOnlyRPriv)
//
// 		result, err = mysqlOps.describeUserOps(ctx, req, resp)
// 		assert.Nil(t, err)
// 		assert.Equal(t, util.RespEveFail, result[util.RespFieldEvent])
// 		assert.Equal(t, ErrNoUserName.Error(), result[util.RespFieldMessage])
//
// 		req.Metadata["userName"] = userName
//
// 		showGrantTpl := "SHOW GRANTS FOR '%s'@'%%';"
// 		descUserCmd := fmt.Sprintf(showGrantTpl, req.Metadata["userName"])
// 		mock.ExpectQuery(descUserCmd).WillReturnRows(rows)
//
// 		result, err = mysqlOps.describeUserOps(ctx, req, resp)
// 		assert.Nil(t, err)
// 		assert.Equal(t, util.RespEveSucc, result[util.RespFieldEvent])
//
// 		data := result[util.RespFieldMessage].(string)
// 		users := []util.UserInfo{}
// 		err = json.Unmarshal([]byte(data), &users)
// 		assert.Nil(t, err)
// 		assert.Equal(t, 1, len(users))
// 		assert.Equal(t, userName, users[0].UserName)
// 		assert.NotEmpty(t, users[0].RoleName)
// 		assert.True(t, util.ReadOnlyRole.EqualTo(users[0].RoleName))
// 	})
//
// 	t.Run("List accounts", func(t *testing.T) {
// 		var err error
// 		var result OpsResult
//
// 		req := &ProbeRequest{}
// 		req.Operation = util.CreateUserOp
// 		req.Metadata = map[string]string{}
//
// 		col1 := sqlmock.NewColumn("userName").OfType("STRING", "turning")
// 		col2 := sqlmock.NewColumn("expired").OfType("STRING", "T")
//
// 		rows := sqlmock.NewRowsWithColumnDefinition(col1, col2).
// 			AddRow(userName, "T").AddRow("testuser", "F")
//
// 		listUserCmd := "SELECT user AS userName, CASE password_expired WHEN 'N' THEN 'F' ELSE 'T' END as expired FROM mysql.user WHERE host = '%' and user <> 'root' and user not like 'kb%';"
// 		mock.ExpectQuery(regexp.QuoteMeta(listUserCmd)).WillReturnRows(rows)
//
// 		result, err = mysqlOps.listUsersOps(ctx, req, resp)
// 		assert.Nil(t, err)
// 		assert.Equal(t, util.RespEveSucc, result[util.RespFieldEvent], result[util.RespFieldMessage])
// 		data := result[util.RespFieldMessage].(string)
// 		users := []util.UserInfo{}
// 		err = json.Unmarshal([]byte(data), &users)
// 		assert.Nil(t, err)
// 		assert.Equal(t, 2, len(users))
// 		assert.Equal(t, userName, users[0].UserName)
// 	})
//
// 	t.Run("Grant Roles", func(t *testing.T) {
// 		var err error
// 		var result OpsResult
//
// 		req := &ProbeRequest{}
// 		req.Operation = util.CreateUserOp
// 		req.Metadata = map[string]string{}
//
// 		result, err = mysqlOps.grantUserRoleOps(ctx, req, resp)
// 		assert.Nil(t, err)
// 		assert.Equal(t, util.RespEveFail, result[util.RespFieldEvent])
// 		assert.Equal(t, ErrNoUserName.Error(), result[util.RespFieldMessage])
//
// 		req.Metadata["userName"] = userName
// 		result, err = mysqlOps.grantUserRoleOps(ctx, req, resp)
// 		assert.Nil(t, err)
// 		assert.Equal(t, util.RespEveFail, result[util.RespFieldEvent])
// 		assert.Equal(t, ErrNoRoleName.Error(), result[util.RespFieldMessage])
//
// 		req.Metadata["roleName"] = roleName
// 		roleDesc, err := mysqlOps.role2Priv(req.Metadata["roleName"])
// 		assert.Nil(t, err)
// 		grantRoleCmd := fmt.Sprintf("GRANT %s TO '%s'@'%%';", roleDesc, req.Metadata["userName"])
//
// 		mock.ExpectExec(grantRoleCmd).WillReturnResult(sqlmock.NewResult(1, 1))
// 		result, err = mysqlOps.grantUserRoleOps(ctx, req, resp)
// 		assert.Nil(t, err)
// 		assert.Equal(t, util.RespEveSucc, result[util.RespFieldEvent], result[util.RespFieldMessage])
// 	})
//
// 	t.Run("Revoke Roles", func(t *testing.T) {
// 		var err error
// 		var result OpsResult
//
// 		req := &ProbeRequest{}
// 		req.Operation = util.CreateUserOp
// 		req.Metadata = map[string]string{}
//
// 		result, err = mysqlOps.revokeUserRoleOps(ctx, req, resp)
// 		assert.Nil(t, err)
// 		assert.Equal(t, util.RespEveFail, result[util.RespFieldEvent])
// 		assert.Equal(t, ErrNoUserName.Error(), result[util.RespFieldMessage])
//
// 		req.Metadata["userName"] = userName
// 		result, err = mysqlOps.revokeUserRoleOps(ctx, req, resp)
// 		assert.Nil(t, err)
// 		assert.Equal(t, util.RespEveFail, result[util.RespFieldEvent])
// 		assert.Equal(t, ErrNoRoleName.Error(), result[util.RespFieldMessage])
//
// 		req.Metadata["roleName"] = roleName
// 		roleDesc, err := mysqlOps.role2Priv(req.Metadata["roleName"])
// 		assert.Nil(t, err)
// 		revokeRoleCmd := fmt.Sprintf("REVOKE %s FROM '%s'@'%%';", roleDesc, req.Metadata["userName"])
//
// 		mock.ExpectExec(revokeRoleCmd).WillReturnResult(sqlmock.NewResult(1, 1))
// 		result, err = mysqlOps.revokeUserRoleOps(ctx, req, resp)
// 		assert.Nil(t, err)
// 		assert.Equal(t, util.RespEveSucc, result[util.RespFieldEvent], result[util.RespFieldMessage])
// 	})
// 	t.Run("List System Accounts", func(t *testing.T) {
// 		var err error
// 		var result OpsResult
//
// 		req := &ProbeRequest{}
// 		req.Operation = util.CreateUserOp
// 		req.Metadata = map[string]string{}
//
// 		col1 := sqlmock.NewColumn("userName").OfType("STRING", "turning")
//
// 		rows := sqlmock.NewRowsWithColumnDefinition(col1).
// 			AddRow("kbadmin")
//
// 		stmt := "SELECT user AS userName FROM mysql.user WHERE host = '%' and user like 'kb%';"
// 		mock.ExpectQuery(regexp.QuoteMeta(stmt)).WillReturnRows(rows)
//
// 		result, err = mysqlOps.listSystemAccountsOps(ctx, req, resp)
// 		assert.Nil(t, err)
// 		assert.Equal(t, util.RespEveSucc, result[util.RespFieldEvent], result[util.RespFieldMessage])
// 		data := result[util.RespFieldMessage].(string)
// 		users := []string{}
// 		err = json.Unmarshal([]byte(data), &users)
// 		assert.Nil(t, err)
// 		assert.Equal(t, 1, len(users))
// 		assert.Equal(t, "kbadmin", users[0])
// 	})
// }
// func mockDatabase(t *testing.T) (*MysqlOperations, sqlmock.Sqlmock, error) {
// 	viper.SetDefault("KB_SERVICE_ROLES", "{\"follower\":\"Readonly\",\"leader\":\"ReadWrite\"}")
// 	viper.Set("KB_POD_NAME", "test-pod-0")
// 	viper.Set(constant.KBEnvWorkloadType, "consensus")
// 	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
// 	if err != nil {
// 		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
// 	}
//
// 	properties := make(component.Properties)
// 	properties["url"] = urlWithPort
// 	mysqlOps := NewMysql()
// 	_ = mysqlOps.Init(properties)
// 	mysqlOps.Manager.(*mysql.WesqlManager).DB = db
//
// 	return mysqlOps, mock, err
// }
//
