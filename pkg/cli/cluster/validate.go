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

package cluster

import (
	"fmt"

	"k8s.io/client-go/dynamic"
)

// ValidateClusterVersion validates the cluster version.
func ValidateClusterVersion(dynamic dynamic.Interface, cd string, cv string) error {
	versions, err := GetVersionByClusterDef(dynamic, cd)
	if err != nil {
		return err
	}

	// check cluster version exists or not
	for _, item := range versions.Items {
		if item.Name == cv {
			return nil
		}
	}

	return fmt.Errorf("failed to find cluster version \"%s\"", cv)
}
