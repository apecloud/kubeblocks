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

package binding

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"testing"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	"github.com/spf13/viper"
)

type fakeOperations struct {
	BaseOperations
}

var (
	testDBPort = 34521
	testRole   = "leader"
)

func TestInit(t *testing.T) {
	p := mockFakeOperations()
	p.Init(bindings.Metadata{})
	if p.OriRole != "" {
		t.Errorf("p.oriRole init failed: %s", p.OriRole)
	}
	if p.CheckRunningFailedCount != 0 {
		t.Errorf("p.CheckRunningFailedCount init failed: %d", p.CheckRunningFailedCount)
	}
	if p.CheckRoleFailedCount != 0 {
		t.Errorf("p.CheckRoleFailedCount init failed: %d", p.CheckRoleFailedCount)
	}
	if p.RoleUnchangedCount != 0 {
		t.Errorf("p.RoleUnchangedCount init failed: %d", p.RoleUnchangedCount)
	}
	if p.FailedEventReportFrequency != defaultFailedEventReportFrequency {
		t.Errorf("p.FailedEventReportFrequency init failed: %d", p.FailedEventReportFrequency)
	}
	if p.RoleDetectionThreshold != defaultRoleDetectionThreshold {
		t.Errorf("p.RoleDetectionThreshold init failed: %d", p.RoleDetectionThreshold)
	}
	if p.DBPort != testDBPort {
		t.Errorf("p.DBPort init failed: %d", p.DBPort)
	}
}

func TestOperations(t *testing.T) {
	p := mockFakeOperations()
	p.Init(bindings.Metadata{})
	ops := p.Operations()

	if len(ops) != 4 {
		t.Errorf("p.OperationMap init failed: %s", p.OriRole)
	}
}

func TestInvoke(t *testing.T) {
	viper.SetDefault("KB_SERVICE_ROLES", "{\"follower\":\"Readonly\",\"leader\":\"ReadWrite\"}")
	p := mockFakeOperations()
	p.Init(bindings.Metadata{})

	t.Run("CheckRunning", func(t *testing.T) {
		opsRes := OpsResult{}
		metadata := map[string]string{"sql": ""}
		req := &bindings.InvokeRequest{
			Data:      nil,
			Metadata:  metadata,
			Operation: CheckRunningOperation,
		}
		t.Run("Failed", func(t *testing.T) {
			resp, err := p.Invoke(context.Background(), req)
			if err != nil {
				t.Errorf("CheckRunning failed: %s", err)
			}
			err = json.Unmarshal(resp.Data, &opsRes)
			if err != nil {
				t.Errorf("CheckRunning failed: %s", err)
			}
			if opsRes["event"] != "CheckRunningFailed" {
				t.Errorf("unexpected response: %s", string(resp.Data))
			}
		})

		t.Run("Success", func(t *testing.T) {
			server := p.startFooServer(t)
			defer stopFooServer(server)
			resp, _ := p.Invoke(context.Background(), req)
			err := json.Unmarshal(resp.Data, &opsRes)
			if err != nil {
				t.Errorf("CheckRunning failed: %s", err)
			}
			if opsRes["event"] != "CheckRunningSuccess" {
				t.Errorf("unexpected response: %s", string(resp.Data))
			}
		})
	})

	t.Run("CheckRole", func(t *testing.T) {
		opsRes := OpsResult{}
		metadata := map[string]string{"sql": ""}
		req := &bindings.InvokeRequest{
			Data:      nil,
			Metadata:  metadata,
			Operation: CheckRoleOperation,
		}

		t.Run("Success", func(t *testing.T) {
			p.BaseOperations.GetRole = p.GetRole
			resp, err := p.Invoke(context.Background(), req)
			if err != nil {
				t.Errorf("CheckRole error: %s", err)
			}
			err = json.Unmarshal(resp.Data, &opsRes)
			if err != nil {
				t.Errorf("CheckRole failed: %s", err)
			}
			if p.OriRole != testRole {
				t.Errorf("CheckRole error: %s", p.OriRole)
			}
			if opsRes["role"] != testRole {
				t.Errorf("CheckRole response error: %s", resp.Data)
			}
		})

		t.Run("roleInvalid", func(t *testing.T) {
			testRole = "leader1"
			resp, err := p.Invoke(context.Background(), req)
			if err != nil {
				t.Errorf("CheckRole error: %s", err)
			}
			err = json.Unmarshal(resp.Data, &opsRes)
			if err != nil {
				t.Errorf("CheckRole failed: %s", err)
			}
			if opsRes["event"] != "roleInvalid" {
				t.Errorf("CheckRole response error: %s", resp.Data)
			}
		})

		t.Run("NotImplemented", func(t *testing.T) {
			p.BaseOperations.GetRole = nil
			resp, err := p.Invoke(context.Background(), req)
			if err != nil {
				t.Errorf("GetRole error: %s", err)
			}
			err = json.Unmarshal(resp.Data, &opsRes)
			if err != nil {
				t.Errorf("GetRole failed: %s", err)
			}
			if opsRes["event"] != OperationNotImplemented {
				t.Errorf("GetRole response error: %s", resp.Data)
			}
		})

		t.Run("Failed", func(t *testing.T) {
			p.BaseOperations.GetRole = p.GetRoleFailed
			resp, err := p.Invoke(context.Background(), req)
			if err != nil {
				t.Errorf("GetRole error: %s", err)
			}
			err = json.Unmarshal(resp.Data, &opsRes)
			if err != nil {
				t.Errorf("GetRole failed: %s", err)
			}
			if opsRes["event"] != "checkRoleFailed" {
				t.Errorf("GetRole response error: %s", resp.Data)
			}
		})
	})

	t.Run("GetRole", func(t *testing.T) {
		opsRes := OpsResult{}
		metadata := map[string]string{"sql": ""}
		req := &bindings.InvokeRequest{
			Data:      nil,
			Metadata:  metadata,
			Operation: GetRoleOperation,
		}

		t.Run("Success", func(t *testing.T) {
			p.BaseOperations.GetRole = p.GetRole
			resp, err := p.Invoke(context.Background(), req)
			if err != nil {
				t.Errorf("GetRole error: %s", err)
			}
			err = json.Unmarshal(resp.Data, &opsRes)
			if err != nil {
				t.Errorf("GetRole failed: %s", err)
			}
			if opsRes["role"] != testRole {
				t.Errorf("GetRole response error: %s", resp.Data)
			}
		})

		t.Run("NotImplemented", func(t *testing.T) {
			p.BaseOperations.GetRole = nil
			resp, err := p.Invoke(context.Background(), req)
			if err != nil {
				t.Errorf("GetRole error: %s", err)
			}
			err = json.Unmarshal(resp.Data, &opsRes)
			if err != nil {
				t.Errorf("GetRole failed: %s", err)
			}
			if opsRes["event"] != OperationNotImplemented {
				t.Errorf("GetRole response error: %s", resp.Data)
			}
		})

		t.Run("Failed", func(t *testing.T) {
			p.BaseOperations.GetRole = p.GetRoleFailed
			resp, err := p.Invoke(context.Background(), req)
			if err != nil {
				t.Errorf("GetRole error: %s", err)
			}
			err = json.Unmarshal(resp.Data, &opsRes)
			if err != nil {
				t.Errorf("GetRole failed: %s", err)
			}
			if opsRes["event"] != "getRoleFailed" {
				t.Errorf("GetRole response error: %s", resp.Data)
			}
		})
	})
}

func stopFooServer(server net.Listener) {
	if server != nil {
		server.Close()
	}
}

func mockFakeOperations() *fakeOperations {
	log := logger.NewLogger("base_test")
	p := BaseOperations{Logger: log}
	return &fakeOperations{BaseOperations: p}
}

// Init initializes the fake binding.
func (fakeOps *fakeOperations) Init(metadata bindings.Metadata) {
	fakeOps.BaseOperations.Init(metadata)
	fakeOps.Logger.Debug("Initializing MySQL binding")
	fakeOps.DBType = "mysql"
	fakeOps.InitIfNeed = fakeOps.initIfNeed
	fakeOps.BaseOperations.GetRole = fakeOps.GetRole
	fakeOps.DBPort = fakeOps.GetRunningPort()
	fakeOps.RegisterOperation(CheckStatusOperation, fakeOps.CheckStatusOps)
}

func (fakeOps *fakeOperations) initIfNeed() bool {
	return false
}

func (fakeOps *fakeOperations) GetRunningPort() int {
	return testDBPort
}

func (fakeOps *fakeOperations) CheckStatusOps(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (OpsResult, error) {
	return OpsResult{}, nil
}

func (fakeOps *fakeOperations) GetRole(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (string, error) {
	return testRole, nil
}

func (fakeOps *fakeOperations) GetRoleFailed(ctx context.Context, req *bindings.InvokeRequest, resp *bindings.InvokeResponse) (string, error) {
	return testRole, fmt.Errorf("mock error")
}

func (fakeOps *fakeOperations) startFooServer(t *testing.T) net.Listener {
	var server net.Listener
	var err error
	for i := 0; i < 3; i++ {
		server, err = net.Listen("tcp", ":"+strconv.Itoa(fakeOps.DBPort))
		if server != nil {
			return server
		}
		fakeOps.DBPort++
	}

	if server == nil {
		t.Errorf("couldn't start listening: %v", err)
	}
	return nil
}
