/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
)

func inDataContext4G() model.GraphOption {
	return model.WithClientOption(multicluster.InDataContext())
}

func multiClusterClientOption(v *model.ObjectVertex) *multicluster.ClientOption {
	if v.ClientOpt != nil {
		opt, ok := v.ClientOpt.(*multicluster.ClientOption)
		if ok {
			return opt
		}
		panic(fmt.Sprintf("unknown client option: %T", v.ClientOpt))
	}
	return multicluster.InControlContext()
}
