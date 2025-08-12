/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	kbaproto "github.com/apecloud/kubeblocks/pkg/kbagent/proto"
	"github.com/apecloud/kubeblocks/pkg/kbagent/util"
)

const (
	defaultBufferSize        = 4096
	defaultConnectTimeout    = 5 * time.Second
	defaultActionCallTimeout = 30 * time.Second
	maxActionCallTimeout     = 60 * time.Second
)

func gather[T interface{}](ch chan T) *T {
	select {
	case v, ok := <-ch:
		if !ok {
			return nil
		}
		return &v
	default:
		return nil
	}
}

type asyncResult struct {
	err    error
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

func blockingCallAction(ctx context.Context, action *kbaproto.Action, parameters map[string]string, timeout *int32) ([]byte, error) {
	resultChan, err := nonBlockingCallAction(ctx, action, parameters, timeout)
	if err != nil {
		return nil, err
	}
	result := <-resultChan
	err = result.err
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			errMsg := fmt.Sprintf("exit code: %d", exitErr.ExitCode())
			if stderrMsg := result.stderr.String(); len(stderrMsg) > 0 {
				errMsg += fmt.Sprintf(", stderr: %s", stderrMsg)
			}
			return nil, errors.Wrapf(kbaproto.ErrFailed, errMsg)
		}
		return nil, err
	}
	return result.stdout.Bytes(), nil
}

func nonBlockingCallAction(ctx context.Context, action *kbaproto.Action, parameters map[string]string, timeout *int32) (chan *asyncResult, error) {
	stdoutBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
	stderrBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
	execErrorChan, err := nonBlockingCallActionX(ctx, action, parameters, timeout, nil, stdoutBuf, stderrBuf)
	if err != nil {
		return nil, err
	}
	resultChan := make(chan *asyncResult, 1)
	go func() {
		// wait for the call to finish
		execErr, ok := <-execErrorChan
		if !ok {
			execErr = errors.New("runtime error: error chan closed unexpectedly")
		}
		resultChan <- &asyncResult{
			err:    execErr,
			stdout: stdoutBuf,
			stderr: stderrBuf,
		}
	}()
	return resultChan, nil
}

func nonBlockingCallActionX(ctx context.Context, action *kbaproto.Action, parameters map[string]string, timeout *int32,
	stdinReader io.Reader, stdoutWriter, stderrWriter io.Writer) (chan error, error) {
	var timeoutCancel context.CancelFunc
	if timeout == nil {
		ctx, timeoutCancel = context.WithTimeout(ctx, defaultActionCallTimeout)
	} else if *timeout > 0 {
		ctx, timeoutCancel = context.WithTimeout(ctx, min(time.Duration(*timeout)*time.Second, maxActionCallTimeout))
	}
	cancelTimeout := func() {
		if timeoutCancel != nil {
			timeoutCancel()
		}
	}

	var err error
	errChan := make(chan error, 1)
	switch {
	case action.Exec != nil:
		err = execActionCallX(ctx, cancelTimeout, action.Exec, parameters, errChan, stdinReader, stdoutWriter, stderrWriter)
	case action.HTTP != nil:
		err = httpActionCallX(ctx, cancelTimeout, action.HTTP, parameters, errChan, stdinReader, stdoutWriter, stderrWriter)
	case action.GRPC != nil:
		err = grpcActionCallX(ctx, cancelTimeout, action.GRPC, parameters, errChan, stdinReader, stdoutWriter, stderrWriter)
	default:
		err = errors.Wrapf(kbaproto.ErrBadRequest, "invalid action type")
	}
	if err != nil {
		return nil, err
	}
	return errChan, nil
}

func execActionCallX(ctx context.Context, cancel context.CancelFunc,
	action *kbaproto.ExecAction, parameters map[string]string, errChan chan error, stdinReader io.Reader, stdoutWriter, stderrWriter io.Writer) error {
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

	go func() {
		defer cancel()
		defer close(errChan)

		if err := cmd.Start(); err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				errChan <- kbaproto.ErrTimedOut
			} else {
				errChan <- errors.Wrapf(kbaproto.ErrFailed, "failed to start command: %v", err)
			}
			return
		}

		execErr := cmd.Wait()
		if execErr != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				execErr = kbaproto.ErrTimedOut
			}
		}
		errChan <- execErr
	}()

	return nil
}

func httpActionCallX(ctx context.Context, cancel context.CancelFunc,
	action *kbaproto.HTTPAction, parameters map[string]string, errChan chan error, _ io.Reader, stdoutWriter, _ io.Writer) error {
	// TODO: http client cache
	// don't use default http-client
	dialer := &net.Dialer{
		Timeout: defaultConnectTimeout,
	}
	transport := &http.Transport{
		DialContext:         dialer.DialContext,
		TLSHandshakeTimeout: defaultConnectTimeout,
	}
	cli := &http.Client{
		// don't set timeout at client level
		// Timeout:   time.Second * 30,
		Transport: transport,
	}
	// TODO: close the client

	url := fmt.Sprintf("%s://%s:%d%s", action.Scheme, action.Host, action.Port, action.Path)
	req, err := http.NewRequestWithContext(ctx, action.Method, url, strings.NewReader(action.Body))
	if err != nil {
		return err
	}

	for k, v := range action.Header {
		req.Header.Add(k, v)
	}
	for k, v := range parameters {
		req.Header.Add(k, v)
	}

	go func() {
		defer cancel()
		defer close(errChan)

		handleError := func(err error, msg string) {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				errChan <- kbaproto.ErrTimedOut
			} else {
				errChan <- errors.Wrapf(kbaproto.ErrFailed, "%s: %v", msg, err)
			}
		}

		rsp, err1 := cli.Do(req)
		if err1 != nil {
			handleError(err1, "failed to issue http request")
			return
		}
		defer safeClose(rsp.Body)

		// TODO: rsp.StatusCode
		// switch rsp.StatusCode {
		// case http.StatusOK, http.StatusInternalServerError:
		//	return rsp.Body, nil
		// default:
		//	return nil, fmt.Errorf("unexpected http status code: %s", rsp.Status)
		// }

		out, err2 := io.ReadAll(rsp.Body)
		if err2 != nil {
			handleError(err2, "failed to read http response")
			return
		}

		if len(out) > 0 {
			_, err3 := stdoutWriter.Write(out)
			if err3 != nil {
				handleError(err3, "failed to write http response")
				return
			}
		}
		errChan <- nil
	}()

	return nil
}

func grpcActionCallX(ctx context.Context, cancel context.CancelFunc,
	action *kbaproto.GRPCAction, parameters map[string]string, errChan chan error, _ io.Reader, stdoutWriter, stderrWriter io.Writer) error {
	// TODO: grpc stub cache
	remote := fmt.Sprintf("%s:%d", action.Host, action.Port)
	conn, err := grpc.DialContext(ctx, remote,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(), // wait for the connection to be established
	)
	if err != nil {
		return err
	}
	defer safeClose(conn)

	methodDesc, err := getMethodDescriptor(ctx, conn, action.Service, action.Method)
	if err != nil {
		return fmt.Errorf("failed to get grpc method descriptor: %v", err)
	}

	reqMsg := dynamicpb.NewMessage(methodDesc.Input())
	for k, v := range action.Messages {
		if err = setField(reqMsg, k, v); err != nil {
			return err
		}
	}
	for k, v := range parameters {
		if err = setField(reqMsg, k, v); err != nil {
			return err
		}
	}

	go func() {
		defer cancel()
		defer close(errChan)

		handleError := func(err error, msg string) {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				errChan <- kbaproto.ErrTimedOut
			} else {
				errChan <- errors.Wrapf(kbaproto.ErrFailed, "%s: %v", msg, err)
			}
		}

		method := fmt.Sprintf("/%s/%s", action.Service, action.Method)
		rspMsg := dynamicpb.NewMessage(methodDesc.Output())
		err1 := conn.Invoke(ctx, method, reqMsg, rspMsg)
		if err1 != nil {
			handleError(err1, "failed to issue grpc request")
			return
		}

		resStatus, err2 := getField(rspMsg, action.Status)
		if err2 != nil {
			handleError(err2, "failed to decode `Status` from grpc response")
			return
		}
		resOutput, err3 := getField(rspMsg, action.Output)
		if err3 != nil {
			handleError(err3, "failed to decode `Output` from grpc response")
			return
		}

		if len(resStatus) > 0 {
			if len(resOutput) > 0 {
				_, _ = stderrWriter.Write([]byte(resOutput))
			}
			errChan <- errors.New(resStatus)
		} else {
			if len(resOutput) > 0 {
				_, _ = stdoutWriter.Write([]byte(resOutput))
			}
			errChan <- nil
		}
	}()

	return nil
}

func getMethodDescriptor(ctx context.Context, conn *grpc.ClientConn, serviceName, methodName string) (protoreflect.MethodDescriptor, error) {
	cli := grpc_reflection_v1.NewServerReflectionClient(conn)
	stream, err := cli.ServerReflectionInfo(ctx, grpc.WaitForReady(true))
	if err != nil {
		return nil, fmt.Errorf("failed to create reflection stream: %w", err)
	}
	defer safeCloseF(stream.CloseSend)

	if err = stream.Send(&grpc_reflection_v1.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1.ServerReflectionRequest_ListServices{},
	}); err != nil {
		return nil, fmt.Errorf("failed to send list services request: %w", err)
	}

	if err = stream.Send(&grpc_reflection_v1.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: fmt.Sprintf("%s.%s", serviceName, methodName),
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to send file request: %w", err)
	}

	for {
		rsp, err := stream.Recv()
		if err != nil {
			return nil, fmt.Errorf("reflection recv error: %w", err)
		}
		switch m := rsp.MessageResponse.(type) {
		case *grpc_reflection_v1.ServerReflectionResponse_FileDescriptorResponse:
			files, err := decodeFileDescriptors(m.FileDescriptorResponse.GetFileDescriptorProto())
			if err != nil {
				return nil, err
			}
			for _, fd := range files {
				for i := 0; i < fd.Services().Len(); i++ {
					svc := fd.Services().Get(i)
					if string(svc.FullName()) == serviceName {
						for j := 0; j < svc.Methods().Len(); j++ {
							method := svc.Methods().Get(j)
							if string(method.Name()) == methodName {
								return method, nil
							}
						}
					}
				}
			}
			return nil, fmt.Errorf("grpc method %s not found in service %s", methodName, serviceName)
		default:
			// suppress the singleCaseSwitch warning
		}
	}
}

func decodeFileDescriptors(descBytes [][]byte) ([]protoreflect.FileDescriptor, error) {
	var result []protoreflect.FileDescriptor
	for _, b := range descBytes {
		fdProto := &descriptorpb.FileDescriptorProto{}
		if err := proto.Unmarshal(b, fdProto); err != nil {
			return nil, err
		}
		fd, err := protodesc.NewFile(fdProto, nil)
		if err != nil {
			return nil, err
		}
		result = append(result, fd)
	}
	return result, nil
}

func setField(msg *dynamicpb.Message, fieldName, value string) error {
	field := msg.Descriptor().Fields().ByTextName(fieldName)
	if field == nil {
		return fmt.Errorf("field %s not found", fieldName)
	}

	var val protoreflect.Value
	switch field.Kind() {
	case protoreflect.StringKind:
		val = protoreflect.ValueOfString(value)
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		i, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return err
		}
		val = protoreflect.ValueOfInt32(int32(i))
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		val = protoreflect.ValueOfInt64(i)
	case protoreflect.BoolKind:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		val = protoreflect.ValueOfBool(b)
	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}

	msg.Set(field, val)
	return nil
}

func getField(msg *dynamicpb.Message, fieldName string) (string, error) {
	field := msg.Descriptor().Fields().ByTextName(fieldName)
	if field == nil {
		return "", fmt.Errorf("field %s not found", fieldName)
	}

	val := msg.Get(field)
	if !val.IsValid() {
		return "", fmt.Errorf("invalid value for field %s", fieldName)
	}

	switch field.Kind() {
	case protoreflect.StringKind:
		return val.String(), nil
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return fmt.Sprintf("%d", val.Int()), nil
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return fmt.Sprintf("%d", val.Int()), nil
	case protoreflect.BoolKind:
		return fmt.Sprintf("%v", val.Bool()), nil
	default:
		return "", fmt.Errorf("unsupported field type: %s", field.Kind())
	}
}

func safeClose(c io.Closer) { _ = c.Close() }

func safeCloseF(c func() error) { _ = c() }
