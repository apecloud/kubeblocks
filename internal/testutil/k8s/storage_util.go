/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package testutil

import (
	"github.com/onsi/gomega"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/kubectl/pkg/util/storage"

	"github.com/apecloud/kubeblocks/internal/testutil"
)

func GetDefaultStorageClass(testCtx *testutil.TestContext) *storagev1.StorageClass {
	scList := &storagev1.StorageClassList{}
	gomega.Expect(testCtx.Cli.List(testCtx.Ctx, scList)).Should(gomega.Succeed())
	if len(scList.Items) == 0 {
		return nil
	}

	for _, sc := range scList.Items {
		annot := sc.Annotations
		if annot == nil {
			continue
		}
		if isDefaultStorageClassAnnotation(&sc) {
			return &sc
		}
	}
	return nil
}

func isDefaultStorageClassAnnotation(storageClass *storagev1.StorageClass) bool {
	return storageClass.Annotations != nil && storageClass.Annotations[storage.IsDefaultStorageClassAnnotation] == "true"
}
