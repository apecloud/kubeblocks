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

package testdata

import (
	"os"
	"path/filepath"
	"runtime"
)

var testDataRoot string

func init() {
	_, file, _, _ := runtime.Caller(0)
	testDataRoot = filepath.Dir(file)
}

// SubTestDataPath gets the file path which belongs to test data directory or its subdirectories.
func SubTestDataPath(subPath string) string {
	return filepath.Join(testDataRoot, subPath)
}

// GetTestDataFileContent gets the file content which belongs to test data directory or its subdirectories.
func GetTestDataFileContent(filePath string) ([]byte, error) {
	return os.ReadFile(SubTestDataPath(filePath))
}
