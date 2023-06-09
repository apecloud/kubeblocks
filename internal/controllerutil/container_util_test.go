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

package controllerutil

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestGetContainerByName(t *testing.T) {
	containerName1 := "test1"
	containerName2 := "test2"
	containers := []corev1.Container{
		{Name: containerName1},
		{Name: containerName2},
	}
	i, container := GetContainerByName(containers, containerName1)
	if i != 0 || container.Name != containerName1 {
		t.Error("expected to return 0 and the corresponding index container!")
	}
	i, container = GetContainerByName(containers, "test3")
	if i != -1 || container != nil {
		t.Error("expected to return 0 and the corresponding index container!")
	}
}
