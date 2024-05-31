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
	"strconv"
	"time"

	"github.com/apecloud/kubeblocks/pkg/lorry/operations"
)

type Job struct {
	Name             string
	Ticker           *time.Ticker
	Operation        operations.Operation
	TimeoutSeconds   int
	PeriodSeconds    int
	SuccessThreshold int
	FailureThreshold int
}

func NewJob(name string, settings map[string]string) (*Job, error) {
	operations := operations.Operations()
	ops, ok := operations[name]
	if !ok {
		logger.Info("Operation not found", "name", name)
		return nil, nil
	}
	job := &Job{
		Name:             name,
		Operation:        ops,
		TimeoutSeconds:   60,
		PeriodSeconds:    60,
		SuccessThreshold: 1,
		FailureThreshold: 3,
	}

	if v, ok := settings["timeoutSeconds"]; ok {
		timeoutSeconds, err := strconv.Atoi(v)
		if err != nil {
			logger.Info("Failed to parse timeoutSeconds", "error", err.Error(), "job", name, "value", v)
			return nil, err
		}
		job.TimeoutSeconds = timeoutSeconds
	}

	if settings["periodSeconds"] != "" {
		periodSeconds, err := strconv.Atoi(settings["periodSeconds"])
		if err != nil {
			logger.Info("Failed to parse periodSeconds", "error", err.Error(), "job", name, "value", settings["periodSeconds"])
			return nil, err
		}
		job.PeriodSeconds = periodSeconds
	}

	if settings["successThreshold"] != "" {
		successThreshold, err := strconv.Atoi(settings["successThreshold"])
		if err != nil {
			logger.Info("Failed to parse successThreshold", "error", err.Error(), "job", name, "value", settings["successThreshold"])
			return nil, err
		}
		job.SuccessThreshold = successThreshold
	}

	if settings["failureThreshold"] != "" {
		failureThreshold, err := strconv.Atoi(settings["failureThreshold"])
		if err != nil {
			logger.Info("Failed to parse failureThreshold", "error", err.Error(), "job", name, "value", settings["failureThreshold"])
			return nil, err
		}
		job.FailureThreshold = failureThreshold
	}
	// operation is initialized in httpserver/apis.go
	job.Operation.SetTimeout(time.Duration(job.TimeoutSeconds) * time.Second)
	return job, nil
}

func (job *Job) Start() {
	job.Ticker = time.NewTicker(time.Duration(job.PeriodSeconds) * time.Second)
	defer job.Ticker.Stop()
	for range job.Ticker.C {
		_, err := job.Operation.Do(context.Background(), nil)
		if err != nil {
			logger.Info("Failed to run job", "name", job.Name, "error", err.Error())
			// Handle error, e.g., increase failure count, stop job if failureThreshold is reached, etc.
		}
	}
}
