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
	"strconv"
	"strings"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	viper "github.com/apecloud/kubeblocks/internal/viperx"
	. "github.com/apecloud/kubeblocks/lorry/binding"
	"github.com/apecloud/kubeblocks/lorry/component"
	. "github.com/apecloud/kubeblocks/lorry/util"
)

// HTTPCustom is a binding for an http url endpoint invocation
type HTTPCustom struct {
	actionSvcPorts *[]int
	client         *http.Client
	BaseOperations
}

// NewHTTPCustom returns a new HTTPCustom.
func NewHTTPCustom() *HTTPCustom {
	logger := ctrl.Log.WithName("Custom")
	return &HTTPCustom{
		actionSvcPorts: &[]int{},
		BaseOperations: BaseOperations{Logger: logger},
	}
}

// Init performs metadata parsing.
func (h *HTTPCustom) Init(metadata component.Properties) error {
	actionSvcList := viper.GetString("KB_RSM_ACTION_SVC_LIST")
	if len(actionSvcList) > 0 {
		err := json.Unmarshal([]byte(actionSvcList), h.actionSvcPorts)
		if err != nil {
			return err
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
	h.client = &http.Client{
		Timeout:   time.Second * 30,
		Transport: netTransport,
	}

	h.BaseOperations.Init(metadata)
	h.BaseOperations.GetRole = h.GetRole
	h.BaseOperations.GetGlobalInfo = h.GetGlobalInfo
	h.OperationsMap[CheckRoleOperation] = h.CheckRoleOps
	h.OperationsMap[GetGlobalInfoOperation] = h.GetGlobalInfoOps

	return nil
}

func (h *HTTPCustom) GetRole(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (string, error) {
	if h.actionSvcPorts == nil {
		return "", nil
	}

	var (
		lastOutput []byte
		err        error
	)

	for _, port := range *h.actionSvcPorts {
		u := fmt.Sprintf("http://127.0.0.1:%d/role?KB_CONSENSUS_SET_LAST_STDOUT=%s", port, url.QueryEscape(string(lastOutput)))
		lastOutput, err = h.callAction(ctx, u)
		if err != nil {
			return "", err
		}
		h.Logger.Info("action succeed", "url", u, "output", lastOutput)
	}
	role := string(lastOutput)

	return strings.TrimSpace(role), nil
}

func (h *HTTPCustom) GetRoleOps(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (OpsResult, error) {
	role, err := h.GetRole(ctx, req, resp)
	if err != nil {
		return nil, err
	}
	opsRes := OpsResult{}
	opsRes["role"] = role
	return opsRes, nil
}

func (h *HTTPCustom) GetGlobalInfo(ctx context.Context, req *ProbeRequest, resp *ProbeResponse) (GlobalInfo, error) {
	if h.actionSvcPorts == nil {
		return GlobalInfo{}, nil
	}

	var (
		lastOutput []byte
		err        error
	)

	for _, port := range *h.actionSvcPorts {
		u := fmt.Sprintf("http://127.0.0.1:%d/role?KB_CONSENSUS_SET_LAST_STDOUT=%s", port, url.QueryEscape(string(lastOutput)))
		lastOutput, err = h.callAction(ctx, u)
		if err != nil {
			return GlobalInfo{}, err
		}
	}

	// csv format: term,podname,role
	parseCSV := func(input []byte) (GlobalInfo, error) {
		res := GlobalInfo{PodName2Role: map[string]string{}}
		str := string(input)
		lines := strings.Split(str, "\n")
		for _, line := range lines {
			fields := strings.Split(line, ",")
			if len(fields) != 3 {
				return res, err
			}
			res.Term, err = strconv.Atoi(fields[0])
			if err != nil {
				return res, err
			}
			k := fields[1]
			v := fields[2]
			res.PodName2Role[k] = v
		}
		return res, nil
	}

	res, err := parseCSV(lastOutput)
	if err != nil {
		return GlobalInfo{}, err
	}
	res.Event = OperationSuccess
	h.Logger.Info("GetGlobalInfo get result", "result", res)

	return res, nil
}

// callAction performs an HTTP request to local HTTP endpoint specified by actionSvcPort
func (h *HTTPCustom) callAction(ctx context.Context, url string) ([]byte, error) {
	// compose http request
	request, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, err
	}

	// send http request
	resp, err := h.client.Do(request)
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
