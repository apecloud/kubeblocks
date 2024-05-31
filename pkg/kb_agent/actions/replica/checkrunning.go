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
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/actions"
	"github.com/apecloud/kubeblocks/pkg/lorry/util"
)

// CheckRunning checks whether the binding service is in running status,
// If check fails continuously, report an event at FailedEventReportFrequency frequency
type CheckRunning struct {
	actions.Base
	Timeout                    time.Duration
	DBAddress                  string
	CheckRunningFailedCount    int
	FailedEventReportFrequency int
}

var checkrunning actions.Action = &CheckRunning{}

func init() {
	err := actions.Register("checkrunning", checkrunning)
	if err != nil {
		panic(err.Error())
	}
}

func (s *CheckRunning) Init(ctx context.Context) error {
	s.FailedEventReportFrequency = viper.GetInt("KB_FAILED_EVENT_REPORT_FREQUENCY")
	if s.FailedEventReportFrequency < 300 {
		s.FailedEventReportFrequency = 300
	} else if s.FailedEventReportFrequency > 3600 {
		s.FailedEventReportFrequency = 3600
	}

	timeoutSeconds := util.DefaultProbeTimeoutSeconds
	if viper.IsSet(constant.KBEnvRoleProbeTimeout) {
		timeoutSeconds = viper.GetInt(constant.KBEnvRoleProbeTimeout)
	}
	// lorry utilizes the pod readiness probe to trigger probe and 'timeoutSeconds' is directly copied from the 'probe.timeoutSeconds' field of pod.
	// here we give 80% of the total time to probe job and leave the remaining 20% to kubelet to handle the readiness probe related tasks.
	s.Timeout = time.Duration(timeoutSeconds) * (800 * time.Millisecond)
	s.DBAddress = s.getAddress()
	s.Logger = ctrl.Log.WithName("CheckRunning")
	s.Action = constant.CheckRunningAction
	return s.Base.Init(ctx)
}

func (s *CheckRunning) Do(ctx context.Context, req *actions.OpsRequest) (*actions.OpsResponse, error) {
	var message string
	opsRsp := &actions.OpsResponse{}
	opsRsp.Data["operation"] = util.CheckRunningOperation

	dbPort, err := s.Handler.GetPort()
	if err != nil {
		return nil, errors.Wrap(err, "get db port failed")
	}

	host := net.JoinHostPort(s.DBAddress, strconv.Itoa(dbPort))
	// sql exec timeout needs to be less than httpget's timeout which by default 1s.
	conn, err := net.DialTimeout("tcp", host, 500*time.Millisecond)
	if err != nil {
		message = fmt.Sprintf("running check %s error", host)
		s.Logger.Error(err, message)
		opsRsp.Data["event"] = util.OperationFailed
		opsRsp.Data["message"] = message
		if s.CheckRunningFailedCount%s.FailedEventReportFrequency == 0 {
			s.Logger.Info("running checks failed continuously", "times", s.CheckRunningFailedCount)
			// resp.Metadata[StatusCode] = OperationFailedHTTPCode
			err = util.SentEventForProbe(ctx, opsRsp.Data)
		}
		s.CheckRunningFailedCount++
		return opsRsp, err
	}
	defer conn.Close()
	s.CheckRunningFailedCount = 0
	message = "TCP Connection Established Successfully!"
	if tcpCon, ok := conn.(*net.TCPConn); ok {
		err := tcpCon.SetLinger(0)
		s.Logger.Error(err, "running check, set tcp linger failed")
	}
	opsRsp.Data["event"] = util.OperationSuccess
	opsRsp.Data["message"] = message
	return opsRsp, nil
}

// getAddress returns component service address, if component is not listening on
// 127.0.0.1, the Operation needs to overwrite this function and set ops.DBAddress
func (s *CheckRunning) getAddress() string {
	return "127.0.0.1"
}
