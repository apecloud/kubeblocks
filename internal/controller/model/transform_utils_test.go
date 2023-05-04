/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
