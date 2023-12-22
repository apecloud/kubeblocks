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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	cmdexec "k8s.io/kubectl/pkg/cmd/exec"
	ctrl "sigs.k8s.io/controller-runtime"

	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// K8sExecClient is a mock client for operation, mainly used to hide curl command details.
type K8sExecClient struct {
	lorryClient
	cmdexec.StreamOptions
	Executor       cmdexec.RemoteExecutor
	restConfig     *rest.Config
	restClient     *rest.RESTClient
	lorryPort      int32
	RequestTimeout time.Duration
	logger         logr.Logger
}

// NewK8sExecClientWithPod create a new OperationHTTPClient with lorry container
func NewK8sExecClientWithPod(restConfig *rest.Config, pod *corev1.Pod) (*K8sExecClient, error) {
	var (
		err error
	)
	logger := ctrl.Log.WithName("Lorry K8S Exec client")

	containerName, err := intctrlutil.GetLorryContainerName(pod)
	if err != nil {
		logger.Info("not lorry in the pod, just return nil without error")
		return nil, nil
	}

	port, err := intctrlutil.GetLorryHTTPPort(pod)
	if err != nil {
		return nil, err
	}

	streamOptions := cmdexec.StreamOptions{
		IOStreams:     genericiooptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
		Stdin:         true,
		TTY:           true,
		PodName:       pod.Name,
		ContainerName: containerName,
		Namespace:     pod.Namespace,
	}

	if restConfig == nil {
		restConfig, err = ctrl.GetConfig()
		if err != nil {
			return nil, errors.Wrap(err, "get k8s config failed")
		}
	}

	restConfig.GroupVersion = &schema.GroupVersion{Group: "", Version: "v1"}
	restConfig.APIPath = "/api"
	restConfig.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	restClient, err := rest.RESTClientFor(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "create k8s client failed")
	}

	client := &K8sExecClient{
		StreamOptions:  streamOptions,
		lorryPort:      port,
		restConfig:     restConfig,
		restClient:     restClient,
		RequestTimeout: 10 * time.Second,
		logger:         logger,
		Executor:       &cmdexec.DefaultRemoteExecutor{},
	}
	client.lorryClient = lorryClient{requester: client}
	return client, nil
}

// Request execs lorry operation, this is a blocking operation and use pod EXEC subresource to send a http request to the lorry pod
func (cli *K8sExecClient) Request(ctx context.Context, operation, method string, req map[string]any) (map[string]any, error) {
	var (
		strBuffer bytes.Buffer
		errBuffer bytes.Buffer
		err       error
	)
	curlCmd := fmt.Sprintf("curl --fail-with-body --silent -X %s -H 'Content-Type: application/json' http://localhost:%d/v1.0/%s",
		strings.ToUpper(method), cli.lorryPort, strings.ToLower(operation))

	if len(req) != 0 {
		jsonData, err := json.Marshal(req)
		if err != nil {
			return nil, err
		}
		// escape single quote
		body := strings.ReplaceAll(string(jsonData), "'", "\\'")
		curlCmd += fmt.Sprintf(" -d '%s'", body)
	}
	cmd := []string{"sh", "-c", curlCmd}

	// redirect output to strBuffer to be parsed later
	if err = cli.k8sExec(cmd, &strBuffer, &errBuffer); err != nil {
		data := strBuffer.Bytes()
		if len(data) != 0 {
			// curl emits result message to output
			return nil, errors.Wrap(err, string(data))
		}

		errData := errBuffer.Bytes()
		if len(errData) != 0 {
			return nil, errors.Wrap(err, string(errData))
		}
		return nil, err
	}

	data := strBuffer.Bytes()
	if len(data) == 0 {
		errData := errBuffer.Bytes()
		if len(errData) != 0 {
			cli.logger.Info("k8s exec error output", "message", string(errData))
			return nil, errors.New(string(errData))
		}

		return nil, nil
	}

	result := map[string]any{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, errors.Wrap(err, "decode result failed")
	}
	return result, nil
}

func (cli *K8sExecClient) k8sExec(cmd []string, outWriter io.Writer, errWriter io.Writer) error {
	// ensure we can recover the terminal while attached
	t := cli.SetupTTY()

	var sizeQueue remotecommand.TerminalSizeQueue
	if t.Raw {
		// this call spawns a goroutine to monitor/update the terminal size
		sizeQueue = t.MonitorSize(t.GetSize())

		// unset p.Err if it was previously set because both stdout and stderr go over p.Out when tty is
		// true
		cli.ErrOut = nil
	}

	fn := func() error {
		req := cli.restClient.Post().
			Resource("pods").
			Name(cli.PodName).
			Namespace(cli.Namespace).
			SubResource("exec")
		req.VersionedParams(&corev1.PodExecOptions{
			Container: cli.ContainerName,
			Command:   cmd,
			Stdin:     cli.Stdin,
			Stdout:    outWriter != nil,
			Stderr:    errWriter != nil,
			TTY:       t.Raw,
		}, scheme.ParameterCodec)

		return cli.Executor.Execute("POST", req.URL(), cli.restConfig, cli.In, outWriter, errWriter, t.Raw, sizeQueue)
	}

	if err := t.Safe(fn); err != nil {
		return err
	}
	return nil
}
