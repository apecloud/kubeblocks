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
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/apecloud/kubeblocks/pkg/kbagent/proto"
)

var _ = Describe("action utils", func() {
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

	Context("exec", func() {
		It("x - ok", func() {
			action := &proto.Action{
				Exec: &proto.ExecAction{
					Commands: []string{"/bin/bash", "-c", "echo -n simple"},
				},
			}
			execErrorChan, err := nonBlockingCallActionX(ctx, action, nil, nil, nil, nil, nil)
			Expect(err).Should(BeNil())

			wait(execErrorChan)
		})

		It("x - stdout", func() {
			action := &proto.Action{
				Exec: &proto.ExecAction{
					Commands: []string{"/bin/bash", "-c", "echo -n stdout"},
				},
			}
			stdoutBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
			execErrorChan, err := nonBlockingCallActionX(ctx, action, nil, nil, nil, stdoutBuf, nil)
			Expect(err).Should(BeNil())

			wait(execErrorChan)
			Expect(stdoutBuf.String()).Should(Equal("stdout"))
		})

		It("x - stderr", func() {
			action := &proto.Action{
				Exec: &proto.ExecAction{
					Commands: []string{"/bin/bash", "-c", "echo -n stderr >&2"},
				},
			}
			stdoutBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
			stderrBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
			execErrorChan, err := nonBlockingCallActionX(ctx, action, nil, nil, nil, stdoutBuf, stderrBuf)
			Expect(err).Should(BeNil())

			wait(execErrorChan)
			Expect(stdoutBuf.String()).Should(HaveLen(0))
			Expect(stderrBuf.String()).Should(Equal("stderr"))
		})

		It("x - stdin", func() {
			action := &proto.Action{
				Exec: &proto.ExecAction{
					Commands: []string{"/bin/bash", "-c", "xargs echo -n"},
				},
			}
			stdinBuf := bytes.NewBuffer([]byte{'s', 't', 'd', 'i', 'n'})
			stdoutBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
			execErrorChan, err := nonBlockingCallActionX(ctx, action, nil, nil, stdinBuf, stdoutBuf, nil)
			Expect(err).Should(BeNil())

			wait(execErrorChan)
			Expect(stdoutBuf.String()).Should(Equal("stdin"))
		})

		It("x - parameters", func() {
			action := &proto.Action{
				Exec: &proto.ExecAction{
					Commands: []string{"/bin/bash", "-c", "echo -n $PARAM"},
				},
			}
			parameters := map[string]string{
				"PARAM":   "parameters",
				"useless": "useless",
			}
			stdoutBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
			execErrorChan, err := nonBlockingCallActionX(ctx, action, parameters, nil, nil, stdoutBuf, nil)
			Expect(err).Should(BeNil())

			wait(execErrorChan)
			Expect(stdoutBuf.String()).Should(Equal("parameters"))
		})

		It("x - timeout", func() {
			action := &proto.Action{
				Exec: &proto.ExecAction{
					Commands: []string{"/bin/bash", "-c", "sleep 60"},
				},
			}
			timeout := int32(1)
			execErrorChan, err := nonBlockingCallActionX(ctx, action, nil, &timeout, nil, nil, nil)
			Expect(err).Should(BeNil())

			err = waitError(execErrorChan)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, proto.ErrTimedOut)).Should(BeTrue())
		})

		It("x - timeout and stdout", func() {
			action := &proto.Action{
				Exec: &proto.ExecAction{
					Commands: []string{"/bin/bash", "-c", "sleep 60 && echo -n timeout"},
				},
			}
			stdoutBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
			timeout := int32(1)
			execErrorChan, err := nonBlockingCallActionX(ctx, action, nil, &timeout, nil, stdoutBuf, nil)
			Expect(err).Should(BeNil())

			err = waitError(execErrorChan)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, proto.ErrTimedOut)).Should(BeTrue())
			Expect(stdoutBuf.String()).Should(HaveLen(0))
		})

		It("x - stdout and timeout", func() {
			action := &proto.Action{
				Exec: &proto.ExecAction{
					Commands: []string{"/bin/bash", "-c", "echo -n timeout && sleep 60"},
				},
			}
			stdoutBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
			timeout := int32(1)
			execErrorChan, err := nonBlockingCallActionX(ctx, action, nil, &timeout, nil, stdoutBuf, nil)
			Expect(err).Should(BeNil())

			err = waitError(execErrorChan)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, proto.ErrTimedOut)).Should(BeTrue())
			Expect(stdoutBuf.String()).Should(Equal("timeout"))
		})

		It("x - fail", func() {
			action := &proto.Action{
				Exec: &proto.ExecAction{
					Commands: []string{"/bin/bash", "-c", "command-not-exist"},
				},
			}
			execErrorChan, err := nonBlockingCallActionX(ctx, action, nil, nil, nil, nil, nil)
			Expect(err).Should(BeNil())

			err = waitError(execErrorChan)
			Expect(err).ShouldNot(BeNil())
			var exitErr *exec.ExitError
			Expect(errors.As(err, &exitErr)).Should(BeTrue())
		})

		It("x - fail with stderr writer", func() {
			action := &proto.Action{
				Exec: &proto.ExecAction{
					Commands: []string{"/bin/bash", "-c", "command-not-found"},
				},
			}
			stderrBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
			execErrorChan, err := nonBlockingCallActionX(ctx, action, nil, nil, nil, nil, stderrBuf)
			Expect(err).Should(BeNil())

			err = waitError(execErrorChan)
			Expect(err).ShouldNot(BeNil())
			var exitErr *exec.ExitError
			Expect(errors.As(err, &exitErr)).Should(BeTrue())
			Expect(stderrBuf.String()).Should(ContainSubstring("command not found"))
		})

		It("non-blocking - ok", func() {
			action := &proto.Action{
				Exec: &proto.ExecAction{
					Commands: []string{"/bin/bash", "-c", "echo -n ok"},
				},
			}
			resultChan, err := nonBlockingCallAction(ctx, action, nil, nil)
			Expect(err).Should(BeNil())

			result := <-resultChan
			Expect(result.err).Should(BeNil())
			Expect(result.stdout.Bytes()).Should(Equal([]byte("ok")))
			Expect(result.stderr.Bytes()).Should(HaveLen(0))
		})

		It("non-blocking - parameters", func() {
			action := &proto.Action{
				Exec: &proto.ExecAction{
					Commands: []string{"/bin/bash", "-c", "echo -n $PARAM"},
				},
			}
			parameters := map[string]string{
				"PARAM": "parameters",
			}
			resultChan, err := nonBlockingCallAction(ctx, action, parameters, nil)
			Expect(err).Should(BeNil())

			result := <-resultChan
			Expect(result.err).Should(BeNil())
			Expect(result.stdout.Bytes()).Should(Equal([]byte("parameters")))
			Expect(result.stderr.Bytes()).Should(HaveLen(0))
		})

		It("non-blocking - fail", func() {
			action := &proto.Action{
				Exec: &proto.ExecAction{
					Commands: []string{"/bin/bash", "-c", "command-not-found"},
				},
			}
			resultChan, err := nonBlockingCallAction(ctx, action, nil, nil)
			Expect(err).Should(BeNil())

			result := <-resultChan
			Expect(result.err).ShouldNot(BeNil())
			var exitErr *exec.ExitError
			Expect(errors.As(result.err, &exitErr)).Should(BeTrue())
			Expect(result.stdout.Bytes()).Should(HaveLen(0))
			Expect(result.stderr.Bytes()).Should(ContainSubstring("command not found"))
		})

		It("non-blocking - timeout", func() {
			action := &proto.Action{
				Exec: &proto.ExecAction{
					Commands: []string{"/bin/bash", "-c", "sleep 60"},
				},
			}
			timeout := int32(1)
			resultChan, err := nonBlockingCallAction(ctx, action, nil, &timeout)
			Expect(err).Should(BeNil())

			result := <-resultChan
			Expect(result.err).ShouldNot(BeNil())
			Expect(errors.Is(result.err, proto.ErrTimedOut)).Should(BeTrue())
			Expect(result.stdout.Bytes()).Should(HaveLen(0))
			Expect(result.stderr.Bytes()).Should(HaveLen(0))
		})

		It("blocking - ok", func() {
			action := &proto.Action{
				Exec: &proto.ExecAction{
					Commands: []string{"/bin/bash", "-c", "echo -n ok"},
				},
			}
			output, err := blockingCallAction(ctx, action, nil, nil)
			Expect(err).Should(BeNil())
			Expect(output).Should(Equal([]byte("ok")))
		})

		It("blocking - parameters", func() {
			action := &proto.Action{
				Exec: &proto.ExecAction{
					Commands: []string{"/bin/bash", "-c", "echo -n $PARAM"},
				},
			}
			parameters := map[string]string{
				"PARAM": "parameters",
			}
			output, err := blockingCallAction(ctx, action, parameters, nil)
			Expect(err).Should(BeNil())
			Expect(output).Should(Equal([]byte("parameters")))
		})

		It("blocking - fail", func() {
			action := &proto.Action{
				Exec: &proto.ExecAction{
					Commands: []string{"/bin/bash", "-c", "command-not-found"},
				},
			}
			output, err := blockingCallAction(ctx, action, nil, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, proto.ErrFailed)).Should(BeTrue())
			Expect(err.Error()).Should(ContainSubstring("command not found"))
			Expect(output).Should(BeNil())
		})

		It("blocking - timeout", func() {
			action := &proto.Action{
				Exec: &proto.ExecAction{
					Commands: []string{"/bin/bash", "-c", "sleep 60"},
				},
			}
			timeout := int32(1)
			output, err := blockingCallAction(ctx, action, nil, &timeout)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, proto.ErrTimedOut)).Should(BeTrue())
			Expect(output).Should(BeNil())
		})
	})

	Context("http", func() {
		var (
			server *httptest.Server
			port   string
		)

		BeforeEach(func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("statusCode") != "" {
					statusCode, _ := strconv.Atoi(r.Header.Get("statusCode"))
					w.WriteHeader(statusCode)
				}
				body, _ := io.ReadAll(r.Body)
				if len(body) > 0 {
					_, _ = w.Write(body)
				}
			}))
			url, _ := url.Parse(server.URL)
			_, port, _ = net.SplitHostPort(url.Host)
		})

		AfterEach(func() {
			server.Close()
		})

		It("x - ok", func() {
			action := &proto.Action{
				HTTP: &proto.HTTPAction{
					Port: port,
					Path: "/echo",
				},
			}
			errChan, err := nonBlockingCallActionX(ctx, action, nil, nil, nil, nil, nil)
			Expect(err).Should(BeNil())

			wait(errChan)
		})

		It("x - stdout", func() {
			action := &proto.Action{
				HTTP: &proto.HTTPAction{
					Port: port,
					Path: "/echo",
					Body: "stdout",
				},
			}
			stdoutBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
			errChan, err := nonBlockingCallActionX(ctx, action, nil, nil, nil, stdoutBuf, nil)
			Expect(err).Should(BeNil())

			wait(errChan)
			Expect(stdoutBuf.String()).Should(Equal("stdout"))
		})

		It("x - stderr", func() {
			action := &proto.Action{
				HTTP: &proto.HTTPAction{
					Port: port,
					Path: "/echo",
					Body: "stderr",
					Headers: []proto.HTTPHeader{
						{
							Name:  "statusCode",
							Value: strconv.Itoa(http.StatusInternalServerError),
						},
					},
				},
			}
			stdoutBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
			stderrBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
			errChan, err := nonBlockingCallActionX(ctx, action, nil, nil, nil, stdoutBuf, stderrBuf)
			Expect(err).Should(BeNil())

			err = waitError(errChan)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, proto.ErrFailed)).Should(BeTrue())
			Expect(stdoutBuf.String()).Should(HaveLen(0))
			Expect(stderrBuf.String()).Should(Equal("stderr"))
		})

		It("x - parameters", func() {
			action := &proto.Action{
				HTTP: &proto.HTTPAction{
					Port: port,
					Path: "/echo",
					Body: "{{ .PARAM }}",
				},
			}
			parameters := map[string]string{
				"PARAM":   "parameters",
				"useless": "useless",
			}
			stdoutBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
			errChan, err := nonBlockingCallActionX(ctx, action, parameters, nil, nil, stdoutBuf, nil)
			Expect(err).Should(BeNil())

			wait(errChan)
			Expect(stdoutBuf.String()).Should(Equal("parameters"))
		})

		It("non-blocking - ok", func() {
			action := &proto.Action{
				HTTP: &proto.HTTPAction{
					Port: port,
					Path: "/echo",
					Body: "ok",
				},
			}
			resultChan, err := nonBlockingCallAction(ctx, action, nil, nil)
			Expect(err).Should(BeNil())

			result := <-resultChan
			Expect(result.err).Should(BeNil())
			Expect(result.stdout.Bytes()).Should(Equal([]byte("ok")))
			Expect(result.stderr.Bytes()).Should(HaveLen(0))
		})

		It("non-blocking - parameters", func() {
			action := &proto.Action{
				HTTP: &proto.HTTPAction{
					Port: port,
					Path: "/echo",
					Body: "{{ .PARAM }}",
				},
			}
			parameters := map[string]string{
				"PARAM": "parameters",
			}
			resultChan, err := nonBlockingCallAction(ctx, action, parameters, nil)
			Expect(err).Should(BeNil())

			result := <-resultChan
			Expect(result.err).Should(BeNil())
			Expect(result.stdout.Bytes()).Should(Equal([]byte("parameters")))
			Expect(result.stderr.Bytes()).Should(HaveLen(0))
		})

		It("non-blocking - fail", func() {
			action := &proto.Action{
				HTTP: &proto.HTTPAction{
					Port: port,
					Path: "/echo",
					Body: "internal server error",
					Headers: []proto.HTTPHeader{
						{
							Name:  "statusCode",
							Value: strconv.Itoa(http.StatusInternalServerError),
						},
					},
				},
			}
			resultChan, err := nonBlockingCallAction(ctx, action, nil, nil)
			Expect(err).Should(BeNil())

			result := <-resultChan
			Expect(result.err).ShouldNot(BeNil())
			Expect(errors.Is(result.err, proto.ErrFailed)).Should(BeTrue())
			Expect(result.stdout.Bytes()).Should(HaveLen(0))
			Expect(result.stderr.Bytes()).Should(ContainSubstring("internal server error"))
		})

		It("blocking - ok", func() {
			action := &proto.Action{
				HTTP: &proto.HTTPAction{
					Port: port,
					Path: "/echo",
					Body: "ok",
				},
			}
			output, err := blockingCallAction(ctx, action, nil, nil)
			Expect(err).Should(BeNil())
			Expect(output).Should(Equal([]byte("ok")))
		})

		It("blocking - parameters", func() {
			action := &proto.Action{
				HTTP: &proto.HTTPAction{
					Port: port,
					Path: "/echo",
					Body: "{{ .PARAM }}",
				},
			}
			parameters := map[string]string{
				"PARAM": "parameters",
			}
			output, err := blockingCallAction(ctx, action, parameters, nil)
			Expect(err).Should(BeNil())
			Expect(output).Should(Equal([]byte("parameters")))
		})

		It("blocking - fail", func() {
			action := &proto.Action{
				HTTP: &proto.HTTPAction{
					Port: port,
					Path: "/echo",
					Body: "internal server error",
					Headers: []proto.HTTPHeader{
						{
							Name:  "statusCode",
							Value: strconv.Itoa(http.StatusInternalServerError),
						},
					},
				},
			}
			output, err := blockingCallAction(ctx, action, nil, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, proto.ErrFailed)).Should(BeTrue())
			Expect(err.Error()).Should(ContainSubstring("internal server error"))
			Expect(output).Should(BeNil())
		})
	})

	Context("grpc", func() {
		const (
			protoFilename = "echo.proto"
			protoData     = `
package test;

service EchoService {
  rpc Echo (EchoRequest) returns (EchoResponse);
}

message EchoRequest {
  optional bool success = 1;
  optional string message = 2;
}

message EchoResponse {
  optional string status  = 1;
  optional string message = 2;
}
`
		)
		var (
			host, port string
			cleanup    func()
		)

		startDynamicGRPCServer := func(filename, src string) (host, port string, cleanup func()) {
			// parse the proto
			parser := &protoparse.Parser{
				Accessor: func(name string) (io.ReadCloser, error) {
					if name == filename {
						return io.NopCloser(strings.NewReader(src)), nil
					}
					return nil, fmt.Errorf("unexpected proto import: %s", name)
				},
			}
			files, err := parser.ParseFiles(filename)
			Expect(err).NotTo(HaveOccurred(), "failed to parse proto")
			Expect(files).To(HaveLen(1), fmt.Sprintf("expect 1 file descriptor, got %d", len(files)))

			fd := files[0]
			fdp := fd.AsFileDescriptorProto()
			fileset := &descriptorpb.FileDescriptorSet{
				File: []*descriptorpb.FileDescriptorProto{fdp},
			}
			gfiles, err := protodesc.NewFiles(fileset)
			Expect(err).NotTo(HaveOccurred(), "protodesc.NewFiles failed")
			// ignore registration conflict
			_ = os.Setenv("GOLANG_PROTOBUF_REGISTRATION_CONFLICT", "ignore")
			gfiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
				_ = protoregistry.GlobalFiles.RegisterFile(fd)
				return true
			})

			services := fd.GetServices()
			Expect(services).To(HaveLen(1), "no service found in proto")

			svc := services[0]
			Expect(svc.GetFile().GetPackage()).Should(Equal("test"))
			Expect(svc.GetName()).Should(Equal("EchoService"))

			method := svc.FindMethodByName("Echo")
			Expect(method).ShouldNot(BeNil(), "method Echo not found")

			fullName := protoreflect.FullName(fmt.Sprintf("%s.%s", svc.GetFile().GetPackage(), svc.GetName()))
			d, err := protoregistry.GlobalFiles.FindDescriptorByName(fullName)
			Expect(err).ShouldNot(HaveOccurred(), fmt.Sprintf("FindDescriptorByName(%s) failed", fullName))

			svcDesc := d.(protoreflect.ServiceDescriptor)
			mDesc := svcDesc.Methods().ByName("Echo")
			Expect(mDesc).ShouldNot(BeNil(), "method Echo not found via protoregistry")

			inDesc := mDesc.Input()
			outDesc := mDesc.Output()
			unaryHandler := func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
				in := dynamicpb.NewMessage(inDesc)
				if err := dec(in); err != nil {
					return nil, err
				}
				successField := inDesc.Fields().ByName("success")
				reqSuccess := in.Get(successField).Bool()
				msgField := inDesc.Fields().ByName("message")
				reqMsg := in.Get(msgField).String()

				out := dynamicpb.NewMessage(outDesc)
				statusField := outDesc.Fields().ByName("status")
				messageField := outDesc.Fields().ByName("message")

				if reqSuccess {
					out.Set(statusField, protoreflect.ValueOfString(""))
					out.Set(messageField, protoreflect.ValueOfString(reqMsg))
				} else {
					out.Set(statusField, protoreflect.ValueOfString("error"))
					out.Set(messageField, protoreflect.ValueOfString(reqMsg))
				}
				return out, nil
			}

			gsd := &grpc.ServiceDesc{
				ServiceName: string(fullName), // "test.EchoService"
				HandlerType: (*interface{})(nil),
				Methods: []grpc.MethodDesc{
					{
						MethodName: "Echo",
						Handler:    unaryHandler,
					},
				},
				Streams:  []grpc.StreamDesc{},
				Metadata: fd.GetName(), // "echo.proto"
			}

			l, err := net.Listen("tcp", "127.0.0.1:0")
			Expect(err).Should(BeNil(), "failed to listen")

			s := grpc.NewServer()
			s.RegisterService(gsd, nil)
			reflection.Register(s)

			go func() {
				if err := s.Serve(l); err != nil {
					panic(err)
				}
			}()

			h, p, _ := net.SplitHostPort(l.Addr().String())
			return h, p, func() {
				s.Stop()
				_ = l.Close()
				_ = fdp
			}
		}

		BeforeEach(func() {
			host, port, cleanup = startDynamicGRPCServer(protoFilename, protoData)
		})

		AfterEach(func() {
			cleanup()
		})

		It("x - ok", func() {
			action := &proto.Action{
				GRPC: &proto.GRPCAction{
					Port:    port,
					Host:    host,
					Service: "test.EchoService",
					Method:  "Echo",
					Request: map[string]string{
						"success": "true",
						"message": "ok",
					},
					Response: proto.GRPCResponse{
						Status:  "status",
						Message: "message",
					},
				},
			}
			errChan, err := nonBlockingCallActionX(ctx, action, nil, nil, nil, nil, nil)
			Expect(err).Should(BeNil())

			wait(errChan)
		})

		It("x - stdout", func() {
			action := &proto.Action{
				GRPC: &proto.GRPCAction{
					Port:    port,
					Host:    host,
					Service: "test.EchoService",
					Method:  "Echo",
					Request: map[string]string{
						"success": "true",
						"message": "stdout",
					},
					Response: proto.GRPCResponse{
						Status:  "status",
						Message: "message",
					},
				},
			}
			stdoutBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
			errChan, err := nonBlockingCallActionX(ctx, action, nil, nil, nil, stdoutBuf, nil)
			Expect(err).Should(BeNil())

			wait(errChan)
			Expect(stdoutBuf.String()).Should(Equal("stdout"))
		})

		It("x - stderr", func() {
			action := &proto.Action{
				GRPC: &proto.GRPCAction{
					Port:    port,
					Host:    host,
					Service: "test.EchoService",
					Method:  "Echo",
					Request: map[string]string{
						"success": "false",
						"message": "stderr",
					},
					Response: proto.GRPCResponse{
						Status:  "status",
						Message: "message",
					},
				},
			}
			stdoutBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
			stderrBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
			errChan, err := nonBlockingCallActionX(ctx, action, nil, nil, nil, stdoutBuf, stderrBuf)
			Expect(err).Should(BeNil())

			err = waitError(errChan)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, proto.ErrFailed)).Should(BeTrue())
			Expect(stdoutBuf.String()).Should(HaveLen(0))
			Expect(stderrBuf.String()).Should(Equal("stderr"))
		})

		It("x - parameters", func() {
			action := &proto.Action{
				GRPC: &proto.GRPCAction{
					Port:    port,
					Host:    host,
					Service: "test.EchoService",
					Method:  "Echo",
					Request: map[string]string{
						"success": "true",
						"message": "{{ .PARAM }}",
					},
					Response: proto.GRPCResponse{
						Status:  "status",
						Message: "message",
					},
				},
			}
			parameters := map[string]string{
				"PARAM":   "parameters",
				"useless": "useless",
			}
			stdoutBuf := bytes.NewBuffer(make([]byte, 0, defaultBufferSize))
			errChan, err := nonBlockingCallActionX(ctx, action, parameters, nil, nil, stdoutBuf, nil)
			Expect(err).Should(BeNil())

			wait(errChan)
			Expect(stdoutBuf.String()).Should(Equal("parameters"))
		})

		It("non-blocking - ok", func() {
			action := &proto.Action{
				GRPC: &proto.GRPCAction{
					Port:    port,
					Host:    host,
					Service: "test.EchoService",
					Method:  "Echo",
					Request: map[string]string{
						"success": "true",
						"message": "ok",
					},
					Response: proto.GRPCResponse{
						Status:  "status",
						Message: "message",
					},
				},
			}
			resultChan, err := nonBlockingCallAction(ctx, action, nil, nil)
			Expect(err).Should(BeNil())

			result := <-resultChan
			Expect(result.err).Should(BeNil())
			Expect(result.stdout.Bytes()).Should(Equal([]byte("ok")))
			Expect(result.stderr.Bytes()).Should(HaveLen(0))
		})

		It("non-blocking - parameters", func() {
			action := &proto.Action{
				GRPC: &proto.GRPCAction{
					Port:    port,
					Host:    host,
					Service: "test.EchoService",
					Method:  "Echo",
					Request: map[string]string{
						"success": "true",
						"message": "{{ .PARAM }}",
					},
					Response: proto.GRPCResponse{
						Status:  "status",
						Message: "message",
					},
				},
			}
			parameters := map[string]string{
				"PARAM": "parameters",
			}
			resultChan, err := nonBlockingCallAction(ctx, action, parameters, nil)
			Expect(err).Should(BeNil())

			result := <-resultChan
			Expect(result.err).Should(BeNil())
			Expect(result.stdout.Bytes()).Should(Equal([]byte("parameters")))
			Expect(result.stderr.Bytes()).Should(HaveLen(0))
		})

		It("non-blocking - fail", func() {
			action := &proto.Action{
				GRPC: &proto.GRPCAction{
					Port:    port,
					Host:    host,
					Service: "test.EchoService",
					Method:  "Echo",
					Request: map[string]string{
						"success": "false",
						"message": "error",
					},
					Response: proto.GRPCResponse{
						Status:  "status",
						Message: "message",
					},
				},
			}
			resultChan, err := nonBlockingCallAction(ctx, action, nil, nil)
			Expect(err).Should(BeNil())

			result := <-resultChan
			Expect(result.err).ShouldNot(BeNil())
			Expect(errors.Is(result.err, proto.ErrFailed)).Should(BeTrue())
			Expect(result.stdout.Bytes()).Should(HaveLen(0))
			Expect(result.stderr.Bytes()).Should(ContainSubstring("error"))
		})

		It("blocking - ok", func() {
			action := &proto.Action{
				GRPC: &proto.GRPCAction{
					Port:    port,
					Host:    host,
					Service: "test.EchoService",
					Method:  "Echo",
					Request: map[string]string{
						"success": "true",
						"message": "ok",
					},
					Response: proto.GRPCResponse{
						Status:  "status",
						Message: "message",
					},
				},
			}
			output, err := blockingCallAction(ctx, action, nil, nil)
			Expect(err).Should(BeNil())
			Expect(output).Should(Equal([]byte("ok")))
		})

		It("blocking - parameters", func() {
			action := &proto.Action{
				GRPC: &proto.GRPCAction{
					Port:    port,
					Host:    host,
					Service: "test.EchoService",
					Method:  "Echo",
					Request: map[string]string{
						"success": "true",
						"message": "{{ .PARAM }}",
					},
					Response: proto.GRPCResponse{
						Status:  "status",
						Message: "message",
					},
				},
			}
			parameters := map[string]string{
				"PARAM": "parameters",
			}
			output, err := blockingCallAction(ctx, action, parameters, nil)
			Expect(err).Should(BeNil())
			Expect(output).Should(Equal([]byte("parameters")))
		})

		It("blocking - fail", func() {
			action := &proto.Action{
				GRPC: &proto.GRPCAction{
					Port:    port,
					Host:    host,
					Service: "test.EchoService",
					Method:  "Echo",
					Request: map[string]string{
						"success": "false",
						"message": "error",
					},
					Response: proto.GRPCResponse{
						Status:  "status",
						Message: "message",
					},
				},
			}
			output, err := blockingCallAction(ctx, action, nil, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(errors.Is(err, proto.ErrFailed)).Should(BeTrue())
			Expect(err.Error()).Should(ContainSubstring("error"))
			Expect(output).Should(BeNil())
		})
	})
})
