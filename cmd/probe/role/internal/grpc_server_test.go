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
