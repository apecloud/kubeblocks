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
