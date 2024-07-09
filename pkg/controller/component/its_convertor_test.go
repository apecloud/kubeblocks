/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package component

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
)

var _ = Describe("Test InstanceSet Convertor", func() {
	Context("InstanceSet convertors", func() {
		var (
			synComp *SynthesizedComponent
		)
		command := []string{"foo", "bar"}
		args := []string{"zoo", "boo"}

		BeforeEach(func() {
			synComp = &SynthesizedComponent{
				LifecycleActions: &appsv1alpha1.ComponentLifecycleActions{
					RoleProbe: &appsv1alpha1.RoleProbe{
						LifecycleActionHandler: appsv1alpha1.LifecycleActionHandler{
							CustomHandler: &appsv1alpha1.Action{
								Exec: &appsv1alpha1.ExecAction{
									Command: command,
									Args:    args,
								},
							},
						},
					},
				},
			}
		})
		It("convert", func() {
			convertor := &itsRoleProbeConvertor{}
			res, err := convertor.convert(synComp)
			Expect(err).Should(Succeed())
			probe := res.(*workloads.RoleProbe)
			Expect(probe.CustomHandler[0].Command).Should(BeEquivalentTo(command))
			Expect(probe.CustomHandler[0].Args).Should(BeEquivalentTo(args))
		})
	})
})
