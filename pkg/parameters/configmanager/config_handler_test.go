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

package configmanager

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/parameters/util"
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

	newConfigSpec := func() appsv1.ComponentFileTemplate {
		return appsv1.ComponentFileTemplate{
			Name:       "config",
			Template:   "config-template",
			VolumeName: "/opt/config",
			Namespace:  "default",
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
		It("CreateShellHandler", func() {
			_, err := createShellHandler(nil, nil)
			Expect(err.Error()).To(ContainSubstring("invalid command"))
			_, err = createShellHandler([]string{}, nil)
			Expect(err.Error()).To(ContainSubstring("invalid command"))
			_, err = createShellHandler([]string{"go", "version"}, &ConfigSpecInfo{
				ConfigSpec: appsv1.ComponentFileTemplate{
					Name: "for_test",
				}})
			Expect(err).Should(Succeed())
		})
	})

	Context("TestConfigHandler", func() {
		Describe("Test ShellHandler", func() {
			var configPath string
			testShellHandlerCommon := func(configPath string, configSpec ConfigSpecInfo) {
				handler, err := CreateCombinedHandler(toJSONString(configSpec))
				Expect(err).Should(Succeed())
				By("change config", func() {
					// mock modify config
					prepareTestConfig(configPath, newVersion)
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
					ConfigSpec:      newConfigSpec(),
					FormatterConfig: newFormatter(),
				}
				testShellHandlerCommon(configPath, configSpec)
			})
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
	})
})
