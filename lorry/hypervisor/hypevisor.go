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

package hypervisor

import (
	"errors"

	"github.com/dapr/kit/logger"
	"github.com/spf13/pflag"
)

type Hypervisor struct {
	Logger    logger.Logger
	DBService *Daemon
	Watcher   *Watcher
}

var hypervisor *Hypervisor

func NewHypervisor(logger logger.Logger) (*Hypervisor, error) {
	args := pflag.Args()
	daemon, err := NewDaemon(args, logger)
	if err != nil {
		return nil, err
	}

	watcher := NewWatcher(logger)
	go watcher.Start()
	hypervisor = &Hypervisor{
		Logger:    logger,
		DBService: daemon,
		Watcher:   watcher,
	}

	return hypervisor, nil
}

func (hypervisor *Hypervisor) Start() {
	if hypervisor.DBService == nil {
		hypervisor.Logger.Info("No DB Service")
		return
	}

	err := hypervisor.DBService.Start()
	if err != nil {
		hypervisor.Logger.Warnf("Start DB Service failed: %s", err)
		return
	}

	hypervisor.Watcher.Watch(hypervisor.DBService)
}

func (hypervisor *Hypervisor) StopAndWait() {
	if hypervisor.DBService == nil {
		hypervisor.Logger.Info("No DB Service")
		return
	}

	_ = hypervisor.DBService.Stop()

	hypervisor.Watcher.StopAndWait()
}

func StopDBService() {
	if hypervisor.DBService == nil {
		hypervisor.Logger.Info("No DB Service")
	}

	_ = hypervisor.DBService.Stop()
}

func StartDBService() error {
	if hypervisor.DBService == nil {
		hypervisor.Logger.Info("No DB Service")
		return errors.New("no db service")
	}

	if IsDBServiceAlive() {
		hypervisor.Logger.Info("DB Service is already running")
		return nil
	}

	err := hypervisor.DBService.Start()
	if err != nil {
		hypervisor.Logger.Warnf("Start DB Service failed: %s", err)
		return err
	}

	hypervisor.Watcher.Watch(hypervisor.DBService)
	return nil
}

func IsDBServiceAlive() bool {
	if hypervisor.DBService == nil || hypervisor.DBService.Process == nil {
		return false
	}

	_, ok := hypervisor.Watcher.Processes[hypervisor.DBService.Process.Pid]
	return ok
}
