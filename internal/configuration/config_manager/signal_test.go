//go:build linux || darwin

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

package configmanager

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSendSignal(t *testing.T) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGUSR1)
	defer stop()

	// for not expect signal
	{
		err := sendSignal(PID(os.Getpid()), syscall.SIGUSR2)
		if err != nil {
			logger.Error(err, "failed to send signal")
		}
		select {
		case <-time.After(time.Second):
			// walk here
			logger.Info("missed signal.")
			require.True(t, true)
		case <-ctx.Done():
			require.True(t, false)
		}
	}

	// for not expect signal
	{
		err := sendSignal(PID(os.Getpid()), syscall.SIGUSR1)
		if err != nil {
			logger.Error(err, "failed to send signal")
		}

		select {
		case <-time.After(time.Second):
			// not walk here
			logger.Info("missed signal.")
			require.True(t, false)
		case <-ctx.Done():
			require.True(t, true)
			// prints "context canceled"
			logger.Info(ctx.Err().Error())
			// stop receiving signal notifications as soon as possible.
			stop()
		}

	}
}
