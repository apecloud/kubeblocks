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
	var containerID []string

	pflag.StringVar((*string)(&containerRuntime),
		"container-runtime", "auto", "the config sets cri runtime type.")
	pflag.StringVar(&runtimeEndpoint,
		"runtime-endpoint", runtimeEndpoint, "the config sets cri runtime endpoint.")
	pflag.StringArrayVar(&containerID,
		"container-id", containerID, "the container-id to be killed.")
	pflag.Parse()

	if len(containerID) == 0 {
		fmt.Fprintf(os.Stderr, " container-id required!\n\n")
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
		logger.Error(err, "failed to create container killing process")
		os.Exit(-1)
	}

	if err := killer.Init(context.Background()); err != nil {
		logger.Error(err, "failed to init killer")
	}

	if err := killer.Kill(context.Background(), containerID, viper.GetString(cfgutil.KillContainerSignalEnvName), nil); err != nil {
		logger.Error(err, fmt.Sprintf("failed to kill container[%s]", containerID))
	}
}
