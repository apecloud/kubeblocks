//go:build windows

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

package configmanager

import (
	"os"
	"syscall"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

var allUnixSignals = map[appsv1alpha1.SignalType]os.Signal{
	appsv1alpha1.SIGHUP:  syscall.SIGHUP,
	appsv1alpha1.SIGINT:  syscall.SIGINT,
	appsv1alpha1.SIGQUIT: syscall.SIGQUIT,
	appsv1alpha1.SIGILL:  syscall.SIGILL,
	appsv1alpha1.SIGTRAP: syscall.SIGTRAP,
	appsv1alpha1.SIGABRT: syscall.SIGABRT,
	appsv1alpha1.SIGBUS:  syscall.SIGBUS,
	appsv1alpha1.SIGFPE:  syscall.SIGFPE,
	appsv1alpha1.SIGKILL: syscall.SIGKILL,
	appsv1alpha1.SIGSEGV: syscall.SIGSEGV,
	appsv1alpha1.SIGPIPE: syscall.SIGPIPE,
	appsv1alpha1.SIGALRM: syscall.SIGALRM,
	appsv1alpha1.SIGTERM: syscall.SIGTERM,
}
