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
	"encoding/json"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/spf13/viper"
)

type Manager struct {
	Jobs map[string]*Job
}

var logger = ctrl.Log.WithName("cronjobs")

func NewManager() (*Manager, error) {
	cronSettings := make(map[string]map[string]string)
	jsonStr := viper.GetString(constant.KBEnvCronJobs)
	if jsonStr == "" {
		logger.Info("env is not set", "env", constant.KBEnvCronJobs)
		return &Manager{}, nil
	}

	err := json.Unmarshal([]byte(jsonStr), &cronSettings)
	if err != nil {
		logger.Info("Failed to unmarshal env", "name", constant.KBEnvCronJobs, "value", jsonStr, "error", err.Error())
		return nil, err
	}

	jobs := make(map[string]*Job)
	for name, setting := range cronSettings {
		job, err := NewJob(name, setting)
		if err != nil {
			logger.Info("Failed to create job", "error", err.Error(), "name", name, "setting", setting)
		}
		jobs[name] = job
	}
	return &Manager{
		Jobs: jobs,
	}, nil
}

func (m *Manager) Start() {
	for _, job := range m.Jobs {
		go job.Start()
	}
}
