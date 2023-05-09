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

package helm

import (
	"errors"
	"fmt"
	"strings"

	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
)

// Working with Errors in Go 1.13
// https://go.dev/blog/go1.13-errors
// Implementing errors should be more friendly to downstream handlers

var ErrReleaseNotDeployed = fmt.Errorf("release: not in deployed status")

func releaseNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, driver.ErrReleaseNotFound) ||
		strings.Contains(err.Error(), driver.ErrReleaseNotFound.Error())
}

func statusDeployed(rl *release.Release) bool {
	if rl == nil {
		return false
	}
	return release.StatusDeployed == rl.Info.Status
}
