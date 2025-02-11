/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package lifecycle

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	configFilesCreated = "KB_CONFIG_FILES_CREATED"
	configFilesRemoved = "KB_CONFIG_FILES_REMOVED"
	configFilesUpdated = "KB_CONFIG_FILES_UPDATED"
)

func FileTemplateChanges(created, removed, updated string) map[string]string {
	return map[string]string{
		configFilesCreated: created,
		configFilesRemoved: removed,
		configFilesUpdated: updated,
	}
}

type reconfigure struct {
	created string
	removed string
	updated string
}

var _ lifecycleAction = &reconfigure{}

func (a *reconfigure) name() string {
	return "reconfigure"
}

func (a *reconfigure) parameters(ctx context.Context, cli client.Reader) (map[string]string, error) {
	// The container executing this action has access to following variables:
	//
	// - KB_CONFIG_FILES_CREATED: file1,file2...
	// - KB_CONFIG_FILES_REMOVED: file1,file2...
	// - KB_CONFIG_FILES_UPDATED: file1:checksum1,file2:checksum2...
	return FileTemplateChanges(a.created, a.removed, a.updated), nil
}
