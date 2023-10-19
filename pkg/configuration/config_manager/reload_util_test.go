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
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/gotemplate"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

func TestCreateUpdatedParamsPatch(t *testing.T) {
	zapLog, _ = zap.NewDevelopment()
	SetLogger(zapLog)

	rootPath := prepareTestData(t, "lastVersion", "currentVersion")
	defer os.RemoveAll(rootPath)

	type args struct {
		newVersion string
		oldVersion string
		formatCfg  *appsv1alpha1.FormatterConfig
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
			formatCfg: &appsv1alpha1.FormatterConfig{
				Format: appsv1alpha1.Ini,
				FormatterOptions: appsv1alpha1.FormatterOptions{IniConfig: &appsv1alpha1.IniConfig{
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

func TestOnlineUpdateParamsHandle(t *testing.T) {
	server := mockRestAPIServer(t)
	defer server.Close()

	partroniPath := "reload.tpl"
	tmpTestData := mockPatroniTestData(t, partroniPath)
	defer os.RemoveAll(tmpTestData)

	type args struct {
		tplScriptPath string
		formatConfig  *appsv1alpha1.FormatterConfig
		dataType      string
		dsn           string
	}
	tests := []struct {
		name    string
		args    args
		want    DynamicUpdater
		wantErr bool
	}{{
		name: "online_update_params_handle_test",
		args: args{
			tplScriptPath: filepath.Join(tmpTestData, partroniPath),
			formatConfig: &appsv1alpha1.FormatterConfig{
				Format: appsv1alpha1.Properties,
			},
			dsn:      server.URL,
			dataType: "patroni",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := OnlineUpdateParamsHandle(tt.args.tplScriptPath, tt.args.formatConfig, tt.args.dataType, tt.args.dsn)
			if (err != nil) != tt.wantErr {
				t.Errorf("OnlineUpdateParamsHandle() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			err = got(context.Background(), "", map[string]string{"key_buffer_size": "128M", "max_connections": "666"})
			require.Nil(t, err)
		})
	}
}

func mockRestAPIServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		default:
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write([]byte(`failed`))
			require.Nil(t, err)
		case "/config", "/restart":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`success`))
			require.Nil(t, err)
		}
	}))
}

func mockPatroniTestData(t *testing.T, reloadScript string) string {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "pg-patroni-test-")
	require.Nil(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, reloadScript), []byte(`
{{- $bootstrap := $.Files.Get "bootstrap.yaml" | fromYamlArray }}
{{- $command := "reload" }}
{{- range $pk, $_ := $.arg0 }}
    {{- if has $pk $bootstrap  }}
        {{- $command = "restart" }}
        {{ break }}
    {{- end }}
{{- end }}
{{ $params := dict "parameters" $.arg0 }}
{{- $err := execSql ( dict "postgresql" $params | toJson ) "config" }}
{{- if $err }}
    {{- failed $err }}
{{- end }}
{{- $err := execSql "" $command }}
{{- if $err }}
    {{- failed $err }}
{{- end }}
`), fs.ModePerm)
	require.Nil(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "bootstrap.yaml"), []byte(`
- archive_mode
- autovacuum_freeze_max_age
- autovacuum_max_workers
- max_connections
`), fs.ModePerm)
	require.Nil(t, err)

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

	createIniFormatter := func(sectionName string) *appsv1alpha1.FormatterConfig {
		return &appsv1alpha1.FormatterConfig{
			FormatterOptions: appsv1alpha1.FormatterOptions{
				IniConfig: &appsv1alpha1.IniConfig{
					SectionName: sectionName,
				}},
			Format: appsv1alpha1.Ini,
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

	Context("NeedSharedProcessNamespace", func() {
		It("Should success with no error", func() {
			tests := []struct {
				name string
				args []ConfigSpecMeta
				want bool
			}{{
				name: "test1",
				args: []ConfigSpecMeta{},
				want: false,
			}, {
				name: "test2",
				args: []ConfigSpecMeta{{
					ConfigSpecInfo: ConfigSpecInfo{
						ConfigSpec: appsv1alpha1.ComponentConfigSpec{
							ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
								Name:        "test",
								TemplateRef: "test_cm",
							}},
					}},
				},
				want: false,
			}, {
				name: "test3",
				args: []ConfigSpecMeta{{
					ConfigSpecInfo: ConfigSpecInfo{
						ConfigSpec: appsv1alpha1.ComponentConfigSpec{
							ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
								Name:        "test",
								TemplateRef: "test_cm",
							},
							ConfigConstraintRef: "cc2",
						},
						ReloadType: appsv1alpha1.ShellType,
					},
				}, {
					ConfigSpecInfo: ConfigSpecInfo{
						ConfigSpec: appsv1alpha1.ComponentConfigSpec{
							ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
								Name:        "test2",
								TemplateRef: "test_cm",
							},
							ConfigConstraintRef: "cc3",
						},
						ReloadType: appsv1alpha1.TPLScriptType,
					}}},
				want: false,
			}, {
				name: "test4",
				args: []ConfigSpecMeta{{
					ConfigSpecInfo: ConfigSpecInfo{
						ConfigSpec: appsv1alpha1.ComponentConfigSpec{
							ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
								Name:        "test",
								TemplateRef: "test_cm",
							},
							ConfigConstraintRef: "cc1",
						},
						ReloadType: appsv1alpha1.UnixSignalType,
					},
				}, {
					ConfigSpecInfo: ConfigSpecInfo{
						ConfigSpec: appsv1alpha1.ComponentConfigSpec{
							ComponentTemplateSpec: appsv1alpha1.ComponentTemplateSpec{
								Name:        "test2",
								TemplateRef: "test_cm",
							},
							ConfigConstraintRef: "cc3",
						},
						ReloadType: appsv1alpha1.TPLScriptType,
					}}},
				want: true,
			}}
			for _, tt := range tests {
				got := NeedSharedProcessNamespace(tt.args)
				Expect(got).Should(BeEquivalentTo(tt.want))
			}

		})
	})
})
