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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SchemeGroupVersion is group version used to register these objects.
var SchemeGroupVersion = GroupVersion

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}
