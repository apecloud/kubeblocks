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

package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/internal/constant"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	"github.com/apecloud/kubeblocks/lorry/engines/models"
)

var _ = Describe("Lorry HTTP Client", func() {
	podName := "pod-for-lorry-test"
	pod := testapps.NewPodFactory("default", podName).
		AddContainer(corev1.Container{
			Name:    testapps.DefaultNginxContainerName,
			Command: []string{"lorry", "--port", "3501"},
			Image:   testapps.NginxImage}).
		GetObject()
	pod.Spec.Containers[0].Ports = []corev1.ContainerPort{
		{
			ContainerPort: int32(lorryHTTPPort),
			Name:          constant.LorryHTTPPortName,
			Protocol:      "TCP",
		},
		{
			ContainerPort: int32(50001),
			Name:          constant.LorryGRPCPortName,
			Protocol:      "TCP",
		},
	}
	pod.Status.PodIP = "127.0.0.1"

	Context("new HTTPClient", func() {
		//port, closer := newTCPServer(t, 50001)
		//defer closer()

		It("without lorry service, return nil", func() {
			podWithoutLorry := pod.DeepCopy()
			podWithoutLorry.Spec.Containers[0].Ports = nil
			lorryClient, err := NewHTTPClientWithPod(podWithoutLorry)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(lorryClient).Should(BeNil())
		})

		It("without pod ip, failed", func() {
			podWithoutPodIP := pod.DeepCopy()
			podWithoutPodIP.Status.PodIP = ""
			_, err := NewHTTPClientWithPod(podWithoutPodIP)
			Expect(err).Should(HaveOccurred())
		})

		It("success", func() {
			lorryClient, err := NewHTTPClientWithPod(pod)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(lorryClient).ShouldNot(BeNil())
		})
	})

	Context("request with timeout", func() {
		var httpServer *httptest.Server
		var port int
		var lorryClient *HTTPClient

		BeforeEach(func() {
			pod1 := pod.DeepCopy()
			body := []byte("{\"role\": \"leader\"}")
			httpServer, port = newHTTPServer(body)
			pod1.Spec.Containers[0].Ports[0].ContainerPort = int32(port)
			lorryClient, _ = NewHTTPClientWithPod(pod1)
			Expect(lorryClient).ShouldNot(BeNil())
		})

		AfterEach(func() {
			httpServer.Close()
		})

		It("response in time", func() {
			lorryClient.ReconcileTimeout = 1 * time.Second
			_, err := lorryClient.GetRole(context.TODO())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(lorryClient.cache).Should(BeEmpty())
		})

		It("response timeout", func() {
			lorryClient.ReconcileTimeout = 50 * time.Millisecond
			_, err := lorryClient.GetRole(context.TODO())
			Expect(err).Should(HaveOccurred())
			// wait client to get response and cache it
			time.Sleep(200 * time.Millisecond)
			Expect(lorryClient.cache).Should(HaveLen(1))
		})

		It("response by cache", func() {
			lorryClient.ReconcileTimeout = 50 * time.Millisecond
			// get response from server, and timeout
			_, err := lorryClient.GetRole(context.TODO())
			Expect(err).Should(HaveOccurred())
			// wait client to get response and cache it
			time.Sleep(200 * time.Millisecond)
			// get response from cache
			_, err = lorryClient.GetRole(context.TODO())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(lorryClient.cache).Should(BeEmpty())
		})
	})

	Context("get replica role", func() {
		var lorryClient *HTTPClient

		BeforeEach(func() {
			lorryClient, _ = NewHTTPClientWithPod(pod)
			Expect(lorryClient).ShouldNot(BeNil())
		})

		It("success", func() {
			role := "leader"
			mockDBManager.EXPECT().GetReplicaRole(gomock.Any(), gomock.Any()).Return(role, nil)
			Expect(lorryClient.GetRole(context.TODO())).Should(Equal(role))
		})

		It("not implemented", func() {
			msg := "not implemented for test"
			mockDBManager.EXPECT().GetReplicaRole(gomock.Any(), gomock.Any()).Return(string(""), fmt.Errorf(msg))
			role, err := lorryClient.GetRole(context.TODO())
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(msg))
			Expect(role).Should(BeEmpty())
		})
	})

	Context("list system accounts", func() {
		var lorryClient *HTTPClient
		var systemAccounts []models.UserInfo

		BeforeEach(func() {
			lorryClient, _ = NewHTTPClientWithPod(pod)
			Expect(lorryClient).ShouldNot(BeNil())
			systemAccounts = []models.UserInfo{
				{
					UserName: "kb-admin1",
				},
				{
					UserName: "kb-admin2",
				},
			}
		})

		It("success", func() {
			mockDBManager.EXPECT().ListSystemAccounts(gomock.Any()).Return(systemAccounts, nil)
			Expect(lorryClient.ListSystemAccounts(context.TODO())).Should(HaveLen(2))
		})

		It("not implemented", func() {
			msg := "not implemented for test"
			mockDBManager.EXPECT().ListSystemAccounts(gomock.Any()).Return(nil, fmt.Errorf(msg))
			accounts, err := lorryClient.ListSystemAccounts(context.TODO())
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(msg))
			Expect(accounts).Should(BeEmpty())
		})
	})

	Context("list users", func() {
		var lorryClient *HTTPClient
		var users []models.UserInfo

		BeforeEach(func() {
			lorryClient, _ = NewHTTPClientWithPod(pod)
			Expect(lorryClient).ShouldNot(BeNil())
			users = []models.UserInfo{
				{
					UserName: "user1",
				},
				{
					UserName: "user2",
				},
			}
		})

		It("success", func() {
			mockDBManager.EXPECT().ListUsers(gomock.Any()).Return(users, nil)
			Expect(lorryClient.ListUsers(context.TODO())).Should(HaveLen(2))
		})

		It("not implemented", func() {
			msg := "not implemented for test"
			mockDBManager.EXPECT().ListUsers(gomock.Any()).Return(nil, fmt.Errorf(msg))
			users, err := lorryClient.ListUsers(context.TODO())
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(msg))
			Expect(users).Should(BeEmpty())
		})
	})

})

// func TestSystemAccounts(t *testing.T) {
// 	roleNames, _ := json.Marshal([]string{"kbadmin", "kbprobe"})
// 	sqlResponse := SQLChannelResponse{
// 		Event:   RespEveSucc,
// 		Message: string(roleNames),
// 	}
// 	respData, _ := json.Marshal(sqlResponse)
//
// 	s := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
// 		writer.WriteHeader(200)
// 		_, _ = writer.Write(respData)
// 	}))
//
// 	addr := s.Listener.Addr().String()
// 	index := strings.LastIndex(addr, ":")
// 	portStr := addr[index+1:]
// 	port, _ := strconv.Atoi(portStr)
//
// 	cli, closer, err := initSQLChannelClient(port, t)
// 	if err != nil {
// 		t.Errorf("new sql channel client error: %v", err)
// 	}
// 	defer closer()
//
// 	t.Run("ResponseByCache", func(t *testing.T) {
// 		cli.ReconcileTimeout = 200 * time.Millisecond
// 		_, err := cli.GetSystemAccounts()
//
// 		if err != nil {
// 			t.Errorf("return reps in cache: %v", err)
// 		}
// 		if len(cli.cache) != 0 {
// 			t.Errorf("cache should be cleared: %v", cli.cache)
// 		}
// 	})
// }

// func TestJoinMember(t *testing.T) {
//
// 	t.Run("Join Member success", func(t *testing.T) {
// 		sqlResponse := SQLChannelResponse{
// 			Event: RespEveSucc,
// 		}
// 		respData, _ := json.Marshal(sqlResponse)
// 		port := initHTTPServer(respData)
//
// 		cli, closer, err := initSQLChannelClient(port, t)
// 		if err != nil {
// 			t.Errorf("new sql channel client error: %v", err)
// 		}
// 		defer closer()
// 		cli.ReconcileTimeout = 200 * time.Millisecond
// 		err = cli.JoinMember(context.TODO())
// 		assert.Nil(t, err)
// 	})
//
// 	t.Run("Join Member fail", func(t *testing.T) {
// 		cli, closer, err := initSQLChannelClient(-1, t)
// 		if err != nil {
// 			t.Errorf("new sql channel client error: %v", err)
// 		}
// 		defer closer()
// 		cli.ReconcileTimeout = 200 * time.Millisecond
// 		err = cli.JoinMember(context.TODO())
// 		assert.NotNil(t, err)
// 	})
// }
//
// func TestLeaveMember(t *testing.T) {
//
// 	t.Run("Leave Member success", func(t *testing.T) {
// 		sqlResponse := SQLChannelResponse{
// 			Event: RespEveSucc,
// 		}
// 		respData, _ := json.Marshal(sqlResponse)
// 		port := initHTTPServer(respData)
//
// 		cli, closer, err := initSQLChannelClient(port, t)
// 		if err != nil {
// 			t.Errorf("new sql channel client error: %v", err)
// 		}
// 		defer closer()
// 		cli.ReconcileTimeout = 200 * time.Millisecond
// 		err = cli.LeaveMember(context.TODO())
// 		assert.Nil(t, err)
// 	})
//
// 	t.Run("Leave Member fail", func(t *testing.T) {
// 		cli, closer, err := initSQLChannelClient(-1, t)
// 		if err != nil {
// 			t.Errorf("new sql channel client error: %v", err)
// 		}
// 		defer closer()
// 		cli.ReconcileTimeout = 200 * time.Millisecond
// 		err = cli.LeaveMember(context.TODO())
// 		assert.NotNil(t, err)
// 	})
// }
//
// func TestParseSqlChannelResult(t *testing.T) {
// 	t.Run("Binding Not Supported", func(t *testing.T) {
// 		result := `
// 	{"errorCode":"ERR_INVOKE_OUTPUT_BINDING","message":"error when invoke output binding mongodb: binding mongodb does not support operation listUsers. supported operations:checkRunning checkRole getRole"}
// 	`
// 		sqlResponse, err := parseResponse(([]byte)(result), "listUsers", "mongodb")
// 		assert.NotNil(t, err)
// 		assert.True(t, IsUnSupportedError(err))
// 		assert.Equal(t, sqlResponse.Event, RespEveFail)
// 		assert.Contains(t, sqlResponse.Message, "not supported")
// 	})
//
// 	t.Run("Binding Exec Failed", func(t *testing.T) {
// 		result := `
// 	{"event":"Failed","message":"db not ready"}
// 	`
// 		sqlResponse, err := parseResponse(([]byte)(result), "listUsers", "mongodb")
// 		assert.Nil(t, err)
// 		assert.Equal(t, sqlResponse.Event, RespEveFail)
// 		assert.Contains(t, sqlResponse.Message, "db not ready")
// 	})
//
// 	t.Run("Binding Exec Success", func(t *testing.T) {
// 		result := `
// 	{"event":"Success","message":"[]"}
// 	`
// 		sqlResponse, err := parseResponse(([]byte)(result), "listUsers", "mongodb")
// 		assert.Nil(t, err)
// 		assert.Equal(t, sqlResponse.Event, RespEveSucc)
// 	})
//
// 	t.Run("Invalid Response Format", func(t *testing.T) {
// 		// msg cannot be parsed to json
// 		result := `
// 	{"event":"Success","message":"[]
// 	`
// 		_, err := parseResponse(([]byte)(result), "listUsers", "mongodb")
// 		assert.NotNil(t, err)
// 	})
// }
//
// func TestErrMsg(t *testing.T) {
// 	err := SQLChannelError{
// 		Reason: UnsupportedOps,
// 	}
// 	assert.True(t, strings.Contains(err.Error(), "unsupported"))
// 	assert.False(t, IsUnSupportedError(nil))
// 	assert.True(t, IsUnSupportedError(err))
// 	assert.False(t, IsUnSupportedError(errors.New("test")))
// }
//
// func initSQLChannelClient(httpPort int, t *testing.T) (*OperationClient, func(), error) {
// 	port, closer := newTCPServer(t, 50001)
// 	podName := "pod-for-sqlchannel-test"
// 	pod := testapps.NewPodFactory("default", podName).
// 		AddContainer(corev1.Container{Name: viper.GetString(constant.KBToolsImage), Image: viper.GetString(constant.KBToolsImage)}).
// 		GetObject()
// 	pod.Spec.Containers[0].Ports = []corev1.ContainerPort{
// 		{
// 			ContainerPort: int32(httpPort),
// 			Name:          constant.LorryHTTPPortName,
// 			Protocol:      "TCP",
// 		},
// 		{
// 			ContainerPort: int32(port),
// 			Name:          constant.LorryGRPCPortName,
// 			Protocol:      "TCP",
// 		},
// 	}
// 	pod.Status.PodIP = "127.0.0.1"
// 	cli, err := NewHTTPClientWithPod(pod, "mysql")
// 	if err != nil {
// 		t.Errorf("new sql channel client error: %v", err)
// 	}
// 	return cli, closer, err
// }

func newHTTPServer(resp []byte) (*httptest.Server, int) {
	s := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		time.Sleep(100 * time.Millisecond)
		writer.WriteHeader(200)
		_, _ = writer.Write(resp)
	}))
	addr := s.Listener.Addr().String()
	index := strings.LastIndex(addr, ":")
	portStr := addr[index+1:]
	port, _ := strconv.Atoi(portStr)
	return s, port
}
