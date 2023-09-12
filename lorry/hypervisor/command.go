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

package hypervisor

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/apecloud/kubeblocks/internal/viperx"
)

var scriptsPath = "/scripts"

type Command struct {
	Cmd    string
	Args   []string
	Stdout []byte
	Err    error
}

func init() {
	if viperx.IsSet(constant.KBEnvScriptsPath) {
		scriptsPath = viperx.GetString(constant.KBEnvScriptsPath)
	}
}

func GetRole() (string, error) {
	program := "get_role.sh"
	program = filepath.Join(scriptsPath, program)

	cmd := exec.Command(program)
	cmd.Env = os.Environ()
	bytes, err := cmd.Output()
	return string(bytes), err
}
