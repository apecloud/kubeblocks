/*
Copyright ApeCloud Inc.

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

package internal

import (
	"context"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	"github.com/spf13/viper"
)

type fakeBinding struct{}

const (
	testDbPort = 34521
	testRole   = "leader"
)

func TestInit(t *testing.T) {
	p := mockProbeBase()
	p.Init()
	if p.oriRole != "" {
		t.Errorf("p.oriRole init failed: %s", p.oriRole)
	}
	if p.runningCheckFailedCount != 0 {
		t.Errorf("p.runningCheckFailedCount init failed: %d", p.runningCheckFailedCount)
	}
	if p.roleCheckFailedCount != 0 {
		t.Errorf("p.roleCheckFailedCount init failed: %d", p.roleCheckFailedCount)
	}
	if p.roleUnchangedCount != 0 {
		t.Errorf("p.roleUnchangedCount init failed: %d", p.roleUnchangedCount)
	}
	if p.checkFailedThreshold != defaultCheckFailedThreshold {
		t.Errorf("p.checkFailedThreshold init failed: %d", p.checkFailedThreshold)
	}
	if p.roleUnchangedThreshold != defaultRoleUnchangedThreshold {
		t.Errorf("p.roleUnchangedThreshold init failed: %d", p.roleUnchangedThreshold)
	}
	if p.dbPort != testDbPort {
		t.Errorf("p.dbPort init failed: %d", p.dbPort)
	}
}

func TestInvoke(t *testing.T) {
	p := mockProbeBase()
	viper.SetDefault("KB_SERVICE_ROLES", "{\"follower\":\"Readonly\",\"leader\":\"ReadWrite\"}")
	p.Init()

	t.Run("runningCheck", func(t *testing.T) {
		metadata := map[string]string{"sql": ""}
		req := &bindings.InvokeRequest{
			Data:      nil,
			Metadata:  metadata,
			Operation: RunningCheckOperation,
		}
		resp, err := p.Invoke(context.Background(), req)
		if err != nil {
			t.Errorf("runningCheck failed: %s", err)
		}
		if strings.Index(string(resp.Data), "{\"event\":\"runningCheckFailed\"") < 0 {
			t.Errorf("unexpected response: %s", string(resp.Data))
		}

		message := "{\"message\":\"TCP Connection Established Successfully!\"}"
		server := startFooServer(p.dbPort, t)
		defer stopFooServer(server)
		resp, _ = p.Invoke(context.Background(), req)
		if string(resp.Data) != message {
			t.Errorf("unexpected response: %s", string(resp.Data))
		}
	})

	t.Run("roleCheck", func(t *testing.T) {
		metadata := map[string]string{"sql": ""}
		req := &bindings.InvokeRequest{
			Data:      nil,
			Metadata:  metadata,
			Operation: RoleCheckOperation,
		}

		resp, err := p.Invoke(context.Background(), req)
		if err != nil {
			t.Errorf("roleCheck error: %s", err)
		}
		if p.oriRole != testRole {
			t.Errorf("getRole error: %s", p.oriRole)
		}
		if string(resp.Data) != "{\"event\":\"roleChanged\",\"role\":\""+testRole+"\"}" {
			t.Errorf("roleCheck response error: %s", resp.Data)
		}
	})
}

func startFooServer(port int, t *testing.T) net.Listener {
	server, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if server == nil {
		t.Errorf("couldn't start listening: %s", err)
	}
	return server
}

func stopFooServer(server net.Listener) {
	if server != nil {
		server.Close()
	}
}

func mockProbeBase() *ProbeBase {
	p := &ProbeBase{}
	p.Logger = logger.NewLogger("base_binding_test")
	p.Operation = &fakeBinding{}

	return p
}

func (f *fakeBinding) InitIfNeed() error {
	return nil
}

func (f *fakeBinding) GetRunningPort() int {
	return testDbPort
}

func (f *fakeBinding) StatusCheck(ctx context.Context, cmd string, response *bindings.InvokeResponse) ([]byte, error) {
	return nil, nil
}

func (f *fakeBinding) GetRole(ctx context.Context, cmd string) (string, error) {
	return testRole, nil
}
