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
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/kb_agent/actions"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
)

type Switchover struct {
	actions.Base
}

var switchover actions.Action = &Switchover{}

func init() {
	err := actions.Register(strings.ToLower(string(util.SwitchoverOperation)), switchover)
	if err != nil {
		panic(err.Error())
	}
}

func (s *Switchover) Init(ctx context.Context) error {
	return s.Base.Init(ctx)
}

func (s *Switchover) PreCheck(ctx context.Context, req *actions.OpsRequest) error {
	primary := req.GetString("primary")
	candidate := req.GetString("candidate")
	if primary == "" && candidate == "" {
		return errors.New("primary or candidate must be set")
	}

	return nil
}

func (s *Switchover) Do(ctx context.Context, req *actions.OpsRequest) (*actions.OpsResponse, error) {
	primary := req.GetString("primary")
	candidate := req.GetString("candidate")

	err := s.Handler.Switchover(ctx, primary, candidate)
	if err != nil {
		message := fmt.Sprintf("Create switchover failed: %v", err)
		return nil, errors.New(message)
	}

	return nil, nil
}
