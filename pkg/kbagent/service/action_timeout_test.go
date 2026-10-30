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

package service

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

func TestActionTimeoutPolicy(t *testing.T) {
	for _, tt := range []struct {
		name, action string
		seconds      *int32
		want         time.Duration
	}{
		{"ordinary cap", "roleProbe", int32Ptr(300), 60 * time.Second},
		{"long switchover", "switchover", int32Ptr(280), 280 * time.Second},
		{"switchover cap", "switchover", int32Ptr(900), 300 * time.Second},
		{"switchover remains bounded", "switchover", int32Ptr(-1), 300 * time.Second},
		{"short switchover", "switchover", int32Ptr(20), 20 * time.Second},
		{"nil default", "switchover", nil, defaultActionCallTimeout},
		{"zero default", "switchover", int32Ptr(0), defaultActionCallTimeout},
	} {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()
			ctx, cancel := actionCallTimeoutContext(context.Background(), tt.action, tt.seconds)
			defer cancel()
			deadline, ok := ctx.Deadline()
			if !ok || deadline.Sub(start) < tt.want || deadline.Sub(start) > tt.want+time.Second {
				t.Fatalf("deadline = %v, want %v", deadline.Sub(start), tt.want)
			}
		})
	}
}
func TestSwitchoverPreservesCallerDeadline(t *testing.T) {
	parent, stop := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer stop()
	child, cancel := actionCallTimeoutContext(parent, "switchover", int32Ptr(300))
	defer cancel()
	pd, _ := parent.Deadline()
	cd, _ := child.Deadline()
	if !pd.Equal(cd) {
		t.Fatal("action extended caller deadline")
	}
	stop()
	<-child.Done()
	if child.Err() != context.Canceled {
		t.Fatal(child.Err())
	}
}
func int32Ptr(x int32) *int32 { return &x }

// The public action handler must not report success while the command is still
// performing recovery, even though the implementation uses a worker goroutine.
func TestSynchronousSwitchoverWaitsForCompletion(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		_, _ = w.Write([]byte("dg_recovery_verified"))
	}))
	defer server.Close()
	defer unblock()
	host, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	service, err := newActionService(logr.Discard(), []proto.Action{{Name: "switchover", TimeoutSeconds: 300, HTTP: &proto.HTTPAction{Host: host, Port: port}}})
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		out []byte
		err error
	}
	done := make(chan result, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() {
		out, err := service.handleRequest(ctx, &proto.ActionRequest{Action: "switchover"})
		done <- result{out, err}
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("action did not start")
	}
	select {
	case r := <-done:
		t.Fatalf("action returned before recovery: %v", r)
	case <-time.After(20 * time.Millisecond):
	}
	unblock()
	select {
	case r := <-done:
		if r.err != nil || string(r.out) != "dg_recovery_verified" {
			t.Fatalf("completion: %q %v", r.out, r.err)
		}
	case <-ctx.Done():
		t.Fatal("action did not return after recovery")
	}
}
