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
	originRole      string
	FailedCount     int
	ReportFrequency int
}

func NewCheckRoleJob(commonJob CommonJob) *CheckRoleJob {
	return &CheckRoleJob{
		CommonJob:       commonJob,
		originRole:      "waitForStart",
		ReportFrequency: 60,
	}
}

func (job *CheckRoleJob) Do() error {
	ctx1, cancel := context.WithTimeout(context.Background(), time.Duration(job.TimeoutSeconds))
	defer cancel()
	resp, err := handlers.Do(ctx1, job.Name, nil)

	if err != nil {
		logger.Info("executing checkRole error", "error", err.Error())
		if job.FailedCount%job.ReportFrequency == 0 {
			logger.Info("role checks failed continuously", "times", job.FailedCount)
			err = util.SentEventForProbe(context.Background(), resp)
		}
		job.FailedCount++
		return err
	}

	role, _ := resp["output"].(string)
	job.FailedCount = 0
	if job.originRole == role {
		return nil
	}

	result := map[string]any{}
	result["operation"] = util.CheckRoleOperation
	result["role"] = role
	result["originalRole"] = job.originRole
	job.originRole = role
	err = util.SentEventForProbe(context.Background(), result)
	return err
}

func (job *CheckRoleJob) Start() {
	job.Ticker = time.NewTicker(time.Duration(job.PeriodSeconds) * time.Second)
	defer job.Ticker.Stop()
	for range job.Ticker.C {
		err := job.Do()
		if err != nil {
			logger.Info("Failed to run job", "name", job.Name, "error", err.Error())
			// Handle error, e.g., increase failure count, stop job if failureThreshold is reached, etc.
		}
	}
}
