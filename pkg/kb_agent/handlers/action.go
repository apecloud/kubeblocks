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
	"encoding/json"
	"fmt"
	"sync"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

var actionHandlers = map[string]util.Handlers{}
var execHandler = &ExecHandler{}
var grpcHandler = &GRPCHandler{}
var lock sync.Mutex

func initHandlerSettings() error {
	lock.Lock()
	defer lock.Unlock()
	if len(actionHandlers) != 0 {
		return nil
	}
	actionJSON := viper.GetString(constant.KBEnvActionHandlers)
	if actionJSON == "" {
		return errors.New("action handlers is not specified")
	}

	err := json.Unmarshal([]byte(actionJSON), &actionHandlers)
	if err != nil {
		msg := fmt.Sprintf("unmarshal action handlers [%s] failed: %s", actionJSON, err.Error())
		return errors.New(msg)
	}
	return nil
}

func Do(ctx context.Context, action string, args map[string]any) (map[string]any, error) {
	err := initHandlerSettings()
	if err != nil {
		return nil, errors.Wrap(err, "init handler settings failed")
	}
	if action == "" {
		return nil, errors.New("action is empty")
	}
	handlers := actionHandlers[action]

	switch {
	case len(handlers.Command) != 0:
		return execHandler.Do(ctx, handlers, args)

	case len(handlers.GPRC) != 0:
		return grpcHandler.Do(ctx, handlers, args)
	}

	return nil, errors.New("no handler found")
}
