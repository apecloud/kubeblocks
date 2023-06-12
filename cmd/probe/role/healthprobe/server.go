package healthprobe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/apecloud/kubeblocks/cmd/probe/role/internal"

	"google.golang.org/grpc/codes"
	health "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

type GrpcServer struct {
	internal.RoleAgent
}

func (s *GrpcServer) Check(ctx context.Context, in *health.HealthCheckRequest) (*health.HealthCheckResponse, error) {
	opsRes, shouldNotify := s.CheckRole(ctx)
	if _, exist := opsRes["event"]; !exist || len(opsRes) == 0 {
		log.Printf("CheckRole returns empty\n")
		return &health.HealthCheckResponse{Status: health.HealthCheckResponse_NOT_SERVING}, errors.New("CheckRole returns empty")
	}

	buf, err := json.Marshal(opsRes)
	if err != nil {
		log.Printf("parse opsresult error: %v\n", err)
		return &health.HealthCheckResponse{Status: health.HealthCheckResponse_NOT_SERVING}, err
	}

	roleInfo := string(buf)
	fmt.Println(roleInfo)
	if shouldNotify {
		return &health.HealthCheckResponse{Status: health.HealthCheckResponse_NOT_SERVING}, errors.New(roleInfo)
	} else {
		return &health.HealthCheckResponse{Status: health.HealthCheckResponse_SERVING}, nil
	}
}

func (s *GrpcServer) Watch(in *health.HealthCheckRequest, _ health.Health_WatchServer) error {
	// todo didn't implement the `watch` function
	return status.Error(codes.Unimplemented, "unimplemented")
}

func NewGrpcServer() (*GrpcServer, error) {
	res := &GrpcServer{
		RoleAgent: *internal.NewRoleAgent(os.Stdin, ""),
	}
	err := res.RoleAgent.Init()
	if err != nil {
		return nil, err
	}
	return res, nil
}
