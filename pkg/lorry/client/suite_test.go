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

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	"github.com/valyala/fasthttp"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/register"
	"github.com/apecloud/kubeblocks/pkg/lorry/httpserver"
	opsregister "github.com/apecloud/kubeblocks/pkg/lorry/operations/register"
)

var (
	lorryHTTPPort = 3501
	tcpListener   net.Listener
	dbManager     engines.DBManager
	mockDBManager *engines.MockDBManager
	dcsStore      dcs.DCS
	mockDCSStore  *dcs.MockDCS
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
	// Init mock db manager
	InitMockDBManager()

	// Init mock dcs store
	InitMockDCSStore()

	// Start lorry HTTP server
	StartHTTPServer()
})

var _ = AfterSuite(func() {
	StopHTTPServer()
})

func newTCPServer(port int) (net.Listener, int) {
	var err error
	for i := 0; i < 3; i++ {
		tcpListener, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%v", port))
		if err == nil {
			break
		}
		port++
	}
	Expect(err).Should(BeNil())
	return tcpListener, port
}

func StartHTTPServer() {
	ops := opsregister.Operations()
	s := httpserver.NewServer(ops)
	handler := s.Router()

	listener, port := newTCPServer(lorryHTTPPort)
	lorryHTTPPort = port

	go func() {
		Expect(fasthttp.Serve(listener, handler)).Should(Succeed())
	}()
}

func StopHTTPServer() {
	if tcpListener == nil {
		return
	}
	_ = tcpListener.Close()
}

func InitMockDBManager() {
	ctrl := gomock.NewController(GinkgoT())
	mockDBManager = engines.NewMockDBManager(ctrl)
	register.SetDBManager(mockDBManager)
	dbManager = mockDBManager
}

func InitMockDCSStore() {
	ctrl := gomock.NewController(GinkgoT())
	mockDCSStore = dcs.NewMockDCS(ctrl)
	mockDCSStore.EXPECT().GetClusterFromCache().Return(&dcs.Cluster{}).AnyTimes()
	dcs.SetStore(mockDCSStore)
	dcsStore = mockDCSStore
}
