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

package app

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	zaplogfmt "github.com/sykesm/zap-logfmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"

	cfgcm "github.com/apecloud/kubeblocks/pkg/parameters/configmanager"
	cfgutil "github.com/apecloud/kubeblocks/pkg/parameters/core"
	cfgproto "github.com/apecloud/kubeblocks/pkg/parameters/proto"
)

const (
	InaddrAny  = "0.0.0.0"
	Inaddr6Any = "::"
)

var logger *zap.SugaredLogger

// NewConfigManagerCommand is used to reload configuration
func NewConfigManagerCommand(ctx context.Context, name string) *cobra.Command {
	opts := newServiceOptions()
	cmd := &cobra.Command{
		Use:   name,
		Short: name + " provides a mechanism to implement reload config files in a sidecar for kubeblocks.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigManagerCommand(ctx, opts)
		},
	}

	cmd.SetContext(ctx)
	installFlags(cmd.Flags(), opts)
	return cmd
}

func runConfigManagerCommand(ctx context.Context, opts *serviceOptions) error {
	zapLog := initLog(opts.LogLevel)
	defer func() {
		_ = zapLog.Sync()
	}()

	logger = zapLog.Sugar()
	cfgcm.SetLogger(zapLog)

	if err := checkOptions(opts); err != nil {
		return err
	}
	return run(ctx, opts)
}

func run(ctx context.Context, opts *serviceOptions) error {
	var (
		handler cfgcm.ConfigHandler
		err     error
	)

	if handler, err = cfgcm.CreateCombinedHandler(opts.CombConfig); err != nil {
		return err
	}

	if err = startGRPCService(opts, handler); err != nil {
		return cfgutil.WrapError(err, "failed to start grpc service")
	}

	logger.Info("config manager started.")
	<-ctx.Done()
	logger.Info("config manager shutdown.")
	return nil
}

func startGRPCService(opts *serviceOptions, handler cfgcm.ConfigHandler) error {
	var (
		server *grpc.Server
		proxy  = &reconfigureProxy{handler: handler, logger: logger.Named("grpcProxy")}
	)

	// ipv4 unspecified address: 0.0.0.0
	hostIP := InaddrAny
	if ip, _ := netip.ParseAddr(opts.PodIP); ip.Is6() {
		// ipv6 unspecified address: ::
		hostIP = Inaddr6Any
	}

	tcpSpec := net.JoinHostPort(hostIP, strconv.Itoa(opts.GrpcPort))
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

func checkOptions(opts *serviceOptions) error {
	if !opts.RemoteOnlineUpdateEnable {
		return cfgutil.MakeError("remote online update is NOT enabled.")
	}
	if opts.CombConfig == "" {
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
