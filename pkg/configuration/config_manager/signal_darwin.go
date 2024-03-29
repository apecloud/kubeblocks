//go:build darwin

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

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

var allUnixSignals = map[appsv1.SignalType]os.Signal{
	appsv1.SIGHUP:  syscall.SIGHUP, // reload signal for mysql 8.x.xxx
	appsv1.SIGINT:  syscall.SIGINT,
	appsv1.SIGQUIT: syscall.SIGQUIT,
	appsv1.SIGILL:  syscall.SIGILL,
	appsv1.SIGTRAP: syscall.SIGTRAP,
	appsv1.SIGABRT: syscall.SIGABRT,
	appsv1.SIGBUS:  syscall.SIGBUS,
	appsv1.SIGFPE:  syscall.SIGFPE,
	appsv1.SIGKILL: syscall.SIGKILL,
	appsv1.SIGUSR1: syscall.SIGUSR1,
	appsv1.SIGSEGV: syscall.SIGSEGV,
	appsv1.SIGUSR2: syscall.SIGUSR2,
	appsv1.SIGPIPE: syscall.SIGPIPE,
	appsv1.SIGALRM: syscall.SIGALRM,
	appsv1.SIGTERM: syscall.SIGTERM,
	// appsv1.SIGSTKFLT: syscall.SIGSTKFLT,
	appsv1.SIGCHLD:   syscall.SIGCHLD,
	appsv1.SIGCONT:   syscall.SIGCONT,
	appsv1.SIGSTOP:   syscall.SIGSTOP,
	appsv1.SIGTSTP:   syscall.SIGTSTP,
	appsv1.SIGTTIN:   syscall.SIGTTIN,
	appsv1.SIGTTOU:   syscall.SIGTTOU,
	appsv1.SIGURG:    syscall.SIGURG,
	appsv1.SIGXCPU:   syscall.SIGXCPU,
	appsv1.SIGXFSZ:   syscall.SIGXFSZ,
	appsv1.SIGVTALRM: syscall.SIGVTALRM,
	appsv1.SIGPROF:   syscall.SIGPROF,
	appsv1.SIGWINCH:  syscall.SIGWINCH,
	appsv1.SIGIO:     syscall.SIGIO,
	// appsv1.SIGPWR:    syscall.SIGPWR,
	appsv1.SIGSYS: syscall.SIGSYS,
}
