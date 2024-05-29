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

package replica

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/register"
	"github.com/apecloud/kubeblocks/pkg/lorry/operations"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

type Join struct {
	operations.Base
	dcsStore dcs.DCS
	logger   logr.Logger
	Timeout  time.Duration
	Command  []string
}

var join operations.Operation = &Join{}

func init() {
	err := operations.Register(strings.ToLower(string(util.JoinMemberOperation)), join)
	if err != nil {
		panic(err.Error())
	}
}

func (s *Join) Init(ctx context.Context) error {
	s.dcsStore = dcs.GetStore()
	if s.dcsStore == nil {
		return errors.New("dcs store init failed")
	}

	actionJSON := viper.GetString(constant.KBEnvActionCommands)
	if actionJSON != "" {
		actionCommands := map[string][]string{}
		err := json.Unmarshal([]byte(actionJSON), &actionCommands)
		if err != nil {
			s.logger.Info("get action commands failed", "error", err.Error())
			return err
		}
		memberJoinCmd, ok := actionCommands[constant.MemberJoinAction]
		if ok && len(memberJoinCmd) > 0 {
			s.Command = memberJoinCmd
		}
	}
	return nil
}

func (s *Join) Do(ctx context.Context, req *operations.OpsRequest) (*operations.OpsResponse, error) {
	manager, err := register.GetDBManager(s.Command)
	if err != nil {
		return nil, errors.Wrap(err, "get manager failed")
	}

	cluster, err := s.dcsStore.GetCluster()
	if err != nil {
		s.logger.Error(err, "get cluster failed")
		return nil, err
	}

	// join current member to db cluster
	err = manager.JoinCurrentMemberToCluster(ctx, cluster)
	if err != nil {
		s.logger.Error(err, "join member to cluster failed")
		return nil, err
	}

	return nil, nil
}
