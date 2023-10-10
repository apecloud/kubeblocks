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
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/pkg/errors"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/embed"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v3client"
	"go.uber.org/zap"

	"github.com/apecloud/kubeblocks/lorry/binding"
)

const (
	etcdStartTimeout = 30
)

func TestETCD(t *testing.T) {
	etcdServer, err := startEtcdServer("http://localhost:0")
	if err != nil {
		t.Errorf("start embedded etcd server error: %s", err)
	}
	defer stopEtcdServer(etcdServer)
	testEndpoint := fmt.Sprintf("http://%s", etcdServer.ETCD.Clients[0].Addr().(*net.TCPAddr).String())

	t.Run("Invoke GetRole", func(t *testing.T) {
		e := mockEtcd(etcdServer)
		role, err := e.GetRole(context.Background(), &binding.ProbeRequest{}, &binding.ProbeResponse{})
		if err != nil {
			t.Errorf("get role error: %s", err)
		}
		if role != "leader" {
			t.Errorf("unexpected role: %s", role)
		}
	})
	t.Run("InitDelay", func(t *testing.T) {
		e := &Etcd{endpoint: testEndpoint}
		err = e.InitDelay()
		if err != nil {
			t.Errorf("etcd client init error: %s", err)
		}
	})
}

func mockEtcd(etcdServer *EmbeddedETCD) *Etcd {
	e := &Etcd{}
	e.etcd = etcdServer.client
	return e
}

func startEtcdServer(peerAddress string) (*EmbeddedETCD, error) {
	etcd := &EmbeddedETCD{}
	development, err := zap.NewDevelopment()
	if err != nil {
		return nil, err
	}
	logger := zapr.NewLogger(development)
	etcd.logger = logger
	return etcd, etcd.Start(peerAddress)
}

func stopEtcdServer(etcdServer *EmbeddedETCD) {
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
	dir, err := ioutil.TempDir("", "ETCD")
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
	os.RemoveAll(e.tmpDir)
}
