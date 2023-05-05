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

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("transform utils test", func() {
	Context("test IsObjectUpdating", func() {
		It("should return false if generation equals", func() {
			object := &apps.StatefulSet{}
			object.Generation = 1
			object.Status.ObservedGeneration = 1
			Expect(IsObjectUpdating(object)).Should(BeFalse())
		})
		It("should return true if generation doesn't equal", func() {
			object := &apps.StatefulSet{}
			object.Generation = 2
			object.Status.ObservedGeneration = 1
			Expect(IsObjectUpdating(object)).Should(BeTrue())
		})
		It("should return false if fields not exist", func() {
			object := &corev1.Secret{}
			Expect(IsObjectUpdating(object)).Should(BeFalse())
		})
	})
})
