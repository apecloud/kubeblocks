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
	"reflect"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/gotemplate"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

var zapLog, _ = zap.NewDevelopment()

func TestCreateUpdatedParamsPatch(t *testing.T) {
	zapLog, _ = zap.NewDevelopment()
	SetLogger(zapLog)

	rootPath := prepareTestData(t, "lastVersion", "currentVersion")
	defer os.RemoveAll(rootPath)

	type args struct {
		newVersion string
		oldVersion string
		formatCfg  *parametersv1alpha1.FileFormatConfig
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr bool
	}{{
		name: "update_params_patch_test",
		args: args{
			newVersion: filepath.Join(rootPath, "currentVersion"),
			oldVersion: filepath.Join(rootPath, "lastVersion"),
			formatCfg: &parametersv1alpha1.FileFormatConfig{
				Format: parametersv1alpha1.Ini,
				FormatterAction: parametersv1alpha1.FormatterAction{IniConfig: &parametersv1alpha1.IniConfig{
					SectionName: "mysqld",
				}},
			}},
		wantErr: false,
		want:    testapps.WithMap("max_connections", "666", "key_buffer_size", "128M"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, _ := createFileRegex("")
			newVer, _ := scanConfigFiles([]string{tt.args.newVersion}, f)
			oldVer, _ := scanConfigFiles([]string{tt.args.oldVersion}, f)
			got, err := createUpdatedParamsPatch(newVer, oldVer, tt.args.formatCfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("createUpdatedParamsPatch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("createUpdatedParamsPatch() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func prepareTestData(t *testing.T, dir1 string, dir2 string) string {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "reload-test-")
	require.Nil(t, err)

	v1 := `
[mysqld]
key_buffer_size=16777216
max_connections=2666
authentication_policy=mysql_native_password,
back_log=5285
binlog_cache_size=32768
binlog_format=MIXED
binlog_order_commits=ON
binlog_row_image=FULL
connect_timeout=10
`
	v2 := `
[mysqld]
authentication_policy=mysql_native_password,
back_log=5285
binlog_cache_size=32768
binlog_format=MIXED
binlog_order_commits=ON
binlog_row_image=FULL
connect_timeout=10
key_buffer_size=128M
max_connections=666
`

	testFileName := "test.conf"
	require.Nil(t, os.MkdirAll(filepath.Join(tmpDir, dir1), fs.ModePerm))
	require.Nil(t, os.MkdirAll(filepath.Join(tmpDir, dir2), fs.ModePerm))

	require.Nil(t, os.WriteFile(filepath.Join(tmpDir, dir1, testFileName), []byte(v1), fs.ModePerm))
	require.Nil(t, os.WriteFile(filepath.Join(tmpDir, dir2, testFileName), []byte(v2), fs.ModePerm))
	return tmpDir
}

var _ = Describe("ReloadUtil Test", func() {

	const configFile = "my.cnf"

	var tmpDir string

	BeforeEach(func() {
		// Add any setup steps that needs to be executed before each test
		tmpDir, _ = os.MkdirTemp(os.TempDir(), "test-")
	})

	AfterEach(func() {
	})

	createIniFormatter := func(sectionName string) *parametersv1alpha1.FileFormatConfig {
		return &parametersv1alpha1.FileFormatConfig{
			FormatterAction: parametersv1alpha1.FormatterAction{
				IniConfig: &parametersv1alpha1.IniConfig{
					SectionName: sectionName,
				}},
			Format: parametersv1alpha1.Ini,
		}
	}

	prepareTestConfig := func(configPath string, config string) {
		fileInfo, err := os.Stat(configPath)
		if err != nil {
			Expect(os.IsNotExist(err)).Should(BeTrue())
		}
		if fileInfo == nil {
			Expect(os.MkdirAll(configPath, fs.ModePerm)).Should(Succeed())
		}
		Expect(os.WriteFile(filepath.Join(configPath, configFile), []byte(config), fs.ModePerm)).Should(Succeed())
	}

	Context("TestScanConfigVolume", func() {
		It(`test scan config volume`, func() {
			mockK8sTestConfigureDirectory(tmpDir, "test.conf", "empty!!!")
			files, err := ScanConfigVolume(tmpDir)
			Expect(err).Should(Succeed())
			Expect(1).Should(Equal(len(files)))
			Expect("test.conf").Should(BeEquivalentTo(filepath.Base(files[0])))

			By("test regex filter")
			filter, _ := createFileRegex("test.conf")
			files2, err := scanConfigFiles([]string{tmpDir}, filter)
			Expect(err).Should(Succeed())
			Expect(files2).Should(BeEquivalentTo(files))
		})
	})

	Context("TestReloadBuiltinFunctions", func() {
		It("test build-in exec", func() {
			engine := gotemplate.NewTplEngine(nil,
				constructReloadBuiltinFuncs(ctx, &mockCommandChannel{}, createIniFormatter("test")), "for_test", nil, nil)
			_, err := engine.Render(`{{ exec "sh" "-c" "echo \"hello world\" " }}`)
			Expect(err).Should(Succeed())
		})

		It("test build-in execSql", func() {
			tpl := `
{{- range $pk, $pv := $.arg0 }}
	{{- execSql ( printf "SET GLOBAL %s = %d" $pk $pv ) }}
{{- end }}
`

			values := gotemplate.ConstructFunctionArgList(map[string]string{
				"key_buffer_size": "128M",
			})
			engine := gotemplate.NewTplEngine(&values, constructReloadBuiltinFuncs(ctx, &mockCommandChannel{}, createIniFormatter("test")), "for_test", nil, nil)
			_, err := engine.Render(tpl)
			Expect(err).Should(Succeed())

		})

		It("test build-in patchParams", func() {
			baseConfig := "[test]\na = 1\nb = 2\n"

			params := map[string]string{
				"key1": "128M",
				"key2": "512M",
			}

			baseFile := filepath.Join(tmpDir, "config", configFile)
			targetFile := filepath.Join(tmpDir, "new_config", configFile)
			prepareTestConfig(filepath.Dir(baseFile), baseConfig)
			prepareTestConfig(filepath.Dir(targetFile), baseConfig)

			values := gotemplate.ConstructFunctionArgList(params)
			engine := gotemplate.NewTplEngine(&values, constructReloadBuiltinFuncs(context.TODO(), nil, createIniFormatter("test")), "for_test", nil, nil)
			_, err := engine.Render(fmt.Sprintf("{{- patchParams $.arg0 \"%s\" \"%s\" }}", baseFile, targetFile))
			Expect(err).Should(Succeed())
			b, _ := os.ReadFile(targetFile)
			Expect("[test]\na=1\nb=2\nkey1=128M\nkey2=512M\n").Should(BeEquivalentTo(string(b)))
		})
	})
})
