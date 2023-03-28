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
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	testapps "github.com/apecloud/kubeblocks/internal/testutil/apps"
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

func TestConstructReloadBuiltinFuncs(t *testing.T) {
	require.NotNil(t, constructReloadBuiltinFuncs(nil, nil))
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
