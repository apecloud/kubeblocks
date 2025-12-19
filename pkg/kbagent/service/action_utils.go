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
	"sync"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
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
	defaultBufferSize = 4096

	defaultConnectTimeout            = 5 * time.Second
	defaultKeepAliveTimeout          = 30 * time.Second
	defaultIdleConnTimeout           = 90 * time.Second
	defaultMaxIdleConnections        = 100
	defaultMaxIdleConnectionsPerHost = 20

	defaultActionCallTimeout = 30 * time.Second
	maxActionCallTimeout     = 60 * time.Second

	defaultHTTPHost   = "127.0.0.1"
	defaultHTTPScheme = "HTTP"
	defaultHTTPMethod = "GET"
	defaultHTTPPath   = "/"
)

var (
	clientCache sync.Map // map[string]<*http.Client|*grpc.ClientConn>

	defaultHTTPTransport = &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   defaultConnectTimeout,
			KeepAlive: defaultKeepAliveTimeout,
		}).DialContext,
		TLSHandshakeTimeout: defaultConnectTimeout,
		IdleConnTimeout:     defaultIdleConnTimeout,
		MaxIdleConns:        defaultMaxIdleConnections,
		MaxIdleConnsPerHost: defaultMaxIdleConnectionsPerHost,
	}
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
			return nil, errors.Wrapf(kbaproto.ErrFailed, "%s", errMsg)
		}
		if errMsg := result.stderr.String(); len(errMsg) > 0 {
			return nil, errors.Wrapf(err, "%s", errMsg)
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
	var cancel context.CancelFunc
	if timeout == nil {
		ctx, cancel = context.WithTimeout(ctx, defaultActionCallTimeout)
	} else if *timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, min(time.Duration(*timeout)*time.Second, maxActionCallTimeout))
	}

	var err error
	errChan := make(chan error, 1)
	switch {
	case action.Exec != nil:
		err = execActionCallX(ctx, cancel, action.Exec, parameters, errChan, stdinReader, stdoutWriter, stderrWriter)
	case action.HTTP != nil:
		err = httpActionCallX(ctx, cancel, action.HTTP, parameters, errChan, stdinReader, stdoutWriter, stderrWriter)
	case action.GRPC != nil:
		err = grpcActionCallX(ctx, cancel, action.GRPC, parameters, errChan, stdinReader, stdoutWriter, stderrWriter)
	default:
		cancel() // cancel the context to release the resources
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
	action *kbaproto.HTTPAction, parameters map[string]string, errChan chan error, _ io.Reader, stdoutWriter, stderrWriter io.Writer) error {
	var (
		// TODO: resolve the port
		cli         = httpClient()
		method, url = httpActionMethodNURL(action)
	)
	body, err1 := renderTemplateData("http body", parameters, action.Body)
	if err1 != nil {
		return err1
	}
	req, err2 := http.NewRequestWithContext(ctx, method, url, strings.NewReader(body))
	if err2 != nil {
		return err2
	}

	for _, h := range action.Headers {
		val, err3 := renderTemplateData("http header", parameters, h.Value)
		if err3 != nil {
			return err3
		}
		req.Header.Add(h.Name, val)
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

		rsp, err4 := cli.Do(req)
		if err4 != nil {
			handleError(err4, "failed to issue http request")
			return
		}
		defer safeClose(rsp.Body)

		output, err5 := io.ReadAll(rsp.Body)
		if err5 != nil {
			handleError(err5, "failed to read http response")
			return
		}

		succeed := rsp.StatusCode >= 200 && rsp.StatusCode < 300
		if succeed {
			if len(output) > 0 && stdoutWriter != nil {
				_, err6 := stdoutWriter.Write(output)
				if err6 != nil {
					handleError(err6, "failed to write http response")
					return
				}
			}
			errChan <- nil
		} else {
			if len(output) > 0 && stderrWriter != nil {
				_, err7 := stderrWriter.Write(output)
				if err7 != nil {
					handleError(err7, "failed to write http response")
					return
				}
			}
			errChan <- errors.Wrapf(kbaproto.ErrFailed, "http request failed, status: %s", rsp.Status)
		}
	}()

	return nil
}

func httpClient() *http.Client {
	key := "http"
	if v, ok := clientCache.Load(key); ok {
		return v.(*http.Client)
	}
	cli, _ := clientCache.LoadOrStore(key, &http.Client{
		Transport: defaultHTTPTransport,
	})
	return cli.(*http.Client)
}

func httpActionMethodNURL(action *kbaproto.HTTPAction) (string, string) {
	host, scheme, method, path := defaultHTTPHost, defaultHTTPScheme, defaultHTTPMethod, defaultHTTPPath
	if len(action.Host) > 0 {
		host = action.Host
	}
	if len(action.Scheme) > 0 {
		scheme = action.Scheme
	}
	if len(action.Method) > 0 {
		method = action.Method
	}
	if len(action.Path) > 0 {
		path = action.Path
	}
	return method, fmt.Sprintf("%s://%s:%s%s", scheme, host, action.Port, path)
}

func grpcActionCallX(ctx context.Context, cancel context.CancelFunc,
	action *kbaproto.GRPCAction, parameters map[string]string, errChan chan error, _ io.Reader, stdoutWriter, stderrWriter io.Writer) error {
	host := defaultHTTPHost
	if len(action.Host) > 0 {
		host = action.Host
	}
	// TODO: resolve the port
	conn, err := grpcClientConnection(ctx, host, action.Port)
	if err != nil {
		return err
	}

	methodDesc, err := getGRPCMethodDescriptor(ctx, conn, action.Service, action.Method)
	if err != nil {
		return fmt.Errorf("failed to get grpc method descriptor: %v", err)
	}

	reqMsg := dynamicpb.NewMessage(methodDesc.Input())
	for k, v := range action.Request {
		vv, err1 := renderTemplateData("grpc", parameters, v)
		if err1 != nil {
			return err1
		}
		if err = setGRPCMessageField(reqMsg, k, vv); err != nil {
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

		var (
			status, output string
			err2, err3     error
		)
		if len(action.Response.Status) > 0 {
			status, err2 = getGRPCMessageField(rspMsg, action.Response.Status)
			if err2 != nil {
				handleError(err2, "failed to decode `Status` from grpc response")
				return
			}
		}
		if len(action.Response.Message) > 0 {
			output, err3 = getGRPCMessageField(rspMsg, action.Response.Message)
			if err3 != nil {
				handleError(err3, "failed to decode `Message` from grpc response")
				return
			}
		}

		if len(status) > 0 {
			if len(output) > 0 && stderrWriter != nil {
				_, _ = stderrWriter.Write([]byte(output))
			}
			errChan <- errors.Wrapf(kbaproto.ErrFailed, "grpc call failed: %s", status)
		} else {
			if len(output) > 0 && stdoutWriter != nil {
				_, _ = stdoutWriter.Write([]byte(output))
			}
			errChan <- nil
		}
	}()

	return nil
}

func grpcClientConnection(ctx context.Context, host, port string) (*grpc.ClientConn, error) {
	remote := fmt.Sprintf("%s:%s", host, port)
	key := fmt.Sprintf("grpc://%s", remote)
	if v, ok := clientCache.Load(key); ok {
		return v.(*grpc.ClientConn), nil
	}

	conn, err := grpc.DialContext(ctx, remote,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(16*1024*1024),
		),
	)
	if err != nil {
		return nil, err
	}

	actual, loaded := clientCache.LoadOrStore(key, conn)
	if loaded {
		_ = conn.Close()
		return actual.(*grpc.ClientConn), nil
	}
	return conn, nil
}

func getGRPCMethodDescriptor(ctx context.Context, conn *grpc.ClientConn, serviceName, methodName string) (protoreflect.MethodDescriptor, error) {
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
			files, err := decodeGRPCFileDescriptors(m.FileDescriptorResponse.GetFileDescriptorProto())
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

func decodeGRPCFileDescriptors(descBytes [][]byte) ([]protoreflect.FileDescriptor, error) {
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

func setGRPCMessageField(msg *dynamicpb.Message, fieldName, value string) error {
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

func getGRPCMessageField(msg *dynamicpb.Message, fieldName string) (string, error) {
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

func renderTemplateData(action string, parameters map[string]string, data string) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	tpl := template.New(action).Option("missingkey=error").Funcs(sprig.TxtFuncMap())
	ptpl, err := tpl.Parse(data)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err = ptpl.Execute(&buf, mergeEnvWith(parameters)); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func mergeEnvWith(parameters map[string]string) map[string]any {
	result := make(map[string]any)
	for k, v := range parameters {
		result[k] = v
	}
	for _, e := range os.Environ() {
		kv := strings.Split(e, "=")
		if _, ok := result[kv[0]]; !ok {
			result[kv[0]] = kv[1]
		}
	}
	return result
}
