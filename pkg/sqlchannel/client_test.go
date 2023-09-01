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

package sqlchannel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	. "github.com/apecloud/kubeblocks/pkg/sqlchannel/util"
	"github.com/apecloud/kubeblocks/pkg/testutil/apps"

	dapr "github.com/dapr/go-sdk/client"
	pb "github.com/dapr/go-sdk/dapr/proto/runtime/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

type testDaprServer struct {
	pb.UnimplementedDaprServer
	state                       map[string][]byte
	configurationSubscriptionID map[string]chan struct{}
	cachedRequest               map[string]*response
}

type response struct {
	bindingResponse *pb.InvokeBindingResponse
	err             error
}

var _ pb.DaprServer = &testDaprServer{}

func (s *testDaprServer) InvokeBinding(ctx context.Context, req *pb.InvokeBindingRequest) (*pb.InvokeBindingResponse, error) {
	time.Sleep(100 * time.Millisecond)
	darpRequest := dapr.InvokeBindingRequest{Name: req.Name, Operation: req.Operation, Data: req.Data, Metadata: req.Metadata}
	resp, ok := s.cachedRequest[GetMapKeyFromRequest(&darpRequest)]
	if ok {
		return resp.bindingResponse, resp.err
	} else {
		return nil, fmt.Errorf("unexpected request")
	}
}

func (s *testDaprServer) ExepctRequest(req *pb.InvokeBindingRequest, resp *pb.InvokeBindingResponse, err error) {
	darpRequest := dapr.InvokeBindingRequest{Name: req.Name, Operation: req.Operation, Data: req.Data, Metadata: req.Metadata}
	s.cachedRequest[GetMapKeyFromRequest(&darpRequest)] = &response{
		bindingResponse: resp,
		err:             err,
	}
}

func TestNewClientWithPod(t *testing.T) {
	daprServer := &testDaprServer{
		state:                       make(map[string][]byte),
		configurationSubscriptionID: map[string]chan struct{}{},
		cachedRequest:               make(map[string]*response),
	}

	port, closer := newTCPServer(t, daprServer, 50001)
	defer closer()
	podName := "pod-for-sqlchannel-test"
	pod := apps.NewPodFactory("default", podName).
		AddContainer(corev1.Container{Name: apps.DefaultNginxContainerName, Image: apps.NginxImage}).
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

	t.Run("WithOutPodGPRCPort", func(t *testing.T) {
		podWithoutGRPCPort := pod.DeepCopy()
		podWithoutGRPCPort.Spec.Containers[0].Ports = podWithoutGRPCPort.Spec.Containers[0].Ports[:1]
		_, err := NewClientWithPod(podWithoutGRPCPort, "mysql")
		if err == nil {
			t.Errorf("new sql channel client union")
		}
	})

	t.Run("Success", func(t *testing.T) {
		_, err := NewClientWithPod(pod, "mysql")
		if err != nil {
			t.Errorf("new sql channel client error: %v", err)
		}
	})
}

func TestGPRC(t *testing.T) {
	url := os.Getenv("PROBE_GRPC_URL")
	if url == "" {
		t.SkipNow()
	}
	req := &dapr.InvokeBindingRequest{
		Name:      "mongodb",
		Operation: "getRole",
		Data:      []byte(""),
		Metadata:  map[string]string{},
	}
	cli, _ := dapr.NewClientWithAddress(url)
	resp, _ := cli.InvokeBinding(context.Background(), req)
	t.Logf("probe response metadata: %v", resp.Metadata)
	result := map[string]string{}
	_ = json.Unmarshal(resp.Data, &result)
	t.Logf("probe response data: %v", result)

}

func TestGetRole(t *testing.T) {
	daprServer, cli, closer, err := initSQLChannelClient(t)
	if err != nil {
		t.Errorf("new sql channel client error: %v", err)
	}
	defer closer()

	daprServer.ExepctRequest(&pb.InvokeBindingRequest{
		Name:      "mysql",
		Operation: "getRole",
	}, &pb.InvokeBindingResponse{
		Data: []byte("{\"role\": \"leader\"}"),
	}, nil)

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
		cli.ReconcileTimeout = 50 * time.Millisecond
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
	daprServer, cli, closer, err := initSQLChannelClient(t)
	if err != nil {
		t.Errorf("new sql channel client error: %v", err)
	}
	defer closer()

	roleNames, _ := json.Marshal([]string{"kbadmin", "kbprobe"})
	sqlResponse := SQLChannelResponse{
		Event:   RespEveSucc,
		Message: string(roleNames),
	}
	respData, _ := json.Marshal(sqlResponse)
	resp := &pb.InvokeBindingResponse{
		Data: respData,
	}

	daprServer.ExepctRequest(&pb.InvokeBindingRequest{
		Name:      "mysql",
		Operation: string(ListSystemAccountsOp),
	}, resp, nil)

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
	daprServer, cli, closer, err := initSQLChannelClient(t)
	if err != nil {
		t.Errorf("new sql channel client error: %v", err)
	}
	defer closer()

	sqlResponse := SQLChannelResponse{
		Event: RespEveSucc,
	}
	respData, _ := json.Marshal(sqlResponse)
	resp := &pb.InvokeBindingResponse{
		Data: respData,
	}

	t.Run("Join Member success", func(t *testing.T) {
		daprServer.ExepctRequest(&pb.InvokeBindingRequest{
			Name:      "mysql",
			Operation: string(JoinMemberOperation),
		}, resp, nil)
		cli.ReconcileTimeout = 200 * time.Millisecond
		err := cli.JoinMember(context.TODO())
		assert.Nil(t, err)
	})

	t.Run("Join Member fail", func(t *testing.T) {
		daprServer.ExepctRequest(&pb.InvokeBindingRequest{
			Name:      "mysql",
			Operation: string(JoinMemberOperation),
		}, nil, errors.New("join member failed"))
		cli.ReconcileTimeout = 200 * time.Millisecond
		err := cli.JoinMember(context.TODO())
		assert.NotNil(t, err)
	})
}

func TestLeaveMember(t *testing.T) {
	daprServer, cli, closer, err := initSQLChannelClient(t)
	if err != nil {
		t.Errorf("new sql channel client error: %v", err)
	}
	defer closer()

	sqlResponse := SQLChannelResponse{
		Event: RespEveSucc,
	}
	respData, _ := json.Marshal(sqlResponse)
	resp := &pb.InvokeBindingResponse{
		Data: respData,
	}

	t.Run("Leave Member success", func(t *testing.T) {
		daprServer.ExepctRequest(&pb.InvokeBindingRequest{
			Name:      "mysql",
			Operation: string(LeaveMemberOperation),
		}, resp, nil)
		cli.ReconcileTimeout = 200 * time.Millisecond
		err := cli.LeaveMember(context.TODO())
		assert.Nil(t, err)
	})

	t.Run("Join Member success", func(t *testing.T) {
		daprServer.ExepctRequest(&pb.InvokeBindingRequest{
			Name:      "mysql",
			Operation: string(LeaveMemberOperation),
		}, nil, errors.New("leave member failed"))
		cli.ReconcileTimeout = 200 * time.Millisecond
		err := cli.LeaveMember(context.TODO())
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

func newTCPServer(t *testing.T, daprServer pb.DaprServer, port int) (int, func()) {
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
	s := grpc.NewServer()
	pb.RegisterDaprServer(s, daprServer)

	go func() {
		if err := s.Serve(l); err != nil && err.Error() != "closed" {
			t.Errorf("test server exited with error: %v", err)
		}
	}()

	closer := func() {
		s.Stop()
		l.Close()
	}
	return port, closer
}

func initSQLChannelClient(t *testing.T) (*testDaprServer, *OperationClient, func(), error) {
	daprServer := &testDaprServer{
		state:                       make(map[string][]byte),
		configurationSubscriptionID: map[string]chan struct{}{},
		cachedRequest:               make(map[string]*response),
	}

	port, closer := newTCPServer(t, daprServer, 50001)
	podName := "pod-for-sqlchannel-test"
	pod := apps.NewPodFactory("default", podName).
		AddContainer(corev1.Container{Name: apps.DefaultNginxContainerName, Image: apps.NginxImage}).GetObject()
	pod.Spec.Containers[0].Ports = []corev1.ContainerPort{
		{
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
	cli, err := NewClientWithPod(pod, "mysql")
	if err != nil {
		t.Errorf("new sql channel client error: %v", err)
	}
	return daprServer, cli, closer, err
}
