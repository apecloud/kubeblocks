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

package operations

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/grpc"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/register"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

type Operation interface {
	Init(context.Context) error
	SetTimeout(timeout time.Duration)
	IsReadonly(context.Context) bool
	PreCheck(context.Context, *OpsRequest) error
	Do(context.Context, *OpsRequest) (*OpsResponse, error)
}

type Base struct {
	// the name of componentdefinition action
	Action  string
	Timeout time.Duration

	Command   []string
	DBManager engines.DBManager

	Logger logr.Logger
}

var actionHandlers = map[string]util.Handlers{}

func init() {
	actionJSON := viper.GetString(constant.KBEnvActionHandlers)
	if actionJSON == "" {
		return
	}

	err := json.Unmarshal([]byte(actionJSON), &actionHandlers)
	if err != nil {
		msg := fmt.Sprintf("unmarshal action handlers [%s] failed: %s", actionJSON, err.Error())
		panic(msg)
	}
}

func (b *Base) Init(ctx context.Context) error {
	var handlers util.Handlers
	if b.Action != "" {
		handlers = actionHandlers[b.Action]
	}

	switch {
	case len(handlers.Command) != 0:
		b.Command = handlers.Command
		b.DBManager = register.GetCustomManager()
	case len(handlers.GPRC) != 0:
		dbManager, err := grpc.NewManager(engines.Properties(handlers.GPRC))
		if err != nil {
			return errors.Wrap(err, "new grpc manager failed")
		}

		b.DBManager = dbManager
	default:
		dbManager, err := register.GetDBManager()
		if err != nil {
			return errors.Wrap(err, "get builtin manager failed")
		}
		b.DBManager = dbManager
	}
	return nil
}

func (b *Base) SetTimeout(timeout time.Duration) {
	b.Timeout = timeout
}

func (b *Base) IsReadonly(ctx context.Context) bool {
	return false
}

func (b *Base) PreCheck(ctx context.Context, request *OpsRequest) error {
	return nil
}

func (b *Base) Do(ctx context.Context, request *OpsRequest) (*OpsResponse, error) {
	return nil, errors.New("not implemented")
}

func (b *Base) ExecCommand(ctx context.Context) error {
	output, err := util.ExecCommand(ctx, b.Command, os.Environ())
	if output != "" {
		b.Logger.Info(b.Action, "output", output)
	}
	return err
}
