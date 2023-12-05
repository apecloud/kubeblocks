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

package etcd

import (
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	"github.com/spf13/viper"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/embed"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v3client"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
)

const (
	etcdStartTimeout = 30
)

var (
	dcsStore     dcs.DCS
	mockDCSStore *dcs.MockDCS
	etcdServer   *EmbeddedETCD
)

func init() {
	viper.AutomaticEnv()
	viper.SetDefault(constant.KBEnvPodName, "pod-test-0")
	viper.SetDefault(constant.KBEnvClusterCompName, "cluster-component-test")
	viper.SetDefault(constant.KBEnvNamespace, "namespace-test")
	ctrl.SetLogger(zap.New())
}

func TestETCDDBManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ETCD DBManager. Suite")
}

var _ = BeforeSuite(func() {
	// Init mock dcs store
	InitMockDCSStore()

	// Start ETCD Server
	// server, err := StartEtcdServer()
	// Expect(err).Should(BeNil())
	// etcdServer = server
})

var _ = AfterSuite(func() {
	StopEtcdServer(etcdServer)
})

func InitMockDCSStore() {
	ctrl := gomock.NewController(GinkgoT())
	mockDCSStore = dcs.NewMockDCS(ctrl)
	mockDCSStore.EXPECT().GetClusterFromCache().Return(&dcs.Cluster{}).AnyTimes()
	dcs.SetStore(mockDCSStore)
	dcsStore = mockDCSStore
}

func StartEtcdServer() (*EmbeddedETCD, error) {
	peerAddress := "http://localhost:0"

	etcdServer := &EmbeddedETCD{}
	logger := ctrl.Log.WithName("ETCD server")
	etcdServer.logger = logger
	return etcdServer, etcdServer.Start(peerAddress)
}

func StopEtcdServer(etcdServer *EmbeddedETCD) {
	if etcdServer != nil {
		etcdServer.Stop()
	}
}

type EmbeddedETCD struct {
	logger logr.Logger
	tmpDir string
	ETCD   *embed.Etcd
	client *clientv3.Client
}

// Start starts embedded ETCD.
func (e *EmbeddedETCD) Start(peerAddress string) error {
	dir, err := os.MkdirTemp("", "ETCD")
	if err != nil {
		return err
	}

	cfg := embed.NewConfig()
	cfg.Dir = dir
	lpurl, _ := url.Parse("http://localhost:0")
	lcurl, _ := url.Parse(peerAddress)
	cfg.ListenPeerUrls = []url.URL{*lpurl}
	cfg.ListenClientUrls = []url.URL{*lcurl}
	e.ETCD, err = embed.StartEtcd(cfg)
	if err != nil {
		return err
	}

	select {
	case <-e.ETCD.Server.ReadyNotify():
		e.logger.Info("ETCD Server is ready!")
	case <-time.After(etcdStartTimeout * time.Second):
		e.ETCD.Server.Stop() // trigger a shutdown
		return errors.New("start embedded etcd server timeout")
	}
	e.client = v3client.New(e.ETCD.Server)

	return nil
}

// Stop stops the embedded ETCD & cleanups the tmp dir.
func (e *EmbeddedETCD) Stop() {
	e.ETCD.Close()
	e.ETCD.Server.Stop()
	os.RemoveAll(e.tmpDir)
}
