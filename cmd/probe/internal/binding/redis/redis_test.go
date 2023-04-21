/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"

	redismock "github.com/go-redis/redismock/v9"

	. "github.com/apecloud/kubeblocks/cmd/probe/internal/binding"
	. "github.com/apecloud/kubeblocks/cmd/probe/util"
)

const (
	testData  = `{"data":"data"}`
	testKey   = "test"
	redisHost = "127.0.0.1:6379"

	userName = "kiminonawa"
	password = "moss"
	roleName = ReadWriteRole
)

type redisTestCase struct {
	testName      string
	testMetaData  map[string]string
	expectEveType string
	expectEveMsg  string
}

func TestRedisInit(t *testing.T) {
	r, _ := mockRedisOps(t)
	defer r.Close()
	// make sure operations are inited
	assert.NotNil(t, r.client)
	assert.NotNil(t, r.OperationMap[ListUsersOp])
	assert.NotNil(t, r.OperationMap[CreateUserOp])
	assert.NotNil(t, r.OperationMap[DeleteUserOp])
	assert.NotNil(t, r.OperationMap[DescribeUserOp])
	assert.NotNil(t, r.OperationMap[GrantUserRoleOp])
	assert.NotNil(t, r.OperationMap[RevokeUserRoleOp])
}
func TestRedisInvokeCreate(t *testing.T) {
	r, mock := mockRedisOps(t)
	defer r.Close()

	result := OpsResult{}
	request := &bindings.InvokeRequest{
		Data:      []byte(testData),
		Metadata:  map[string]string{"key": testKey},
		Operation: bindings.CreateOperation,
	}
	// mock expectation
	mock.ExpectDo("SET", testKey, testData).SetVal("ok")

	// invoke
	bindingRes, err := r.Invoke(context.TODO(), request)
	assert.Equal(t, nil, err)
	assert.NotNil(t, bindingRes)
	assert.NotNil(t, bindingRes.Data)

	err = json.Unmarshal(bindingRes.Data, &result)
	assert.Nil(t, err)
	assert.Equal(t, RespEveSucc, result[RespTypEve], result[RespTypMsg])
}

func TestRedisInvokeGet(t *testing.T) {
	r, mock := mockRedisOps(t)
	defer r.Close()

	opsResult := OpsResult{}
	request := &bindings.InvokeRequest{
		Metadata:  map[string]string{"key": testKey},
		Operation: bindings.GetOperation,
	}
	// mock expectation, set to nil
	mock.ExpectDo("GET", testKey).RedisNil()
	mock.ExpectDo("GET", testKey).SetVal(testData)

	// invoke create
	bindingRes, err := r.Invoke(context.TODO(), request)
	assert.Nil(t, err)
	assert.NotNil(t, bindingRes)
	assert.NotNil(t, bindingRes.Data)
	err = json.Unmarshal(bindingRes.Data, &opsResult)
	assert.Nil(t, err)
	assert.Equal(t, RespEveFail, opsResult[RespTypEve])

	// invoke one more time
	bindingRes, err = r.Invoke(context.TODO(), request)
	assert.Nil(t, err)
	assert.NotNil(t, bindingRes.Data)
	err = json.Unmarshal(bindingRes.Data, &opsResult)
	assert.Nil(t, err)
	assert.Equal(t, RespEveSucc, opsResult[RespTypEve])
	var o1 interface{}
	_ = json.Unmarshal([]byte(opsResult[RespTypMsg].(string)), &o1)
	assert.Equal(t, testData, o1)
}

func TestRedisInvokeDelete(t *testing.T) {
	r, mock := mockRedisOps(t)
	defer r.Close()

	opsResult := OpsResult{}
	request := &bindings.InvokeRequest{
		Metadata:  map[string]string{"key": testKey},
		Operation: bindings.DeleteOperation,
	}
	// mock expectation, set to err
	mock.ExpectDo("DEL", testKey).SetVal("ok")

	// invoke delete
	bindingRes, err := r.Invoke(context.TODO(), request)
	assert.Nil(t, err)
	assert.NotNil(t, bindingRes)
	assert.NotNil(t, bindingRes.Data)
	err = json.Unmarshal(bindingRes.Data, &opsResult)
	assert.Nil(t, err)
	assert.Equal(t, RespEveSucc, opsResult[RespTypEve])
}

func TestRedisGetRoles(t *testing.T) {
	r, mock := mockRedisOps(t)
	defer r.Close()

	opsResult := OpsResult{}
	request := &bindings.InvokeRequest{
		Operation: GetRoleOperation,
	}

	// mock expectation, set to err
	mock.ExpectInfo("Replication").SetVal("role:master\r\nconnected_slaves:1")
	mock.ExpectInfo("Replication").SetVal("role:slave\r\nmaster_port:6379")
	// invoke request
	bindingRes, err := r.Invoke(context.TODO(), request)
	assert.Nil(t, err)
	assert.NotNil(t, bindingRes)
	assert.NotNil(t, bindingRes.Data)
	err = json.Unmarshal(bindingRes.Data, &opsResult)
	assert.Nil(t, err)
	assert.Equal(t, RespEveSucc, opsResult[RespTypEve])
	assert.Equal(t, PRIMARY, opsResult["role"])

	// invoke one more time
	bindingRes, err = r.Invoke(context.TODO(), request)
	assert.Nil(t, err)
	err = json.Unmarshal(bindingRes.Data, &opsResult)
	assert.Nil(t, err)
	assert.Equal(t, RespEveSucc, opsResult[RespTypEve])
	assert.Equal(t, SECONDARY, opsResult["role"])
}

func TestRedisAccounts(t *testing.T) {
	// prepare
	r, mock := mockRedisOps(t)
	defer r.Close()

	ctx := context.TODO()
	// list accounts
	t.Run("List Accounts", func(t *testing.T) {
		mock.ExpectDo("ACL", "USERS").SetVal([]string{"ape", "default", "kbadmin"})

		response, err := r.Invoke(ctx, &bindings.InvokeRequest{
			Operation: ListUsersOp,
		})

		assert.Nil(t, err)
		assert.NotNil(t, response)
		assert.NotNil(t, response.Data)
		// parse result
		opsResult := OpsResult{}
		_ = json.Unmarshal(response.Data, &opsResult)
		assert.Equal(t, RespEveSucc, opsResult[RespTypEve], opsResult[RespTypMsg])

		users := make([]UserInfo, 0)
		err = json.Unmarshal([]byte(opsResult[RespTypMsg].(string)), &users)
		assert.Nil(t, err)
		assert.NotEmpty(t, users)
		user := users[0]
		assert.Equal(t, "ape", user.UserName)
		mock.ClearExpect()
	})

	// create accounts
	t.Run("Create Accounts", func(t *testing.T) {

		var (
			err       error
			opsResult = OpsResult{}
			response  *bindings.InvokeResponse
			request   = &bindings.InvokeRequest{
				Operation: CreateUserOp,
			}
		)

		testCases := []redisTestCase{
			{
				testName:      "emptymeta",
				testMetaData:  map[string]string{},
				expectEveType: RespEveFail,
				expectEveMsg:  ErrNoUserName.Error(),
			},
			{
				testName:      "nousername",
				testMetaData:  map[string]string{"password": "moli"},
				expectEveType: RespEveFail,
				expectEveMsg:  ErrNoUserName.Error(),
			},
			{
				testName:      "nopasswd",
				testMetaData:  map[string]string{"userName": "namae"},
				expectEveType: RespEveFail,
				expectEveMsg:  ErrNoPassword.Error(),
			},
			{
				testName: "validInput",
				testMetaData: map[string]string{
					"userName": userName,
					"password": password,
				},
				expectEveType: RespEveSucc,
				expectEveMsg:  fmt.Sprintf("created user: %s", userName),
			},
		}
		// mock a user
		mock.ExpectDo("ACL", "SETUSER", userName, ">"+password).SetVal("ok")

		for _, accTest := range testCases {
			request.Metadata = accTest.testMetaData
			response, err = r.Invoke(ctx, request)
			assert.Nil(t, err)
			assert.NotNil(t, response.Data)
			err = json.Unmarshal(response.Data, &opsResult)
			assert.Nil(t, err)
			assert.Equal(t, accTest.expectEveType, opsResult[RespTypEve], opsResult[RespTypMsg])
			assert.Contains(t, opsResult[RespTypMsg], accTest.expectEveMsg)
		}
		mock.ClearExpect()
	})
	// grant and revoke role
	t.Run("Grant Accounts", func(t *testing.T) {

		var (
			err       error
			opsResult = OpsResult{}
			response  *bindings.InvokeResponse
		)

		testCases := []redisTestCase{
			{
				testName:      "emptymeta",
				testMetaData:  map[string]string{},
				expectEveType: RespEveFail,
				expectEveMsg:  ErrNoUserName.Error(),
			},
			{
				testName:      "nousername",
				testMetaData:  map[string]string{"password": "moli"},
				expectEveType: RespEveFail,
				expectEveMsg:  ErrNoUserName.Error(),
			},
			{
				testName:      "norolename",
				testMetaData:  map[string]string{"userName": "namae"},
				expectEveType: RespEveFail,
				expectEveMsg:  ErrNoRoleName.Error(),
			},
			{
				testName:      "invalidRoleName",
				testMetaData:  map[string]string{"userName": "namae", "roleName": "superman"},
				expectEveType: RespEveFail,
				expectEveMsg:  ErrInvalidRoleName.Error(),
			},
			{
				testName: "validInput",
				testMetaData: map[string]string{
					"userName": userName,
					"roleName": (string)(roleName),
				},
				expectEveType: RespEveSucc,
			},
		}

		for _, ops := range []bindings.OperationKind{GrantUserRoleOp, RevokeUserRoleOp} {
			// mock exepctation
			args := tokenizeCmd2Args(fmt.Sprintf("ACL SETUSER %s %s", userName, r.role2Priv(ops, (string)(roleName))))
			mock.ExpectDo(args...).SetVal("ok")

			request := &bindings.InvokeRequest{
				Operation: ops,
			}
			for _, accTest := range testCases {
				request.Metadata = accTest.testMetaData
				response, err = r.Invoke(ctx, request)
				assert.Nil(t, err)
				assert.NotNil(t, response.Data)
				err = json.Unmarshal(response.Data, &opsResult)
				assert.Nil(t, err)
				assert.Equal(t, accTest.expectEveType, opsResult[RespTypEve], opsResult[RespTypMsg])
				if len(accTest.expectEveMsg) > 0 {
					assert.Contains(t, accTest.expectEveMsg, opsResult[RespTypMsg])
				}
			}
		}
		mock.ClearExpect()
	})

	// desc accounts
	t.Run("Desc Accounts", func(t *testing.T) {
		var (
			err       error
			opsResult = OpsResult{}
			response  *bindings.InvokeResponse
			request   = &bindings.InvokeRequest{
				Operation: DescribeUserOp,
			}
			// mock a user, describing it as an array of interface{}
			userInfo = []interface{}{
				"flags",
				[]interface{}{"on"},
				"passwords",
				[]interface{}{"mock-password"},
				"commands",
				"+@all",
				"keys",
				"~*",
				"channels",
				"",
				"selectors",
				[]interface{}{},
			}

			userInfoMap = map[string]interface{}{
				"flags":     []interface{}{"on"},
				"passwords": []interface{}{"mock-password"},
				"commands":  "+@all",
				"keys":      "~*",
				"channels":  "",
				"selectors": []interface{}{},
			}
		)

		testCases := []redisTestCase{
			{
				testName:      "emptymeta",
				testMetaData:  map[string]string{},
				expectEveType: RespEveFail,
				expectEveMsg:  ErrNoUserName.Error(),
			},
			{
				testName:      "nousername",
				testMetaData:  map[string]string{"password": "moli"},
				expectEveType: RespEveFail,
				expectEveMsg:  ErrNoUserName.Error(),
			},
			{
				testName: "validInputButNil",
				testMetaData: map[string]string{
					"userName": userName,
				},
				expectEveType: RespEveFail,
				expectEveMsg:  "redis: nil",
			},
			{
				testName: "validInput",
				testMetaData: map[string]string{
					"userName": userName,
				},
				expectEveType: RespEveSucc,
			},
			{
				testName: "validInputAsMap",
				testMetaData: map[string]string{
					"userName": userName,
				},
				expectEveType: RespEveSucc,
			},
		}

		mock.ExpectDo("ACL", "GETUSER", userName).RedisNil()
		mock.ExpectDo("ACL", "GETUSER", userName).SetVal(userInfo)
		mock.ExpectDo("ACL", "GETUSER", userName).SetVal(userInfoMap)

		for _, accTest := range testCases {
			request.Metadata = accTest.testMetaData
			response, err = r.Invoke(ctx, request)
			assert.Nil(t, err)
			assert.NotNil(t, response.Data)
			err = json.Unmarshal(response.Data, &opsResult)
			assert.Nil(t, err)
			assert.Equal(t, accTest.expectEveType, opsResult[RespTypEve], opsResult[RespTypMsg])
			if len(accTest.expectEveMsg) > 0 {
				assert.Contains(t, opsResult[RespTypMsg], accTest.expectEveMsg)
			}
			if RespEveSucc == opsResult[RespTypEve] {
				// parse user info
				users := make([]UserInfo, 0)
				err = json.Unmarshal([]byte(opsResult[RespTypMsg].(string)), &users)
				assert.Nil(t, err)
				assert.Len(t, users, 1)
				user := users[0]
				assert.Equal(t, userName, user.UserName)
				assert.True(t, SuperUserRole.EqualTo(user.RoleName))
			}
		}
		mock.ClearExpect()
	})
	// delete accounts
	t.Run("Delete Accounts", func(t *testing.T) {

		var (
			err       error
			opsResult = OpsResult{}
			response  *bindings.InvokeResponse
			request   = &bindings.InvokeRequest{
				Operation: DeleteUserOp,
			}
		)

		testCases := []redisTestCase{
			{
				testName:      "emptymeta",
				testMetaData:  map[string]string{},
				expectEveType: RespEveFail,
				expectEveMsg:  ErrNoUserName.Error(),
			},
			{
				testName:      "nousername",
				testMetaData:  map[string]string{"password": "moli"},
				expectEveType: RespEveFail,
				expectEveMsg:  ErrNoUserName.Error(),
			},
			{
				testName: "validInput",
				testMetaData: map[string]string{
					"userName": userName,
				},
				expectEveType: RespEveSucc,
				expectEveMsg:  fmt.Sprintf("deleted user: %s", userName),
			},
		}
		// mock a user
		mock.ExpectDo("ACL", "DELUSER", userName).SetVal("ok")

		for _, accTest := range testCases {
			request.Metadata = accTest.testMetaData
			response, err = r.Invoke(ctx, request)
			assert.Nil(t, err)
			assert.NotNil(t, response.Data)
			err = json.Unmarshal(response.Data, &opsResult)
			assert.Nil(t, err)
			assert.Equal(t, accTest.expectEveType, opsResult[RespTypEve], opsResult[RespTypMsg])
			assert.Contains(t, opsResult[RespTypMsg], accTest.expectEveMsg)
		}
		mock.ClearExpect()
	})

	t.Run("RoleName Conversion", func(t *testing.T) {
		type roleTestCase struct {
			roleName   RoleType
			redisPrivs string
		}
		grantTestCases := []roleTestCase{
			{
				SuperUserRole,
				"+@all allkeys",
			},
			{
				ReadWriteRole,
				"-@all +@write +@read allkeys",
			},
			{
				ReadOnlyRole,
				"-@all +@read allkeys",
			},
		}
		for _, test := range grantTestCases {
			cmd := r.role2Priv(GrantUserRoleOp, (string)(test.roleName))
			assert.Equal(t, test.redisPrivs, cmd)

			// allkeys -> ~*
			cmd = strings.Replace(cmd, "allkeys", "~*", 1)
			inferredRole := r.priv2Role(cmd)
			assert.Equal(t, test.roleName, inferredRole)
		}

		revokeTestCases := []roleTestCase{
			{
				SuperUserRole,
				"-@all allkeys",
			},
			{
				ReadWriteRole,
				"-@all -@write -@read allkeys",
			},
			{
				ReadOnlyRole,
				"-@all -@read allkeys",
			},
		}
		for _, test := range revokeTestCases {
			cmd := r.role2Priv(RevokeUserRoleOp, (string)(test.roleName))
			assert.Equal(t, test.redisPrivs, cmd)
		}
	})
	// list accounts
	t.Run("List System Accounts", func(t *testing.T) {
		mock.ExpectDo("ACL", "USERS").SetVal([]string{"ape", "default", "kbadmin"})

		response, err := r.Invoke(ctx, &bindings.InvokeRequest{
			Operation: ListSystemAccountsOp,
		})

		assert.Nil(t, err)
		assert.NotNil(t, response)
		assert.NotNil(t, response.Data)
		// parse result
		opsResult := OpsResult{}
		_ = json.Unmarshal(response.Data, &opsResult)
		assert.Equal(t, RespEveSucc, opsResult[RespTypEve], opsResult[RespTypMsg])

		users := []string{}
		err = json.Unmarshal([]byte(opsResult[RespTypMsg].(string)), &users)
		assert.Nil(t, err)
		assert.NotEmpty(t, users)
		assert.Len(t, users, 2)
		assert.Contains(t, users, "kbadmin")
		assert.Contains(t, users, "default")
		mock.ClearExpect()
	})
}

func mockRedisOps(t *testing.T) (*Redis, redismock.ClientMock) {
	client, mock := redismock.NewClientMock()

	if client == nil || mock == nil {
		t.Fatalf("failed to mock a redis client")
		return nil, nil
	}
	r := &Redis{}
	r.Logger = logger.NewLogger("test")
	r.client = client
	r.ctx, r.cancel = context.WithCancel(context.Background())
	_ = r.Init(bindings.Metadata{})
	r.DBPort = 6379
	return r, mock
}
