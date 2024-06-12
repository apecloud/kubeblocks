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
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/actions"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
)

type Join struct {
	actions.Base
	Timeout time.Duration
}

var join actions.Action = &Join{}

func init() {
	err := actions.Register(strings.ToLower(string(util.JoinMemberOperation)), join)
	if err != nil {
		panic(err.Error())
	}
}

func (s *Join) Init(ctx context.Context) error {
	s.Logger = ctrl.Log.WithName("MemberJoin")
	s.Action = constant.MemberJoinAction
	return s.Base.Init(ctx)
}

func (s *Join) Do(ctx context.Context, req *actions.ActionRequest) (*actions.ActionResponse, error) {
	leader := req.GetString("leader")
	// join current member to db cluster
	err := s.Handler.JoinMember(ctx, leader)
	if err != nil {
		s.Logger.Info("join member to cluster failed", "error", err.Error())
		return nil, err
	}
	s.Logger.Info("join member to cluster success")

	return nil, nil
}
