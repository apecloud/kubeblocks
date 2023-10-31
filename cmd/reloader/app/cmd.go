/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package app

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	zaplogfmt "github.com/sykesm/zap-logfmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"

	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/config_manager"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgproto "github.com/apecloud/kubeblocks/pkg/configuration/proto"
)

var logger *zap.SugaredLogger

// NewConfigManagerCommand is used to reload configuration
func NewConfigManagerCommand(ctx context.Context, name string) *cobra.Command {
	opt := NewVolumeWatcherOpts()
	cmd := &cobra.Command{
		Use:   name,
		Short: name + " provides a mechanism to implement reload config files in a sidecar for kubeblocks.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigManagerCommand(ctx, opt)
		},
	}

	cmd.SetContext(ctx)
	InstallFlags(cmd.Flags(), opt)
	return cmd
}

func runConfigManagerCommand(ctx context.Context, opt *VolumeWatcherOpts) error {
	zapLog := initLog(opt.LogLevel)
	defer func() {
		_ = zapLog.Sync()
	}()

	logger = zapLog.Sugar()
	cfgcore.SetLogger(zapLog)

	if err := checkOptions(opt); err != nil {
		return err
	}
	if opt.BackupPath == "" {
		tmpDir, err := os.MkdirTemp(os.TempDir(), "reload-backup-")
		if err != nil {
			return err
		}
		opt.BackupPath = tmpDir
		defer os.RemoveAll(tmpDir)
	}
	return run(ctx, opt)
}

func run(ctx context.Context, opt *VolumeWatcherOpts) error {
	var (
		err           error
		volumeWatcher *cfgcore.ConfigMapVolumeWatcher
		configHandler cfgcore.ConfigHandler
	)

	if configHandler, err = cfgcore.CreateCombinedHandler(opt.CombConfig, opt.BackupPath); err != nil {
		return err
	}
	if len(opt.VolumeDirs) > 0 {
		if volumeWatcher, err = startVolumeWatcher(ctx, opt, configHandler); err != nil {
			return err
		}
		defer volumeWatcher.Close()
	}

	if err = checkAndCreateService(ctx, opt, configHandler); err != nil {
		return err
	}

	logger.Info("config manager started.")
	<-ctx.Done()
	logger.Info("config manager shutdown.")
	return nil
}

func checkAndCreateService(ctx context.Context, opt *VolumeWatcherOpts, handler cfgcore.ConfigHandler) error {
	serviceOpt := opt.ServiceOpt
	if !serviceOpt.ContainerRuntimeEnable && !serviceOpt.RemoteOnlineUpdateEnable {
		return nil
	}
	if err := startGRPCService(opt, ctx, handler); err != nil {
		return cfgutil.WrapError(err, "failed to start grpc service")
	}
	return nil
}

func startVolumeWatcher(ctx context.Context, opt *VolumeWatcherOpts, handler cfgcore.ConfigHandler) (*cfgcore.ConfigMapVolumeWatcher, error) {
	eventHandler := func(ctx context.Context, event fsnotify.Event) error {
		return handler.VolumeHandle(ctx, event)
	}
	logger.Info("starting fsnotify VolumeWatcher.")
	volumeWatcher := cfgcore.NewVolumeWatcher(opt.VolumeDirs, ctx, logger)
	err := volumeWatcher.AddHandler(eventHandler).Run()
	if err != nil {
		logger.Error(err, "failed to handle VolumeWatcher.")
		return nil, err
	}
	logger.Info("fsnotify VolumeWatcher started.")
	return volumeWatcher, nil
}

func startGRPCService(opt *VolumeWatcherOpts, ctx context.Context, handler cfgcore.ConfigHandler) error {
	var (
		server *grpc.Server
		proxy  = &reconfigureProxy{opt: opt.ServiceOpt, ctx: ctx, logger: logger.Named("grpcProxy")}
	)

	if err := proxy.Init(handler); err != nil {
		return err
	}

	tcpSpec := fmt.Sprintf("%s:%d", proxy.opt.PodIP, proxy.opt.GrpcPort)

	logger.Infof("starting reconfigure service: %s", tcpSpec)
	listener, err := net.Listen("tcp", tcpSpec)
	if err != nil {
		return cfgutil.WrapError(err, "failed to create listener: [%s]", tcpSpec)
	}

	server = grpc.NewServer(grpc.UnaryInterceptor(logUnaryServerInterceptor))
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

func logUnaryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	logger.Debugf("info: [%+v]", info)
	return handler(ctx, req)
}

func checkOptions(opt *VolumeWatcherOpts) error {
	if len(opt.VolumeDirs) == 0 && !opt.ServiceOpt.RemoteOnlineUpdateEnable {
		return cfgutil.MakeError("require volume directory is null.")
	}
	if opt.CombConfig == "" {
		return cfgutil.MakeError("required config is empty.")
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
		fmt.Printf("not supported log level[%s], set default info", level)
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
