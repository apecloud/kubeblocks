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

package backuprepo

import (
	"encoding/json"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	trueVal = "true"

	associatedBackupRepoKey = "dataprotection.kubeblocks.io/backup-repo-name"
)

func flagsToValues(fs *pflag.FlagSet, explicit bool) map[string]string {
	values := make(map[string]string)
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" || (explicit && !f.Changed) {
			return
		}
		val, _ := fs.GetString(f.Name)
		values[f.Name] = val
	})
	return values
}

func createPatchData(oldObj, newObj runtime.Object) ([]byte, error) {
	oldData, err := json.Marshal(oldObj)
	if err != nil {
		return nil, err
	}
	newData, err := json.Marshal(newObj)
	if err != nil {
		return nil, err
	}
	return jsonpatch.CreateMergePatch(oldData, newData)
}
