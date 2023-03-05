/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
