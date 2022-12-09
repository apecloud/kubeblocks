/*
Copyright ApeCloud Inc.

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
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

const (
	dockerCommand     = "docker"
	containerdCommand = "crictl"
)

// dockerContainer support docker cri
type dockerContainer struct {
}

func (d *dockerContainer) IsReady() error {
	return printCriVersion(dockerCommand)
}

func (d *dockerContainer) Kill(containerIDs []string, signal string, timeout *time.Duration) error {
	cmd := exec.Command("sudo", dockerCommand, "kill")
	if signal != "" {
		cmd.Args = append(cmd.Args, "--signal", signal)
	}
	cmd.Args = append(cmd.Args, containerIDs...)

	_, err := execShellCommand(cmd)
	return err
}

// dockerContainer support containerd or crio cri
type containerdContainer struct {
	socketPath string
}

func (c *containerdContainer) IsReady() error {
	return printCriVersion(containerdCommand)
}

func (c *containerdContainer) Kill(containerIDs []string, signal string, timeout *time.Duration) error {
	cmd := exec.Command("sudo", containerdCommand, "-i", c.socketPath, "-r", c.socketPath, "stop")

	// reference cri-api url: https://github.com/kubernetes/cri-api/blob/master/pkg/apis/runtime/v1/api.proto#L1108
	// reference containerd url: https://github.com/containerd/containerd/blob/main/pkg/cri/server/container_stop.go#L124
	switch {
	case signal == "SIGKILL":
		cmd.Args = append(cmd.Args, "--timeout=0")
	case timeout != nil:
		cmd.Args = append(cmd.Args, fmt.Sprintf("--timeout=%d", timeout))
	}

	cmd.Args = append(cmd.Args, containerIDs...)
	_, err := execShellCommand(cmd)
	return err
}

func NewContainerKiller(criType CRIType, socketPath string) (ContainerKiller, error) {
	var killer ContainerKiller

	if criType == AutoType {
		criType = autoCheckCRIType()
	}

	switch criType {
	case DockerType:
		killer = &dockerContainer{}
	case ContainerdType:
		killer = &containerdContainer{
			socketPath: getContainerdRuntimePath(socketPath),
		}
	default:
		return nil, cfgcore.MakeError("not support cri type: %s", criType)
	}
	return killer, nil
}

func autoCheckCRIType() CRIType {
	if err := printCriVersion(containerdCommand); err == nil {
		return ContainerdType
	}
	if err := printCriVersion(dockerCommand); err == nil {
		return DockerType
	}
	return ""
}

func getContainerdRuntimePath(socketPath string) string {
	if socketPath != "" {
		return formatSocketPath(socketPath)
	}
	runtimePath, err := getContainerdRuntimeSocketPath()
	if err != nil {
		logrus.Warnf("failed to get cri runtime socket path, error output: %s", err)
		return ""
	}
	return formatSocketPath(strings.TrimSpace(runtimePath))
}

func getContainerdRuntimeSocketPath() (string, error) {
	// config --get runtime-endpoint
	return execShellCommand(exec.Command(containerdCommand, "config", "--get", "runtime-endpoint"))
}

func formatSocketPath(path string) string {
	const sockPrefix = "unix://"
	if strings.HasPrefix(path, sockPrefix) {
		return path
	}
	return fmt.Sprintf("%s%s", sockPrefix, path)
}

func printCriVersion(cmdName string) error {
	cmd := exec.Command(cmdName, "-v")
	_, err := execShellCommand(cmd)
	return err
}
