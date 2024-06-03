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
	"github.com/apecloud/kubeblocks/pkg/kb_agent/dcs"
	"github.com/apecloud/kubeblocks/pkg/kb_agent/handlers"
	"github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ = Describe("Handler", func() {
	var (
		handler *Handler
	)

	BeforeEach(func() {
		logger := ctrl.Log.WithName("exec handler")
		viperx.Set(constant.KBEnvPodName, "test-pod-0")
		handlerBase, err := handlers.NewHandlerBase(logger)
		Expect(err).NotTo(HaveOccurred())
		handlerBase.DBStartupReady = true
		handler = &Handler{
			HandlerBase:    *handlerBase,
			Executor:       &MockExecutor{},
			actionCommands: make(map[string][]string),
		}
	})

	Describe("NewHandler", func() {
		It("should initialize a new Handler", func() {
			properties := make(map[string]string)
			h, err := NewHandler(properties)
			Expect(err).NotTo(HaveOccurred())
			Expect(h).NotTo(BeNil())
		})
	})

	Describe("InitComponentDefinitionActions", func() {
		It("should initialize component definition actions", func() {
			viperx.Set(constant.KBEnvActionHandlers, `{
				"action1": {
					"Command": ["command1"]
				},
				"action2": {
					"Command": ["command2"]
				}
			}`)
			err := handler.InitComponentDefinitionActions()
			Expect(err).NotTo(HaveOccurred())
			Expect(handler.actionCommands).To(HaveLen(2))
			Expect(handler.actionCommands["action1"]).To(Equal([]string{"command1"}))
			Expect(handler.actionCommands["action2"]).To(Equal([]string{"command2"}))
		})

		It("should handle empty action commands", func() {
			viperx.Set(constant.KBEnvActionHandlers, `{}`)
			err := handler.InitComponentDefinitionActions()
			Expect(err).NotTo(HaveOccurred())
			Expect(handler.actionCommands).To(HaveLen(0))
		})

		It("should handle invalid JSON", func() {
			viperx.Set(constant.KBEnvActionHandlers, `invalid-json`)
			err := handler.InitComponentDefinitionActions()
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("JoinMember", func() {
		It("should execute member join command", func() {
			ctx := context.TODO()
			cluster := &dcs.Cluster{}
			memberName := "member1"
			handler.actionCommands[constant.MemberJoinAction] = []string{"command1"}
			handler.Executor.(*MockExecutor).MockExecCommand = func(ctx context.Context, command []string, envs []string) (string, error) {
				Expect(command).To(Equal([]string{"command1"}))
				return "output for test", nil
			}
			err := handler.JoinMember(ctx, cluster, memberName)
			Expect(err).NotTo(HaveOccurred())
			// Add your assertions here
		})

		It("should handle empty member join command", func() {
			ctx := context.TODO()
			cluster := &dcs.Cluster{}
			memberName := "member1"

			err := handler.JoinMember(ctx, cluster, memberName)
			Expect(err).NotTo(HaveOccurred())
			// Add your assertions here
		})
	})

	Describe("LeaveMember", func() {
		It("should execute member leave command", func() {
			ctx := context.TODO()
			cluster := &dcs.Cluster{}
			memberName := "member1"
			handler.actionCommands[constant.MemberLeaveAction] = []string{"command1"}
			handler.Executor.(*MockExecutor).MockExecCommand = func(ctx context.Context, command []string, envs []string) (string, error) {
				Expect(command).To(Equal([]string{"command1"}))
				return "output for test", nil
			}
			err := handler.LeaveMember(ctx, cluster, memberName)
			Expect(err).NotTo(HaveOccurred())
			// Add your assertions here
		})

		It("should handle empty member leave command", func() {
			ctx := context.TODO()
			cluster := &dcs.Cluster{}
			memberName := "member1"

			err := handler.LeaveMember(ctx, cluster, memberName)
			Expect(err).NotTo(HaveOccurred())
			// Add your assertions here
		})
	})

	Describe("MemberHealthCheck", func() {
		It("should execute member health check command", func() {
			ctx := context.TODO()
			cluster := &dcs.Cluster{}
			member := &dcs.Member{}
			handler.actionCommands[constant.CheckHealthyAction] = []string{"command1"}

			handler.Executor.(*MockExecutor).MockExecCommand = func(ctx context.Context, command []string, envs []string) (string, error) {
				Expect(command).To(Equal([]string{"command1"}))
				return "output for test", nil
			}
			err := handler.MemberHealthCheck(ctx, cluster, member)
			Expect(err).NotTo(HaveOccurred())
			// Add your assertions here
		})

		It("should handle empty member health check command", func() {
			ctx := context.TODO()
			cluster := &dcs.Cluster{}
			member := &dcs.Member{}

			err := handler.MemberHealthCheck(ctx, cluster, member)
			Expect(err).NotTo(HaveOccurred())
			// Add your assertions here
		})
	})

	Describe("Lock", func() {
		It("should execute lock command", func() {
			ctx := context.TODO()
			reason := "reason1"
			handler.actionCommands[constant.ReadonlyAction] = []string{"command1"}

			handler.Executor.(*MockExecutor).MockExecCommand = func(ctx context.Context, command []string, envs []string) (string, error) {
				Expect(command).To(Equal([]string{"command1"}))
				return "output for test", nil
			}
			err := handler.Lock(ctx, reason)
			Expect(err).NotTo(HaveOccurred())
			// Add your assertions here
		})

		It("should handle empty lock command", func() {
			ctx := context.TODO()
			reason := "reason1"

			err := handler.Lock(ctx, reason)
			Expect(err).NotTo(HaveOccurred())
			// Add your assertions here
		})
	})

	Describe("Unlock", func() {
		It("should execute unlock command", func() {
			ctx := context.TODO()
			reason := "reason1"
			handler.actionCommands[constant.ReadWriteAction] = []string{"command1"}

			handler.Executor.(*MockExecutor).MockExecCommand = func(ctx context.Context, command []string, envs []string) (string, error) {
				Expect(command).To(Equal([]string{"command1"}))
				return "output for test", nil
			}
			err := handler.Unlock(ctx, reason)
			Expect(err).NotTo(HaveOccurred())
			// Add your assertions here
		})

		It("should handle empty unlock command", func() {
			ctx := context.TODO()
			reason := "reason1"

			err := handler.Unlock(ctx, reason)
			Expect(err).NotTo(HaveOccurred())
			// Add your assertions here
		})
	})

	Describe("PostProvision", func() {
		It("should execute post-provision command", func() {
			ctx := context.TODO()
			cluster := &dcs.Cluster{}
			handler.actionCommands[constant.PostProvisionAction] = []string{"command1"}

			handler.Executor.(*MockExecutor).MockExecCommand = func(ctx context.Context, command []string, envs []string) (string, error) {
				Expect(command).To(Equal([]string{"command1"}))
				return "output for test", nil
			}
			err := handler.PostProvision(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			// Add your assertions here
		})

		It("should handle empty post-provision command", func() {
			ctx := context.TODO()
			cluster := &dcs.Cluster{}

			err := handler.PostProvision(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			// Add your assertions here
		})
	})

	Describe("PreTerminate", func() {
		It("should execute pre-terminate command", func() {
			ctx := context.TODO()
			cluster := &dcs.Cluster{}
			handler.actionCommands[constant.PreTerminateAction] = []string{"command1"}

			handler.Executor.(*MockExecutor).MockExecCommand = func(ctx context.Context, command []string, envs []string) (string, error) {
				Expect(command).To(Equal([]string{"command1"}))
				return "output for test", nil
			}
			err := handler.PreTerminate(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			// Add your assertions here
		})

		It("should handle empty pre-terminate command", func() {
			ctx := context.TODO()
			cluster := &dcs.Cluster{}

			err := handler.PreTerminate(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			// Add your assertions here
		})
	})
})

type MockExecutor struct {
	MockExecCommand func(ctx context.Context, command []string, envs []string) (string, error)
}

func (m *MockExecutor) ExecCommand(ctx context.Context, command []string, envs []string) (string, error) {
	if m.MockExecCommand != nil {
		return m.MockExecCommand(ctx, command, envs)
	}
	return "", nil
}
