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
