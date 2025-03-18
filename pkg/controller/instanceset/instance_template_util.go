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

package instanceset

import (
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset/instancetemplate"
)

// TODO: remove these after extract the Schema of the API types from Kubeblocks into a separate Go package.

// InstanceSetExt serves as a Public Struct,
// used as the type for the input parameters of BuildInstanceTemplateExts.
type InstanceSetExt = instancetemplate.InstanceSetExt

// InstanceTemplateExt serves as a Public Struct,
// used as the type for the construction results returned by BuildInstanceTemplateExts.
type InstanceTemplateExt = instancetemplate.InstanceTemplateExt

// BuildInstanceName2TemplateMap serves as a Public API, through which users can obtain InstanceName2TemplateMap objects
// processed by the buildInstanceName2TemplateMap function.
var BuildInstanceName2TemplateMap = instancetemplate.BuildInstanceName2TemplateMap

// BuildInstanceTemplateExts serves as a Public API, through which users can obtain InstanceTemplateExt objects
// processed by the buildInstanceTemplateExts function.
// Its main purpose is to acquire the PodTemplate rendered by InstanceTemplate.
var BuildInstanceTemplateExts = instancetemplate.BuildInstanceTemplateExts

// BuildInstanceTemplates serves as a Public API, allowing users to construct InstanceTemplates.
// The constructed InstanceTemplates can be used as part of the input for BuildInstanceTemplateExts.
var BuildInstanceTemplates = instancetemplate.BuildInstanceTemplates
