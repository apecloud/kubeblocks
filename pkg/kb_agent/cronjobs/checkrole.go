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
)

type CheckRoleJob struct {
	CommonJob
	originRole string
}

func NewCheckRoleJob(commonJob CommonJob) *CheckRoleJob {
	checkRoleJob := &CheckRoleJob{
		CommonJob:  commonJob,
		originRole: "waitForStart",
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

	role, _ := resp["output"].(string)
	if job.originRole == role {
		return nil
	}

	result := util.RoleProbeMessage{
		MessageBase: util.MessageBase{
			Event:  util.OperationSuccess,
			Action: job.Name,
		},
		Role: role,
	}
	job.originRole = role
	err = util.SentEventForProbe(context.Background(), result)
	return err
}
