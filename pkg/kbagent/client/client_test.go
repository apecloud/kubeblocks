/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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
	"errors"
	"io"
	"testing"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

type stubClient struct{}

func (stubClient) Close() error {
	return nil
}

func (stubClient) Action(context.Context, proto.ActionRequest) (proto.ActionResponse, error) {
	return proto.ActionResponse{Message: "ok"}, nil
}

func TestMockClientLifecycle(t *testing.T) {
	t.Cleanup(UnsetMockClient)

	mock := stubClient{}
	SetMockClient(mock, nil)
	if GetMockClient() != mock {
		t.Fatalf("GetMockClient() did not return mock client")
	}
	got, err := NewClient(func() (string, int32, error) {
		t.Fatal("endpoint should not be called when mock client is set")
		return "", 0, nil
	})
	if err != nil || got != mock {
		t.Fatalf("NewClient() = %v, %v, want mock nil-error", got, err)
	}

	mockErr := errors.New("mock")
	SetMockClient(nil, mockErr)
	got, err = NewClient(func() (string, int32, error) {
		t.Fatal("endpoint should not be called when mock error is set")
		return "", 0, nil
	})
	if got != nil || !errors.Is(err, mockErr) {
		t.Fatalf("NewClient() = %v, %v, want nil mockErr", got, err)
	}

	UnsetMockClient()
	if GetMockClient() != nil {
		t.Fatalf("mock client should be cleared")
	}
}

func TestNewClientEndpointBranches(t *testing.T) {
	endpointErr := errors.New("endpoint")
	if got, err := NewClient(func() (string, int32, error) { return "", 0, endpointErr }); got != nil || !errors.Is(err, endpointErr) {
		t.Fatalf("NewClient endpoint error = %v, %v", got, err)
	}

	if got, err := NewClient(func() (string, int32, error) { return "", 0, nil }); got != nil || err != nil {
		t.Fatalf("NewClient empty endpoint = %v, %v, want nil nil", got, err)
	}

	got, err := NewClient(func() (string, int32, error) { return "127.0.0.1", 3501, nil })
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	if _, ok := got.(*httpClient); !ok {
		t.Fatalf("NewClient returned %T, want *httpClient", got)
	}
	if err := got.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestGeneratedMockClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := NewMockClient(ctrl)
	mock.EXPECT().
		Action(gomock.Any(), proto.ActionRequest{Action: "backup"}).
		Return(proto.ActionResponse{Message: "ok"}, nil)

	resp, err := mock.Action(context.Background(), proto.ActionRequest{Action: "backup"})
	if err != nil {
		t.Fatalf("mock Action() error = %v", err)
	}
	if resp.Message != "ok" {
		t.Fatalf("mock Action() response = %#v", resp)
	}
	if err := mock.Close(); err != nil {
		t.Fatalf("mock Close() error = %v", err)
	}
}

func TestNewPortForwardClientStableBranches(t *testing.T) {
	t.Cleanup(UnsetMockClient)

	mock := stubClient{}
	SetMockClient(mock, nil)
	got, err := NewPortForwardClient(&corev1.Pod{}, func() (string, int32, error) {
		t.Fatal("endpoint should not be called when mock client is set")
		return "", 0, nil
	})
	if err != nil || got != mock {
		t.Fatalf("NewPortForwardClient mock = %v, %v", got, err)
	}

	UnsetMockClient()
	endpointErr := errors.New("endpoint")
	got, err = NewPortForwardClient(&corev1.Pod{}, func() (string, int32, error) {
		return "", 0, endpointErr
	})
	if got != nil || !errors.Is(err, endpointErr) {
		t.Fatalf("NewPortForwardClient endpoint error = %v, %v", got, err)
	}
}

func TestPortForwardClientStableErrors(t *testing.T) {
	pf := &portForwardClient{
		pod:    &corev1.Pod{},
		port:   "3501",
		config: &rest.Config{},
		logger: logr.Discard(),
	}
	if err := pf.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	_, _ = pf.createDialer("POST", nil, &rest.Config{})
	readyCh := make(chan struct{})
	stopCh := make(chan struct{})
	forwarder, err := pf.newPortForwarder(readyCh, stopCh, io.Discard)
	close(stopCh)
	if err != nil {
		t.Fatalf("newPortForwarder() error = %v", err)
	}
	if forwarder == nil {
		t.Fatalf("newPortForwarder() returned nil")
	}
	if resp, err := pf.Action(context.Background(), proto.ActionRequest{Action: "backup"}); err == nil || resp.Message != "" {
		t.Fatalf("expected Action error for empty rest config, got %#v, %v", resp, err)
	}
}
