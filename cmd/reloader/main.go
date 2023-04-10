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

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/apecloud/kubeblocks/cmd/reloader/app"
)

// Reload configuration
// TODO(zt) support unix signal
// TODO(zt) support shell

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	viper.AutomaticEnv()
	cmd := app.NewConfigManagerCommand(ctx, filepath.Base(os.Args[0]))
	if err := cmd.Execute(); err != nil && errors.Cause(err) != context.Canceled {
		fmt.Println(err)
		os.Exit(-1)
	}
}
