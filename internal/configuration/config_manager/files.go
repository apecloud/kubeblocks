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
	"errors"
	"os"
	"path/filepath"

	cfgutil "github.com/apecloud/kubeblocks/internal/configuration"
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
