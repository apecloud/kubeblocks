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

package internal

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	health "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
)

type fakeGrpcServer struct {
	*GrpcServer
}

func mockFakeGrpcServer(path string) *fakeGrpcServer {
	return &fakeGrpcServer{GrpcServer: NewGrpcServer(path)}
}

func TestCheck(t *testing.T) {

	t.Run("Success", func(t *testing.T) {
		// init environment
		var ports = []int{8080}
		initEnv(ports)

		roleExpected := "leader"
		path := "check_success"
		initHTTP(roleExpected, path, 8080, true)
		time.Sleep(time.Second * 2)

		faker := mockFakeGrpcServer(path)
		if err := faker.Init(); err != nil {
			t.Errorf("fake init failed: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()

		resp, err := faker.Check(metadata.NewOutgoingContext(ctx, make(metadata.MD)), &health.HealthCheckRequest{})
		if resp.Status != health.HealthCheckResponse_NOT_SERVING || err == nil {
			t.Error("faker check didn't get role change ...")
		}
		msg := err.Error()
		var res OpsResult
		err = json.Unmarshal([]byte(msg), &res)
		if err != nil {
			t.Errorf("faker unmarshall failed: %v", err)
		}

		event, has := res["event"]
		if !has || event != OperationSuccess {
			t.Error("faker CheckRole failed: no event or event failed")
		}

		role, has := res["role"]
		if !has {
			t.Error("faker CheckRole failed")
		} else if role != roleExpected {
			t.Errorf("fake CheckRole failed:\n role should be %s, but %s", roleExpected, role)
		}
	})

	t.Run("Failure", func(t *testing.T) {
		// init environment
		var ports = []int{8081}
		initEnv(ports)

		path := "check_failed"
		initHTTP("", path, 8081, false)
		time.Sleep(1 * time.Second)

		faker := mockFakeGrpcServer(path)
		if err := faker.Init(); err != nil {
			t.Errorf("fake init failed: %v", err)
		}

		ctx, cacel := context.WithTimeout(context.Background(), time.Second*30)
		defer cacel()

		resp, err := faker.Check(metadata.NewOutgoingContext(ctx, make(metadata.MD)), &health.HealthCheckRequest{})

		if resp.Status != health.HealthCheckResponse_NOT_SERVING || err == nil {
			t.Error("faker check didn't get role change ...")
		}
		msg := err.Error()
		var res OpsResult
		err = json.Unmarshal([]byte(msg), &res)
		if err != nil {
			t.Errorf("faker unmarshall failed: %v", err)
		}

		event, has := res["event"]
		if !has || event != OperationFailed {
			t.Error("faker CheckRole failed: no event or event failed")
		}
	})

}
