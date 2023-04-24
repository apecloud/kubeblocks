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
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	pb "github.com/dapr/go-sdk/dapr/proto/runtime/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"

	intctrlutil "github.com/apecloud/kubeblocks/internal/constant"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
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
		Name:          intctrlutil.ProbeHTTPPortName,
		Protocol:      "TCP",
	},
		{
			ContainerPort: int32(port),
			Name:          intctrlutil.ProbeGRPCPortName,
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
			t.Errorf("new sql channel client unexpection")
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
	port, closer := newTCPServer(t, 50001)
	defer closer()
	podName := "pod-for-sqlchannel-test"
	pod := testapps.NewPodFactory("default", podName).
		AddContainer(corev1.Container{Name: testapps.DefaultNginxContainerName, Image: testapps.NginxImage}).GetObject()
	pod.Spec.Containers[0].Ports = []corev1.ContainerPort{{
		ContainerPort: int32(3501),
		Name:          intctrlutil.ProbeHTTPPortName,
		Protocol:      "TCP",
	},
		{
			ContainerPort: int32(port),
			Name:          intctrlutil.ProbeGRPCPortName,
			Protocol:      "TCP",
		},
	}
	pod.Status.PodIP = "127.0.0.1"
	cli, err := NewClientWithPod(pod, "mysql")
	if err != nil {
		t.Errorf("new sql channel client error: %v", err)
	}

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

func TestParseSqlChannelResult(t *testing.T) {
	t.Run("Binding Not Supported", func(t *testing.T) {
		result := `
	{"errorCode":"ERR_INVOKE_OUTPUT_BINDING","message":"error when invoke output binding mongodb: binding mongodb does not support operation listUsers. supported operations:checkRunning checkRole getRole"}
	`
		sqlResposne, err := parseResponse(([]byte)(result), "listUsers", "mongodb")
		assert.Nil(t, err)
		assert.Equal(t, sqlResposne.Event, RespEveFail)
		assert.Contains(t, sqlResposne.Message, "not supported")
	})

	t.Run("Binding Exec Failed", func(t *testing.T) {
		result := `
	{"event":"Failed","message":"db not ready"}
	`
		sqlResposne, err := parseResponse(([]byte)(result), "listUsers", "mongodb")
		assert.Nil(t, err)
		assert.Equal(t, sqlResposne.Event, RespEveFail)
		assert.Contains(t, sqlResposne.Message, "db not ready")
	})

	t.Run("Binding Exec Success", func(t *testing.T) {
		result := `
	{"event":"Success","message":"[]"}
	`
		sqlResposne, err := parseResponse(([]byte)(result), "listUsers", "mongodb")
		assert.Nil(t, err)
		assert.Equal(t, sqlResposne.Event, RespEveSucc)
	})

	t.Run("Invalid Resonse Format", func(t *testing.T) {
		// msg cannot be parsed to json
		result := `
	{"event":"Success","message":"[]
	`
		_, err := parseResponse(([]byte)(result), "listUsers", "mongodb")
		assert.NotNil(t, err)
	})
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
	s := grpc.NewServer()
	pb.RegisterDaprServer(s, &testDaprServer{
		state:                       make(map[string][]byte),
		configurationSubscriptionID: map[string]chan struct{}{},
	})

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

type testDaprServer struct {
	pb.UnimplementedDaprServer
	state                       map[string][]byte
	configurationSubscriptionID map[string]chan struct{}
}

func (s *testDaprServer) InvokeBinding(ctx context.Context, req *pb.InvokeBindingRequest) (*pb.InvokeBindingResponse, error) {
	time.Sleep(100 * time.Millisecond)
	if req.Data == nil {
		return &pb.InvokeBindingResponse{
			Data:     []byte("{\"role\": \"leader\"}"),
			Metadata: map[string]string{"k1": "v1", "k2": "v2"},
		}, nil
	}
	return &pb.InvokeBindingResponse{
		Data:     req.Data,
		Metadata: req.Metadata,
	}, nil
}
