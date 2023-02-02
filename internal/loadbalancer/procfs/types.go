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

package procfs

import (
	"os"
	"path"
)

type ProcFS interface {
	Set(key, value string) error

	Get(key string) (string, error)
}

type procFs struct {
}

func (p *procFs) path(key string) string {
	return path.Join("/proc/sys", key)
}

func (p *procFs) Set(key, value string) error {
	return os.WriteFile(p.path(key), []byte(value), 0644)
}

func (p *procFs) Get(key string) (string, error) {
	data, err := os.ReadFile(p.path(key))
	return string(data), err
}

func NewProcFS() ProcFS {
	return &procFs{}
}
