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

	. "github.com/apecloud/kubeblocks/cmd/probe/util"
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
			if opsRes["event"] != OperationFailed {
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
			if opsRes["event"] != OperationSuccess {
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
			if opsRes["event"] != OperationInvalid {
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
			if opsRes["event"] != OperationFailed {
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
			if opsRes["event"] != OperationFailed {
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
