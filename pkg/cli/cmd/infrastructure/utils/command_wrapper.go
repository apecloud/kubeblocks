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

package utils

import (
	"errors"
	"fmt"
	"io"
	"os/exec"

	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/logger"
)

func RunCommand(cmd *exec.Cmd) error {
	logger.Log.Messagef(common.LocalHost, "Running: %s", cmd.String())
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err = cmd.Start(); err != nil {
		return err
	}

	// read from stdout
	for {
		tmp := make([]byte, 1024)
		_, err := stdout.Read(tmp)
		fmt.Print(string(tmp))
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			logger.Log.Errorln(err)
			break
		}
	}
	return cmd.Wait()
}
