/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package etcd

import (
	"context"
	"io/ioutil"
	"math/rand"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/kit/logger"
	"github.com/pkg/errors"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/embed"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v3client"
)

const (
	etcdStartTimeout = 30
)

// randomize the port to avoid conflicting
var testEndpoint = "http://localhost:" + strconv.Itoa(52600+rand.Intn(1000))

func TestETCD(t *testing.T) {
	etcdServer, err := startEtcdServer(testEndpoint)
	defer stopEtcdServer(etcdServer)
	if err != nil {
		t.Errorf("start embedded etcd server error: %s", err)
	}
	t.Run("Invoke GetRole", func(t *testing.T) {
		e := mockEtcd(etcdServer)
		role, err := e.GetRole(context.Background(), &bindings.InvokeRequest{}, &bindings.InvokeResponse{})
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
	etcd.logger = logger.NewLogger("embedded-etcd-server")
	return etcd, etcd.Start(peerAddress)
}

func stopEtcdServer(etcdServer *EmbeddedETCD) {
	if etcdServer != nil {
		etcdServer.Stop()
	}
}

type EmbeddedETCD struct {
	logger logger.Logger
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
	cfg.LPUrls = []url.URL{*lpurl}
	cfg.LCUrls = []url.URL{*lcurl}
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
