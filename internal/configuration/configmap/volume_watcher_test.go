//go:build linux

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

package configmap

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var zapLog, _ = zap.NewDevelopment()

func TestConfigMapVolumeWatcher(t *testing.T) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "volume-watcher-test-")
	require.Nil(t, err)
	defer os.RemoveAll(tmpDir)

	mockVolume := filepath.Join(tmpDir, "mock_volume")

	volumeWatcher := NewVolumeWatcher([]string{mockVolume}, context.Background(), zapLog.Sugar())
	require.NotNil(t, volumeWatcher)
	defer volumeWatcher.Close()

	started := make(chan bool)
	trigger := make(chan bool)

	volumeWatcher.AddHandler(func(event fsnotify.Event) error {
		trigger <- true
		return nil
	})
	require.NotNil(t, volumeWatcher.Run())

	// mock kubelet create configmapVolume
	go func() {
		var (
			tmpVolumeDir   = filepath.Join(mockVolume, "..2023_02_16_06_06_06.1234567")
			configFilePath = filepath.Join(tmpVolumeDir, "test.conf")
			tmpDataDir     = filepath.Join(mockVolume, "..data_tmp")
			watchedDataDir = filepath.Join(mockVolume, "..data")

			configFileContext = []byte("empty!!!")
		)

		// wait inotify ready
		<-started
		if err := os.MkdirAll(tmpVolumeDir, fs.ModePerm); err != nil {
			t.Errorf("failed to create directory: %s", tmpVolumeDir)
		}
		if err := os.WriteFile(configFilePath, configFileContext, fs.ModePerm); err != nil {
			t.Errorf("failed to  write file: %s", configFilePath)
		}
		if err := os.Chmod(configFilePath, fs.ModePerm); err != nil {
			t.Errorf("failed to chmod file: %s", configFilePath)
		}
		if err := os.Symlink(tmpVolumeDir, tmpDataDir); err != nil {
			t.Errorf("failed to create symbolic link for atomic update: %v", err)
		}
		if err := os.Rename(tmpDataDir, watchedDataDir); err != nil {
			t.Errorf("failed to rename symbolic link for data directory %s: %v", tmpDataDir, err)
		}
	}()

	// wait inotify to run...
	time.Sleep(2 * time.Second)
	started <- true
	select {
	case <-time.After(10 * time.Second):
		logger.Info("failed to watch volume.")
		require.True(t, false)
	case <-trigger:
		require.True(t, true)
	}
}
