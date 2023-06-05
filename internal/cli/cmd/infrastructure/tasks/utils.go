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

package tasks

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/logger"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/util"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/files"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

func runCommand(cmd *exec.Cmd) error {
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

func getSha256sumFile(binary *files.KubeBinary) string {
	return fmt.Sprintf("%s.sum.%s", binary.Path(), "sha256")
}

func checkSha256sum(binary *files.KubeBinary) error {
	checksumFile := getSha256sumFile(binary)
	if !util.IsExist(checksumFile) {
		return cfgcore.MakeError("checksum file %s is not exist", checksumFile)
	}

	checksum, err := calSha256sum(binary.Path())
	if err != nil {
		return err
	}

	data, err := os.ReadFile(checksumFile)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(data)) == checksum {
		return nil
	}
	return cfgcore.MakeError("checksum of %s is not match, [%s] vs [%s]", binary.ID, string(data), checksum)
}

func calSha256sum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func writeSha256sum(binary *files.KubeBinary) error {
	checksumFile := getSha256sumFile(binary)
	sum, err := calSha256sum(binary.Path())
	if err != nil {
		return err
	}
	return util.WriteFile(checksumFile, []byte(sum))
}
