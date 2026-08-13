/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package kbagent

import (
	"slices"

	corev1 "k8s.io/api/core/v1"
)

const (
	BinaryPath       = "/bin/kbagent"
	TiniPath         = "/bin/tini-static"
	SharedMountPath  = "/kubeblocks"
	SharedBinaryPath = SharedMountPath + "/kbagent"
	SharedVolumeName = "kubeblocks"
)

// CurrentInitCopyCommand returns the current init-kbagent copy command.
func CurrentInitCopyCommand() []string {
	return []string{"cp", "-r", BinaryPath, TiniPath, SharedMountPath + "/"}
}

// LegacyInitCopyCommand returns the init-kbagent copy command used before
// tini-static became part of the shared kbagent runtime contract.
func LegacyInitCopyCommand() []string {
	return []string{"cp", "-r", BinaryPath, SharedMountPath + "/"}
}

// IsCurrentInitCopyCommand reports whether command implements the current
// init-kbagent copy contract.
func IsCurrentInitCopyCommand(command []string) bool {
	return slices.Equal(command, CurrentInitCopyCommand())
}

// IsLegacyInitCopyCommand reports whether command implements the legacy
// init-kbagent copy contract.
func IsLegacyInitCopyCommand(command []string) bool {
	return slices.Equal(command, LegacyInitCopyCommand())
}

// IsContainer reports whether container belongs to the kbagent runtime.
func IsContainer(container *corev1.Container) bool {
	return container != nil && (container.Name == ContainerName ||
		container.Name == ContainerName4Worker || container.Name == InitContainerName)
}

// UsesSharedBinary reports whether the kbagent server container runs the
// binary copied to the shared mount.
func UsesSharedBinary(containers []corev1.Container) bool {
	index := slices.IndexFunc(containers, func(container corev1.Container) bool {
		return container.Name == ContainerName
	})
	return index >= 0 && slices.Equal(containers[index].Command, []string{SharedBinaryPath})
}

// SharedVolumeMount returns the volume mount used by the shared kbagent binary.
func SharedVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{Name: SharedVolumeName, MountPath: SharedMountPath}
}
