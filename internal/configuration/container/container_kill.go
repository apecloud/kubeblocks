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

package container

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	dockerapi "github.com/docker/docker/client"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/configuration/util"
)

const (
	maxMsgSize     = 1024 * 256 // 256k
	defaultTimeout = 2 * time.Second
	defaultSignal  = "SIGKILL"

	KillContainerSignalEnvName = "KILL_CONTAINER_SIGNAL"
)

// dockerContainer support docker cri
type dockerContainer struct {
	dockerEndpoint string
	logger         *zap.SugaredLogger

	dc dockerapi.ContainerAPIClient
}

func init() {
	if err := viper.BindEnv(KillContainerSignalEnvName); err != nil {
		fmt.Printf("failed to bind env for viper, env name: [%s]\n", KillContainerSignalEnvName)
		os.Exit(-2)
	}

	viper.SetDefault(KillContainerSignalEnvName, defaultSignal)
}

func (d *dockerContainer) Kill(ctx context.Context, containerIDs []string, signal string, _ *time.Duration) error {
	d.logger.Debugf("following docker containers are going to be stopped: %v", containerIDs)
	if signal == "" {
		signal = defaultSignal
	}

	allContainer, err := getExistsContainers(ctx, containerIDs, d.dc)
	if err != nil {
		return cfgcore.WrapError(err, "failed to search container")
	}

	errs := make([]error, 0, len(containerIDs))
	d.logger.Debugf("all docker container: %v", util.ToSet(allContainer).AsSlice())
	for _, containerID := range containerIDs {
		d.logger.Infof("stopping docker container: %s", containerID)
		container, ok := allContainer[containerID]
		if !ok {
			d.logger.Infof("container[%s] not exist and pass.", containerID)
			continue
		}
		if container.State == "exited" {
			d.logger.Infof("container[%s] is exited, status: %s", containerID, container.Status)
			continue
		}
		if err := d.dc.ContainerKill(ctx, containerID, signal); err != nil {
			errs = append(errs, err)
			continue
		}
		d.logger.Infof("docker container[%s] stopped.", containerID)
	}
	if len(errs) > 0 {
		return utilerrors.NewAggregate(errs)
	}
	return nil
}

func getExistsContainers(ctx context.Context, containerIDs []string, dc dockerapi.ContainerAPIClient) (map[string]*types.Container, error) {
	var (
		optionsArgs  = filters.NewArgs()
		allContainer map[string]*types.Container
	)

	for _, containerID := range containerIDs {
		optionsArgs.Add("id", containerID)
	}

	containers, err := dc.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: optionsArgs,
	})
	if err != nil {
		return nil, err
	}
	allContainer = make(map[string]*types.Container, len(containerIDs))
	for _, c := range containers {
		allContainer[c.ID] = &c
	}
	return allContainer, nil
}

func (d *dockerContainer) Init(ctx context.Context) error {
	client, err := createDockerClient(d.dockerEndpoint, d.logger)
	d.dc = client
	if err == nil {
		ping, err := client.Ping(ctx)
		if err != nil {
			return err
		}
		d.logger.Infof("create docker client success! docker info: %v", ping)
	}
	return err
}

func createDockerClient(dockerEndpoint string, logger *zap.SugaredLogger) (*dockerapi.Client, error) {
	if len(dockerEndpoint) == 0 {
		dockerEndpoint = dockerapi.DefaultDockerHost
	}

	logger.Infof("connecting to docker on the endpoint: %s", dockerEndpoint)
	return dockerapi.NewClientWithOpts(
		dockerapi.WithHost(formatSocketPath(dockerEndpoint)),
		dockerapi.WithVersion(""),
	)
}

// dockerContainer support docker cri
type containerdContainer struct {
	runtimeEndpoint string
	logger          *zap.SugaredLogger

	backendRuntime runtimeapi.RuntimeServiceClient
}

func (c *containerdContainer) Kill(ctx context.Context, containerIDs []string, signal string, timeout *time.Duration) error {
	var (
		request = &runtimeapi.StopContainerRequest{}
		errs    = make([]error, 0, len(containerIDs))
	)

	switch {
	case signal == defaultSignal:
		request.Timeout = 0
	case timeout != nil:
		request.Timeout = timeout.Milliseconds()
	}

	// reference cri-api url: https://github.com/kubernetes/cri-api/blob/master/pkg/apis/runtime/v1/api.proto#L1108
	// reference containerd url: https://github.com/containerd/containerd/blob/main/pkg/cri/server/container_stop.go#L124
	for _, containerID := range containerIDs {
		c.logger.Infof("stopping container: %s", containerID)
		containers, err := c.backendRuntime.ListContainers(ctx, &runtimeapi.ListContainersRequest{
			Filter: &runtimeapi.ContainerFilter{Id: containerID},
		})

		switch {
		case err != nil:
			errs = append(errs, err)
		case containers == nil || len(containers.Containers) == 0:
			c.logger.Infof("container[%s] not exist and pass.", containerID)
		case containers.Containers[0].State == runtimeapi.ContainerState_CONTAINER_EXITED:
			c.logger.Infof("container[%s] not exited and pass.", containerID)
		default:
			request.ContainerId = containerID
			_, err = c.backendRuntime.StopContainer(ctx, request)
			if err != nil {
				c.logger.Infof("failed to stop container[%s], error: %v", containerID, err)
				errs = append(errs, err)
				continue
			}
			c.logger.Infof("docker container[%s] stopped.", containerID)
		}
	}

	if len(errs) > 0 {
		return utilerrors.NewAggregate(errs)
	}
	return nil
}

func (c *containerdContainer) Init(ctx context.Context) error {
	var (
		err       error
		conn      *grpc.ClientConn
		endpoints = defaultContainerdEndpoints
	)

	if c.runtimeEndpoint != "" {
		endpoints = []string{formatSocketPath(c.runtimeEndpoint)}
	}

	for _, endpoint := range endpoints {
		conn, err = createGrpcConnection(ctx, endpoint)
		if err != nil {
			c.logger.Warnf("failed to connect containerd endpoint: %s, error : %v", endpoint, err)
		} else {
			c.backendRuntime = runtimeapi.NewRuntimeServiceClient(conn)
			if err = c.pingCRI(ctx, c.backendRuntime); err != nil {
				return nil
			}
		}
	}
	return err
}

func (c *containerdContainer) pingCRI(ctx context.Context, runtime runtimeapi.RuntimeServiceClient) error {
	status, err := runtime.Status(ctx, &runtimeapi.StatusRequest{
		Verbose: true,
	})
	if err != nil {
		return err
	}
	c.logger.Infof("cri status: %v", status)
	return nil
}

func NewContainerKiller(containerRuntime CRIType, runtimeEndpoint string, logger *zap.SugaredLogger) (ContainerKiller, error) {
	var (
		killer ContainerKiller
	)

	if containerRuntime == AutoType {
		containerRuntime = autoCheckCRIType(defaultContainerdEndpoints, dockerapi.DefaultDockerHost, logger)
		runtimeEndpoint = ""
	}

	switch containerRuntime {
	case DockerType:
		killer = &dockerContainer{
			dockerEndpoint: runtimeEndpoint,
			logger:         logger,
		}
	case ContainerdType:
		killer = &containerdContainer{
			runtimeEndpoint: runtimeEndpoint,
			logger:          logger,
		}
	default:
		return nil, cfgcore.MakeError("not support cri type: %s", containerRuntime)
	}
	return killer, nil
}

func autoCheckCRIType(criEndpoints []string, dockerEndpoints string, logger *zap.SugaredLogger) CRIType {
	for _, f := range criEndpoints {
		if isSocketFile(f) && hasValidCRISocket(f, logger) {
			return ContainerdType
		}
	}
	if isSocketFile(dockerEndpoints) {
		return DockerType
	}
	return ""
}

func hasValidCRISocket(sockPath string, logger *zap.SugaredLogger) bool {
	connection, err := createGrpcConnection(context.Background(), sockPath)
	if err != nil {
		logger.Warnf("failed to connect socket path: %s, error: %v", sockPath, err)
		return false
	}
	_ = connection.Close()
	return true
}

func createGrpcConnection(ctx context.Context, socketAddress string) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	return grpc.DialContext(ctx, socketAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", addr)
		}),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize)))
}
