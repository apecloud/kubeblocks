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
	"context"
	"fmt"
	"os/exec"
	"time"
)

const (
	dockerCommand     = "docker"
	containerdCommand = "crictl"
)

// dockerContainer support docker cri
type dockerContainer struct {
}

var _ = &dockerContainer{}

func (d *dockerContainer) Init(context.Context) error {
	return printCriVersion(dockerCommand)
}

func (d *dockerContainer) Kill(ctx context.Context, containerIDs []string, signal string, timeout *time.Duration) error {
	cmd := exec.Command(dockerCommand, "kill")
	if signal != "" {
		cmd.Args = append(cmd.Args, "--signal", signal)
	}
	cmd.Args = append(cmd.Args, containerIDs...)

	_, err := execShellCommand(cmd)
	return err
}

// dockerContainer support containerd or crio cri
type containerdContainer struct {
	runtimeEndpoint string
}

var _ = &containerdContainer{}

func (c *containerdContainer) Init(context.Context) error {
	return printCriVersion(containerdCommand)
}

func (c *containerdContainer) Kill(ctx context.Context, containerIDs []string, signal string, timeout *time.Duration) error {
	cmd := exec.Command("sudo", containerdCommand, "-i", c.runtimeEndpoint, "-r", c.runtimeEndpoint, "stop")

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
