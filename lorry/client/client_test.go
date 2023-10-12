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
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/internal/constant"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
	viper "github.com/apecloud/kubeblocks/internal/viperx"
	. "github.com/apecloud/kubeblocks/lorry/util"
)

func TestNewClientWithPod(t *testing.T) {
	port, closer := newTCPServer(t, 50001)
	defer closer()

	podName := "pod-for-sqlchannel-test"
	pod := testapps.NewPodFactory("default", podName).
		AddContainer(corev1.Container{Name: testapps.DefaultNginxContainerName, Image: testapps.NginxImage}).
		GetObject()
	pod.Spec.Containers[0].Ports = []corev1.ContainerPort{{
		ContainerPort: int32(3501),
		Name:          constant.ProbeHTTPPortName,
		Protocol:      "TCP",
	},
		{
			ContainerPort: int32(port),
			Name:          constant.ProbeGRPCPortName,
			Protocol:      "TCP",
		},
	}
	pod.Status.PodIP = "127.0.0.1"

	t.Run("WithOutCharacterType", func(t *testing.T) {
		_, err := NewClientWithPod(pod, "")
		if !strings.Contains(err.Error(), "chacterType must be set") {
			t.Errorf("new sql channel client unexpection: %v", err)
		}
	})

	t.Run("WithOutPodIP", func(t *testing.T) {
		podWithoutPodIP := pod.DeepCopy()
		podWithoutPodIP.Status.PodIP = ""
		_, err := NewClientWithPod(podWithoutPodIP, "mysql")
		if !(err != nil && strings.Contains(err.Error(), "has no ip")) {
			t.Errorf("new sql channel client unexpection: %v", err)
		}
	})

	t.Run("Success", func(t *testing.T) {
		_, err := NewClientWithPod(pod, "mysql")
		if err != nil {
			t.Errorf("new sql channel client error: %v", err)
		}
	})
}

func TestGetRole(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(200)
		_, _ = writer.Write([]byte("{\"role\": \"leader\"}"))
	}))

	addr := s.Listener.Addr().String()
	index := strings.LastIndex(addr, ":")
	portStr := addr[index+1:]
	port, _ := strconv.Atoi(portStr)

	viper.Set(constant.KBToolsImage, "lorry")

	cli, closer, err := initSQLChannelClient(port, t)
	if err != nil {
		t.Errorf("new sql channel client error: %v", err)
	}
	defer closer()

	t.Run("ResponseInTime", func(t *testing.T) {
		cli.ReconcileTimeout = 1 * time.Second
		_, err := cli.GetRole()
		if err != nil {
			t.Errorf("get role error: %v", err)
		}
		if len(cli.cache) != 0 {
			t.Errorf("cache should be empty")
		}
	})

	t.Run("ResponseTimeout", func(t *testing.T) {
		cli.ReconcileTimeout = 0
		_, err := cli.GetRole()
		t.Logf("err: %v", err)
		if err == nil {
			t.Errorf("request should be timeout")
		}
		time.Sleep(200 * time.Millisecond)
		if len(cli.cache) != 1 {
			t.Errorf("cache should not be empty: %v", cli.cache)
		}
	})

	t.Run("ResponseByCache", func(t *testing.T) {
		cli.ReconcileTimeout = 50 * time.Millisecond
		_, err := cli.GetRole()

		if err != nil {
			t.Errorf("return reps in cache: %v", err)
		}
		if len(cli.cache) != 0 {
			t.Errorf("cache should be cleared: %v", cli.cache)
		}
	})
}

func TestSystemAccounts(t *testing.T) {
	roleNames, _ := json.Marshal([]string{"kbadmin", "kbprobe"})
	sqlResponse := SQLChannelResponse{
		Event:   RespEveSucc,
		Message: string(roleNames),
	}
	respData, _ := json.Marshal(sqlResponse)

	s := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(200)
		_, _ = writer.Write(respData)
	}))

	addr := s.Listener.Addr().String()
	index := strings.LastIndex(addr, ":")
	portStr := addr[index+1:]
	port, _ := strconv.Atoi(portStr)

	cli, closer, err := initSQLChannelClient(port, t)
	if err != nil {
		t.Errorf("new sql channel client error: %v", err)
	}
	defer closer()

	t.Run("ResponseByCache", func(t *testing.T) {
		cli.ReconcileTimeout = 200 * time.Millisecond
		_, err := cli.GetSystemAccounts()

		if err != nil {
			t.Errorf("return reps in cache: %v", err)
		}
		if len(cli.cache) != 0 {
			t.Errorf("cache should be cleared: %v", cli.cache)
		}
	})
}

func TestJoinMember(t *testing.T) {

	t.Run("Join Member success", func(t *testing.T) {
		sqlResponse := SQLChannelResponse{
			Event: RespEveSucc,
		}
		respData, _ := json.Marshal(sqlResponse)
		port := initHTTPServer(respData)

		cli, closer, err := initSQLChannelClient(port, t)
		if err != nil {
			t.Errorf("new sql channel client error: %v", err)
		}
		defer closer()
		cli.ReconcileTimeout = 200 * time.Millisecond
		err = cli.JoinMember(context.TODO())
		assert.Nil(t, err)
	})

	t.Run("Join Member fail", func(t *testing.T) {
		cli, closer, err := initSQLChannelClient(-1, t)
		if err != nil {
			t.Errorf("new sql channel client error: %v", err)
		}
		defer closer()
		cli.ReconcileTimeout = 200 * time.Millisecond
		err = cli.JoinMember(context.TODO())
		assert.NotNil(t, err)
	})
}

func TestLeaveMember(t *testing.T) {

	t.Run("Leave Member success", func(t *testing.T) {
		sqlResponse := SQLChannelResponse{
			Event: RespEveSucc,
		}
		respData, _ := json.Marshal(sqlResponse)
		port := initHTTPServer(respData)

		cli, closer, err := initSQLChannelClient(port, t)
		if err != nil {
			t.Errorf("new sql channel client error: %v", err)
		}
		defer closer()
		cli.ReconcileTimeout = 200 * time.Millisecond
		err = cli.LeaveMember(context.TODO())
		assert.Nil(t, err)
	})

	t.Run("Leave Member fail", func(t *testing.T) {
		cli, closer, err := initSQLChannelClient(-1, t)
		if err != nil {
			t.Errorf("new sql channel client error: %v", err)
		}
		defer closer()
		cli.ReconcileTimeout = 200 * time.Millisecond
		err = cli.LeaveMember(context.TODO())
		assert.NotNil(t, err)
	})
}

func TestParseSqlChannelResult(t *testing.T) {
	t.Run("Binding Not Supported", func(t *testing.T) {
		result := `
	{"errorCode":"ERR_INVOKE_OUTPUT_BINDING","message":"error when invoke output binding mongodb: binding mongodb does not support operation listUsers. supported operations:checkRunning checkRole getRole"}
	`
		sqlResponse, err := parseResponse(([]byte)(result), "listUsers", "mongodb")
		assert.NotNil(t, err)
		assert.True(t, IsUnSupportedError(err))
		assert.Equal(t, sqlResponse.Event, RespEveFail)
		assert.Contains(t, sqlResponse.Message, "not supported")
	})

	t.Run("Binding Exec Failed", func(t *testing.T) {
		result := `
	{"event":"Failed","message":"db not ready"}
	`
		sqlResponse, err := parseResponse(([]byte)(result), "listUsers", "mongodb")
		assert.Nil(t, err)
		assert.Equal(t, sqlResponse.Event, RespEveFail)
		assert.Contains(t, sqlResponse.Message, "db not ready")
	})

	t.Run("Binding Exec Success", func(t *testing.T) {
		result := `
	{"event":"Success","message":"[]"}
	`
		sqlResponse, err := parseResponse(([]byte)(result), "listUsers", "mongodb")
		assert.Nil(t, err)
		assert.Equal(t, sqlResponse.Event, RespEveSucc)
	})

	t.Run("Invalid Response Format", func(t *testing.T) {
		// msg cannot be parsed to json
		result := `
	{"event":"Success","message":"[]
	`
		_, err := parseResponse(([]byte)(result), "listUsers", "mongodb")
		assert.NotNil(t, err)
	})
}

func TestErrMsg(t *testing.T) {
	err := SQLChannelError{
		Reason: UnsupportedOps,
	}
	assert.True(t, strings.Contains(err.Error(), "unsupported"))
	assert.False(t, IsUnSupportedError(nil))
	assert.True(t, IsUnSupportedError(err))
	assert.False(t, IsUnSupportedError(errors.New("test")))
}

func newTCPServer(t *testing.T, port int) (int, func()) {
	var l net.Listener
	for i := 0; i < 3; i++ {
		l, _ = net.Listen("tcp", fmt.Sprintf(":%v", port))
		if l != nil {
			break
		}
		port++
	}
	if l == nil {
		t.Errorf("couldn't start listening")
	}
	closer := func() {
		l.Close()
	}
	return port, closer
}

func initSQLChannelClient(httpPort int, t *testing.T) (*OperationClient, func(), error) {
	port, closer := newTCPServer(t, 50001)
	podName := "pod-for-sqlchannel-test"
	pod := testapps.NewPodFactory("default", podName).
		AddContainer(corev1.Container{Name: viper.GetString(constant.KBToolsImage), Image: viper.GetString(constant.KBToolsImage)}).
		GetObject()
	pod.Spec.Containers[0].Ports = []corev1.ContainerPort{
		{
			ContainerPort: int32(httpPort),
			Name:          constant.ProbeHTTPPortName,
			Protocol:      "TCP",
		},
		{
			ContainerPort: int32(port),
			Name:          constant.ProbeGRPCPortName,
			Protocol:      "TCP",
		},
	}
	pod.Status.PodIP = "127.0.0.1"
	cli, err := NewClientWithPod(pod, "mysql")
	if err != nil {
		t.Errorf("new sql channel client error: %v", err)
	}
	return cli, closer, err
}

func initHTTPServer(resp []byte) int {
	s := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(200)
		_, _ = writer.Write(resp)
	}))
	addr := s.Listener.Addr().String()
	index := strings.LastIndex(addr, ":")
	portStr := addr[index+1:]
	port, _ := strconv.Atoi(portStr)
	return port
}
