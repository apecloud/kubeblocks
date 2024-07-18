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
	"fmt"
	"time"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kbagent/util"
)

type Job interface {
	Start()
	Stop()
}

type CommonJob struct {
	Name             string
	Ticker           *time.Ticker
	TimeoutSeconds   int
	PeriodSeconds    int
	SuccessThreshold int
	FailureThreshold int
	FailedCount      int
	ReportFrequency  int
	Do               func() error
}

func NewJob(name string, cronJob *util.CronJob) (Job, error) {
	job := &CommonJob{
		Name:             name,
		TimeoutSeconds:   60,
		PeriodSeconds:    60,
		SuccessThreshold: 1,
		FailureThreshold: 3,
		ReportFrequency:  60,
	}

	if cronJob.PeriodSeconds != 0 {
		job.PeriodSeconds = cronJob.PeriodSeconds
	}

	if cronJob.SuccessThreshold != 0 {
		job.SuccessThreshold = cronJob.SuccessThreshold
	}

	if cronJob.FailureThreshold != 0 {
		job.FailureThreshold = cronJob.FailureThreshold
	}

	if cronJob.ReportFrequency != 0 {
		job.ReportFrequency = cronJob.ReportFrequency
	}

	if name == constant.RoleProbeAction {
		return NewCheckRoleJob(*job), nil
	}

	return nil, fmt.Errorf("%s not implemented", name)
}

func (job *CommonJob) Start() {
	job.Ticker = time.NewTicker(time.Duration(job.PeriodSeconds) * time.Second)
	defer job.Ticker.Stop()
	for range job.Ticker.C {
		err := job.Do()
		if err != nil {
			logger.Info("Failed to run job", "name", job.Name, "error", err.Error())
			if job.FailedCount%job.ReportFrequency == 0 {
				logger.Info("job failed continuously", "name", job.Name, "times", job.FailedCount)
				msg := util.MessageBase{
					Event:   util.OperationFailed,
					Action:  job.Name,
					Message: err.Error(),
				}
				_ = util.SentEventForProbe(context.Background(), msg)
			}
			job.FailedCount++
		} else {
			job.FailedCount = 0
		}
	}
}

func (job *CommonJob) Stop() {
	job.Ticker.Stop()
}
