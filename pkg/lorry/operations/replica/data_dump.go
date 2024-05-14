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

package replica

import (
	"context"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/operations"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

type dataDump struct {
	operations.Base
}

func init() {
	err := operations.Register(strings.ToLower(string(util.DataDumpOperation)), &dataDump{})
	if err != nil {
		panic(err.Error())
	}
}

func (s *dataDump) Init(ctx context.Context) error {
	s.Logger = ctrl.Log.WithName("DataDump")
	s.Action = constant.DataDumpAction
	return s.Base.Init(ctx)
}

func (s *dataDump) Do(ctx context.Context, req *operations.OpsRequest) (*operations.OpsResponse, error) {
	return nil, s.ExecCommand(ctx)
}
