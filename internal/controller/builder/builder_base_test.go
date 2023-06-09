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
	"k8s.io/apimachinery/pkg/types"

	appsv1 "k8s.io/api/apps/v1"
)

var _ = Describe("base builder", func() {
	It("should work well", func() {
		const (
			name                             = "foo"
			ns                               = "default"
			uid                              = types.UID("foo-bar")
			labelKey1, labelValue1           = "foo-1", "bar-1"
			labelKey2, labelValue2           = "foo-2", "bar-2"
			labelKey3, labelValue3           = "foo-3", "bar-3"
			annotationKey1, annotationValue1 = "foo-1", "bar-1"
			annotationKey2, annotationValue2 = "foo-2", "bar-2"
			annotationKey3, annotationValue3 = "foo-3", "bar-3"
		)
		labels := map[string]string{labelKey3: labelValue3}
		annotations := map[string]string{annotationKey3: annotationValue3}
		controllerRevision := "wer-23e23-sedfwe--34r23"
		finalizer := "foo-bar"
		owner := NewStatefulReplicaSetBuilder(ns, name).GetObject()
		owner.UID = "sdfwsedqw-swed-sdswe"
		ownerAPIVersion := "workloads.kubeblocks.io/v1alpha1"
		ownerKind := "StatefulReplicaSet"
		obj := NewConfigMapBuilder(ns, name).
			SetUID(uid).
			AddLabels(labelKey1, labelValue1, labelKey2, labelValue2).
			AddLabelsInMap(labels).
			AddAnnotations(annotationKey1, annotationValue1, annotationKey2, annotationValue2).
			AddAnnotationsInMap(annotations).
			AddControllerRevisionHashLabel(controllerRevision).
			AddFinalizers([]string{finalizer}).
			SetOwnerReferences(ownerAPIVersion, ownerKind, owner).
			GetObject()

		Expect(obj.Name).Should(Equal(name))
		Expect(obj.Namespace).Should(Equal(ns))
		Expect(obj.UID).Should(Equal(uid))
		Expect(len(obj.Labels)).Should(Equal(4))
		Expect(obj.Labels[labelKey1]).Should(Equal(labelValue1))
		Expect(obj.Labels[labelKey2]).Should(Equal(labelValue2))
		Expect(obj.Labels[labelKey3]).Should(Equal(labelValue3))
		Expect(obj.Labels[appsv1.ControllerRevisionHashLabelKey]).Should(Equal(controllerRevision))
		Expect(len(obj.Annotations)).Should(Equal(3))
		Expect(obj.Annotations[annotationKey1]).Should(Equal(annotationValue1))
		Expect(obj.Annotations[annotationKey2]).Should(Equal(annotationValue2))
		Expect(obj.Annotations[annotationKey3]).Should(Equal(annotationValue3))
		Expect(len(obj.Finalizers)).Should(Equal(1))
		Expect(obj.Finalizers[0]).Should(Equal(finalizer))
		Expect(len(obj.OwnerReferences)).Should(Equal(1))
		Expect(obj.OwnerReferences[0].APIVersion).Should(Equal(ownerAPIVersion))
		Expect(obj.OwnerReferences[0].Kind).Should(Equal(ownerKind))
		Expect(obj.OwnerReferences[0].Name).Should(Equal(owner.Name))
		Expect(obj.OwnerReferences[0].UID).Should(Equal(owner.UID))
	})
})
