package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	zaplogfmt "github.com/sykesm/zap-logfmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	cfgutil "github.com/apecloud/kubeblocks/internal/configuration/container"
)

const (
	rfc3339Mills = "2006-01-02T15:04:05.000"
)

var logger logr.Logger

func main() {
	var containerRuntime cfgutil.CRIType
	var runtimeEndpoint string
	var contaienrID []string

	pflag.StringVar((*string)(&containerRuntime),
		"container-runtime", "auto", "the config set cri runtime type.")
	pflag.StringVar(&runtimeEndpoint,
		"runtime-endpoint", runtimeEndpoint, "the config set cri runtime endpoint.")
	pflag.StringArrayVar(&contaienrID,
		"container-id", contaienrID, "the container-id killed.")
	pflag.Parse()

	if len(contaienrID) == 0 {
		fmt.Fprintf(os.Stderr, "require container-id!\n\n")
		pflag.Usage()
		os.Exit(-1)
	}

	logCfg := zap.NewProductionEncoderConfig()
	logCfg.EncodeTime = func(ts time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(ts.UTC().Format(rfc3339Mills))
	}

	zapLogger := zap.New(zapcore.NewCore(zaplogfmt.NewEncoder(logCfg), os.Stdout, zap.DebugLevel))
	logger = zapr.NewLogger(zapLogger)

	killer, err := cfgutil.NewContainerKiller(containerRuntime, runtimeEndpoint, zapLogger.Sugar())
	if err != nil {
		logger.Error(err, "failed to create container killer")
		os.Exit(-1)
	}

	if err := killer.Init(context.Background()); err != nil {
		logger.Error(err, "failed to init killer")
	}

	if err := killer.Kill(context.Background(), contaienrID, viper.GetString(cfgutil.KillContainerSignalEnvName), nil); err != nil {
		logger.Error(err, fmt.Sprintf("failed to kill container[%s]", contaienrID))
	}
}
