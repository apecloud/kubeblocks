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

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/actions"
)

type Rebuild struct {
	actions.Base
}

var rebuild actions.Action = &Rebuild{}

func init() {
	err := actions.Register("rebuild", rebuild)
	if err != nil {
		panic(err.Error())
	}
}

func (s *Rebuild) Init(ctx context.Context) error {
	s.Logger = ctrl.Log.WithName("Rebuild")
	s.Action = constant.RebuildAction
	return s.Base.Init(ctx)
}

func (s *Rebuild) IsReadonly(ctx context.Context) bool {
	return false
}

func (s *Rebuild) Do(ctx context.Context, req *actions.OpsRequest) (*actions.OpsResponse, error) {
	resp := &actions.OpsResponse{
		Data: map[string]any{},
	}
	resp.Data["operation"] = constant.RebuildAction

	err := s.Handler.Rebuild(ctx)
	if err != nil {
		s.Logger.Info("Rebuild failed", "error", err.Error())
		return nil, err
	}
	s.Logger.Info("Rebuild success")
	return resp, nil
}