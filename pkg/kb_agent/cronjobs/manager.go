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
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/kb_agent/handlers"
)

type Manager struct {
	Jobs map[string]Job
}

var logger = ctrl.Log.WithName("cronjobs")

func NewManager() (*Manager, error) {
	actionHandlers := handlers.GetHandlers()
	jobs := make(map[string]Job)
	for name, handler := range actionHandlers {
		if handler.CronJob == nil {
			continue
		}
		logger.Info("cronjob found", "name", name)
		job := NewJob(name, handler.CronJob)
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
