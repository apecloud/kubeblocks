//go:build linux

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

package configmanager

import (
	"os"
	"syscall"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

var allUnixSignals = map[appsv1alpha1.SignalType]os.Signal{
	appsv1alpha1.SIGHUP:    syscall.SIGHUP, // reload signal for mysql 8.x.xxx
	appsv1alpha1.SIGINT:    syscall.SIGINT,
	appsv1alpha1.SIGQUIT:   syscall.SIGQUIT,
	appsv1alpha1.SIGILL:    syscall.SIGILL,
	appsv1alpha1.SIGTRAP:   syscall.SIGTRAP,
	appsv1alpha1.SIGABRT:   syscall.SIGABRT,
	appsv1alpha1.SIGBUS:    syscall.SIGBUS,
	appsv1alpha1.SIGFPE:    syscall.SIGFPE,
	appsv1alpha1.SIGKILL:   syscall.SIGKILL,
	appsv1alpha1.SIGUSR1:   syscall.SIGUSR1,
	appsv1alpha1.SIGSEGV:   syscall.SIGSEGV,
	appsv1alpha1.SIGUSR2:   syscall.SIGUSR2,
	appsv1alpha1.SIGPIPE:   syscall.SIGPIPE,
	appsv1alpha1.SIGALRM:   syscall.SIGALRM,
	appsv1alpha1.SIGTERM:   syscall.SIGTERM,
	appsv1alpha1.SIGSTKFLT: syscall.SIGSTKFLT,
	appsv1alpha1.SIGCHLD:   syscall.SIGCHLD,
	appsv1alpha1.SIGCONT:   syscall.SIGCONT,
	appsv1alpha1.SIGSTOP:   syscall.SIGSTOP,
	appsv1alpha1.SIGTSTP:   syscall.SIGTSTP,
	appsv1alpha1.SIGTTIN:   syscall.SIGTTIN,
	appsv1alpha1.SIGTTOU:   syscall.SIGTTOU,
	appsv1alpha1.SIGURG:    syscall.SIGURG,
	appsv1alpha1.SIGXCPU:   syscall.SIGXCPU,
	appsv1alpha1.SIGXFSZ:   syscall.SIGXFSZ,
	appsv1alpha1.SIGVTALRM: syscall.SIGVTALRM,
	appsv1alpha1.SIGPROF:   syscall.SIGPROF,
	appsv1alpha1.SIGWINCH:  syscall.SIGWINCH,
	appsv1alpha1.SIGIO:     syscall.SIGIO,
	appsv1alpha1.SIGPWR:    syscall.SIGPWR,
	appsv1alpha1.SIGSYS:    syscall.SIGSYS,
}
