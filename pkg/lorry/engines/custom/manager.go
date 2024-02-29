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

package custom

import (
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/spf13/viper"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
)

type Manager struct {
	engines.DBManagerBase

	// For ASM Actions
	actionSvcPorts *[]int
	client         *http.Client

	// For ComponentDefinition Actions
	actionCommands map[string][]string
}

func NewManager(properties engines.Properties) (engines.DBManager, error) {
	logger := ctrl.Log.WithName("custom")

	managerBase, err := engines.NewDBManagerBase(logger)
	if err != nil {
		return nil, err
	}

	managerBase.DBStartupReady = true
	mgr := &Manager{
		actionSvcPorts: &[]int{},
		DBManagerBase:  *managerBase,
	}

	err = mgr.InitASMActions()
	if err != nil {
		mgr.Logger.Info("init RSM commands failed", "error", err.Error())
		return nil, err
	}
	err = mgr.InitComponentDefintionActions()
	if err != nil {
		mgr.Logger.Info("init component definition commands failed", "error", err.Error())
		return nil, err
	}
	return mgr, nil
}

func (mgr *Manager) InitASMActions() error {
	actionSvcList := viper.GetString("KB_RSM_ACTION_SVC_LIST")
	if actionSvcList == "" {
		return nil
	}
	err := json.Unmarshal([]byte(actionSvcList), mgr.actionSvcPorts)
	if err != nil {
		return err
	}

	// See guidance on proper HTTP client settings here:
	// https://medium.com/@nate510/don-t-use-go-s-default-http-client-4804cb19f779
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}
	netTransport := &http.Transport{
		Dial:                dialer.Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	mgr.client = &http.Client{
		Timeout:   time.Second * 30,
		Transport: netTransport,
	}

	return nil
}

func (mgr *Manager) InitComponentDefintionActions() error {
	actionJSON := viper.GetString(constant.KBEnvActionCommands)
	if actionJSON != "" {
		err := json.Unmarshal([]byte(actionJSON), &mgr.actionCommands)
		if err != nil {
			return err
		}
	}
	return nil
}
