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
