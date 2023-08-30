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
	dataprotection "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

type BackupBuilder struct {
	BaseBuilder[dataprotection.Backup, *dataprotection.Backup, BackupBuilder]
}

func NewBackupBuilder(namespace, name string) *BackupBuilder {
	builder := &BackupBuilder{}
	builder.init(namespace, name, &dataprotection.Backup{}, builder)
	return builder
}

func (builder *BackupBuilder) SetBackupPolicyName(policyName string) *BackupBuilder {
	builder.get().Spec.BackupPolicyName = policyName
	return builder
}

func (builder *BackupBuilder) SetBackupMethod(method string) *BackupBuilder {
	builder.get().Spec.BackupMethod = method
	return builder
}

func (builder *BackupBuilder) SetParentBackupName(parent string) *BackupBuilder {
	builder.get().Spec.ParentBackupName = parent
	return builder
}
