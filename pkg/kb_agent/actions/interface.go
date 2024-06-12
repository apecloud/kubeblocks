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

package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/handlers"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/handlers/exec"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/handlers/grpc"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
)

type Action interface {
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

	Command []string
	Handler handlers.Handler

	Logger logr.Logger
}

var actionHandlers = map[string]util.Handlers{}
var defaultGRPCSetting map[string]string
var lock sync.Mutex

func initHandlerSettings() {
	if len(actionHandlers) != 0 {
		return
	}
	lock.Lock()
	defer lock.Unlock()
	if len(actionHandlers) != 0 {
		return
	}
	actionJSON := viper.GetString(constant.KBEnvActionHandlers)
	if actionJSON == "" {
		panic("action handlers is not specified")
	}

	err := json.Unmarshal([]byte(actionJSON), &actionHandlers)
	if err != nil {
		msg := fmt.Sprintf("unmarshal action handlers [%s] failed: %s", actionJSON, err.Error())
		panic(msg)
	}

	for _, handlers := range actionHandlers {
		if len(handlers.GPRC) != 0 {
			defaultGRPCSetting = handlers.GPRC
			break
		}
	}
}

func (b *Base) Init(ctx context.Context) error {
	initHandlerSettings()
	var handlers util.Handlers
	if b.Action != "" {
		handlers = actionHandlers[b.Action]
	}

	switch {
	case len(handlers.Command) != 0:
		b.Command = handlers.Command
		handler, err := exec.NewHandler(nil)
		if err != nil {
			return errors.Wrap(err, "new exec handler failed")
		}
		b.Handler = handler

	case len(handlers.GPRC) != 0 || len(defaultGRPCSetting) != 0:
		grpcSetting := handlers.GPRC
		if len(grpcSetting) == 0 {
			grpcSetting = defaultGRPCSetting
		}
		handler, err := grpc.NewHandler(grpcSetting)
		if err != nil {
			return errors.Wrap(err, "new grpc handler failed")
		}

		b.Handler = handler
	default:
		return errors.New("no handler found")
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
