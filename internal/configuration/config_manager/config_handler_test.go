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
	"encoding/json"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fsnotify/fsnotify"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/configuration/util"
	testutil "github.com/apecloud/kubeblocks/internal/testutil/k8s"
)

var _ = Describe("Config Handler Test", func() {

	var tmpWorkDir string
	var mockK8sCli *testutil.K8sClientMockHelper

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		mockK8sCli = testutil.NewK8sMockClient()
		tmpWorkDir, _ = os.MkdirTemp(os.TempDir(), "test-handle-")
	})

	AfterEach(func() {
		os.RemoveAll(tmpWorkDir)
		DeferCleanup(mockK8sCli.Finish)
	})

	newConfigSpec := func() appsv1alpha1.ComponentConfigSpec {
		return appsv1alpha1.ComponentConfigSpec{
			ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
				Name:        "config",
				TemplateRef: "config-template",
				VolumeName:  "/opt/config",
				Namespace:   "default",
			},
			ConfigConstraintRef: "config-constraint",
		}
	}

	newUnixSignalConfig := func() string {
		configInfos := []ConfigSpecInfo{{
			ReloadOptions: &appsv1alpha1.ReloadOptions{
				UnixSignalTrigger: &appsv1alpha1.UnixSignalTrigger{
					ProcessName: findCurrProcName(),
					Signal:      appsv1alpha1.SIGHUP,
				}},
			ReloadType: appsv1alpha1.UnixSignalType,
			MountPoint: "/tmp/test",
			ConfigSpec: newConfigSpec(),
		}}
		b, err := json.Marshal(configInfos)
		Expect(err).Should(Succeed())
		return string(b)
	}

	Context("TestSimpleHandler", func() {
		It("CreateSignalHandler", func() {
			_, err := CreateSignalHandler(appsv1alpha1.SIGALRM, "test", "")
			Expect(err).Should(Succeed())
			_, err = CreateSignalHandler("NOSIGNAL", "test", "")
			Expect(err.Error()).To(ContainSubstring("not supported unix signal: NOSIGNAL"))
		})

		It("CreateShellHandler", func() {
			_, err := CreateExecHandler(nil, "", nil, "")
			Expect(err.Error()).To(ContainSubstring("invalid command"))
			_, err = CreateExecHandler([]string{}, "", nil, "")
			Expect(err.Error()).To(ContainSubstring("invalid command"))
			c, err := CreateExecHandler([]string{"go", "version"}, "", nil, "")
			Expect(err).Should(Succeed())
			Expect(c.VolumeHandle(context.Background(), fsnotify.Event{})).Should(Succeed())
		})

		It("CreateTPLScriptHandler", func() {
			mockK8sTestConfigureDirectory(filepath.Join(tmpWorkDir, "config"), "my.cnf", "xxxx")
			tplFile := filepath.Join(tmpWorkDir, "test.tpl")
			configFile := filepath.Join(tmpWorkDir, "config.yaml")
			Expect(os.WriteFile(tplFile, []byte(``), fs.ModePerm)).Should(Succeed())

			tplConfig := TPLScriptConfig{Scripts: "test.tpl"}
			b, _ := util.ToYamlConfig(tplConfig)
			Expect(os.WriteFile(configFile, b, fs.ModePerm)).Should(Succeed())

			_, err := CreateTPLScriptHandler("", configFile, []string{filepath.Join(tmpWorkDir, "config")}, "")
			Expect(err).Should(Succeed())
		})

	})

	Context("TestConfigHandler", func() {
		It("SignalHandler", func() {
			config := newUnixSignalConfig()
			handler, err := CreateCombinedHandler(config, filepath.Join(tmpWorkDir, "backup"))
			Expect(err).Should(Succeed())
			// Expect(handler.MountPoint()).Should(BeEquivalentTo(""))

			// process unix signal
			trigger := make(chan bool)
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGHUP)
			defer stop()

			go func() {
				select {
				case <-time.After(10 * time.Second):
					// not walk here
					Expect(true).Should(BeFalse())
				case <-ctx.Done():
					// prints "context canceled"
					stop()
					trigger <- true
				}
			}()
			By("process unix signal")
			Expect(handler.VolumeHandle(ctx, fsnotify.Event{Name: "/tmp/test"})).Should(Succeed())

			select {
			case <-time.After(20 * time.Second):
				logger.Info("failed to watch volume.")
				Expect(true).Should(BeFalse())
			case <-trigger:
				logger.Info("success to watch volume.")
				Expect(true).To(BeTrue())
			}
		})

		It("ShellHandler", func() {
		})

		It("TplScriptsHandler", func() {
		})

		It("DownwardAPIsHandler", func() {
		})

		It("MultiHandler", func() {
		})
	})
})

func mockK8sTestConfigureDirectory(mockDirectory string, cfgFile, content string) {
	var (
		tmpVolumeDir   = filepath.Join(mockDirectory, "..2023_06_16_06_06_06.1234567")
		configFilePath = filepath.Join(tmpVolumeDir, cfgFile)
		tmpDataDir     = filepath.Join(mockDirectory, "..data_tmp")
		watchedDataDir = filepath.Join(mockDirectory, "..data")
	)

	// wait inotify ready
	Expect(os.MkdirAll(tmpVolumeDir, fs.ModePerm)).Should(Succeed())
	Expect(os.WriteFile(configFilePath, []byte(content), fs.ModePerm)).Should(Succeed())
	Expect(os.Chmod(configFilePath, fs.ModePerm)).Should(Succeed())

	pwd, err := os.Getwd()
	Expect(err).Should(Succeed())
	defer func() {
		_ = os.Chdir(pwd)
	}()

	Expect(os.Chdir(mockDirectory))
	Expect(os.Symlink(filepath.Base(tmpVolumeDir), filepath.Base(tmpDataDir))).Should(Succeed())
	Expect(os.Rename(tmpDataDir, watchedDataDir)).Should(Succeed())
	Expect(os.Symlink(filepath.Join(filepath.Base(watchedDataDir), cfgFile), cfgFile)).Should(Succeed())
}
