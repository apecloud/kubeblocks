//go:build linux

/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package configmap

import (
	"os"
	"syscall"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

var allUnixSignals = map[dbaasv1alpha1.SignalType]os.Signal{
	dbaasv1alpha1.SIGHUP:    syscall.SIGHUP, // reload signal for mysql 8.x.xxx
	dbaasv1alpha1.SIGINT:    syscall.SIGINT,
	dbaasv1alpha1.SIGQUIT:   syscall.SIGQUIT,
	dbaasv1alpha1.SIGILL:    syscall.SIGILL,
	dbaasv1alpha1.SIGTRAP:   syscall.SIGTRAP,
	dbaasv1alpha1.SIGABRT:   syscall.SIGABRT,
	dbaasv1alpha1.SIGBUS:    syscall.SIGBUS,
	dbaasv1alpha1.SIGFPE:    syscall.SIGFPE,
	dbaasv1alpha1.SIGKILL:   syscall.SIGKILL,
	dbaasv1alpha1.SIGUSR1:   syscall.SIGUSR1,
	dbaasv1alpha1.SIGSEGV:   syscall.SIGSEGV,
	dbaasv1alpha1.SIGUSR2:   syscall.SIGUSR2,
	dbaasv1alpha1.SIGPIPE:   syscall.SIGPIPE,
	dbaasv1alpha1.SIGALRM:   syscall.SIGALRM,
	dbaasv1alpha1.SIGTERM:   syscall.SIGTERM,
	dbaasv1alpha1.SIGSTKFLT: syscall.SIGSTKFLT,
	dbaasv1alpha1.SIGCHLD:   syscall.SIGCHLD,
	dbaasv1alpha1.SIGCONT:   syscall.SIGCONT,
	dbaasv1alpha1.SIGSTOP:   syscall.SIGSTOP,
	dbaasv1alpha1.SIGTSTP:   syscall.SIGTSTP,
	dbaasv1alpha1.SIGTTIN:   syscall.SIGTTIN,
	dbaasv1alpha1.SIGTTOU:   syscall.SIGTTOU,
	dbaasv1alpha1.SIGURG:    syscall.SIGURG,
	dbaasv1alpha1.SIGXCPU:   syscall.SIGXCPU,
	dbaasv1alpha1.SIGXFSZ:   syscall.SIGXFSZ,
	dbaasv1alpha1.SIGVTALRM: syscall.SIGVTALRM,
	dbaasv1alpha1.SIGPROF:   syscall.SIGPROF,
	dbaasv1alpha1.SIGWINCH:  syscall.SIGWINCH,
	dbaasv1alpha1.SIGIO:     syscall.SIGIO,
	dbaasv1alpha1.SIGPWR:    syscall.SIGPWR,
	dbaasv1alpha1.SIGSYS:    syscall.SIGSYS,
}
