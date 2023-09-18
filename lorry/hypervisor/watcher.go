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

	"github.com/go-logr/logr"
)

type Watcher struct {
	ctx        context.Context
	cancel     context.CancelFunc
	DaemonChan chan *Daemon
	Processes  map[int]chan *ProcStatus
	Logger     logr.Logger
}

type ProcStatus struct {
	state *os.ProcessState
	err   error
}

func NewWatcher(logger logr.Logger) *Watcher {
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
			watcher.Logger.Info("Daemon is stopped by manual. Will not be restarted.", "daemon", daemon)
			continue
		}
		watcher.Logger.Info("Restarting daemon")
		if daemon.IsAlive() {
			watcher.Logger.Info("Daemon was supposed to be dead, but it is alive.", "daemon", daemon)
		}

		time.Sleep(time.Second)
		err := daemon.Start()
		if err != nil {
			watcher.Logger.Error(err, "Could not restart daemon.", "daemon", daemon)
		}

		watcher.Watch(daemon)
	}
}

func (watcher *Watcher) Watch(daemon *Daemon) {
	if daemon.Process == nil {
		watcher.Logger.Info("There is no process for the daemon", "daemon", daemon)
		return
	}

	if _, ok := watcher.Processes[daemon.Process.Pid]; ok {
		watcher.Logger.Info("The daemon is arealdy under watching")
	}

	statusChan := make(chan *ProcStatus, 1)
	pid := daemon.Process.Pid

	watcher.Processes[pid] = statusChan
	go func() {
		watcher.Logger.Info("Starting watcher on daemon", "daemon", daemon)
		state, err := daemon.Wait()
		statusChan <- &ProcStatus{
			state: state,
			err:   err,
		}
	}()

	go func() {
		defer delete(watcher.Processes, pid)
		select {
		case procStatus := <-statusChan:
			if procStatus != nil {
				watcher.Logger.Info("Daemon exists", "daemon", daemon, "state", procStatus.state.String())
				watcher.DaemonChan <- daemon
				close(statusChan)
			}
			break
		case <-watcher.ctx.Done():
			break
		}
	}()
}

func (watcher *Watcher) StopAndWait() {
	for _, statusChan := range watcher.Processes {
		procStatus := <-statusChan
		if procStatus != nil {
			watcher.Logger.Info("Daemon exists", "state", procStatus.state.String())
			close(statusChan)
		}
	}
	watcher.cancel()
}
