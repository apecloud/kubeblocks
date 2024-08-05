/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package configuration

import (
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
)

func inDataContext() *multicluster.ClientOption {
	return multicluster.InDataContext()
}

func inDataContextUnspecified() *multicluster.ClientOption {
	return multicluster.InDataContextUnspecified()
}

func copyMap(data map[string]string) map[string]string {
	r := make(map[string]string, len(data))
	for k, v := range data {
		r[k] = v
	}
	return r
}
