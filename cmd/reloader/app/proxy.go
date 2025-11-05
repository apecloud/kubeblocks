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

package app

import (
	"context"

	"go.uber.org/zap"

	cfgcm "github.com/apecloud/kubeblocks/pkg/parameters/config_manager"
	cfgcore "github.com/apecloud/kubeblocks/pkg/parameters/core"
	cfgproto "github.com/apecloud/kubeblocks/pkg/parameters/proto"
)

type reconfigureProxy struct {
	cfgproto.ReconfigureServer
	updater cfgcm.DynamicUpdater

	ctx context.Context
	opt ReconfigureServiceOptions

	logger *zap.SugaredLogger
}

func (r *reconfigureProxy) Init(handler cfgcm.ConfigHandler) error {
	if err := r.initOnlineUpdater(handler); err != nil {
		r.logger.Errorf("init online updater failed: %+v", err)
		return err
	}
	return nil
}

func (r *reconfigureProxy) OnlineUpgradeParams(ctx context.Context, request *cfgproto.OnlineUpgradeParamsRequest) (*cfgproto.OnlineUpgradeParamsResponse, error) {
	if r.updater == nil {
		return nil, cfgcore.MakeError("online updating process is not initialized.")
	}
	params := request.GetParams()
	if len(params) == 0 {
		return nil, cfgcore.MakeError("update params is empty.")
	}
	key := request.ConfigSpec
	if request.ConfigFile != nil && *request.ConfigFile != "" {
		key = key + "/" + *request.ConfigFile
	}
	if err := r.updater(ctx, key, params); err != nil {
		return nil, err
	}
	return &cfgproto.OnlineUpgradeParamsResponse{}, nil
}

func (r *reconfigureProxy) initOnlineUpdater(handler cfgcm.ConfigHandler) error {
	if !r.opt.RemoteOnlineUpdateEnable {
		return nil
	}

	r.updater = func(ctx context.Context, name string, updatedParams map[string]string) error {
		return handler.OnlineUpdate(ctx, name, updatedParams)
	}
	return nil
}
