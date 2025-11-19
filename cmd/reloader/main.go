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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/pflag"

	cfgcm "github.com/apecloud/kubeblocks/pkg/parameters/configmanager"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

type options struct {
	config     string
	configSpec string
	parameters string
	configFile string
	timeout    int
}

var opts = &options{}

func init() {
	viper.AutomaticEnv()

	pflag.StringVar(&opts.config, "config", "", "the reload config")
	pflag.StringVar(&opts.configSpec, "config-spec", "", "the config spec")
	pflag.StringVar(&opts.parameters, "parameters", "", "the parameters to update")
	pflag.StringVar(&opts.configFile, "config-file", "", "the config file to update")
	pflag.IntVar(&opts.timeout, "timeout", 0, "the timeout to wait for the update to complete")
}

func main() {
	var (
		handler cfgcm.ConfigHandler
		err     error
	)
	if handler, err = cfgcm.CreateCombinedHandler(opts.config); err != nil {
		fmt.Printf("create combined handler error: %v\n", err)
		os.Exit(-1)
	}

	key := opts.configSpec
	if opts.configFile != "" {
		key = key + "/" + opts.configFile
	}

	if len(opts.parameters) == 0 {
		// fmt.Printf("update parameters is empty\n")
		os.Exit(0)
	}
	var parameters map[string]string
	if err = json.Unmarshal([]byte(opts.parameters), &parameters); err != nil {
		fmt.Printf("unmarshal parameters error: %v\n", err)
		os.Exit(-1)
	}

	if err = update(handler, key, parameters); err != nil {
		fmt.Printf("update parameters error: %v\n", err)
		os.Exit(-1)
	}
}

func update(handler cfgcm.ConfigHandler, key string, parameters map[string]string) error {
	ctx := context.Background()
	var cancel context.CancelFunc
	if opts.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(opts.timeout)*time.Second)
		defer cancel()
	}
	return handler.OnlineUpdate(ctx, key, parameters)
}
