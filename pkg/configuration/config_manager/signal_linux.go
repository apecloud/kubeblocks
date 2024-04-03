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

	appsv1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
)

var allUnixSignals = map[appsv1beta1.SignalType]os.Signal{
	appsv1beta1.SIGHUP:    syscall.SIGHUP, // reload signal for mysql 8.x.xxx
	appsv1beta1.SIGINT:    syscall.SIGINT,
	appsv1beta1.SIGQUIT:   syscall.SIGQUIT,
	appsv1beta1.SIGILL:    syscall.SIGILL,
	appsv1beta1.SIGTRAP:   syscall.SIGTRAP,
	appsv1beta1.SIGABRT:   syscall.SIGABRT,
	appsv1beta1.SIGBUS:    syscall.SIGBUS,
	appsv1beta1.SIGFPE:    syscall.SIGFPE,
	appsv1beta1.SIGKILL:   syscall.SIGKILL,
	appsv1beta1.SIGUSR1:   syscall.SIGUSR1,
	appsv1beta1.SIGSEGV:   syscall.SIGSEGV,
	appsv1beta1.SIGUSR2:   syscall.SIGUSR2,
	appsv1beta1.SIGPIPE:   syscall.SIGPIPE,
	appsv1beta1.SIGALRM:   syscall.SIGALRM,
	appsv1beta1.SIGTERM:   syscall.SIGTERM,
	appsv1beta1.SIGSTKFLT: syscall.SIGSTKFLT,
	appsv1beta1.SIGCHLD:   syscall.SIGCHLD,
	appsv1beta1.SIGCONT:   syscall.SIGCONT,
	appsv1beta1.SIGSTOP:   syscall.SIGSTOP,
	appsv1beta1.SIGTSTP:   syscall.SIGTSTP,
	appsv1beta1.SIGTTIN:   syscall.SIGTTIN,
	appsv1beta1.SIGTTOU:   syscall.SIGTTOU,
	appsv1beta1.SIGURG:    syscall.SIGURG,
	appsv1beta1.SIGXCPU:   syscall.SIGXCPU,
	appsv1beta1.SIGXFSZ:   syscall.SIGXFSZ,
	appsv1beta1.SIGVTALRM: syscall.SIGVTALRM,
	appsv1beta1.SIGPROF:   syscall.SIGPROF,
	appsv1beta1.SIGWINCH:  syscall.SIGWINCH,
	appsv1beta1.SIGIO:     syscall.SIGIO,
	appsv1beta1.SIGPWR:    syscall.SIGPWR,
	appsv1beta1.SIGSYS:    syscall.SIGSYS,
}
