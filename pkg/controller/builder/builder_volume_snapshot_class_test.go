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

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
)

var _ = Describe("volume snapshot class builder", func() {
	It("should work well", func() {
		const (
			name = "foo"
			ns   = "default"
		)

		driver := "openebs-snapshot"
		policy := snapshotv1.VolumeSnapshotContentRetain
		vsc := NewVolumeSnapshotClassBuilder(ns, name).
			SetDriver(driver).
			SetDeletionPolicy(policy).
			GetObject()

		Expect(vsc.Name).Should(Equal(name))
		Expect(vsc.Namespace).Should(Equal(ns))
		Expect(vsc.Driver).Should(Equal(driver))
		Expect(vsc.DeletionPolicy).Should(Equal(policy))
	})
})
