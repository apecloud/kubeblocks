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

package tasks

import (
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/bootstrap/confirm"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
	"github.com/mitchellh/mapstructure"
)

type DependenciesChecker struct {
	common.KubePrepare
}

func (n *DependenciesChecker) PreCheck(runtime connector.Runtime) (bool, error) {
	host := runtime.RemoteHost()

	v, ok := host.GetCache().Get(common.NodePreCheck)
	if !ok {
		return false, nil
	}

	var result confirm.PreCheckResults
	if err := mapstructure.Decode(v, &result); err != nil {
		return false, cfgcore.WrapError(err, "failed to decode precheck result")
	}
	return true, nil
}
