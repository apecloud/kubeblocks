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

package component

import (
	"context"
	"strings"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/actions"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
)

type PreTerminate struct {
	actions.Base
	Timeout time.Duration
}

type PreTerminateManager interface {
	PreTerminate(ctx context.Context) error
}

var preTerminate actions.Action = &PreTerminate{}

func init() {
	err := actions.Register(strings.ToLower(string(util.PreTerminateOperation)), preTerminate)
	if err != nil {
		panic(err.Error())
	}
}

func (s *PreTerminate) Init(ctx context.Context) error {
	s.Logger = ctrl.Log.WithName("PreTerminate")
	s.Action = constant.PreTerminateAction
	return s.Base.Init(ctx)
}

func (s *PreTerminate) Do(ctx context.Context, req *actions.OpsRequest) (*actions.OpsResponse, error) {
	err := s.Handler.PreTerminate(ctx, nil)
	return nil, err
}
