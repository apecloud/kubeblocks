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

package builder

import (
	"path/filepath"

	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/action"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/connector"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/core/util"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	"github.com/apecloud/kubeblocks/internal/gotemplate"
)

type Template struct {
	action.BaseAction
	Template string
	Dst      string
	Values   gotemplate.TplValues
}

func (t *Template) Execute(runtime connector.Runtime) error {
	templateStr, err := BuildFromTemplate(&t.Values, t.Template)
	if err != nil {
		return cfgcore.WrapError(err, "failed to render template %s", t.Template)
	}

	fileName := filepath.Join(runtime.GetHostWorkDir(), t.Template)
	if err := util.WriteFile(fileName, []byte(templateStr)); err != nil {
		return cfgcore.WrapError(err, "failed to write file %s", fileName)
	}

	if err := runtime.GetRunner().SudoScp(fileName, t.Dst); err != nil {
		return cfgcore.WrapError(err, "failed to scp file %s to remote %s", fileName, t.Dst)
	}
	return nil
}
