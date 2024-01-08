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

package model

type GraphOptions struct {
	replaceIfExisting     bool
	haveDifferentTypeWith bool
	clientOpt             any
}

type GraphOption interface {
	ApplyTo(*GraphOptions)
}

// ReplaceIfExistingOption tells the GraphWriter methods to replace Obj and OriObj with the given ones if already existing.
// used in Action methods: Create, Update, Patch, Status, Noop and Delete
type ReplaceIfExistingOption struct{}

var _ GraphOption = &ReplaceIfExistingOption{}

func (o *ReplaceIfExistingOption) ApplyTo(opts *GraphOptions) {
	opts.replaceIfExisting = true
}

// HaveDifferentTypeWithOption is used in FindAll method to find all objects have different type with the given one.
type HaveDifferentTypeWithOption struct{}

var _ GraphOption = &HaveDifferentTypeWithOption{}

func (o *HaveDifferentTypeWithOption) ApplyTo(opts *GraphOptions) {
	opts.haveDifferentTypeWith = true
}

type clientOption struct {
	opt any
}

var _ GraphOption = &clientOption{}

func (o *clientOption) ApplyTo(opts *GraphOptions) {
	opts.clientOpt = o.opt
}

func WithClientOption(opt any) GraphOption {
	return &clientOption{
		opt: opt,
	}
}
