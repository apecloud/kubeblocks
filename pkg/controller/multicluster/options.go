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

package multicluster

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func InControlContext() *ClientOption {
	return &ClientOption{
		control: true,
	}
}

func InDataContext() *ClientOption {
	return &ClientOption{}
}

func InDataContextUnspecified() *ClientOption {
	return &ClientOption{
		unspecified: true,
	}
}

func InUniversalContext() *ClientOption {
	return &ClientOption{
		universal: true,
	}
}

func Oneshot() *ClientOption {
	return &ClientOption{
		oneshot: true,
	}
}

func MultiCheck() *ClientOption {
	return &ClientOption{
		multiCheck: true,
	}
}

type ClientOption struct {
	control     bool // control plane
	universal   bool // both control and data planes
	unspecified bool // data planes, but don't know which ones exactly
	oneshot     bool
	multiCheck  bool // only support the Get operation
}

func (o *ClientOption) ApplyToGet(*client.GetOptions) {
}

func (o *ClientOption) ApplyToList(*client.ListOptions) {
}

func (o *ClientOption) ApplyToCreate(*client.CreateOptions) {
}

func (o *ClientOption) ApplyToDelete(*client.DeleteOptions) {
}

func (o *ClientOption) ApplyToUpdate(*client.UpdateOptions) {
}

func (o *ClientOption) ApplyToPatch(*client.PatchOptions) {
}

func (o *ClientOption) ApplyToDeleteAllOf(*client.DeleteAllOfOptions) {
}

func (o *ClientOption) ApplyToSubResourceGet(*client.SubResourceGetOptions) {
}

func (o *ClientOption) ApplyToSubResourceCreate(*client.SubResourceCreateOptions) {
}

func (o *ClientOption) ApplyToSubResourceUpdate(*client.SubResourceUpdateOptions) {
}

func (o *ClientOption) ApplyToSubResourcePatch(*client.SubResourcePatchOptions) {
}
