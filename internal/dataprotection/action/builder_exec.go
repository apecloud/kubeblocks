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

package action

import (
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

type ExecActionBuilder struct {
	Name      string
	Namespace string
	PodName   string
	Exec      *dpv1alpha1.ExecAction
}

var _ Builder = &ExecActionBuilder{}

func (e *ExecActionBuilder) Build() Action {
	return &KubeExec{
		Name:      e.Name,
		Command:   e.Exec.Command,
		Container: e.Exec.Container,
		Namespace: e.Namespace,
		PodName:   e.PodName,
		Timeout:   e.Exec.Timeout,
	}
}
