/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/viper"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
)

type Manager struct {
	engines.DBManagerBase
	actionSvcPorts *[]int
	client         *http.Client
}

var perNodeRegx = regexp.MustCompile("^[^,]*$")

func NewManager(properties engines.Properties) (engines.DBManager, error) {
	logger := ctrl.Log.WithName("custom")

	managerBase, err := engines.NewDBManagerBase(logger)
	if err != nil {
		return nil, err
	}

	mgr := &Manager{
		actionSvcPorts: &[]int{},
		DBManagerBase:  *managerBase,
	}

	actionSvcList := viper.GetString("KB_RSM_ACTION_SVC_LIST")
	if len(actionSvcList) > 0 {
		err := json.Unmarshal([]byte(actionSvcList), mgr.actionSvcPorts)
		if err != nil {
			return nil, err
		}
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

	return mgr, nil
}

func (mgr *Manager) GetReplicaRole(ctx context.Context, cluster *dcs.Cluster) (string, error) {
	if mgr.actionSvcPorts == nil {
		return "", nil
	}

	var (
		lastOutput []byte
		err        error
	)

	for _, port := range *mgr.actionSvcPorts {
		u := fmt.Sprintf("http://127.0.0.1:%d/role?KB_RSM_LAST_STDOUT=%s", port, url.QueryEscape(string(lastOutput)))
		lastOutput, err = mgr.callAction(ctx, u)
		if err != nil {
			return "", err
		}
		mgr.Logger.Info("action succeed", "url", u, "output", string(lastOutput))
	}
	finalOutput := strings.TrimSpace(string(lastOutput))

	if perNodeRegx.MatchString(finalOutput) {
		return finalOutput, nil
	}

	// csv format: term,podName,role
	parseCSV := func(input string) (string, error) {
		res := common.GlobalRoleSnapshot{}
		lines := strings.Split(input, "\n")
		for _, line := range lines {
			fields := strings.Split(strings.TrimSpace(line), ",")
			if len(fields) != 3 {
				return "", err
			}
			res.Version = strings.TrimSpace(fields[0])
			pair := common.PodRoleNamePair{
				PodName:  strings.TrimSpace(fields[1]),
				RoleName: strings.ToLower(strings.TrimSpace(fields[2])),
			}
			res.PodRoleNamePairs = append(res.PodRoleNamePairs, pair)
		}
		resByte, err := json.Marshal(res)
		return string(resByte), err
	}
	return parseCSV(finalOutput)
}

// callAction performs an HTTP request to local HTTP endpoint specified by actionSvcPort
func (mgr *Manager) callAction(ctx context.Context, url string) ([]byte, error) {
	// compose http request
	request, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, err
	}

	// send http request
	resp, err := mgr.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// parse http response
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("received status code %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return b, err
}
