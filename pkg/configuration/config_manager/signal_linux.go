//go:build linux

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

package configmanager

import (
	"os"
	"syscall"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
)

var allUnixSignals = map[parametersv1alpha1.SignalType]os.Signal{
	parametersv1alpha1.SIGHUP:    syscall.SIGHUP, // reload signal for mysql 8.x.xxx
	parametersv1alpha1.SIGINT:    syscall.SIGINT,
	parametersv1alpha1.SIGQUIT:   syscall.SIGQUIT,
	parametersv1alpha1.SIGILL:    syscall.SIGILL,
	parametersv1alpha1.SIGTRAP:   syscall.SIGTRAP,
	parametersv1alpha1.SIGABRT:   syscall.SIGABRT,
	parametersv1alpha1.SIGBUS:    syscall.SIGBUS,
	parametersv1alpha1.SIGFPE:    syscall.SIGFPE,
	parametersv1alpha1.SIGKILL:   syscall.SIGKILL,
	parametersv1alpha1.SIGUSR1:   syscall.SIGUSR1,
	parametersv1alpha1.SIGSEGV:   syscall.SIGSEGV,
	parametersv1alpha1.SIGUSR2:   syscall.SIGUSR2,
	parametersv1alpha1.SIGPIPE:   syscall.SIGPIPE,
	parametersv1alpha1.SIGALRM:   syscall.SIGALRM,
	parametersv1alpha1.SIGTERM:   syscall.SIGTERM,
	parametersv1alpha1.SIGSTKFLT: syscall.SIGSTKFLT,
	parametersv1alpha1.SIGCHLD:   syscall.SIGCHLD,
	parametersv1alpha1.SIGCONT:   syscall.SIGCONT,
	parametersv1alpha1.SIGSTOP:   syscall.SIGSTOP,
	parametersv1alpha1.SIGTSTP:   syscall.SIGTSTP,
	parametersv1alpha1.SIGTTIN:   syscall.SIGTTIN,
	parametersv1alpha1.SIGTTOU:   syscall.SIGTTOU,
	parametersv1alpha1.SIGURG:    syscall.SIGURG,
	parametersv1alpha1.SIGXCPU:   syscall.SIGXCPU,
	parametersv1alpha1.SIGXFSZ:   syscall.SIGXFSZ,
	parametersv1alpha1.SIGVTALRM: syscall.SIGVTALRM,
	parametersv1alpha1.SIGPROF:   syscall.SIGPROF,
	parametersv1alpha1.SIGWINCH:  syscall.SIGWINCH,
	parametersv1alpha1.SIGIO:     syscall.SIGIO,
	parametersv1alpha1.SIGPWR:    syscall.SIGPWR,
	parametersv1alpha1.SIGSYS:    syscall.SIGSYS,
}
