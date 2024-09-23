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

package service

import (
	"bytes"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

var _ = Describe("command", func() {
	wait := func(errChan chan error) {
		Expect(errChan).ShouldNot(BeNil())
		err, ok := <-errChan
		Expect(ok).Should(BeTrue())
		Expect(err).Should(BeNil())
	}

	waitError := func(errChan chan error) error {
		Expect(errChan).ShouldNot(BeNil())
		err, ok := <-errChan
		Expect(ok).Should(BeTrue())
		return err
	}

	PContext("runCommandX", func() {
		It("simple", func() {
			action := &proto.ExecAction{
				Commands: []string{"/bin/bash", "-c", "echo -n simple"},
			}
			execErrorChan, err := runCommandX(ctx, action, nil, nil, nil, nil, nil)
			Expect(err).Should(BeNil())

			wait(execErrorChan)
		})

		It("stdout", func() {
			action := &proto.ExecAction{
				Commands: []string{"/bin/bash", "-c", "echo -n stdout"},
			}
			stdoutBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
			execErrorChan, err := runCommandX(ctx, action, nil, nil, nil, stdoutBuf, nil)
			Expect(err).Should(BeNil())

			wait(execErrorChan)
			Expect(stdoutBuf.String()).Should(Equal("stdout"))
		})

		It("stderr", func() {
			action := &proto.ExecAction{
				Commands: []string{"/bin/bash", "-c", "echo -n stderr >&2"},
			}
			stdoutBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
			stderrBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
			execErrorChan, err := runCommandX(ctx, action, nil, nil, nil, stdoutBuf, stderrBuf)
			Expect(err).Should(BeNil())

			wait(execErrorChan)
			Expect(stdoutBuf.String()).Should(HaveLen(0))
			Expect(stderrBuf.String()).Should(Equal("stderr"))
		})

		It("stdin", func() {
			action := &proto.ExecAction{
				Commands: []string{"/bin/bash", "-c", "xargs echo -n"},
			}
			stdinBuf := bytes.NewBuffer([]byte{'s', 't', 'd', 'i', 'n'})
			stdoutBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
			execErrorChan, err := runCommandX(ctx, action, nil, nil, stdinBuf, stdoutBuf, nil)
			Expect(err).Should(BeNil())

			wait(execErrorChan)
			Expect(stdoutBuf.String()).Should(Equal("stdin"))
		})

		It("parameters", func() {
			action := &proto.ExecAction{
				Commands: []string{"/bin/bash", "-c", "echo -n $PARAM"},
			}
			parameters := map[string]string{
				"PARAM":   "parameters",
				"useless": "useless",
			}
			stdoutBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
			execErrorChan, err := runCommandX(ctx, action, parameters, nil, nil, stdoutBuf, nil)
			Expect(err).Should(BeNil())

			wait(execErrorChan)
			Expect(stdoutBuf.String()).Should(Equal("parameters"))
		})

		It("timeout", func() {
			action := &proto.ExecAction{
				Commands: []string{"/bin/bash", "-c", "sleep 60"},
			}
			timeout := int32(1)
			execErrorChan, err := runCommandX(ctx, action, nil, &timeout, nil, nil, nil)
			Expect(err).Should(BeNil())

			err = waitError(execErrorChan)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, proto.ErrTimedOut)).Should(BeTrue())
		})

		It("timeout and stdout", func() {
			action := &proto.ExecAction{
				Commands: []string{"/bin/bash", "-c", "sleep 60 && echo -n timeout"},
			}
			stdoutBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
			timeout := int32(1)
			execErrorChan, err := runCommandX(ctx, action, nil, &timeout, nil, stdoutBuf, nil)
			Expect(err).Should(BeNil())

			err = waitError(execErrorChan)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, proto.ErrTimedOut)).Should(BeTrue())
			Expect(stdoutBuf.String()).Should(HaveLen(0))
		})

		It("stdout and timeout", func() {
			action := &proto.ExecAction{
				Commands: []string{"/bin/bash", "-c", "echo -n timeout && sleep 60"},
			}
			stdoutBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
			timeout := int32(1)
			execErrorChan, err := runCommandX(ctx, action, nil, &timeout, nil, stdoutBuf, nil)
			Expect(err).Should(BeNil())

			err = waitError(execErrorChan)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, proto.ErrTimedOut)).Should(BeTrue())
			Expect(stdoutBuf.String()).Should(Equal("timeout"))
		})

		It("fail", func() {
			action := &proto.ExecAction{
				Commands: []string{"/bin/bash", "-c", "command-not-exist"},
			}
			execErrorChan, err := runCommandX(ctx, action, nil, nil, nil, nil, nil)
			Expect(err).Should(BeNil())

			err = waitError(execErrorChan)
			Expect(err).ShouldNot(BeNil())
			var exitErr *exec.ExitError
			Expect(errors.As(err, &exitErr)).Should(BeTrue())
		})

		It("fail with stderr writer", func() {
			action := &proto.ExecAction{
				Commands: []string{"/bin/bash", "-c", "command-not-found"},
			}
			stderrBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
			execErrorChan, err := runCommandX(ctx, action, nil, nil, nil, nil, stderrBuf)
			Expect(err).Should(BeNil())

			err = waitError(execErrorChan)
			Expect(err).ShouldNot(BeNil())
			var exitErr *exec.ExitError
			Expect(errors.As(err, &exitErr)).Should(BeTrue())
			Expect(stderrBuf.String()).Should(ContainSubstring("command not found"))
		})
	})

	PContext("runCommandNonBlocking", func() {
		It("ok", func() {
			action := &proto.ExecAction{
				Commands: []string{"/bin/bash", "-c", "echo -n ok"},
			}
			resultChan, err := runCommandNonBlocking(ctx, action, nil, nil)
			Expect(err).Should(BeNil())

			result := <-resultChan
			Expect(result.err).Should(BeNil())
			Expect(result.stdout.Bytes()).Should(Equal([]byte("ok")))
			Expect(result.stderr.Bytes()).Should(HaveLen(0))
		})

		It("parameters", func() {
			action := &proto.ExecAction{
				Commands: []string{"/bin/bash", "-c", "echo -n $PARAM"},
			}
			parameters := map[string]string{
				"PARAM": "parameters",
			}
			resultChan, err := runCommandNonBlocking(ctx, action, parameters, nil)
			Expect(err).Should(BeNil())

			result := <-resultChan
			Expect(result.err).Should(BeNil())
			Expect(result.stdout.Bytes()).Should(Equal([]byte("parameters")))
			Expect(result.stderr.Bytes()).Should(HaveLen(0))
		})

		It("fail", func() {
			action := &proto.ExecAction{
				Commands: []string{"/bin/bash", "-c", "command-not-found"},
			}
			resultChan, err := runCommandNonBlocking(ctx, action, nil, nil)
			Expect(err).Should(BeNil())

			result := <-resultChan
			Expect(result.err).ShouldNot(BeNil())
			var exitErr *exec.ExitError
			Expect(errors.As(result.err, &exitErr)).Should(BeTrue())
			Expect(result.stdout.Bytes()).Should(HaveLen(0))
			Expect(result.stderr.Bytes()).Should(ContainSubstring("command not found"))
		})

		It("timeout", func() {
			action := &proto.ExecAction{
				Commands: []string{"/bin/bash", "-c", "sleep 60"},
			}
			timeout := int32(1)
			resultChan, err := runCommandNonBlocking(ctx, action, nil, &timeout)
			Expect(err).Should(BeNil())

			result := <-resultChan
			Expect(result.err).ShouldNot(BeNil())
			Expect(errors.Is(result.err, proto.ErrTimedOut)).Should(BeTrue())
			Expect(result.stdout.Bytes()).Should(HaveLen(0))
			Expect(result.stderr.Bytes()).Should(HaveLen(0))
		})
	})

	PContext("runCommand", func() {
		It("ok", func() {
			action := &proto.ExecAction{
				Commands: []string{"/bin/bash", "-c", "echo -n ok"},
			}
			output, err := runCommand(ctx, action, nil, nil)
			Expect(err).Should(BeNil())
			Expect(output).Should(Equal([]byte("ok")))
		})

		It("parameters", func() {
			action := &proto.ExecAction{
				Commands: []string{"/bin/bash", "-c", "echo -n $PARAM"},
			}
			parameters := map[string]string{
				"PARAM": "parameters",
			}
			output, err := runCommand(ctx, action, parameters, nil)
			Expect(err).Should(BeNil())
			Expect(output).Should(Equal([]byte("parameters")))
		})

		It("fail", func() {
			action := &proto.ExecAction{
				Commands: []string{"/bin/bash", "-c", "command-not-found"},
			}
			output, err := runCommand(ctx, action, nil, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, proto.ErrFailed)).Should(BeTrue())
			Expect(err.Error()).Should(ContainSubstring("command not found"))
			Expect(output).Should(BeNil())
		})

		It("timeout", func() {
			action := &proto.ExecAction{
				Commands: []string{"/bin/bash", "-c", "sleep 60"},
			}
			timeout := int32(1)
			output, err := runCommand(ctx, action, nil, &timeout)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, proto.ErrTimedOut)).Should(BeTrue())
			Expect(output).Should(BeNil())
		})
	})
})
