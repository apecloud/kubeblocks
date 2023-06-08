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

package consensusset

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/controller/model"
)

func init() {
	model.AddScheme(workloads.AddToScheme)
}

var _ = Describe("enqueue ancestor", func() {
	scheme := model.GetScheme()
	var handler *EnqueueRequestForAncestor
	BeforeEach(func() {
		handler = &EnqueueRequestForAncestor{
			OwnerType: &workloads.ConsensusSet{},
			UpToLevel: 2,
			InTypes:   []runtime.Object{&appsv1.StatefulSet{}},
		}
	})

	Context("parseOwnerTypeGroupKind", func() {
		It("should work well", func() {
			Expect(handler.parseOwnerTypeGroupKind(scheme)).Should(Succeed())
			Expect(handler.groupKind.Group).Should(Equal(" workloads.kubeblocks.io"))
			Expect(handler.groupKind.Kind).Should(Equal("ConsensusSet"))
		})
	})

	Context("parseInTypesGroupKind", func() {
		It("should work well", func() {
			Expect(handler.parseInTypesGroupKind(scheme)).Should(Succeed())
			Expect(len(handler.ancestorGroupKinds)).Should(Equal(1))
			Expect(handler.ancestorGroupKinds[0].Group).Should(Equal("apps"))
			Expect(handler.ancestorGroupKinds[0].Kind).Should(Equal("StatefulSet"))
		})
	})
})
