/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fsnotify/fsnotify"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/util"
	testutil "github.com/apecloud/kubeblocks/pkg/testutil/k8s"
)

var _ = Describe("Config Handler Test", func() {

	var tmpWorkDir string
	var mockK8sCli *testutil.K8sClientMockHelper

	const (
		oldVersion = "[test]\na = 1\nb = 2\n"
		newVersion = "[test]\na = 2\nb = 2\n\nc = 100"

		defaultBatchInputTemplate string = `{{- range $pKey, $pValue := $ }}
{{ printf "%s=%s" $pKey $pValue }}
{{- end }}`
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

	newConfigSpec := func() appsv1.ComponentTemplateSpec {
		return appsv1.ComponentTemplateSpec{
			Name:        "config",
			TemplateRef: "config-template",
			VolumeName:  "/opt/config",
			Namespace:   "default",
		}
	}

	newFormatter := func() parametersv1alpha1.FileFormatConfig {
		return parametersv1alpha1.FileFormatConfig{
			FormatterAction: parametersv1alpha1.FormatterAction{
				IniConfig: &parametersv1alpha1.IniConfig{
					SectionName: "test",
				},
			},
			Format: parametersv1alpha1.Ini,
		}
	}

	newUnixSignalConfig := func() ConfigSpecInfo {
		return ConfigSpecInfo{
			ReloadAction: &parametersv1alpha1.ReloadAction{
				UnixSignalTrigger: &parametersv1alpha1.UnixSignalTrigger{
					ProcessName: findCurrProcName(),
					Signal:      parametersv1alpha1.SIGHUP,
				}},
			ReloadType: parametersv1alpha1.UnixSignalType,
			MountPoint: "/tmp/test",
			ConfigSpec: newConfigSpec(),
		}
	}

	newDownwardAPIOptions := func() []parametersv1alpha1.DownwardAPIChangeTriggeredAction {
		return []parametersv1alpha1.DownwardAPIChangeTriggeredAction{
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
			ReloadAction: &parametersv1alpha1.ReloadAction{
				ShellTrigger: &parametersv1alpha1.ShellTrigger{
					Command: []string{"sh", "-c", `echo "hello world" "$@"`},
				},
			},
			ReloadType:         parametersv1alpha1.ShellType,
			MountPoint:         tmpWorkDir,
			ConfigSpec:         newConfigSpec(),
			FormatterConfig:    newFormatter(),
			DownwardAPIOptions: newDownwardAPIOptions(),
		}
	}

	newTPLScriptsConfig := func(configPath string) ConfigSpecInfo {
		return ConfigSpecInfo{
			ReloadAction: &parametersv1alpha1.ReloadAction{
				TPLScriptTrigger: &parametersv1alpha1.TPLScriptTrigger{},
			},
			ReloadType:      parametersv1alpha1.TPLScriptType,
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
			_, err := CreateSignalHandler(parametersv1alpha1.SIGALRM, "test", "")
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
				ConfigSpec: appsv1.ComponentTemplateSpec{
					Name: "for_test",
				}},
				"")
			Expect(err).Should(Succeed())
			Expect(c.VolumeHandle(context.Background(), fsnotify.Event{})).Should(Succeed())
		})

		It("CreateTPLScriptHandler", func() {
			mockK8sTestConfigureDirectory(filepath.Join(tmpWorkDir, "config"), "my.cnf", "xxxx")
			tplFile := filepath.Join(tmpWorkDir, "test.tpl")
			configFile := filepath.Join(tmpWorkDir, "config.yaml")
			Expect(os.WriteFile(tplFile, []byte(``), fs.ModePerm)).Should(Succeed())

			os.Setenv("MYSQL_USER", "admin")
			os.Setenv("MYSQL_PASSWORD", "admin")
			tplConfig := TPLScriptConfig{Scripts: "test.tpl", DSN: `{%- expandenv "${MYSQL_USER}:${MYSQL_PASSWORD}@(localhost:3306)/" | trim %}`}
			b, _ := util.ToYamlConfig(tplConfig)
			Expect(os.WriteFile(configFile, b, fs.ModePerm)).Should(Succeed())

			handler, err := CreateTPLScriptHandler("", configFile, []string{filepath.Join(tmpWorkDir, "config")}, "")
			Expect(err).Should(Succeed())
			tplHandler := handler.(*tplScriptHandler)
			Expect(tplHandler.dsn).Should(BeEquivalentTo("admin:admin@(localhost:3306)/"))
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

		Describe("Test ShellHandler", func() {
			var configPath string
			testShellHandlerCommon := func(configPath string, configSpec ConfigSpecInfo) {
				handler, err := CreateCombinedHandler(toJSONString(configSpec), filepath.Join(tmpWorkDir, "backup"))
				Expect(err).Should(Succeed())
				Expect(handler.MountPoint()).Should(ContainElement(configPath))
				By("change config", func() {
					// mock modify config
					prepareTestConfig(configPath, newVersion)
					Expect(handler.VolumeHandle(context.TODO(), fsnotify.Event{Name: configPath})).Should(Succeed())
				})
				By("not change config", func() {
					Expect(handler.VolumeHandle(context.TODO(), fsnotify.Event{Name: configPath})).Should(Succeed())
				})
				By("not support onlineUpdate", func() {
					Expect(handler.OnlineUpdate(context.TODO(), configSpec.ConfigSpec.Name, nil)).Should(Succeed())
				})
			}
			BeforeEach(func() {
				configPath = filepath.Join(tmpWorkDir, "config")
				prepareTestConfig(configPath, oldVersion)
			})
			It("should succeed on reload individually", func() {
				configSpec := ConfigSpecInfo{
					ReloadAction: &parametersv1alpha1.ReloadAction{
						ShellTrigger: &parametersv1alpha1.ShellTrigger{
							Command: []string{"sh", "-c", `echo "hello world" "$@"`, "sh"},
						}},
					ReloadType:      parametersv1alpha1.ShellType,
					MountPoint:      configPath,
					ConfigSpec:      newConfigSpec(),
					FormatterConfig: newFormatter(),
				}
				testShellHandlerCommon(configPath, configSpec)
			})
			Describe("Test reload in a batch", func() {
				It("should succeed on the default batch input format", func() {
					configSpec := ConfigSpecInfo{
						ReloadAction: &parametersv1alpha1.ReloadAction{
							ShellTrigger: &parametersv1alpha1.ShellTrigger{
								Command: []string{"sh", "-c",
									`while IFS="=" read -r the_key the_val; do echo "key='$the_key'; val='$the_val'"; done`,
								},
								BatchReload:                  util.ToPointer(true),
								BatchParamsFormatterTemplate: defaultBatchInputTemplate,
							}},
						ReloadType:      parametersv1alpha1.ShellType,
						MountPoint:      configPath,
						ConfigSpec:      newConfigSpec(),
						FormatterConfig: newFormatter(),
					}
					testShellHandlerCommon(configPath, configSpec)
				})
				It("should succeed on the custom batch input format", func() {
					customBatchInputTemplate := `{{- range $pKey, $pValue := $ }}
{{ printf "%s:%s" $pKey $pValue }}
{{- end }}`
					configSpec := ConfigSpecInfo{
						ReloadAction: &parametersv1alpha1.ReloadAction{
							ShellTrigger: &parametersv1alpha1.ShellTrigger{
								Command: []string{"sh", "-c",
									`while IFS=":" read -r the_key the_val; do echo "key='$the_key'; val='$the_val'"; done`,
								},
								BatchReload:                  util.ToPointer(true),
								BatchParamsFormatterTemplate: customBatchInputTemplate,
							}},
						ReloadType:      parametersv1alpha1.ShellType,
						MountPoint:      configPath,
						ConfigSpec:      newConfigSpec(),
						FormatterConfig: newFormatter(),
					}
					testShellHandlerCommon(configPath, configSpec)
				})
			})
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

	Describe("Test exec reload command", func() {
		updatedParams := map[string]string{
			"key2": "val2",
			"key1": "val1",
			"key3": "",
		}
		Describe("Test execute command with separate reload", func() {
			var (
				stdouts []string
				err     error
			)
			BeforeEach(func() {
				err = doReloadAction(context.TODO(),
					updatedParams,
					func(output string, _ error) {
						stdouts = append(stdouts, output)
					},
					"echo",
					"hello",
				)
			})
			It("should execute the script successfully and have correct stdout content", func() {
				By("checking that should execute the script successfully", func() {
					Expect(err).Should(Succeed())
				})
				By("checking that should have correct stdout content", func() {
					Expect(len(stdouts)).Should(Equal(len(updatedParams)))
					// sort the result stdout and the iteratation key sequence
					keys := make([]string, 0, len(updatedParams))
					for k := range updatedParams {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					sort.Strings(stdouts)
					for i, theStdout := range stdouts {
						k := keys[i]
						v := updatedParams[k]
						Expect(theStdout).Should(Equal(fmt.Sprintf("hello %s %s\n", k, v)))
					}
				})
			})
		})
		Describe("Test execute command with batch reload", func() {
			var (
				stdout string
				err    error
			)
			BeforeEach(func() {
				err = doBatchReloadAction(context.TODO(),
					updatedParams,
					func(out string, err error) {
						stdout = out
					},
					defaultBatchInputTemplate,
					"/bin/sh",
					"-c",
					`while IFS="=" read -r the_key the_val; do echo "key='$the_key'; val='$the_val'"; done`,
				)
			})
			It("should execute the script successfully and have correct stdout content", func() {
				By("checking that should execute the script successfully", func() {
					Expect(err).Should(Succeed())
				})
				By("checking that should have correct stdout content", func() {
					keys := make([]string, 0, len(updatedParams))
					for k := range updatedParams {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					var expectOutputSB strings.Builder
					for _, k := range keys { // iterate the map in sorted order
						v := updatedParams[k]
						expectOutputSB.WriteString(fmt.Sprintf("key='%s'; val='%s'\n", k, v))
					}
					Expect(stdout).Should(Equal(expectOutputSB.String()))
				})
			})
		})
	})

	Describe("Test generate batch stdin data", func() {
		It("should pass the base scenario", func() {
			// deliberately make the keys unsorted
			updatedParams := map[string]string{
				"key2": "val2",
				"key1": "val1",
				"key3": "",
			}
			// According to 'https://pkg.go.dev/text/template' :
			// For `range`, if the value is a map and the keys can be sorted, the elements will be visited in sorted key order.
			batchInputTemplate := `{{- range $pKey, $pValue := $ }}
{{ printf "%s:%s" $pKey $pValue }}
{{- end }}`
			stdinStr, err := generateBatchStdinData(context.TODO(), updatedParams, batchInputTemplate)
			By("checking there's no error", func() {
				Expect(err).Should(Succeed())
			})
			By("checking the generated content is correct", func() {
				keys := make([]string, 0, len(updatedParams))
				for k := range updatedParams {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				var expectOutputSB strings.Builder
				for _, k := range keys { // iterate the map in sorted order
					v := updatedParams[k]
					expectOutputSB.WriteString(fmt.Sprintf("%s:%s\n", k, v))
				}

				Expect(stdinStr).Should(Equal(expectOutputSB.String()))
			})
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
