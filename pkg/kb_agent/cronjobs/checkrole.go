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

package cronjobs

import (
	"context"
	"time"

	"github.com/apecloud/kubeblocks/pkg/kb_agent/handlers"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/util"
	"github.com/spf13/pflag"
)

type CheckRoleJob struct {
	CommonJob
	lastRole                string
	roleUnchangedEventCount int
}

var sendRoleEventPeriodically bool
var sendRoleEventFrequency int = 300

func init() {
	pflag.BoolVar(&sendRoleEventPeriodically, "send-role-event-periodically", false, "Enable the mechanism to send role events periodically to prevent event loss.")
}

func NewCheckRoleJob(commonJob CommonJob) *CheckRoleJob {
	checkRoleJob := &CheckRoleJob{
		CommonJob: commonJob,
		lastRole:  "waitForStart",
	}

	checkRoleJob.Do = checkRoleJob.do
	return checkRoleJob
}

func (job *CheckRoleJob) do() error {
	ctx1, cancel := context.WithTimeout(context.Background(), time.Duration(job.TimeoutSeconds))
	defer cancel()
	resp, err := handlers.Do(ctx1, job.Name, nil)

	if err != nil {
		return err
	}

	role := resp.Message
	if job.lastRole == role {
		if !sendRoleEventPeriodically {
			return nil
		}
		job.roleUnchangedEventCount++
		if job.roleUnchangedEventCount%sendRoleEventFrequency != 0 {
			return nil
		}
		logger.Info("send role event periodically", "role", role)
	} else {
		job.roleUnchangedEventCount = 0
		logger.Info("send role changed event", "role", role)
	}

	result := util.RoleProbeMessage{
		MessageBase: util.MessageBase{
			Event:  util.OperationSuccess,
			Action: job.Name,
		},
		Role: role,
	}
	job.lastRole = role
	err = util.SentEventForProbe(context.Background(), result)
	return err
}
