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

package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/spf13/viper"
)

type OpsResult map[string]interface{}

type RoleAgent struct {
	CheckRoleFailedCount       int
	RoleUnchangedCount         int
	FailedEventReportFrequency int

	// RoleDetectionThreshold is used to set the report duration of role event after role changed,
	// then event controller can always get rolechanged events to maintain pod label accurately
	// in cases of:
	// 1 rolechanged event lost;
	// 2 pod role label deleted or updated incorrectly.
	RoleObservationThreshold int
	OriRole                  string
	Logger                   log.Logger
	actionSvcPorts           *[]int
	client                   *http.Client
}

func init() {
	viper.SetDefault("KB_FAILED_EVENT_REPORT_FREQUENCY", defaultFailedEventReportFrequency)
	viper.SetDefault("KB_ROLE_OBSERVATION_THRESHOLD", defaultRoleObservationThreshold)
}

func NewRoleAgent(writer io.Writer, prefix string) *RoleAgent {
	return &RoleAgent{
		Logger:         *log.New(writer, prefix, log.LstdFlags),
		actionSvcPorts: &[]int{},
	}
}

func (roleAgent *RoleAgent) Init() error {
	roleAgent.FailedEventReportFrequency = viper.GetInt("KB_FAILED_EVENT_REPORT_FREQUENCY")
	if roleAgent.FailedEventReportFrequency < 300 {
		roleAgent.FailedEventReportFrequency = 300
	} else if roleAgent.FailedEventReportFrequency > 3600 {
		roleAgent.FailedEventReportFrequency = 3600
	}

	roleAgent.RoleObservationThreshold = viper.GetInt("KB_ROLE_OBSERVATION_THRESHOLD")
	if roleAgent.RoleObservationThreshold < 60 {
		roleAgent.RoleObservationThreshold = 60
	} else if roleAgent.RoleObservationThreshold > 300 {
		roleAgent.RoleObservationThreshold = 300
	}

	actionSvcList := viper.GetString("KB_CONSENSUS_SET_ACTION_SVC_LIST")
	if len(actionSvcList) > 0 {
		err := json.Unmarshal([]byte(actionSvcList), roleAgent.actionSvcPorts)
		if err != nil {
			return err
		}
	}

	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}
	netTransport := &http.Transport{
		Dial:                dialer.Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	roleAgent.client = &http.Client{
		Timeout:   time.Second * 30,
		Transport: netTransport,
	}
	return nil
}

func (roleAgent *RoleAgent) ShutDownClient() {
	roleAgent.client.CloseIdleConnections()
}

func (roleAgent *RoleAgent) CheckRole(ctx context.Context) (OpsResult, bool) {
	opsRes := OpsResult{}
	needNotify := false
	role, err := roleAgent.getRole(ctx)

	// OpsResult organized like below:
	// Success: event originalRole role
	// Failure: event message

	if err != nil {
		roleAgent.Logger.Printf("error executing checkRole: %v", err)
		opsRes["event"] = OperationFailed
		opsRes["message"] = err.Error()
		if roleAgent.CheckRoleFailedCount%roleAgent.FailedEventReportFrequency == 0 {
			roleAgent.Logger.Printf("role checks failed %v times continuously", roleAgent.CheckRoleFailedCount)
		}
		roleAgent.CheckRoleFailedCount++
		return opsRes, true
	}

	roleAgent.CheckRoleFailedCount = 0

	opsRes["event"] = OperationSuccess
	opsRes["originalRole"] = roleAgent.OriRole
	opsRes["role"] = role

	if roleAgent.OriRole != role {
		roleAgent.OriRole = role
		needNotify = true
	} else {
		roleAgent.RoleUnchangedCount++
	}

	// RoleUnchangedCount is the count of consecutive role unchanged checks.
	// If the role remains unchanged consecutively in RoleObservationThreshold checks after it has changed,
	// then the roleCheck event will be reported at roleEventReportFrequency so that the event controller
	// can always get relevant roleCheck events in order to maintain the pod label accurately, even in cases
	// of roleChanged events being lost or the pod role label being deleted or updated incorrectly.
	if roleAgent.RoleUnchangedCount < roleAgent.RoleObservationThreshold && roleAgent.RoleUnchangedCount%roleEventReportFrequency == 0 {
		needNotify = true
		roleAgent.RoleUnchangedCount = 0
	}
	return opsRes, needNotify
}

func (roleAgent *RoleAgent) getRole(ctx context.Context) (string, error) {
	if roleAgent.actionSvcPorts == nil {
		return "", nil
	}

	var (
		lastOutput string
		err        error
	)

	for _, port := range *roleAgent.actionSvcPorts {
		u := fmt.Sprintf("http://127.0.0.1:%d/role?KB_CONSENSUS_SET_LAST_STDOUT=%s", port, url.QueryEscape(lastOutput))
		lastOutput, err = roleAgent.callAction(ctx, u)
		if err != nil {
			return "", err
		}
	}

	return lastOutput, nil
}

func (roleAgent *RoleAgent) callAction(ctx context.Context, url string) (string, error) {
	// compose http request
	request, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	// send http request
	resp, err := roleAgent.client.Do(request)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// parse http response
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("received status code %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(b), err
}
