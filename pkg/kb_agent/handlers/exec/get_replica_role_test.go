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

package exec

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/handlers"
	"github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("Handler", func() {
	var (
		handler *Handler
		ctx     context.Context
	)

	BeforeEach(func() {
		logger := ctrl.Log.WithName("exec handler")
		viperx.Set(constant.KBEnvPodName, "test-pod-0")
		handlerBase, err := handlers.NewHandlerBase(logger)
		Expect(err).NotTo(HaveOccurred())
		ctx = context.Background()
		handlerBase.DBStartupReady = true
		handler = &Handler{
			HandlerBase:    *handlerBase,
			Executor:       &MockExecutor{},
			actionCommands: make(map[string][]string),
		}
	})

	Describe("GetReplicaRole", func() {
		It("should execute role probe command successfully", func() {
			expectedRole := "primary"
			roleProbeCmd := []string{"sh", "-c", "your-role-probe-command"}

			handler.actionCommands[constant.RoleProbeAction] = roleProbeCmd

			mockExecCommand := func(ctx context.Context, cmd []string, env []string) (string, error) {
				Expect(cmd).To(Equal(roleProbeCmd))
				return expectedRole, nil
			}
			handler.Executor.(*MockExecutor).MockExecCommand = mockExecCommand

			role, err := handler.GetReplicaRole(ctx)
			Expect(err).To(BeNil())
			Expect(role).To(Equal(expectedRole))
		})

		It("should execute role probe command successfully with shell completion", func() {
			expectedRole := "primary"
			roleProbeCmd := []string{"your-role-probe-command"}
			roleProbeCmdCompletion := []string{"sh", "-c", "your-role-probe-command"}

			handler.actionCommands[constant.RoleProbeAction] = roleProbeCmd

			mockExecCommand := func(ctx context.Context, cmd []string, env []string) (string, error) {
				Expect(cmd).To(Equal(roleProbeCmdCompletion))
				return expectedRole, nil
			}
			handler.Executor.(*MockExecutor).MockExecCommand = mockExecCommand

			role, err := handler.GetReplicaRole(ctx)
			Expect(err).To(BeNil())
			Expect(role).To(Equal(expectedRole))
		})

		It("should return an error when role probe command is empty", func() {
			handler.actionCommands[constant.RoleProbeAction] = nil

			role, err := handler.GetReplicaRole(ctx)
			Expect(err).To(MatchError("role probe commands is empty!"))
			Expect(role).To(BeEmpty())
		})
	})
})
