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
	"context"
	"os"
	"time"

	"github.com/dapr/kit/logger"
)

type Watcher struct {
	ctx        context.Context
	cancel     context.CancelFunc
	DaemonChan chan *Daemon
	Processes  map[int]chan *ProcStatus
	Logger     logger.Logger
}

type ProcStatus struct {
	state *os.ProcessState
	err   error
}

func NewWatcher(logger logger.Logger) *Watcher {
	ctx, cancel := context.WithCancel(context.Background())
	daemonChan := make(chan *Daemon, 1)
	processes := make(map[int]chan *ProcStatus)

	return &Watcher{
		ctx:        ctx,
		cancel:     cancel,
		DaemonChan: daemonChan,
		Processes:  processes,
		Logger:     logger,
	}
}

func (watcher *Watcher) Start() {
	for daemon := range watcher.DaemonChan {
		if daemon.Stopped {
			watcher.Logger.Infof("Daemon %s is stopped by manul. Will not be restarted.", daemon)
			continue
		}
		watcher.Logger.Infof("Restarting daemon")
		if daemon.IsAlive() {
			watcher.Logger.Warnf("Daemon %s was supposed to be dead, but it is alive.", daemon)
		}

		time.Sleep(time.Second)
		err := daemon.Start()
		if err != nil {
			watcher.Logger.Warnf("Could not restart process %s due to %s.", daemon, err)
		}

		watcher.Watch(daemon)
	}
}

func (watcher *Watcher) Watch(daemon *Daemon) {
	if daemon.Process == nil {
		watcher.Logger.Infof("There is no process for the daemon %s", daemon)
		return
	}

	if _, ok := watcher.Processes[daemon.Process.Pid]; ok {
		watcher.Logger.Infof("The daemon is arealdy under watching")
	}

	statusChan := make(chan *ProcStatus, 1)
	pid := daemon.Process.Pid

	watcher.Processes[pid] = statusChan
	go func() {
		watcher.Logger.Infof("Starting watcher on daemon %s", daemon)
		state, err := daemon.Wait()
		statusChan <- &ProcStatus{
			state: state,
			err:   err,
		}
	}()

	go func() {
		defer delete(watcher.Processes, pid)
		defer close(statusChan)
		select {
		case procStatus := <-statusChan:
			watcher.Logger.Infof("Daemon %s exists", daemon)
			watcher.Logger.Infof("State is %s", procStatus.state.String())
			watcher.DaemonChan <- daemon
			break
		case <-watcher.ctx.Done():
			break
		}
	}()
}

func (watcher *Watcher) Stop() {
	watcher.cancel()
}
