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

package plugin

import (
	"fmt"
	"os"
	"runtime"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
)

// GetMatchingPlatform finds the platform spec in the specified plugin that
// matches the os/arch of the current machine (can be overridden via KREW_OS
// and/or KREW_ARCH).
func GetMatchingPlatform(platforms []Platform) (Platform, bool, error) {
	return matchPlatform(platforms, OSArch())
}

// matchPlatform returns the first matching platform to given os/arch.
func matchPlatform(platforms []Platform, env OSArchPair) (Platform, bool, error) {
	envLabels := labels.Set{
		"os":   env.OS,
		"arch": env.Arch,
	}
	klog.V(2).Infof("Matching platform for labels(%v)", envLabels)

	for i, platform := range platforms {
		sel, err := metav1.LabelSelectorAsSelector(platform.Selector)
		if err != nil {
			return Platform{}, false, errors.Wrap(err, "failed to compile label selector")
		}
		if sel.Matches(envLabels) {
			klog.V(2).Infof("Found matching platform with index (%d)", i)
			return platform, true, nil
		}
	}
	return Platform{}, false, nil
}

// OSArchPair is wrapper around operating system and architecture
type OSArchPair struct {
	OS, Arch string
}

// String converts environment into a string
func (p OSArchPair) String() string {
	return fmt.Sprintf("%s/%s", p.OS, p.Arch)
}

// OSArch returns the OS/arch combination to be used on the current system. It
// can be overridden by setting KREW_OS and/or KREW_ARCH environment variables.
func OSArch() OSArchPair {
	return OSArchPair{
		OS:   getEnvOrDefault("KBLCI_OS", runtime.GOOS),
		Arch: getEnvOrDefault("KBCLI_ARCH", runtime.GOARCH),
	}
}

func getEnvOrDefault(env, absent string) string {
	v := os.Getenv(env)
	if v != "" {
		return v
	}
	return absent
}
