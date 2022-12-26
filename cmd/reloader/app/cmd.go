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
	"time"

	"github.com/spf13/cobra"
	zaplogfmt "github.com/sykesm/zap-logfmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"

	cfgutil "github.com/apecloud/kubeblocks/internal/configuration"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration/configmap"
	cfgproto "github.com/apecloud/kubeblocks/internal/configuration/proto"
)

var logger *zap.SugaredLogger

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
	zapLog := initLog(opt.LogLevel)
	defer func() {
		_ = zapLog.Sync()
	}()

	logger = zapLog.Sugar()
	cfgcore.SetLogger(zapLog)
	if err := checkOptions(opt); err != nil {
		return err
	}

	// new volume watcher
	watcher := cfgcore.NewVolumeWatcher(opt.VolumeDirs, ctx, logger)

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
		logger.Error(err, "failed to handle VolumeWatcher.")
		return err
	}

	if !opt.ServiceOpt.Disable {
		if err := startGRPCService(opt.ServiceOpt, ctx); err != nil {
			logger.Error(err, "failed to start grpc service.")
			return err
		}
	}

	logger.Info("reload started.")
	<-ctx.Done()
	logger.Info("reload started shutdown.")

	return nil
}

func startGRPCService(opt ReconfigureServiceOptions, ctx context.Context) error {
	var (
		server *grpc.Server
		proxy  = &reconfigureProxy{opt: opt, ctx: ctx}
	)

	tcpSpec := fmt.Sprintf("%s:%d", proxy.opt.PodIP, proxy.opt.GrpcPort)

	logger.Infof("starting reconfigure service: %s", tcpSpec)
	listener, err := net.Listen("tcp", tcpSpec)
	if err != nil {
		return cfgutil.WrapError(err, "failed to create listener: [%s]", tcpSpec)
	}

	if err := proxy.Init(logger); err != nil {
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
			logger.Error(err, "failed to serve connections from cri")
			os.Exit(1)
		}
	}()
	logger.Info("reconfigure service started.")
	return nil
}

func logStreamServerInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	logger.Infof("info: [%+v]", info)
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

func initLog(level string) *zap.Logger {
	const (
		rfc3339Mills = "2006-01-02T15:04:05.000"
	)

	levelStrings := map[string]zapcore.Level{
		"debug": zap.DebugLevel,
		"info":  zap.InfoLevel,
		"error": zap.ErrorLevel,
	}

	if _, ok := levelStrings[level]; !ok {
		fmt.Printf("not support log level[%s], set default info", level)
		level = "info"
	}

	logCfg := zap.NewProductionEncoderConfig()
	logCfg.EncodeTime = func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(ts.UTC().Format(rfc3339Mills))
	}

	// NOTES:
	// zap is "Blazing fast, structured, leveled logging in Go.", DON'T event try
	// to refactor this logging lib to anything else. Check FAQ - https://github.com/uber-go/zap/blob/master/FAQ.md
	zapLog := zap.New(zapcore.NewCore(zaplogfmt.NewEncoder(logCfg), os.Stdout, levelStrings[level]))
	return zapLog
}

func createHandlerWithWatchType(opt *VolumeWatcherOpts) cfgcore.WatchEventHandler {
	logger.Infof("access info: [%d] [%s]", opt.NotifyHandType, opt.ProcessName)
	switch opt.NotifyHandType {
	case UnixSignal:
		return cfgcore.CreateSignalHandler(opt.Signal, opt.ProcessName)
	case SQL, ShellTool, WebHook:
		logger.Fatalf("event type[%s]: not yet, but in the future", opt.NotifyHandType.String())
	default:
		logger.Fatal("not support event type.")
	}
	return nil
}
