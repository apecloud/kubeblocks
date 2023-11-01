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

package redis

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
)

const (
// testData  = `{"data":"data"}`
// testKey   = "test"
// redisHost = "127.0.0.1:6379"

// userName = "kiminonawa"
// password = "moss"
// roleName = util.ReadWriteRole
)

var _ = Describe("Redis DBManager", func() {
	// Set up relevant viper config variables
	viper.Set("KB_SERVICE_USER", "testuser")
	viper.Set("KB_SERVICE_PASSWORD", "testpassword")
	Context("new db manager", func() {
		It("with rigth configurations", func() {
			properties := engines.Properties{
				"url": "127.0.0.1",
			}
			dbManger, err := NewManager(properties)
			Expect(err).Should(Succeed())
			Expect(dbManger).ShouldNot(BeNil())
		})

		It("with wrong configurations", func() {
			properties := engines.Properties{
				"poolSize": "wrong-number",
			}
			dbManger, err := NewManager(properties)
			Expect(err).Should(HaveOccurred())
			Expect(dbManger).Should(BeNil())
		})
	})
})

// func TestRedisInit(t *testing.T) {
// 	r, _ := mockRedisOps(t)
// 	defer r.Close()
// 	// make sure operations are inited
// 	assert.NotNil(t, r.client)
// 	assert.NotNil(t, r.OperationsMap[util.ListUsersOp])
// 	assert.NotNil(t, r.OperationsMap[util.CreateUserOp])
// 	assert.NotNil(t, r.OperationsMap[util.DeleteUserOp])
// 	assert.NotNil(t, r.OperationsMap[util.DescribeUserOp])
// 	assert.NotNil(t, r.OperationsMap[util.GrantUserRoleOp])
// 	assert.NotNil(t, r.OperationsMap[util.RevokeUserRoleOp])
// }
// func TestRedisInvokeCreate(t *testing.T) {
// 	r, mock := mockRedisOps(t)
// 	defer r.Close()
//
// 	result := OpsResult{}
// 	request := &ProbeRequest{
// 		Data:      []byte(testData),
// 		Metadata:  map[string]string{"key": testKey},
// 		Operation: util.CreateOperation,
// 	}
// 	// mock expectation
// 	mock.ExpectDo("SET", testKey, testData).SetVal("ok")
//
// 	// invoke
// 	bindingRes, err := r.Invoke(context.TODO(), request)
// 	assert.Equal(t, nil, err)
// 	assert.NotNil(t, bindingRes)
// 	assert.NotNil(t, bindingRes.Data)
//
// 	err = json.Unmarshal(bindingRes.Data, &result)
// 	assert.Nil(t, err)
// 	assert.Equal(t, util.RespEveSucc, result[util.RespFieldEvent], result[util.RespFieldMessage])
// }
//
// func TestRedisInvokeGet(t *testing.T) {
// 	r, mock := mockRedisOps(t)
// 	defer r.Close()
//
// 	opsResult := OpsResult{}
// 	request := &ProbeRequest{
// 		Metadata:  map[string]string{"key": testKey},
// 		Operation: util.GetOperation,
// 	}
// 	// mock expectation, set to nil
// 	mock.ExpectDo("GET", testKey).RedisNil()
// 	mock.ExpectDo("GET", testKey).SetVal(testData)
//
// 	// invoke create
// 	bindingRes, err := r.Invoke(context.TODO(), request)
// 	assert.Nil(t, err)
// 	assert.NotNil(t, bindingRes)
// 	assert.NotNil(t, bindingRes.Data)
// 	err = json.Unmarshal(bindingRes.Data, &opsResult)
// 	assert.Nil(t, err)
// 	assert.Equal(t, util.RespEveFail, opsResult[util.RespFieldEvent])
//
// 	// invoke one more time
// 	bindingRes, err = r.Invoke(context.TODO(), request)
// 	assert.Nil(t, err)
// 	assert.NotNil(t, bindingRes.Data)
// 	err = json.Unmarshal(bindingRes.Data, &opsResult)
// 	assert.Nil(t, err)
// 	assert.Equal(t, util.RespEveSucc, opsResult[util.RespFieldEvent])
// 	var o1 interface{}
// 	_ = json.Unmarshal([]byte(opsResult[util.RespFieldMessage].(string)), &o1)
// 	assert.Equal(t, testData, o1)
// }
//
// func TestRedisInvokeDelete(t *testing.T) {
// 	r, mock := mockRedisOps(t)
// 	defer r.Close()
//
// 	opsResult := OpsResult{}
// 	request := &ProbeRequest{
// 		Metadata:  map[string]string{"key": testKey},
// 		Operation: util.DeleteOperation,
// 	}
// 	// mock expectation, set to err
// 	mock.ExpectDo("DEL", testKey).SetVal("ok")
//
// 	// invoke delete
// 	bindingRes, err := r.Invoke(context.TODO(), request)
// 	assert.Nil(t, err)
// 	assert.NotNil(t, bindingRes)
// 	assert.NotNil(t, bindingRes.Data)
// 	err = json.Unmarshal(bindingRes.Data, &opsResult)
// 	assert.Nil(t, err)
// 	assert.Equal(t, util.RespEveSucc, opsResult[util.RespFieldEvent])
// }
//
// func TestRedisGetRoles(t *testing.T) {
// 	r, mock := mockRedisOps(t)
// 	defer r.Close()
//
// 	opsResult := OpsResult{}
// 	request := &ProbeRequest{
// 		Operation: util.GetRoleOperation,
// 	}
//
// 	// mock expectation, set to err
// 	mock.ExpectInfo("Replication").SetVal("role:master\r\nconnected_slaves:1")
// 	mock.ExpectInfo("Replication").SetVal("role:slave\r\nmaster_port:6379")
// 	// invoke request
// 	bindingRes, err := r.Invoke(context.TODO(), request)
// 	assert.Nil(t, err)
// 	assert.NotNil(t, bindingRes)
// 	assert.NotNil(t, bindingRes.Data)
// 	err = json.Unmarshal(bindingRes.Data, &opsResult)
// 	assert.Nil(t, err)
// 	assert.Equal(t, util.RespEveSucc, opsResult[util.RespFieldEvent])
// 	assert.Equal(t, PRIMARY, opsResult["role"])
//
// 	// invoke one more time
// 	bindingRes, err = r.Invoke(context.TODO(), request)
// 	assert.Nil(t, err)
// 	err = json.Unmarshal(bindingRes.Data, &opsResult)
// 	assert.Nil(t, err)
// 	assert.Equal(t, util.RespEveSucc, opsResult[util.RespFieldEvent])
// 	assert.Equal(t, SECONDARY, opsResult["role"])
// }
//
// func TestRedisAccounts(t *testing.T) {
// 	// prepare
// 	r, mock := mockRedisOps(t)
// 	defer r.Close()
//
// 	ctx := context.TODO()
// 	// list accounts
// 	t.Run("List Accounts", func(t *testing.T) {
// 		mock.ExpectDo("ACL", "USERS").SetVal([]string{"ape", "default", "kbadmin"})
//
// 		response, err := r.Invoke(ctx, &ProbeRequest{
// 			Operation: util.ListUsersOp,
// 		})
//
// 		assert.Nil(t, err)
// 		assert.NotNil(t, response)
// 		assert.NotNil(t, response.Data)
// 		// parse result
// 		opsResult := OpsResult{}
// 		_ = json.Unmarshal(response.Data, &opsResult)
// 		assert.Equal(t, util.RespEveSucc, opsResult[util.RespFieldEvent], opsResult[util.RespFieldMessage])
//
// 		users := make([]util.UserInfo, 0)
// 		err = json.Unmarshal([]byte(opsResult[util.RespFieldMessage].(string)), &users)
// 		assert.Nil(t, err)
// 		assert.NotEmpty(t, users)
// 		user := users[0]
// 		assert.Equal(t, "ape", user.UserName)
// 		mock.ClearExpect()
// 	})
//
// 	// create accounts
// 	t.Run("Create Accounts", func(t *testing.T) {
//
// 		var (
// 			err       error
// 			opsResult = OpsResult{}
// 			response  *ProbeResponse
// 			request   = &ProbeRequest{
// 				Operation: util.CreateUserOp,
// 			}
// 		)
//
// 		testCases := []redisTestCase{
// 			{
// 				testName:      "emptymeta",
// 				testMetaData:  map[string]string{},
// 				expectEveType: util.RespEveFail,
// 				expectEveMsg:  ErrNoUserName.Error(),
// 			},
// 			{
// 				testName:      "nousername",
// 				testMetaData:  map[string]string{"password": "moli"},
// 				expectEveType: util.RespEveFail,
// 				expectEveMsg:  ErrNoUserName.Error(),
// 			},
// 			{
// 				testName:      "nopasswd",
// 				testMetaData:  map[string]string{"userName": "namae"},
// 				expectEveType: util.RespEveFail,
// 				expectEveMsg:  ErrNoPassword.Error(),
// 			},
// 			{
// 				testName: "validInput",
// 				testMetaData: map[string]string{
// 					"userName": userName,
// 					"password": password,
// 				},
// 				expectEveType: util.RespEveSucc,
// 				expectEveMsg:  fmt.Sprintf("created user: %s", userName),
// 			},
// 		}
// 		// mock a user
// 		mock.ExpectDo("ACL", "SETUSER", userName, ">"+password).SetVal("ok")
//
// 		for _, accTest := range testCases {
// 			request.Metadata = accTest.testMetaData
// 			response, err = r.Invoke(ctx, request)
// 			assert.Nil(t, err)
// 			assert.NotNil(t, response.Data)
// 			err = json.Unmarshal(response.Data, &opsResult)
// 			assert.Nil(t, err)
// 			assert.Equal(t, accTest.expectEveType, opsResult[util.RespFieldEvent], opsResult[util.RespFieldMessage])
// 			assert.Contains(t, opsResult[util.RespFieldMessage], accTest.expectEveMsg)
// 		}
// 		mock.ClearExpect()
// 	})
// 	// grant and revoke role
// 	t.Run("Grant Accounts", func(t *testing.T) {
//
// 		var (
// 			err       error
// 			opsResult = OpsResult{}
// 			response  *ProbeResponse
// 		)
//
// 		testCases := []redisTestCase{
// 			{
// 				testName:      "emptymeta",
// 				testMetaData:  map[string]string{},
// 				expectEveType: util.RespEveFail,
// 				expectEveMsg:  ErrNoUserName.Error(),
// 			},
// 			{
// 				testName:      "nousername",
// 				testMetaData:  map[string]string{"password": "moli"},
// 				expectEveType: util.RespEveFail,
// 				expectEveMsg:  ErrNoUserName.Error(),
// 			},
// 			{
// 				testName:      "norolename",
// 				testMetaData:  map[string]string{"userName": "namae"},
// 				expectEveType: util.RespEveFail,
// 				expectEveMsg:  ErrNoRoleName.Error(),
// 			},
// 			{
// 				testName:      "invalidRoleName",
// 				testMetaData:  map[string]string{"userName": "namae", "roleName": "superman"},
// 				expectEveType: util.RespEveFail,
// 				expectEveMsg:  ErrInvalidRoleName.Error(),
// 			},
// 			{
// 				testName: "validInput",
// 				testMetaData: map[string]string{
// 					"userName": userName,
// 					"roleName": (string)(roleName),
// 				},
// 				expectEveType: util.RespEveSucc,
// 			},
// 		}
//
// 		for _, ops := range []util.OperationKind{util.GrantUserRoleOp, util.RevokeUserRoleOp} {
// 			// mock exepctation
// 			args := tokenizeCmd2Args(fmt.Sprintf("ACL SETUSER %s %s", userName, r.role2Priv(ops, (string)(roleName))))
// 			mock.ExpectDo(args...).SetVal("ok")
//
// 			request := &ProbeRequest{
// 				Operation: ops,
// 			}
// 			for _, accTest := range testCases {
// 				request.Metadata = accTest.testMetaData
// 				response, err = r.Invoke(ctx, request)
// 				assert.Nil(t, err)
// 				assert.NotNil(t, response.Data)
// 				err = json.Unmarshal(response.Data, &opsResult)
// 				assert.Nil(t, err)
// 				assert.Equal(t, accTest.expectEveType, opsResult[util.RespFieldEvent], opsResult[util.RespFieldMessage])
// 				if len(accTest.expectEveMsg) > 0 {
// 					assert.Contains(t, accTest.expectEveMsg, opsResult[util.RespFieldMessage])
// 				}
// 			}
// 		}
// 		mock.ClearExpect()
// 	})
//
// 	// desc accounts
// 	t.Run("Desc Accounts", func(t *testing.T) {
// 		var (
// 			err       error
// 			opsResult = OpsResult{}
// 			response  *ProbeResponse
// 			request   = &ProbeRequest{
// 				Operation: util.DescribeUserOp,
// 			}
// 			// mock a user, describing it as an array of interface{}
// 			userInfo = []interface{}{
// 				"flags",
// 				[]interface{}{"on"},
// 				"passwords",
// 				[]interface{}{"mock-password"},
// 				"commands",
// 				"+@all",
// 				"keys",
// 				"~*",
// 				"channels",
// 				"",
// 				"selectors",
// 				[]interface{}{},
// 			}
//
// 			userInfoMap = map[string]interface{}{
// 				"flags":     []interface{}{"on"},
// 				"passwords": []interface{}{"mock-password"},
// 				"commands":  "+@all",
// 				"keys":      "~*",
// 				"channels":  "",
// 				"selectors": []interface{}{},
// 			}
// 		)
//
// 		testCases := []redisTestCase{
// 			{
// 				testName:      "emptymeta",
// 				testMetaData:  map[string]string{},
// 				expectEveType: util.RespEveFail,
// 				expectEveMsg:  ErrNoUserName.Error(),
// 			},
// 			{
// 				testName:      "nousername",
// 				testMetaData:  map[string]string{"password": "moli"},
// 				expectEveType: util.RespEveFail,
// 				expectEveMsg:  ErrNoUserName.Error(),
// 			},
// 			{
// 				testName: "validInputButNil",
// 				testMetaData: map[string]string{
// 					"userName": userName,
// 				},
// 				expectEveType: util.RespEveFail,
// 				expectEveMsg:  "redis: nil",
// 			},
// 			{
// 				testName: "validInput",
// 				testMetaData: map[string]string{
// 					"userName": userName,
// 				},
// 				expectEveType: util.RespEveSucc,
// 			},
// 			{
// 				testName: "validInputAsMap",
// 				testMetaData: map[string]string{
// 					"userName": userName,
// 				},
// 				expectEveType: util.RespEveSucc,
// 			},
// 		}
//
// 		mock.ExpectDo("ACL", "GETUSER", userName).RedisNil()
// 		mock.ExpectDo("ACL", "GETUSER", userName).SetVal(userInfo)
// 		mock.ExpectDo("ACL", "GETUSER", userName).SetVal(userInfoMap)
//
// 		for _, accTest := range testCases {
// 			request.Metadata = accTest.testMetaData
// 			response, err = r.Invoke(ctx, request)
// 			assert.Nil(t, err)
// 			assert.NotNil(t, response.Data)
// 			err = json.Unmarshal(response.Data, &opsResult)
// 			assert.Nil(t, err)
// 			assert.Equal(t, accTest.expectEveType, opsResult[util.RespFieldEvent], opsResult[util.RespFieldMessage])
// 			if len(accTest.expectEveMsg) > 0 {
// 				assert.Contains(t, opsResult[util.RespFieldMessage], accTest.expectEveMsg)
// 			}
// 			if util.RespEveSucc == opsResult[util.RespFieldEvent] {
// 				// parse user info
// 				users := make([]util.UserInfo, 0)
// 				err = json.Unmarshal([]byte(opsResult[util.RespFieldMessage].(string)), &users)
// 				assert.Nil(t, err)
// 				assert.Len(t, users, 1)
// 				user := users[0]
// 				assert.Equal(t, userName, user.UserName)
// 				assert.True(t, util.SuperUserRole.EqualTo(user.RoleName))
// 			}
// 		}
// 		mock.ClearExpect()
// 	})
// 	// delete accounts
// 	t.Run("Delete Accounts", func(t *testing.T) {
//
// 		var (
// 			err       error
// 			opsResult = OpsResult{}
// 			response  *ProbeResponse
// 			request   = &ProbeRequest{
// 				Operation: util.DeleteUserOp,
// 			}
// 		)
//
// 		testCases := []redisTestCase{
// 			{
// 				testName:      "emptymeta",
// 				testMetaData:  map[string]string{},
// 				expectEveType: util.RespEveFail,
// 				expectEveMsg:  ErrNoUserName.Error(),
// 			},
// 			{
// 				testName:      "nousername",
// 				testMetaData:  map[string]string{"password": "moli"},
// 				expectEveType: util.RespEveFail,
// 				expectEveMsg:  ErrNoUserName.Error(),
// 			},
// 			{
// 				testName: "validInput",
// 				testMetaData: map[string]string{
// 					"userName": userName,
// 				},
// 				expectEveType: util.RespEveSucc,
// 				expectEveMsg:  fmt.Sprintf("deleted user: %s", userName),
// 			},
// 		}
// 		// mock a user
// 		mock.ExpectDo("ACL", "DELUSER", userName).SetVal("ok")
//
// 		for _, accTest := range testCases {
// 			request.Metadata = accTest.testMetaData
// 			response, err = r.Invoke(ctx, request)
// 			assert.Nil(t, err)
// 			assert.NotNil(t, response.Data)
// 			err = json.Unmarshal(response.Data, &opsResult)
// 			assert.Nil(t, err)
// 			assert.Equal(t, accTest.expectEveType, opsResult[util.RespFieldEvent], opsResult[util.RespFieldMessage])
// 			assert.Contains(t, opsResult[util.RespFieldMessage], accTest.expectEveMsg)
// 		}
// 		mock.ClearExpect()
// 	})
//
// 	t.Run("RoleName Conversion", func(t *testing.T) {
// 		type roleTestCase struct {
// 			roleName   util.RoleType
// 			redisPrivs string
// 		}
// 		grantTestCases := []roleTestCase{
// 			{
// 				util.SuperUserRole,
// 				"+@all allkeys",
// 			},
// 			{
// 				util.ReadWriteRole,
// 				"-@all +@write +@read allkeys",
// 			},
// 			{
// 				util.ReadOnlyRole,
// 				"-@all +@read allkeys",
// 			},
// 		}
// 		for _, test := range grantTestCases {
// 			cmd := r.role2Priv(util.GrantUserRoleOp, (string)(test.roleName))
// 			assert.Equal(t, test.redisPrivs, cmd)
//
// 			// allkeys -> ~*
// 			cmd = strings.Replace(cmd, "allkeys", "~*", 1)
// 			inferredRole := r.priv2Role(cmd)
// 			assert.Equal(t, test.roleName, inferredRole)
// 		}
//
// 		revokeTestCases := []roleTestCase{
// 			{
// 				util.SuperUserRole,
// 				"-@all allkeys",
// 			},
// 			{
// 				util.ReadWriteRole,
// 				"-@all -@write -@read allkeys",
// 			},
// 			{
// 				util.ReadOnlyRole,
// 				"-@all -@read allkeys",
// 			},
// 		}
// 		for _, test := range revokeTestCases {
// 			cmd := r.role2Priv(util.RevokeUserRoleOp, (string)(test.roleName))
// 			assert.Equal(t, test.redisPrivs, cmd)
// 		}
// 	})
// 	// list accounts
// 	t.Run("List System Accounts", func(t *testing.T) {
// 		mock.ExpectDo("ACL", "USERS").SetVal([]string{"ape", "default", "kbadmin"})
//
// 		response, err := r.Invoke(ctx, &ProbeRequest{
// 			Operation: util.ListSystemAccountsOp,
// 		})
//
// 		assert.Nil(t, err)
// 		assert.NotNil(t, response)
// 		assert.NotNil(t, response.Data)
// 		// parse result
// 		opsResult := OpsResult{}
// 		_ = json.Unmarshal(response.Data, &opsResult)
// 		assert.Equal(t, util.RespEveSucc, opsResult[util.RespFieldEvent], opsResult[util.RespFieldMessage])
//
// 		users := []string{}
// 		err = json.Unmarshal([]byte(opsResult[util.RespFieldMessage].(string)), &users)
// 		assert.Nil(t, err)
// 		assert.NotEmpty(t, users)
// 		assert.Len(t, users, 2)
// 		assert.Contains(t, users, "kbadmin")
// 		assert.Contains(t, users, "default")
// 		mock.ClearExpect()
// 	})
// }
//
// func mockRedisOps(t *testing.T) (*Redis, redismock.ClientMock) {
// 	client, mock := redismock.NewClientMock()
// 	viper.SetDefault("KB_ROLECHECK_DELAY", "0")
//
// 	if client == nil || mock == nil {
// 		t.Fatalf("failed to mock a redis client")
// 		return nil, nil
// 	}
// 	r := &Redis{}
// 	development, _ := zap.NewDevelopment()
// 	r.Logger = zapr.NewLogger(development)
// 	r.client = client
// 	r.ctx, r.cancel = context.WithCancel(context.Background())
// 	_ = r.Init(nil)
// 	r.DBPort = 6379
// 	return r, mock
// }
//
