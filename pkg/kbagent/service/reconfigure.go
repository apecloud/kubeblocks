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

package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

const (
	configFilesCreated = "KB_CONFIG_FILES_CREATED"
	configFilesRemoved = "KB_CONFIG_FILES_REMOVED"
	configFilesUpdated = "KB_CONFIG_FILES_UPDATED"
)

func reconfigure(_ context.Context, req *proto.ActionRequest) error {
	if req.Action != "reconfigure" && !strings.HasPrefix(req.Action, "udf-reconfigure") {
		return nil
	}

	if err := checkReconfigureCreated(req); err != nil {
		return err
	}
	if err := checkReconfigureRemoved(req); err != nil {
		return err
	}
	if err := checkReconfigureUpdated(req); err != nil {
		return err
	}
	return nil
}

func checkReconfigureCreated(req *proto.ActionRequest) error {
	created := req.Parameters[configFilesCreated]
	if len(created) > 0 {
		for _, file := range strings.Split(created, ",") {
			exist, err := checkLocalFileExist(file)
			if err != nil {
				return err
			}
			if !exist {
				return errors.Wrapf(proto.ErrPreconditionFailed, "reconfigure - created file is not exist: %s", file)
			}
		}
	}
	return nil
}

func checkReconfigureRemoved(req *proto.ActionRequest) error {
	removed := req.Parameters[configFilesRemoved]
	if len(removed) > 0 {
		for _, file := range strings.Split(removed, ",") {
			exist, err := checkLocalFileExist(file)
			if err != nil {
				return err
			}
			if exist {
				return errors.Wrapf(proto.ErrPreconditionFailed, "reconfigure - removed file is still exist: %s", file)
			}
		}
	}
	return nil
}

func checkReconfigureUpdated(req *proto.ActionRequest) error {
	updated := req.Parameters[configFilesUpdated]
	if len(updated) == 0 {
		return nil
	}

	files := strings.Split(updated, ",")
	for _, item := range files {
		tokens := strings.Split(item, ":")
		if len(tokens) != 2 {
			return errors.Wrapf(proto.ErrBadRequest, "reconfigure - updated files format error: %s", updated)
		}
		file, checksum := tokens[0], tokens[1]
		if err := checkLocalFileUpToDate(file, checksum); err != nil {
			return errors.Wrapf(proto.ErrPreconditionFailed, "reconfigure - %s", err.Error())
		}
	}
	return nil
}

func checkLocalFileExist(file string) (bool, error) {
	_, err := os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func checkLocalFileUpToDate(file, checksum string) error {
	content, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	actual := fmt.Sprintf("%x", sha256.Sum256(content))
	if actual != checksum {
		return fmt.Errorf("updated file is not up-to-date %s: expected %s, got %s", file, checksum, actual)
	}
	return nil
}
