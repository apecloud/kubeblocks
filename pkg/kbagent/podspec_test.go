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
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestInitCopyCommandContract(t *testing.T) {
	current := CurrentInitCopyCommand()
	if !slices.Equal(current, []string{"cp", "-r", BinaryPath, TiniPath, SharedMountPath + "/"}) {
		t.Fatalf("unexpected current copy command: %v", current)
	}
	if !IsCurrentInitCopyCommand(current) || IsLegacyInitCopyCommand(current) {
		t.Fatalf("current copy command was not recognized correctly: %v", current)
	}

	legacy := LegacyInitCopyCommand()
	if !slices.Equal(legacy, []string{"cp", "-r", BinaryPath, SharedMountPath + "/"}) {
		t.Fatalf("unexpected legacy copy command: %v", legacy)
	}
	if !IsLegacyInitCopyCommand(legacy) || IsCurrentInitCopyCommand(legacy) {
		t.Fatalf("legacy copy command was not recognized correctly: %v", legacy)
	}

	current[0] = "changed"
	legacy[0] = "changed"
	if CurrentInitCopyCommand()[0] != "cp" || LegacyInitCopyCommand()[0] != "cp" {
		t.Fatal("copy command constructors returned shared mutable slices")
	}
}

func TestPodSpecPredicates(t *testing.T) {
	containers := []corev1.Container{{
		Name:    ContainerName,
		Command: []string{SharedBinaryPath},
	}}
	if !UsesSharedBinary(containers) {
		t.Fatal("shared kbagent binary was not recognized")
	}
	if !IsContainer(&containers[0]) || IsContainer(nil) {
		t.Fatal("kbagent container identity was not recognized correctly")
	}

	mount := SharedVolumeMount()
	if mount.Name != SharedVolumeName || mount.MountPath != SharedMountPath {
		t.Fatalf("unexpected shared volume mount: %#v", mount)
	}
}
