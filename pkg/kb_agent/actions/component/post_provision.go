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
	"github.com/apecloud/kubeblocks/pkg/kb_agent/handlers/models"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
)

type PostProvision struct {
	actions.Base
	Timeout time.Duration
}

type PostProvisionManager interface {
	PostProvision(ctx context.Context, componentNames, podNames, podIPs, podHostNames, podHostIPs string) error
}

var postProvision actions.Action = &PostProvision{}

func init() {
	err := actions.Register(strings.ToLower(string(util.PostProvisionOperation)), postProvision)
	if err != nil {
		panic(err.Error())
	}
}

func (s *PostProvision) Init(ctx context.Context) error {
	s.Logger = ctrl.Log.WithName("PostProvision")
	s.Action = constant.PostProvisionAction
	return s.Base.Init(ctx)
}

func (s *PostProvision) PreCheck(ctx context.Context, req *actions.OpsRequest) error {
	return nil
}

func (s *PostProvision) Do(ctx context.Context, req *actions.OpsRequest) (*actions.OpsResponse, error) {
	componentNames := req.GetString("componentNames")
	podNames := req.GetString("podNames")
	podIPs := req.GetString("podIPs")
	podHostNames := req.GetString("podHostNames")
	podHostIPs := req.GetString("podHostIPs")
	ppManager, ok := s.Handler.(PostProvisionManager)
	if !ok {
		return nil, models.ErrNotImplemented
	}
	err := ppManager.PostProvision(ctx, componentNames, podNames, podIPs, podHostNames, podHostIPs)
	return nil, err
}
