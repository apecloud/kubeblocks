/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

type portForwardClient struct {
	pod    *corev1.Pod
	port   string
	config *rest.Config
	logger logr.Logger
}

var _ Client = &portForwardClient{}

// Action forwards the target port to localhost, and then execute the action.
// Since we can't know httpClient's lifecycle, a portforward is bound to one request.
// It's not efficient, but enough for debugging purposes.
func (pf *portForwardClient) Action(ctx context.Context, req proto.ActionRequest) (proto.ActionResponse, error) {
	emptyResp := proto.ActionResponse{}
	stopCh := make(chan struct{})
	defer close(stopCh) // this will stop forwarder
	readyCh := make(chan struct{})
	errCh := make(chan error)
	outReader, outWriter := io.Pipe()
	defer outWriter.Close() // this will stop the next goroutine
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := outReader.Read(buf)
			if err != nil {
				if err != io.EOF {
					pf.logger.Error(err, "portForward error")
				}
				return
			}
			pf.logger.V(3).Info("portForwarder output", "data", string(buf[:n]))
		}
	}()

	forwarder, err := pf.newPortForwarder(readyCh, stopCh, outWriter)
	if err != nil {
		return emptyResp, err
	}
	go func() {
		err := forwarder.ForwardPorts()
		if err != nil {
			errCh <- err
		}
	}()

	select {
	case <-readyCh:
		// do nothing
	case err := <-errCh:
		pf.logger.Error(err, "port forward failed")
		return emptyResp, err
	}

	ports, err := forwarder.GetPorts()
	if err != nil {
		return emptyResp, err
	}
	if len(ports) == 0 {
		return emptyResp, fmt.Errorf("no port was forwarded")
	}

	endpoint := func() (string, int32, error) {
		return "localhost", int32(ports[0].Local), nil
	}
	client, err := NewClient(endpoint)
	if err != nil {
		return emptyResp, err
	}

	return client.Action(ctx, req)
}

func (pf *portForwardClient) createDialer(method string, url *url.URL, config *rest.Config) (httpstream.Dialer, error) {
	transport, upgrader, err := spdy.RoundTripperFor(config)
	if err != nil {
		return nil, err
	}
	// spdy is deprecated. k8s 1.31 supports a new websocket dialer, maybe we can use it in the future
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, method, url)
	return dialer, nil
}

func (pf *portForwardClient) newPortForwarder(readyCh, stopCh chan struct{}, outWriter io.Writer) (*portforward.PortForwarder, error) {
	clientset, err := kubernetes.NewForConfig(pf.config)
	if err != nil {
		return nil, err
	}
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(pf.pod.Namespace).
		Name(pf.pod.Name).
		SubResource("portforward")
	dialer, err := pf.createDialer("POST", req.URL(), pf.config)
	if err != nil {
		return nil, err
	}
	port := fmt.Sprintf("0:%v", pf.port) // this will selects a random available local port
	fw, err := portforward.New(dialer, []string{port}, stopCh, readyCh, outWriter, outWriter)
	if err != nil {
		return nil, err
	}
	return fw, nil
}

func NewPortForwardClient(pod *corev1.Pod, port string) (Client, error) {
	if mockClient != nil || mockClientError != nil {
		return mockClient, mockClientError
	}

	config := ctrl.GetConfigOrDie()
	return &portForwardClient{
		pod:    pod,
		port:   port,
		config: config,
		logger: ctrl.Log.WithName("portforward"),
	}, nil
}
