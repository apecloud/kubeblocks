//go:build windows

/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	parametersv1alpha1.SIGHUP:  syscall.SIGHUP,
	parametersv1alpha1.SIGINT:  syscall.SIGINT,
	parametersv1alpha1.SIGQUIT: syscall.SIGQUIT,
	parametersv1alpha1.SIGILL:  syscall.SIGILL,
	parametersv1alpha1.SIGTRAP: syscall.SIGTRAP,
	parametersv1alpha1.SIGABRT: syscall.SIGABRT,
	parametersv1alpha1.SIGBUS:  syscall.SIGBUS,
	parametersv1alpha1.SIGFPE:  syscall.SIGFPE,
	parametersv1alpha1.SIGKILL: syscall.SIGKILL,
	parametersv1alpha1.SIGSEGV: syscall.SIGSEGV,
	parametersv1alpha1.SIGPIPE: syscall.SIGPIPE,
	parametersv1alpha1.SIGALRM: syscall.SIGALRM,
	parametersv1alpha1.SIGTERM: syscall.SIGTERM,
}
