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

// dockerContainer supports docker cri
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
	d.logger.Debugf("docker containers going to be stopped: %v", containerIDs)
	if signal == "" {
		signal = defaultSignal
	}

	allContainer, err := getExistsContainers(ctx, containerIDs, d.dc)
	if err != nil {
		return cfgcore.WrapError(err, "failed to search docker container")
	}

	errs := make([]error, 0, len(containerIDs))
	d.logger.Debugf("all containers: %v", util.ToSet(allContainer).AsSlice())
	for _, containerID := range containerIDs {
		d.logger.Infof("stopping docker container: %s", containerID)
		container, ok := allContainer[containerID]
		if !ok {
			d.logger.Infof("docker container[%s] not existed and continue.", containerID)
			continue
		}
		if container.State == "exited" {
			d.logger.Infof("docker container[%s] exited, status: %s", containerID, container.Status)
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
		d.logger.Infof("create docker client succeed, docker info: %v", ping)
	}
	return err
}

func createDockerClient(dockerEndpoint string, logger *zap.SugaredLogger) (*dockerapi.Client, error) {
	if len(dockerEndpoint) == 0 {
		dockerEndpoint = dockerapi.DefaultDockerHost
	}

	logger.Infof("connecting to docker container endpoint: %s", dockerEndpoint)
	return dockerapi.NewClientWithOpts(
		dockerapi.WithHost(formatSocketPath(dockerEndpoint)),
		dockerapi.WithVersion(""),
	)
}

// dockerContainer supports docker cri
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
			c.logger.Infof("containerd container[%s] not existed and continue.", containerID)
		case containers.Containers[0].State == runtimeapi.ContainerState_CONTAINER_EXITED:
			c.logger.Infof("containerd container[%s] not exited and continue.", containerID)
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
		return nil, cfgcore.MakeError("not supported cri type: %s", containerRuntime)
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
