/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package apps

import (
	"fmt"
)

const (
	CompDefinitionName = "test-component-definition"
	CompVersionName    = "test-component-version"

	AppName           = "app"
	AppNameSamePrefix = "app-same-prefix"

	ReleasePrefix        = "v0.0.1"
	ServiceVersionPrefix = "8.0.30"
)

func AppImage(app, tag string) string {
	return fmt.Sprintf("%s:%s", app, tag)
}
func CompDefName(r string) string {
	return fmt.Sprintf("%s-%s", CompDefinitionName, r)
}
func ReleaseID(r string) string {
	return fmt.Sprintf("%s-%s", ReleasePrefix, r)
}
func ServiceVersion(r string) string {
	if len(r) == 0 {
		return ServiceVersionPrefix
	}
	return fmt.Sprintf("%s-%s", ServiceVersionPrefix, r)
}
