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

package testdata

import (
	"os"
	"path/filepath"
	"runtime"
)

var testDataRoot string

func init() {
	_, rootPath, _, _ := runtime.Caller(0)
	testDataRoot = filepath.Dir(rootPath)
}

// SubTestDataPath get the file path which belongs to test data directory or its subdirectories.
func SubTestDataPath(subPath string) string {
	return filepath.Join(testDataRoot, subPath)
}

// GetTestDataFileContent get the file content which belongs to test data directory or its subdirectories.
func GetTestDataFileContent(filePath string) ([]byte, error) {
	return os.ReadFile(SubTestDataPath(filePath))
}
