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
	"encoding/json"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/models"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/register"
	"github.com/apecloud/kubeblocks/pkg/lorry/operations"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

type PreTerminate struct {
	operations.Base
	logger  logr.Logger
	Timeout time.Duration
	Command []string
}

type PreTerminateManager interface {
	PreTerminate(ctx context.Context) error
}

var preTerminate operations.Operation = &PreTerminate{}

func init() {
	err := operations.Register(strings.ToLower(string(util.PreTerminateOperation)), preTerminate)
	if err != nil {
		panic(err.Error())
	}
}

func (s *PreTerminate) Init(_ context.Context) error {
	actionJSON := viper.GetString(constant.KBEnvActionCommands)
	if actionJSON != "" {
		actionCommands := map[string][]string{}
		err := json.Unmarshal([]byte(actionJSON), &actionCommands)
		if err != nil {
			s.logger.Info("get action commands failed", "error", err.Error())
			return err
		}
		preTermianteCmd, ok := actionCommands[constant.PreTerminateAction]
		if ok && len(preTermianteCmd) > 0 {
			s.Command = preTermianteCmd
		}
	}
	return nil
}

func (s *PreTerminate) PreCheck(ctx context.Context, req *operations.OpsRequest) error {
	return nil
}

func (s *PreTerminate) Do(ctx context.Context, req *operations.OpsRequest) (*operations.OpsResponse, error) {
	manager, err := register.GetDBManager(s.Command)
	if err != nil {
		return nil, errors.Wrap(err, "get manager failed")
	}

	ptManager, ok := manager.(PreTerminateManager)
	if !ok {
		return nil, models.ErrNotImplemented
	}
	err = ptManager.PreTerminate(ctx)
	return nil, err
}
