//go:build linux || darwin

/*
Copyright ApeCloud Inc.

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

	"github.com/sirupsen/logrus"
)

type PID int32

const (
	InvalidPid PID = 0
)

func sendSignal(pid PID, sig os.Signal) error {
	process, err := os.FindProcess(int(pid))
	if err != nil {
		return err
	}

	logrus.Tracef("send pid[%d] to signal: %s", pid, sig.String())
	err = process.Signal(sig)
	if err != nil {
		return err
	}

	return nil
}
