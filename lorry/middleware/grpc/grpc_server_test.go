package grpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	health "google.golang.org/grpc/health/grpc_health_v1"

	viper "github.com/apecloud/kubeblocks/internal/viperx"
	"github.com/apecloud/kubeblocks/lorry/binding"
	"github.com/apecloud/kubeblocks/lorry/middleware/probe"
)

func TestNewServer(t *testing.T) {
	server := NewGRPCServer()
	assert.NotNil(t, server.logger)
	assert.Error(t, server.Watch(nil, nil))
}

func TestCheck(t *testing.T) {
	// set up the host
	s := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			_, _ = w.Write([]byte("leader"))
		}),
	)
	addr := s.Listener.Addr().String()
	index := strings.LastIndex(addr, ":")
	portStr := addr[index+1:]

	// set up the environment
	viper.Set("KB_RSM_ACTION_SVC_LIST", "["+portStr+"]")
	viper.Set("KB_RSM_ROLE_UPDATE_MECHANISM", "ReadinessProbeEventUpdate")

	assert.Nil(t, probe.RegisterBuiltin(""))
	server := NewGRPCServer()
	check, err := server.Check(context.Background(), nil)

	assert.Error(t, err)
	assert.Equal(t, health.HealthCheckResponse_NOT_SERVING, check.Status)

	// set up the expected answer
	result := binding.OpsResult{}
	result["event"] = "Success"
	result["operation"] = "checkRole"
	result["originalRole"] = ""
	result["role"] = "leader"
	ans, _ := json.Marshal(result)

	assert.Equal(t, string(ans), err.Error())
}
