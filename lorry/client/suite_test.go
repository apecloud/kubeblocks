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

package client

import (
	"fmt"
	"net"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	"github.com/valyala/fasthttp"

	"github.com/apecloud/kubeblocks/lorry/engines"
	"github.com/apecloud/kubeblocks/lorry/engines/register"
	"github.com/apecloud/kubeblocks/lorry/httpserver"
	opsregister "github.com/apecloud/kubeblocks/lorry/operations/register"
)

var (
	lorryHTTPPort = 3501
)

func init() {
	viper.AutomaticEnv()
	viper.SetDefault("KB_POD_NAME", "pod-test")
	viper.SetDefault("KB_CLUSTER_COMP_NAME", "cluster-component-test")
	viper.SetDefault("KB_NAMESPACE", "namespace-test")
}
func TestLorryClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lorry Client. Suite")
}

var _ = BeforeSuite(func() {
	mockManager, err := engines.NewMockManager(nil)
	Expect(err).Should(BeNil())
	register.SetDBManager(mockManager)
	ops := opsregister.Operations()
	httpServer := httpserver.NewServer(ops)
	StartHTTPServerNonBlocking(httpServer)
})

func newTCPServer(port int) (net.Listener, int) {
	var l net.Listener
	for i := 0; i < 3; i++ {
		l, _ = net.Listen("tcp", fmt.Sprintf(":%v", port))
		if l != nil {
			break
		}
		port++
	}
	Expect(l).ShouldNot(BeNil())
	return l, port
}

func StartHTTPServerNonBlocking(s httpserver.Server) {
	handler := s.Router()

	listener, port := newTCPServer(lorryHTTPPort)
	lorryHTTPPort = port

	go func() {
		Expect(fasthttp.Serve(listener, handler)).Should(Succeed())
	}()
}
