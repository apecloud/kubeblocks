//go:build linux || darwin

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

	// for expect signal
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
