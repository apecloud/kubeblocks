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
	"github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/register"
	"github.com/apecloud/kubeblocks/pkg/lorry/plugin"
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

	Command        []string
	DBPluginClient plugin.DBPluginClient
	DBManager      engines.DBManager

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
	handlers := actionHandlers[b.Action]
	if len(handlers.Command) != 0 {
		b.Command = handlers.Command
	} else if len(handlers.GPRC) != 0 {
		host := "127.0.0.1"
		if h, ok := handlers.GPRC["host"]; ok {
			host = h
		}
		port, ok := handlers.GPRC["port"]
		if !ok || port == "" {
			return errors.New("grpc port is not set")
		}
		client, err := plugin.NewPluginClient(host + ":" + port)
		if err != nil {
			return errors.Wrap(err, "new grpc client failed")
		}
		b.DBPluginClient = client
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

func (b *Base) GetDBManager() (engines.DBManager, error) {
	if !controllerutil.IsNil(b.DBManager) {
		return b.DBManager, nil
	}
	dbManager, err := register.GetDBManager(b.Command)
	if err != nil {
		return nil, errors.Wrap(err, "get manager failed")
	}
	if controllerutil.IsNil(dbManager) {
		return nil, errors.New("not implemented")
	}
	b.DBManager = dbManager
	return dbManager, nil
}
