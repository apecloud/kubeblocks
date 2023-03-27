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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func init() {
	var zapLog, _ = zap.NewDevelopment()
	SetLogger(zapLog)
}

func TestFindParentPidFromProcessName(t *testing.T) {
	processName := getProcName()
	fmt.Printf("current test program name: %s\n", processName)
	pid, err := findPidFromProcessName(processName)
	require.Nil(t, err)
	require.Equal(t, PID(os.Getpid()), pid)
}

func getProcName() string {
	pid := int32(os.Getpid())
	procs, _ := process.Processes()
	for _, proc := range procs {
		if pid == proc.Pid {
			name, _ := proc.Name()
			return name
		}
	}
	return ""
}

func TestCreateSignalHandler(t *testing.T) {
	_, err := CreateSignalHandler(appsv1alpha1.SIGALRM, "test")
	require.Nil(t, err)
	_, err = CreateSignalHandler("NOSIGNAL", "test")
	require.ErrorContains(t, err, "not support unix signal")
}

func TestCreateExecHandler(t *testing.T) {
	_, err := CreateExecHandler("")
	require.ErrorContains(t, err, "invalid command")
	_, err = CreateExecHandler(" ")
	require.ErrorContains(t, err, "invalid command")
	c, err := CreateExecHandler("go 	version")
	require.Nil(t, err)
	require.Nil(t, c(fsnotify.Event{}))
}

func TestCreateTPLScriptHandler(t *testing.T) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "gotemplate-sqlhandle-")
	require.Nil(t, err)
	defer os.RemoveAll(tmpDir)

	createTestConfigureDirectory(t, filepath.Join(tmpDir, "config"), "my.cnf", "xxxx")
	tplFile := filepath.Join(tmpDir, "test.tpl")
	require.Nil(t, os.WriteFile(tplFile, []byte("xxx"), fs.ModePerm))

	_, err = CreateTPLScriptHandler(tplFile, []string{filepath.Join(tmpDir, "config")}, "", filepath.Join(tmpDir, "backup"), nil)
	require.Nil(t, err)
}

func createTestConfigureDirectory(t *testing.T, mockDirectory string, cfgFile, content string) {
	var (
		tmpVolumeDir   = filepath.Join(mockDirectory, "..2023_02_16_06_06_06.1234567")
		configFilePath = filepath.Join(tmpVolumeDir, cfgFile)
		tmpDataDir     = filepath.Join(mockDirectory, "..data_tmp")
		watchedDataDir = filepath.Join(mockDirectory, "..data")
	)

	// wait inotify ready
	if err := os.MkdirAll(tmpVolumeDir, fs.ModePerm); err != nil {
		t.Errorf("failed to create directory: %s", tmpVolumeDir)
	}
	if err := os.WriteFile(configFilePath, []byte(content), fs.ModePerm); err != nil {
		t.Errorf("failed to  write file: %s", configFilePath)
	}
	if err := os.Chmod(configFilePath, fs.ModePerm); err != nil {
		t.Errorf("failed to chmod file: %s", configFilePath)
	}

	pwd, err := os.Getwd()
	if err != nil {
		t.Errorf("failed to Getwd directory")
	}
	defer func() {
		_ = os.Chdir(pwd)
	}()
	if err := os.Chdir(mockDirectory); err != nil {
		t.Errorf("failed to chdir directory: %s", tmpVolumeDir)
	}
	if err := os.Symlink(filepath.Base(tmpVolumeDir), filepath.Base(tmpDataDir)); err != nil {
		t.Errorf("failed to create symbolic link for atomic update: %v", err)
	}
	if err := os.Rename(tmpDataDir, watchedDataDir); err != nil {
		t.Errorf("failed to rename symbolic link for data directory %s: %v", tmpDataDir, err)
	}
	if err := os.Symlink(filepath.Join(filepath.Base(watchedDataDir), cfgFile), cfgFile); err != nil {
		t.Errorf("failed to create symbolic link for atomic update: %v", err)
	}
}
