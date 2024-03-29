//go:build windows

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
	appsv1.SIGHUP:  syscall.SIGHUP,
	appsv1.SIGINT:  syscall.SIGINT,
	appsv1.SIGQUIT: syscall.SIGQUIT,
	appsv1.SIGILL:  syscall.SIGILL,
	appsv1.SIGTRAP: syscall.SIGTRAP,
	appsv1.SIGABRT: syscall.SIGABRT,
	appsv1.SIGBUS:  syscall.SIGBUS,
	appsv1.SIGFPE:  syscall.SIGFPE,
	appsv1.SIGKILL: syscall.SIGKILL,
	appsv1.SIGSEGV: syscall.SIGSEGV,
	appsv1.SIGPIPE: syscall.SIGPIPE,
	appsv1.SIGALRM: syscall.SIGALRM,
	appsv1.SIGTERM: syscall.SIGTERM,
}
