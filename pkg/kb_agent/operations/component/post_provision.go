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

type PostProvision struct {
	operations.Base
	logger  logr.Logger
	Timeout time.Duration
	Command []string
}

type PostProvisionManager interface {
	PostProvision(ctx context.Context, componentNames, podNames, podIPs, podHostNames, podHostIPs string) error
}

var postProvision operations.Operation = &PostProvision{}

func init() {
	err := operations.Register(strings.ToLower(string(util.PostProvisionOperation)), postProvision)
	if err != nil {
		panic(err.Error())
	}
}

func (s *PostProvision) Init(_ context.Context) error {
	actionJSON := viper.GetString(constant.KBEnvActionCommands)
	if actionJSON != "" {
		actionCommands := map[string][]string{}
		err := json.Unmarshal([]byte(actionJSON), &actionCommands)
		if err != nil {
			s.logger.Info("get action commands failed", "error", err.Error())
			return err
		}
		postProvisionCmd, ok := actionCommands[constant.PostProvisionAction]
		if ok && len(postProvisionCmd) > 0 {
			s.Command = postProvisionCmd
		}
	}
	return nil
}

func (s *PostProvision) PreCheck(ctx context.Context, req *operations.OpsRequest) error {
	return nil
}

func (s *PostProvision) Do(ctx context.Context, req *operations.OpsRequest) (*operations.OpsResponse, error) {
	componentNames := req.GetString("componentNames")
	podNames := req.GetString("podNames")
	podIPs := req.GetString("podIPs")
	podHostNames := req.GetString("podHostNames")
	podHostIPs := req.GetString("podHostIPs")
	manager, err := register.GetDBManager(s.Command)
	if err != nil {
		return nil, errors.Wrap(err, "get manager failed")
	}

	ppManager, ok := manager.(PostProvisionManager)
	if !ok {
		return nil, models.ErrNotImplemented
	}
	err = ppManager.PostProvision(ctx, componentNames, podNames, podIPs, podHostNames, podHostIPs)
	return nil, err
}
