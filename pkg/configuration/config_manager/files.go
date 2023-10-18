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
	"errors"
	"os"
	"path/filepath"

	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/core"
)

type files string

func newFiles(basePath string) files {
	return files(basePath)
}

func (f files) Get(filename string) (string, error) {
	path := filepath.Join(string(f), filename)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return "", cfgutil.MakeError("file %s not found", filename)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return "", cfgutil.WrapError(err, "failed to read file %s", filename)
	}
	return string(b), nil
}
