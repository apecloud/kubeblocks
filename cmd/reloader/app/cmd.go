/*
Copyright ApeCloud, Inc.

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
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	zaplogfmt "github.com/sykesm/zap-logfmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/util/yaml"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgutil "github.com/apecloud/kubeblocks/internal/configuration"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration/config_manager"
	cfgproto "github.com/apecloud/kubeblocks/internal/configuration/proto"
)

var logger *zap.SugaredLogger

// NewConfigManagerCommand This command is used to reload configuration
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

	if opt.NotifyHandType == TPLScript && opt.BackupPath == "" {
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
	volumeWatcher, err := startVolumeWatcher(ctx, opt)
	if err != nil {
		return err
	}
	defer volumeWatcher.Close()

	serviceOpt := opt.ServiceOpt
	if serviceOpt.ContainerRuntimeEnable || serviceOpt.RemoteOnlineUpdateEnable {
		if err := startGRPCService(opt, ctx); err != nil {
			logger.Error(err, "failed to start grpc service.")
			return err
		}
	}

	logger.Info("config manager started.")
	<-ctx.Done()
	logger.Info("config manager shutdown.")
	return nil

}

func startVolumeWatcher(ctx context.Context, opt *VolumeWatcherOpts) (*cfgcore.ConfigMapVolumeWatcher, error) {
	volumeWatcher := cfgcore.NewVolumeWatcher(opt.VolumeDirs, ctx, logger)

	logger.Info("config backup path: ", opt.BackupPath)
	eventHandle, err := createHandlerWithVolumeWatch(opt)
	if err != nil {
		logger.Error(err, "failed to create event handle.")
		return nil, err
	}
	err = volumeWatcher.AddHandler(eventHandle).Run()
	if err != nil {
		logger.Error(err, "failed to handle VolumeWatcher.")
		return nil, err
	}
	return volumeWatcher, nil
}

func startGRPCService(opt *VolumeWatcherOpts, ctx context.Context) error {
	var (
		server *grpc.Server
		proxy  = &reconfigureProxy{opt: opt.ServiceOpt, ctx: ctx, logger: logger.Named("grpcProxy")}
	)

	if err := proxy.Init(opt); err != nil {
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
	if len(opt.VolumeDirs) == 0 && opt.NotifyHandType != TPLScript {
		return cfgutil.MakeError("require volume directory is null.")
	}

	if opt.NotifyHandType == TPLScript {
		return checkTPLScriptOptions(opt)
	}

	if opt.NotifyHandType == ShellTool && opt.Command == "" {
		return cfgutil.MakeError("require command is null.")
	}

	if len(opt.ProcessName) == 0 {
		return cfgutil.MakeError("require process name is null.")
	}
	return nil
}

type TplScriptConfig struct {
	Scripts         string                       `json:"scripts"`
	FileRegex       string                       `json:"fileRegex"`
	DataType        string                       `json:"dataType"`
	DSN             string                       `json:"dsn"`
	FormatterConfig appsv1alpha1.FormatterConfig `json:"formatterConfig"`
}

func checkTPLScriptOptions(opt *VolumeWatcherOpts) error {
	if opt.TPLConfig == "" {
		return cfgutil.MakeError("require tpl config is not null")
	}

	if _, err := os.Stat(opt.TPLConfig); err != nil {
		return err
	}

	b, err := os.ReadFile(opt.TPLConfig)
	if err != nil {
		return err
	}
	tplConfig := TplScriptConfig{}
	if err := yaml.Unmarshal(b, &tplConfig); err != nil {
		return err
	}

	opt.FormatterConfig = &tplConfig.FormatterConfig
	opt.DSN = tplConfig.DSN
	opt.DataType = tplConfig.DataType
	opt.FileRegex = tplConfig.FileRegex
	opt.TPLScriptPath = filepath.Join(filepath.Dir(opt.TPLConfig), tplConfig.Scripts)
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

func createHandlerWithVolumeWatch(opt *VolumeWatcherOpts) (cfgcore.WatchEventHandler, error) {
	logger.Infof("access info: [%d] [%s]", opt.NotifyHandType, opt.ProcessName)
	switch opt.NotifyHandType {
	case UnixSignal:
		return cfgcore.CreateSignalHandler(opt.Signal, opt.ProcessName)
	case ShellTool:
		return cfgcore.CreateExecHandler(opt.Command)
	case TPLScript:
		return cfgcore.CreateTPLScriptHandler(opt.TPLScriptPath, opt.VolumeDirs, opt.FileRegex, opt.BackupPath, opt.FormatterConfig, opt.DataType, opt.DSN)
	case SQL, WebHook:
		return nil, cfgutil.MakeError("event type[%s]: not yet, but in the future", opt.NotifyHandType.String())
	default:
		return nil, cfgutil.MakeError("not support event type.")
	}
}
