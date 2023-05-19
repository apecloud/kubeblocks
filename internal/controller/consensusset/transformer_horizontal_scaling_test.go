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

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("horizontal scaling transformer test", func() {
	Context("test buildFinalActionList", func() {
		It("works fine", func() {
			actionNameList := []string{
				// 3 -> 7
				"ac-mysql-3-3-member-join",
				"ac-mysql-3-3-sync-log",
				"ac-mysql-3-3-promote",
				"ac-mysql-3-4-member-join", // active action
				"ac-mysql-3-4-sync-log",
				"ac-mysql-3-5-member-join",
				"ac-mysql-3-6-member-join",
				// spec update to 7 -> 1 when 'ac-mysql-3-4-member-join' is active
				"ac-mysql-4-4-switchover",
				"ac-mysql-4-4-member-leave",
				"ac-mysql-4-3-switchover",
				"ac-mysql-4-3-member-leave",
				"ac-mysql-4-2-member-leave",
				"ac-mysql-4-1-member-leave",
				// 1->3
				"ac-mysql-5-4-switchover",
				"ac-mysql-5-4-member-leave",
				"ac-mysql-5-3-member-leave",
			}
			actionTypeLabelList := []string{
				// 3 -> 7
				"member-join",
				"sync-log",
				"promote",
				"member-join", // active action
				"sync-log",
				"member-join",
				"member-join",
				// spec update to 7 -> 1 when 'ac-mysql-3-4-member-join' is active
				"switchover",
				"member-leave",
				"switchover",
				"member-leave",
				"member-leave",
				"member-leave",
				// 1->3
				"switchover",
				"member-leave",
				"member-leave",
			}
			expectedNameList := []string{
				"ac-mysql-3-3-member-join",
				"ac-mysql-3-3-sync-log",
				"ac-mysql-3-3-promote",
				"ac-mysql-3-4-member-join",
				"ac-mysql-3-4-sync-log",
				"ac-mysql-4-4-switchover",
				"ac-mysql-4-4-member-leave",
				"ac-mysql-4-3-switchover",
				"ac-mysql-4-3-member-leave",
			}
			var allActionList []*batchv1.Job
			for i, name := range actionNameList {
				allActionList = append(allActionList,
					&batchv1.Job{
						ObjectMeta: metav1.ObjectMeta{
							Name: name,
							Labels: map[string]string{
								jobTypeLabel: actionTypeLabelList[i],
							},
						}})
			}
			for i := 0; i < 4; i++ {
				suspend := false
				allActionList[i].Spec.Suspend = &suspend
				allActionList[i].Status.Succeeded = 1
			}
			for i := 4; i < len(allActionList); i++ {
				suspend := true
				allActionList[i].Spec.Suspend = &suspend
			}
			finalActionList := buildFinalActionList(allActionList, 4, 3)
			var finalNameList []string
			for _, action := range finalActionList {
				finalNameList = append(finalNameList, action.Name)
			}
			Expect(finalNameList).Should(Equal(expectedNameList))
		})
	})
})