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

	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
)

type GRPCHandler struct {
	HandlerBase
}

var _ Handler = &GRPCHandler{}

func NewGRPCHandler(properties map[string]string) (*GRPCHandler, error) {
	logger := ctrl.Log.WithName("GRPC handler")
	managerBase, err := NewHandlerBase(logger)
	if err != nil {
		return nil, err
	}

	h := &GRPCHandler{
		HandlerBase: *managerBase,
	}

	return h, nil
}

func (h *GRPCHandler) Do(ctx context.Context, setting util.Handlers, args map[string]any) (map[string]any, error) {
	if setting.GPRC == nil {
		return nil, errors.New("grpc setting is nil")
	}
	// TODO: implement grpc handler
	return nil, ErrNotImplemented
}
