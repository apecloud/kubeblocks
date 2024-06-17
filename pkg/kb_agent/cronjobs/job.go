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

type Job interface {
	Start()
	Do() error
	Stop()
}

type CommonJob struct {
	Name             string
	Ticker           *time.Ticker
	TimeoutSeconds   int
	PeriodSeconds    int
	SuccessThreshold int
	FailureThreshold int
}

func NewJob(name string, cronJob *util.CronJob) (*CommonJob, error) {
	job := &CommonJob{
		Name:             name,
		TimeoutSeconds:   60,
		PeriodSeconds:    60,
		SuccessThreshold: 1,
		FailureThreshold: 3,
	}

	if cronJob.TimeoutSeconds != 0 {
		job.TimeoutSeconds = cronJob.TimeoutSeconds
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

	return job, nil
}

func (job *CommonJob) Start() {
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

func (job *CommonJob) Do() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(job.TimeoutSeconds)*time.Second)
	defer cancel()
	_, err := handlers.Do(ctx, job.Name, nil)
	return err
}

func (job *CommonJob) Stop() {
	job.Ticker.Stop()
}
