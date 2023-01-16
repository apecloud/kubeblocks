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
