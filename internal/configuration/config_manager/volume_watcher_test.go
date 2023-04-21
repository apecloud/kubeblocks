//go:build linux || darwin

/*
Copyright (C) 2022 ApeCloud Co., Ltd

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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

var zapLog, _ = zap.NewDevelopment()

func TestConfigMapVolumeWatcherFailed(t *testing.T) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "volume-watcher-test-failed-")
	require.Nil(t, err)
	defer os.RemoveAll(tmpDir)

	volumeWatcher := NewVolumeWatcher([]string{filepath.Join(tmpDir, "not_exist")}, context.Background(), zapLog.Sugar())
	defer volumeWatcher.Close()

	require.EqualError(t, volumeWatcher.Run(), "require process event handler.")
	volumeWatcher.AddHandler(func(_ context.Context, event fsnotify.Event) error {
		return nil
	})
	require.Regexp(t, "no such file or directory", volumeWatcher.Run().Error())
}

func TestConfigMapVolumeWatcher(t *testing.T) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "volume-watcher-test-")
	require.Nil(t, err)
	defer os.RemoveAll(tmpDir)

	var (
		mockVolume    = filepath.Join(tmpDir, "mock_volume")
		volumeWatcher *ConfigMapVolumeWatcher
		retryCount    = 0

		started = make(chan bool)
		trigger = make(chan bool)
	)

	if err := os.MkdirAll(mockVolume, fs.ModePerm); err != nil {
		t.Errorf("failed to create directory: %s", mockVolume)
	}

	volumeWatcher = NewVolumeWatcher([]string{mockVolume}, context.Background(), zapLog.Sugar())
	defer volumeWatcher.Close()

	regexFilter, err := CreateCfgRegexFilter(`.*`)
	require.Nil(t, err)
	volumeWatcher.SetRetryCount(2).
		AddHandler(func(_ context.Context, event fsnotify.Event) error {
			zapLog.Info(fmt.Sprintf("handl volume event: %v", event))
			retryCount++
			// mock failed to handle
			if retryCount <= 1 {
				return cfgcore.MakeError("failed to handle...")
			}
			trigger <- true
			return nil
		}).AddFilter(regexFilter)
	require.Nil(t, volumeWatcher.Run())

	// mock kubelet create configmapVolume
	go func() {
		// wait inotify ready
		<-started
		createTestConfigureDirectory(t, mockVolume, "test.conf", "empty!!!")
	}()

	// wait inotify to run...
	time.Sleep(1 * time.Second)
	started <- true
	select {
	case <-time.After(5 * time.Second):
		logger.Info("failed to watch volume.")
		require.True(t, false)
	case <-trigger:
		require.True(t, true)
	}
}
