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

package grpcserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	health "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/apecloud/kubeblocks/pkg/lorry/engines/custom"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/register"
	"github.com/apecloud/kubeblocks/pkg/lorry/operations"
	"github.com/apecloud/kubeblocks/pkg/lorry/operations/replica"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("GRPC Server", func() {
	Context("new GRPC server", func() {
		It("fail -- no check role operation", func() {
			delete(operations.Operations(), strings.ToLower(string(util.CheckRoleOperation)))
			_, err := NewGRPCServer()
			Expect(err).Should(HaveOccurred())
		})

		It("success", func() {
			err := operations.Register(strings.ToLower(string(util.CheckRoleOperation)), &replica.CheckRole{})
			Expect(err).ShouldNot(HaveOccurred())
			server, err := NewGRPCServer()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(server).ShouldNot(BeNil())
			Expect(server.Watch(nil, nil)).ShouldNot(Succeed())
		})
	})

	Context("check role", func() {
		It("role changed", func() {
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

			customManager, err := custom.NewManager(nil)
			Expect(err).Should(BeNil())
			register.SetDBManager(customManager)

			server, _ := NewGRPCServer()
			check, err := server.Check(context.Background(), nil)

			Expect(err).Should(HaveOccurred())
			Expect(check.Status).Should(Equal(health.HealthCheckResponse_NOT_SERVING))

			// set up the expected answer
			result := map[string]string{}
			result["event"] = "Success"
			result["operation"] = "checkRole"
			result["originalRole"] = ""
			result["role"] = "leader"
			ans, _ := json.Marshal(result)

			Expect(err.Error()).Should(Equal(string(ans)))
		})
	})
})
