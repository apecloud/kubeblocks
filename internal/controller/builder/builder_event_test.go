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

package builder

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("event builder", func() {
	It("should work well", func() {
		const (
			name = "foo"
			ns   = "default"
			uid  = types.UID("bar")
		)
		objectRef := corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Pod",
			Namespace:  ns,
			Name:       name,
			UID:        uid,
		}
		message := "foo-bar"
		reason := "reason"
		tp := corev1.EventTypeNormal
		event := NewEventBuilder(ns, "foo").
			SetInvolvedObject(objectRef).
			SetMessage(message).
			SetReason(reason).
			SetType(tp).
			GetObject()

		Expect(event.Name).Should(Equal(name))
		Expect(event.Namespace).Should(Equal(ns))
		Expect(event.InvolvedObject).Should(Equal(objectRef))
		Expect(event.Message).Should(Equal(message))
		Expect(event.Reason).Should(Equal(reason))
		Expect(event.Type).Should(Equal(tp))
	})
})
