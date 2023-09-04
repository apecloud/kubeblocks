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

package backup

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

// GetBackupPolicy returns the BackupPolicy with the given namespace and name.
func GetBackupPolicy(ctx context.Context, cli client.Client, namespace, name string) (*dpv1alpha1.BackupPolicy, error) {
	backupPolicy := &dpv1alpha1.BackupPolicy{}
	if err := cli.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, backupPolicy); err != nil {
		return nil, err
	}
	return backupPolicy, nil
}

func GetActionSet(ctx context.Context, cli client.Client, namespace, name string) (*dpv1alpha1.ActionSet, error) {
	actionSet := &dpv1alpha1.ActionSet{}
	if err := cli.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, actionSet); err != nil {
		return nil, err
	}
	return actionSet, nil
}
