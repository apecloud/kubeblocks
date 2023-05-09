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
	"time"
)

type TargetContainer struct {
	Name          string // Name is pod name
	Namespace     string // Namespace is pod namespace
	ContainerName string // ContainerName is container name
}

type CRIType string

const (
	DockerType     CRIType = "docker"
	ContainerdType CRIType = "containerd"
	AutoType       CRIType = "auto"
)

var defaultContainerdEndpoints = []string{
	"unix:///var/run/dockershim.sock",
	"unix:///run/containerd/containerd.sock",
	"unix:///run/crio/crio.sock",
	"unix:///var/run/cri-dockerd.sock",
}

// ContainerKiller kill container interface
type ContainerKiller interface {

	// Kill containers in the pod by cri
	// e.g. docker or containerd
	Kill(ctx context.Context, containerIDs []string, signal string, timeout *time.Duration) error

	// Init cri killer init interface
	Init(ctx context.Context) error
}
