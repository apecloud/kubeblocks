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
	"github.com/apecloud/kubeblocks/pkg/configuration/util"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("Config Handler Test", func() {

	var tmpWorkDir string
	var mockK8sCli *testutil.K8sClientMockHelper

	const (
		oldVersion = "[test]\na = 1\nb = 2\n"
		newVersion = "[test]\na = 2\nb = 2\n\nc = 100"
	)

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

	newFormatter := func() appsv1alpha1.FormatterConfig {
		return appsv1alpha1.FormatterConfig{
			FormatterOptions: appsv1alpha1.FormatterOptions{
				IniConfig: &appsv1alpha1.IniConfig{
					SectionName: "test",
				},
			},
			Format: appsv1alpha1.Ini,
		}
	}

	newUnixSignalConfig := func() ConfigSpecInfo {
		return ConfigSpecInfo{
			ReloadOptions: &appsv1alpha1.ReloadOptions{
				UnixSignalTrigger: &appsv1alpha1.UnixSignalTrigger{
					ProcessName: findCurrProcName(),
					Signal:      appsv1alpha1.SIGHUP,
				}},
			ReloadType: appsv1alpha1.UnixSignalType,
			MountPoint: "/tmp/test",
			ConfigSpec: newConfigSpec(),
		}
	}

	newShellConfig := func(mountPoint string) ConfigSpecInfo {
		return ConfigSpecInfo{
			ReloadOptions: &appsv1alpha1.ReloadOptions{
				ShellTrigger: &appsv1alpha1.ShellTrigger{
					Command: []string{"sh", "-c", `echo "hello world" "$@"`},
				}},
			ReloadType:      appsv1alpha1.ShellType,
			MountPoint:      mountPoint,
			ConfigSpec:      newConfigSpec(),
			FormatterConfig: newFormatter(),
		}
	}

	newDownwardAPIOptions := func() []appsv1alpha1.DownwardAPIOption {
		return []appsv1alpha1.DownwardAPIOption{
			{
				Name:       "labels",
				MountPoint: filepath.Join(tmpWorkDir, "labels"),
				Command:    []string{"sh", "-c", `echo "labels trigger"`},
			},
			{
				Name:       "annotations",
				MountPoint: filepath.Join(tmpWorkDir, "annotations"),
				Command:    []string{"sh", "-c", `echo "annotation trigger"`},
			},
		}
	}

	newDownwardAPIConfig := func() ConfigSpecInfo {
		return ConfigSpecInfo{
			ReloadOptions: &appsv1alpha1.ReloadOptions{
				ShellTrigger: &appsv1alpha1.ShellTrigger{
					Command: []string{"sh", "-c", `echo "hello world" "$@"`},
				},
			},
			ReloadType:         appsv1alpha1.ShellType,
			MountPoint:         tmpWorkDir,
			ConfigSpec:         newConfigSpec(),
			FormatterConfig:    newFormatter(),
			DownwardAPIOptions: newDownwardAPIOptions(),
		}
	}

	newTPLScriptsConfig := func(configPath string) ConfigSpecInfo {
		return ConfigSpecInfo{
			ReloadOptions: &appsv1alpha1.ReloadOptions{
				TPLScriptTrigger: &appsv1alpha1.TPLScriptTrigger{},
			},
			ReloadType:      appsv1alpha1.TPLScriptType,
			MountPoint:      "/tmp/test",
			ConfigSpec:      newConfigSpec(),
			FormatterConfig: newFormatter(),
			TPLConfig:       configPath,
		}
	}

	prepareTestConfig := func(configPath string, config string) {
		fileInfo, err := os.Stat(configPath)
		if err != nil {
			Expect(os.IsNotExist(err)).To(BeTrue())
		}
		if fileInfo == nil {
			Expect(os.MkdirAll(configPath, fs.ModePerm)).Should(Succeed())
		}
		Expect(os.WriteFile(filepath.Join(configPath, "my.cnf"), []byte(config), fs.ModePerm)).Should(Succeed())
	}

	toJSONString := func(v ConfigSpecInfo) string {
		b, err := util.ToYamlConfig([]ConfigSpecInfo{v})
		Expect(err).Should(Succeed())
		configFile := filepath.Join(tmpWorkDir, configManagerConfig)
		Expect(os.WriteFile(configFile, b, fs.ModePerm)).Should(Succeed())
		return configFile
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
			c, err := CreateExecHandler([]string{"go", "version"}, "", &ConfigSpecInfo{
				ConfigSpec: appsv1alpha1.ComponentConfigSpec{
					ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
						Name: "for_test",
					}}},
				"")
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
			handler, err := CreateCombinedHandler(toJSONString(config), filepath.Join(tmpWorkDir, "backup"))
			Expect(err).Should(Succeed())
			Expect(handler.MountPoint()).Should(ContainElement(config.MountPoint))

			// process unix signal
			trigger := make(chan bool)
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGHUP)
			defer stop()

			go func() {
				select {
				case <-time.After(5 * time.Second):
					// not walk here
					Expect(true).Should(BeFalse())
				case <-ctx.Done():
					stop()
					trigger <- true
				}
			}()
			By("process unix signal")
			Expect(handler.VolumeHandle(ctx, fsnotify.Event{Name: config.MountPoint})).Should(Succeed())

			select {
			case <-time.After(10 * time.Second):
				logger.Info("failed to watch volume.")
				Expect(true).Should(BeFalse())
			case <-trigger:
				logger.Info("success to watch volume.")
				Expect(true).To(BeTrue())
			}

			By("not support handler")
			Expect(handler.OnlineUpdate(ctx, "", nil).Error()).Should(ContainSubstring("not found handler for config name"))
			Expect(handler.OnlineUpdate(ctx, config.ConfigSpec.Name, nil).Error()).Should(ContainSubstring("not support online update"))
			By("not match mount point")
			Expect(handler.VolumeHandle(ctx, fsnotify.Event{Name: "not_exist_mount_point"})).Should(Succeed())
		})

		It("ShellHandler", func() {
			configPath := filepath.Join(tmpWorkDir, "config")
			prepareTestConfig(configPath, oldVersion)
			config := newShellConfig(configPath)
			handler, err := CreateCombinedHandler(toJSONString(config), filepath.Join(tmpWorkDir, "backup"))
			Expect(err).Should(Succeed())
			Expect(handler.MountPoint()).Should(ContainElement(configPath))

			// mock modify config
			prepareTestConfig(configPath, newVersion)
			By("change config")
			Expect(handler.VolumeHandle(context.TODO(), fsnotify.Event{Name: configPath})).Should(Succeed())
			By("not change config")
			Expect(handler.VolumeHandle(context.TODO(), fsnotify.Event{Name: configPath})).Should(Succeed())
			By("not support onlineUpdate")
			Expect(handler.OnlineUpdate(context.TODO(), config.ConfigSpec.Name, nil)).Should(Succeed())
		})

		It("TplScriptsHandler", func() {
			By("mock command channel")
			newCommandChannel = func(ctx context.Context, dataType, dsn string) (DynamicParamUpdater, error) {
				return mockCChannel, nil
			}

			tplFile := filepath.Join(tmpWorkDir, "test.tpl")
			configFile := filepath.Join(tmpWorkDir, "config.yaml")
			Expect(os.WriteFile(tplFile, []byte(``), fs.ModePerm)).Should(Succeed())

			tplConfig := TPLScriptConfig{
				Scripts:         "test.tpl",
				FormatterConfig: newFormatter(),
			}
			b, _ := util.ToYamlConfig(tplConfig)
			Expect(os.WriteFile(configFile, b, fs.ModePerm)).Should(Succeed())

			config := newTPLScriptsConfig(configFile)
			handler, err := CreateCombinedHandler(toJSONString(config), "")
			Expect(err).Should(Succeed())
			Expect(handler.OnlineUpdate(context.TODO(), config.ConfigSpec.Name, map[string]string{
				"param_a": "a",
				"param_b": "b",
			})).Should(Succeed())
		})

		It("TplScriptsHandler Volume Event", func() {
			By("mock command channel")
			newCommandChannel = func(ctx context.Context, dataType, dsn string) (DynamicParamUpdater, error) {
				return mockCChannel, nil
			}

			By("prepare config data")
			configPath := filepath.Join(tmpWorkDir, "config")
			prepareTestConfig(configPath, oldVersion)

			tplFile := filepath.Join(tmpWorkDir, "test.tpl")
			configFile := filepath.Join(tmpWorkDir, "config.yaml")
			Expect(os.WriteFile(tplFile, []byte(``), fs.ModePerm)).Should(Succeed())

			tplConfig := TPLScriptConfig{
				Scripts:         "test.tpl",
				FormatterConfig: newFormatter(),
			}
			b, _ := util.ToYamlConfig(tplConfig)
			Expect(os.WriteFile(configFile, b, fs.ModePerm)).Should(Succeed())

			config := newTPLScriptsConfig(configFile)
			config.MountPoint = configPath
			handler, err := CreateCombinedHandler(toJSONString(config), filepath.Join(tmpWorkDir, "backup"))
			Expect(err).Should(Succeed())

			By("change config")
			prepareTestConfig(configPath, newVersion)
			Expect(handler.VolumeHandle(context.TODO(), fsnotify.Event{Name: configPath})).Should(Succeed())

		})

		It("DownwardAPIsHandler", func() {
			config := newDownwardAPIConfig()
			handler, err := CreateCombinedHandler(toJSONString(config), filepath.Join(tmpWorkDir, "backup"))
			Expect(err).Should(Succeed())
			Expect(handler.MountPoint()).Should(ContainElement(config.MountPoint))
			Expect(handler.VolumeHandle(context.TODO(), fsnotify.Event{Name: config.DownwardAPIOptions[0].MountPoint})).Should(Succeed())
			Expect(handler.VolumeHandle(context.TODO(), fsnotify.Event{Name: config.DownwardAPIOptions[1].MountPoint})).Should(Succeed())
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

type mockCommandChannel struct {
}

func (m *mockCommandChannel) ExecCommand(ctx context.Context, command string, args ...string) (string, error) {
	return "", nil
}

func (m *mockCommandChannel) Close() {
}

var mockCChannel = &mockCommandChannel{}
