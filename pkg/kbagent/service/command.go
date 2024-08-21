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
	"sync"
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
	stdoutChan, _, errChan, err := runCommandNonBlocking(ctx, action, parameters, timeout)
	if err != nil {
		return nil, err
	}
	err = <-errChan
	if err != nil {
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
	errChan := make(chan error)
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
	mergedArgs := func() []string {
		args := make([]string, 0)
		if len(action.Commands) > 1 {
			args = append(args, action.Commands[1:]...)
		}
		args = append(args, action.Args...)
		return args
	}()

	mergedEnv := func() []string {
		// env order: parameters (action specific variables) | os env (defined by vars) | user-defined env in action
		env := util.EnvM2L(parameters)
		if len(env) > 0 || len(action.Env) > 0 {
			env = append(env, os.Environ()...)
		}
		if len(action.Env) > 0 {
			env = append(env, action.Env...)
		}
		return env
	}()

	cmd := exec.CommandContext(ctx, action.Commands[0], mergedArgs...)
	if len(mergedEnv) > 0 {
		cmd.Env = mergedEnv
	}

	var (
		stdin          io.WriteCloser
		stdout, stderr io.ReadCloser
	)
	if stdinReader != nil {
		var stdinErr error
		stdin, stdinErr = cmd.StdinPipe()
		if stdinErr != nil {
			return nil, errors.Wrapf(ErrInternalError, "failed to create stdin pipe: %v", stdinErr)
		}
	}
	if stdoutWriter != nil {
		var stdoutErr error
		stdout, stdoutErr = cmd.StdoutPipe()
		if stdoutErr != nil {
			return nil, errors.Wrapf(ErrInternalError, "failed to create stdout pipe: %v", stdoutErr)
		}
	}
	if stderrWriter != nil {
		var stderrErr error
		stderr, stderrErr = cmd.StderrPipe()
		if stderrErr != nil {
			return nil, errors.Wrapf(ErrInternalError, "failed to create stderr pipe: %v", stderrErr)
		}
	}

	errChan := make(chan error)
	go func() {
		defer close(errChan)
		if timeout != nil && *timeout > 0 {
			timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(*timeout)*time.Second)
			defer cancel()
			ctx = timeoutCtx
		}
		if err := cmd.Start(); err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				errChan <- ErrTimeout
			} else {
				errChan <- errors.Wrapf(ErrFailed, "failed to start command: %v", err)
			}
			return
		}

		var wg sync.WaitGroup
		wg.Add(3)

		go func() {
			defer wg.Done()
			if stdinReader != nil {
				defer stdin.Close()
				_, copyErr := io.Copy(stdin, stdinReader)
				if copyErr != nil {
					errChan <- errors.Wrapf(ErrFailed, "failed to copy from input reader to stdin: %v", copyErr)
				}
			}
		}()
		go func() {
			defer wg.Done()
			if stdoutWriter != nil {
				_, copyErr := io.Copy(stdoutWriter, stdout)
				if copyErr != nil {
					errChan <- errors.Wrapf(ErrFailed, "failed to copy stdout to output writer: %v", copyErr)
				}
			}
		}()
		go func() {
			defer wg.Done()
			if stderrWriter != nil {
				_, copyErr := io.Copy(stderrWriter, stderr)
				if copyErr != nil {
					errChan <- errors.Wrapf(ErrFailed, "failed to copy stderr to error writer: %v", copyErr)
				}
			}
		}()

		wg.Wait()

		execErr := cmd.Wait()
		if execErr != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				execErr = ErrTimeout
			} else {
				var exitErr *exec.ExitError
				if errors.As(execErr, &exitErr) && stderrWriter == nil {
					execErr = errors.Wrap(ErrFailed, string(exitErr.Stderr))
				}
			}
		}
		errChan <- execErr
	}()
	return errChan, nil
}
