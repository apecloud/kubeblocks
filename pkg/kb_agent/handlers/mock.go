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

package handlers

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/kb_agent/dcs"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/handlers/models"
)

type MockHandler struct {
	HandlerBase
}

var _ Handler = &MockHandler{}

func NewMockHandler(properties Properties) (Handler, error) {
	logger := ctrl.Log.WithName("MockHandler")

	managerBase, err := NewHandlerBase(logger)
	if err != nil {
		return nil, err
	}

	Mgr := &MockHandler{
		HandlerBase: *managerBase,
	}

	return Mgr, nil
}
func (*MockHandler) IsRunning() bool {
	return true
}

func (*MockHandler) IsDBStartupReady() bool {
	return true
}

func (*MockHandler) IsLeader(context.Context, *dcs.Cluster) (bool, error) {
	return false, fmt.Errorf("NotSupported")
}

func (*MockHandler) JoinMember(context.Context, *dcs.Cluster, string) error {
	return models.ErrNotImplemented
}

func (*MockHandler) LeaveMember(context.Context, *dcs.Cluster, string) error {
	return models.ErrNotImplemented
}

func (*MockHandler) Lock(context.Context, string) error {
	return fmt.Errorf("NotSupported")
}

func (*MockHandler) Unlock(context.Context, string) error {
	return fmt.Errorf("NotSupported")
}
