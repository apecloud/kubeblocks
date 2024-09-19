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
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	"github.com/apecloud/kubeblocks/pkg/kbagent/util"
)

const (
	defaultBufferSize = 4096
)

func gather[T interface{}](ch chan T) *T {
	select {
	case v := <-ch:
		return &v
	default:
		return nil
	}
}

func runCommand(ctx context.Context, action *proto.ExecAction, parameters map[string]string, timeout *int32) ([]byte, error) {
	stdoutChan, stderrChan, errChan, err := runCommandNonBlocking(ctx, action, parameters, timeout)
	if err != nil {
		return nil, err
	}
	err = <-errChan
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderrMsg := string(<-stderrChan)
			if len(stderrMsg) > 0 {
				err = errors.Wrapf(proto.ErrFailed, "exec exit %d and stderr: %s", exitErr.ExitCode(), stderrMsg)
			} else {
				err = errors.Wrapf(proto.ErrFailed, "exec exit %d but stderr is blank", exitErr.ExitCode())
			}
		}
		return nil, err
	}
	return <-stdoutChan, nil
}

func runCommandNonBlocking(ctx context.Context, action *proto.ExecAction, parameters map[string]string, timeout *int32) (chan []byte, chan []byte, chan error, error) {
	stdoutBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
	stderrBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
	execErrorChan, err := runCommandX(ctx, action, parameters, timeout, nil, stdoutBuf, stderrBuf)
	if err != nil {
		return nil, nil, nil, err
	}

	stdoutChan := make(chan []byte, defaultBufferSize)
	stderrChan := make(chan []byte, defaultBufferSize)
	errChan := make(chan error, 1)
	go func() {
		defer close(errChan)
		defer close(stderrChan)
		defer close(stdoutChan)

		// wait for the command to finish
		execErr, ok := <-execErrorChan
		if !ok {
			execErr = errors.New("runtime error: error chan closed unexpectedly")
		}

		stdoutChan <- stdoutBuf.Bytes()
		stderrChan <- stderrBuf.Bytes()
		errChan <- execErr
	}()
	return stdoutChan, stderrChan, errChan, nil
}

func runCommandX(ctx context.Context, action *proto.ExecAction, parameters map[string]string, timeout *int32,
	stdinReader io.Reader, stdoutWriter, stderrWriter io.Writer) (chan error, error) {
	var timeoutCancel context.CancelFunc
	if timeout != nil && *timeout > 0 {
		ctx, timeoutCancel = context.WithTimeout(ctx, time.Duration(*timeout)*time.Second)
	}
	cancelTimeout := func() {
		if timeoutCancel != nil {
			timeoutCancel()
		}
	}

	mergedArgs := func() []string {
		args := make([]string, 0)
		if len(action.Commands) > 1 {
			args = append(args, action.Commands[1:]...)
		}
		args = append(args, action.Args...)
		return args
	}()

	mergedEnv := func() []string {
		// order: parameters (action specific variables) | var | action env
		filterDuplicates := func(osEnv []string, filter func(string) bool) []string {
			unionEnv := make([]string, 0, len(osEnv))
			for _, e := range osEnv {
				if filter(e) {
					unionEnv = append(unionEnv, e)
				}
			}
			return unionEnv
		}
		env := append(util.EnvM2L(parameters), filterDuplicates(os.Environ(), func(env string) bool {
			kv := strings.Split(env, "=")
			_, ok := parameters[kv[0]]
			return !ok
		})...)
		return env
	}()

	cmd := exec.CommandContext(ctx, action.Commands[0], mergedArgs...)
	if len(mergedEnv) > 0 {
		cmd.Env = mergedEnv
	}

	cmd.Stdin = stdinReader
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter

	errChan := make(chan error, 1)
	go func() {
		defer cancelTimeout()
		defer close(errChan)

		if err := cmd.Start(); err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				errChan <- proto.ErrTimedOut
			} else {
				errChan <- errors.Wrapf(proto.ErrFailed, "failed to start command: %v", err)
			}
			return
		}

		execErr := cmd.Wait()
		if execErr != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				execErr = proto.ErrTimedOut
			}
		}
		errChan <- execErr
	}()
	return errChan, nil
}
