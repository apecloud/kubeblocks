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

	"github.com/go-logr/logr"
	"github.com/spf13/viper"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
)

type HandlerBase struct {
	CurrentMemberName string
	CurrentMemberIP   string
	ClusterCompName   string
	Namespace         string
	DataDir           string
	Logger            logr.Logger
	DBStartupReady    bool
	IsLocked          bool
}

func NewHandlerBase(logger logr.Logger) (*HandlerBase, error) {
	currentMemberName := viper.GetString(constant.KBEnvPodName)
	if currentMemberName == "" {
		return nil, fmt.Errorf("%s is not set", constant.KBEnvPodName)
	}

	mgr := HandlerBase{
		CurrentMemberName: currentMemberName,
		CurrentMemberIP:   viper.GetString(constant.KBEnvPodIP),
		ClusterCompName:   viper.GetString(constant.KBEnvClusterCompName),
		Namespace:         viper.GetString(constant.KBEnvNamespace),
		Logger:            logger,
	}
	return &mgr, nil
}

func (mgr *HandlerBase) IsDBStartupReady() bool {
	return mgr.DBStartupReady
}

func (mgr *HandlerBase) GetLogger() logr.Logger {
	return mgr.Logger
}

func (mgr *HandlerBase) SetLogger(logger logr.Logger) {
	mgr.Logger = logger
}

func (mgr *HandlerBase) GetCurrentMemberName() string {
	return mgr.CurrentMemberName
}

func (mgr *HandlerBase) Do(ctx context.Context, settings util.Handlers, args map[string]any) (map[string]any, error) {
	return nil, ErrNotImplemented
}
