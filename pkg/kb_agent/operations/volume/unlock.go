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

package volume

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/register"
	"github.com/apecloud/kubeblocks/pkg/lorry/operations"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

type Unlock struct {
	operations.Base
	logger  logr.Logger
	Timeout time.Duration
	Command []string
}

var unlock operations.Operation = &Unlock{}

func init() {
	err := operations.Register(strings.ToLower(string(util.UnlockOperation)), unlock)
	if err != nil {
		panic(err.Error())
	}
}

func (s *Unlock) Init(ctx context.Context) error {
	actionJSON := viper.GetString(constant.KBEnvActionCommands)
	if actionJSON != "" {
		actionCommands := map[string][]string{}
		err := json.Unmarshal([]byte(actionJSON), &actionCommands)
		if err != nil {
			s.logger.Info("get action commands failed", "error", err.Error())
			return err
		}
		readWriteCmd, ok := actionCommands[constant.ReadWriteAction]
		if ok && len(readWriteCmd) > 0 {
			s.Command = readWriteCmd
		}
	}
	return nil
}

func (s *Unlock) Do(ctx context.Context, req *operations.OpsRequest) (*operations.OpsResponse, error) {
	manager, err := register.GetDBManager(s.Command)
	if err != nil {
		return nil, errors.Wrap(err, "Get DB manager failed")
	}

	err = manager.Unlock(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Unlock DB failed")
	}

	return nil, nil
}
