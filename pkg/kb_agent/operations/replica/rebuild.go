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
	"fmt"
	"io"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/register"
	"github.com/apecloud/kubeblocks/pkg/lorry/operations"
)

type Rebuild struct {
	operations.Base
	dcsStore  dcs.DCS
	dbManager engines.DBManager
	logger    logr.Logger
}

var rebuild operations.Operation = &Rebuild{}

func init() {
	err := operations.Register("rebuild", rebuild)
	if err != nil {
		panic(err.Error())
	}
}

func (s *Rebuild) Init(ctx context.Context) error {
	s.dcsStore = dcs.GetStore()
	if s.dcsStore == nil {
		return errors.New("dcs store init failed")
	}

	s.logger = ctrl.Log.WithName("Rebuild")

	actionJSON := viper.GetString(constant.KBEnvActionCommands)
	if actionJSON != "" {
		actionCommands := map[string][]string{}
		err := json.Unmarshal([]byte(actionJSON), &actionCommands)
		if err != nil {
			s.logger.Info("get action commands failed", "error", err.Error())
			return err
		}
		rebuildCmd, ok := actionCommands[constant.RebuildAction]
		if ok && len(rebuildCmd) > 0 {
			s.Command = rebuildCmd
		}
	}
	dbManager, err := register.GetDBManager(s.Command)
	if err != nil {
		return errors.Wrap(err, "get manager failed")
	}

	s.dbManager = dbManager

	return nil
}

func (s *Rebuild) IsReadonly(ctx context.Context) bool {
	return false
}

func (s *Rebuild) Do(ctx context.Context, req *operations.OpsRequest) (*operations.OpsResponse, error) {
	resp := &operations.OpsResponse{
		Data: map[string]any{},
	}
	resp.Data["operation"] = constant.RebuildAction

	cluster := s.dcsStore.GetClusterFromCache()
	currentMember := cluster.GetMemberWithName(s.dbManager.GetCurrentMemberName())
	if currentMember == nil || currentMember.HAPort == "" {
		return nil, errors.Errorf("current node does not support rebuild, there is no ha service yet")
	}

	haAddr := fmt.Sprintf("http://127.0.0.1:%s/v1.0/rebuild", currentMember.HAPort)
	httpResp, err := http.Post(haAddr, "application/json", nil)
	if err != nil {
		return nil, errors.Wrap(err, "request ha service failed")
	}
	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return resp, errors.Wrap(err, "error reading response body")
	}
	bodyString := string(bodyBytes)
	if httpResp.StatusCode/100 == 2 {
		resp.Data["message"] = bodyString
		return resp, nil
	}

	s.logger.Info("request ha service failed", "status code", httpResp.StatusCode, "body", bodyString)
	errResult := make(map[string]string)
	err = json.Unmarshal(bodyBytes, &errResult)
	if err != nil {
		return nil, errors.New(bodyString)
	}
	if msg, ok := errResult["message"]; ok {
		return nil, errors.New(msg)
	}

	return nil, errors.New(bodyString)
}
