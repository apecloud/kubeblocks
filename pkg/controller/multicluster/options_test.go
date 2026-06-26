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

package multicluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestInControlContext(t *testing.T) {
	opt := InControlContext()
	assert.NotNil(t, opt)
	assert.True(t, opt.control)
	assert.False(t, opt.universal)
	assert.False(t, opt.unspecified)
	assert.False(t, opt.oneshot)
	assert.False(t, opt.multiCheck)
}

func TestInDataContext(t *testing.T) {
	opt := InDataContext()
	assert.NotNil(t, opt)
	assert.False(t, opt.control)
	assert.False(t, opt.universal)
	assert.False(t, opt.unspecified)
	assert.False(t, opt.oneshot)
	assert.False(t, opt.multiCheck)
}

func TestInDataContextUnspecified(t *testing.T) {
	opt := InDataContextUnspecified()
	assert.NotNil(t, opt)
	assert.False(t, opt.control)
	assert.False(t, opt.universal)
	assert.True(t, opt.unspecified)
	assert.False(t, opt.oneshot)
	assert.False(t, opt.multiCheck)
}

func TestInUniversalContext(t *testing.T) {
	opt := InUniversalContext()
	assert.NotNil(t, opt)
	assert.False(t, opt.control)
	assert.True(t, opt.universal)
	assert.False(t, opt.unspecified)
	assert.False(t, opt.oneshot)
	assert.False(t, opt.multiCheck)
}

func TestOneshot(t *testing.T) {
	opt := Oneshot()
	assert.NotNil(t, opt)
	assert.False(t, opt.control)
	assert.False(t, opt.universal)
	assert.False(t, opt.unspecified)
	assert.True(t, opt.oneshot)
	assert.False(t, opt.multiCheck)
}

func TestMultiCheck(t *testing.T) {
	opt := MultiCheck()
	assert.NotNil(t, opt)
	assert.False(t, opt.control)
	assert.False(t, opt.universal)
	assert.False(t, opt.unspecified)
	assert.False(t, opt.oneshot)
	assert.True(t, opt.multiCheck)
}

func TestClientOption_ApplyToMethods(t *testing.T) {
	opt := InControlContext()
	// all ApplyTo* methods should be callable and not panic
	assert.NotPanics(t, func() { opt.ApplyToGet(&client.GetOptions{}) })
	assert.NotPanics(t, func() { opt.ApplyToList(&client.ListOptions{}) })
	assert.NotPanics(t, func() { opt.ApplyToCreate(&client.CreateOptions{}) })
	assert.NotPanics(t, func() { opt.ApplyToDelete(&client.DeleteOptions{}) })
	assert.NotPanics(t, func() { opt.ApplyToUpdate(&client.UpdateOptions{}) })
	assert.NotPanics(t, func() { opt.ApplyToPatch(&client.PatchOptions{}) })
	assert.NotPanics(t, func() { opt.ApplyToDeleteAllOf(&client.DeleteAllOfOptions{}) })
	assert.NotPanics(t, func() { opt.ApplyToSubResourceGet(&client.SubResourceGetOptions{}) })
	assert.NotPanics(t, func() { opt.ApplyToSubResourceCreate(&client.SubResourceCreateOptions{}) })
	assert.NotPanics(t, func() { opt.ApplyToSubResourceUpdate(&client.SubResourceUpdateOptions{}) })
	assert.NotPanics(t, func() { opt.ApplyToSubResourcePatch(&client.SubResourcePatchOptions{}) })
}

func TestClientOption_UniqueFlags(t *testing.T) {
	// verify that each option constructor sets exactly one flag
	tests := []struct {
		name     string
		opt      *ClientOption
		checkFn  func(*ClientOption) bool
	}{
		{"control", InControlContext(), func(o *ClientOption) bool { return o.control }},
		{"unspecified", InDataContextUnspecified(), func(o *ClientOption) bool { return o.unspecified }},
		{"universal", InUniversalContext(), func(o *ClientOption) bool { return o.universal }},
		{"oneshot", Oneshot(), func(o *ClientOption) bool { return o.oneshot }},
		{"multiCheck", MultiCheck(), func(o *ClientOption) bool { return o.multiCheck }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.True(t, tt.checkFn(tt.opt), "expected %s flag to be true", tt.name)
		})
	}
}
