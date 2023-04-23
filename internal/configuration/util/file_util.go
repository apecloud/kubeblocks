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

package util

import (
	"os"
	"path/filepath"
)

func FromConfigFiles(files []string) (map[string]string, error) {
	m := make(map[string]string)
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		m[filepath.Base(file)] = string(b)
	}
	return m, nil
}

func ToArgs(m map[string]string) []string {
	args := make([]string, 0, len(m)*2)
	for k, v := range m {
		args = append(args, k, v)
	}
	return args
}
