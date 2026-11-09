/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package types

import "fmt"

// InvalidRestorePolicyError identifies a rejected legacy policy setting.
type InvalidRestorePolicyError struct {
	Path   string
	Policy string
}

func (e *InvalidRestorePolicyError) Error() string {
	if e.Policy == "Serial" {
		return fmt.Sprintf("%s: Serial is no longer supported; omit this setting or use Parallel", e.Path)
	}
	return fmt.Sprintf("%s: invalid volume restore policy %q; omit this setting or use Parallel", e.Path, e.Policy)
}

// ValidateVolumeRestorePolicy accepts omitted and legacy Parallel settings.
func ValidateVolumeRestorePolicy(policy, path string) error {
	if policy == "" || policy == "Parallel" {
		return nil
	}
	return &InvalidRestorePolicyError{Path: path, Policy: policy}
}
