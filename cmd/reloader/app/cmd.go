/*
Copyright ApeCloud Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package app

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	cfgutil "github.com/apecloud/kubeblocks/internal/configuration"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration/configmap"
	cfgproto "github.com/apecloud/kubeblocks/internal/configuration/proto"
)

// NewConfigReloadCommand This command is used to reload configuration
func NewConfigReloadCommand(ctx context.Context, name string) *cobra.Command {
	opt := NewVolumeWatcherOpts()
	cmd := &cobra.Command{
		Use:   name,
		Short: name + " Provides a mechanism to implement reload config files in a sidecar for kubeblocks.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVolumeWatchCommand(ctx, opt)
		},
	}

	cmd.SetContext(ctx)
	InstallFlags(cmd.Flags(), opt)
	return cmd
}

func runVolumeWatchCommand(ctx context.Context, opt *VolumeWatcherOpts) error {
	initLog(opt.LogLevel)
	if err := checkOptions(opt); err != nil {
		return err
	}

	// new volume watcher
	watcher := cfgcore.NewVolumeWatcher(opt.VolumeDirs, ctx)

	// set regex filter
	if len(opt.FileRegex) > 0 {
		filter, err := cfgcore.CreateCfgRegexFilter(opt.FileRegex)
		if err != nil {
			return err
		}
		watcher.AddFilter(filter)
	}

	defer watcher.Close()
	err := watcher.AddHandler(createHandlerWithWatchType(opt)).Run()
	if err != nil {
		return err
	}

	if err := startGRPCService(opt.ServiceOpt, ctx); err != nil {
		return err
	}

	// testGRPCService(opt.ServiceOpt)

	logrus.Info("reload started.")
	<-ctx.Done()
	logrus.Info("reload started shutdown.")

	return nil
}

// func testGRPCService(opt ReconfigureServiceOptions) {
//	tcpSpec := fmt.Sprintf("%s:%d", opt.PodIP, opt.GrpcPort)
//	conn, err := grpc.Dial(tcpSpec, grpc.WithTransportCredentials(insecure.NewCredentials()))
//	if err != nil {
//		os.Exit(-1)
//	}
//
//	client := cfgproto.NewReconfigureClient(conn)
//	response, err := client.StopContainer(context.Background(), &cfgproto.StopContainerRequest{
//		ContainerIDs: []string{"abc", "efg", "37fc47bc5a6d8ad34ec927382488c698cd0a931b2755e28ed6f16c74de00c1d9"},
//	})
//
//	if err != nil {
//		logrus.Errorf("stop container failed, error: %v", err)
//		os.Exit(-2)
//	}
//
//	logrus.Info(response.ErrMessage)
// }

func startGRPCService(opt ReconfigureServiceOptions, ctx context.Context) error {
	var (
		server *grpc.Server
		proxy  = &reconfigureProxy{opt: opt, ctx: ctx}
	)

	tcpSpec := fmt.Sprintf("%s:%d", proxy.opt.PodIP, proxy.opt.GrpcPort)

	logrus.Infof("starting reconfigure service: %s", tcpSpec)
	listener, err := net.Listen("tcp", tcpSpec)
	if err != nil {
		return cfgutil.WrapError(err, "failed to create listener: [%s]", tcpSpec)
	}

	if err := proxy.Init(); err != nil {
		return err
	}

	if opt.DebugMode {
		server = grpc.NewServer(grpc.StreamInterceptor(logStreamServerInterceptor))
	} else {
		server = grpc.NewServer()
	}
	cfgproto.RegisterReconfigureServer(server, proxy)

	go func() {
		if err := server.Serve(listener); err != nil {
			logrus.Error("Failed to serve connections from cri")
			os.Exit(1)
		}
	}()
	logrus.Info("reconfigure service started.")
	return nil
}

func logStreamServerInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	logrus.Info(fmt.Sprintf("info: [%+v]", info))
	return handler(srv, ss)
}

func checkOptions(opt *VolumeWatcherOpts) error {
	if len(opt.ProcessName) == 0 {
		return cfgutil.MakeError("require process name is null.")
	}

	if len(opt.VolumeDirs) == 0 {
		return cfgutil.MakeError("require volume directory is null.")
	}

	return nil
}

func initLog(level string) {
	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.SetLevel(logLevel)
}

func createHandlerWithWatchType(opt *VolumeWatcherOpts) cfgcore.WatchEventHandler {
	logrus.Tracef("access info: [%d] [%s]", opt.NotifyHandType, opt.ProcessName)
	switch opt.NotifyHandType {
	case UnixSignal:
		return cfgcore.CreateSignalHandler(opt.Signal, opt.ProcessName)
	case SQL, ShellTool, WebHook:
		logrus.Fatalf("event type[%s]: not yet, but in the future", opt.NotifyHandType.String())
	default:
		logrus.Fatal("not support event type.")
	}
	return nil
}
