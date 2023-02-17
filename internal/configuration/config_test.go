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

package configuration

import (
	"context"
	"testing"

	"github.com/bhmj/jsonslice"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

var iniConfig = `
[mysqld]
innodb-buffer-pool-size=512M
log-bin=master-bin
gtid_mode=OFF
consensus_auto_leader_transfer=ON

log_error=/data/mysql/log/mysqld.err
character-sets-dir=/usr/share/mysql-8.0/charsets
datadir=/data/mysql/data
port=3306
general_log=1
general_log_file=/data/mysql/mysqld.log
pid-file=/data/mysql/run/mysqld.pid
server-id=1
slow_query_log=1
#slow_query_log_file=/data/mysql/mysqld-slow.log2
slow_query_log_file=/data/mysql/mysqld-slow.log
socket=/data/mysql/tmp/mysqld.sock
ssl-ca=/data/mysql/std_data/cacert.pem
ssl-cert=/data/mysql/std_data/server-cert.pem
ssl-key=/data/mysql/std_data/server-key.pem
tmpdir=/data/mysql/tmp/
loose-sha256_password_auto_generate_rsa_keys=0
loose-caching_sha2_password_auto_generate_rsa_keys=0
secure-file-priv=/data/mysql

[client]
socket=/data/mysql/tmp/mysqld.sock
host=localhost
`

func TestRawConfig(t *testing.T) {

	cfg, err := NewConfigLoader(CfgOption{
		Type:    CfgRawType,
		Log:     log.FromContext(context.Background()),
		CfgType: appsv1alpha1.INI,
		RawData: []byte(iniConfig),
	})

	if err != nil {
		t.Fatalf("new config loader failed [%v]", err)
	}

	ctx := NewCfgOptions("",
		func(ctx *CfgOpOption) {
			// filter mysqld
			ctx.IniContext = &IniContext{
				SectionName: "mysqld",
			}
		})

	// ctx := NewCfgOptions("$..slow_query_log_file", "")

	result, err := cfg.Query("$..slow_query_log_file", NewCfgOptions(""))
	require.Nil(t, err)
	require.NotNil(t, result)
	require.Equal(t, "[\"/data/mysql/mysqld-slow.log\"]", string(result))

	require.Nil(t,
		cfg.MergeFrom(map[string]interface{}{
			"slow_query_log": 1,
			"server-id":      2,
			"socket":         "xxxxxxxxxxxxxxx",
		}, ctx))

	content, err := cfg.ToCfgContent()
	require.NotNil(t, content)
	require.Nil(t, err)

	newContent, exist := content[cfg.name]
	require.True(t, exist)
	patch, err := CreateMergePatch([]byte(iniConfig), []byte(newContent), cfg.Option)
	require.Nil(t, err)
	log.Log.Info("patch : %v", patch)
	require.True(t, patch.IsModify)
	require.Equal(t, string(patch.UpdateConfig["raw"]), `{"mysqld":{"server-id":"2","socket":"xxxxxxxxxxxxxxx"}}`)

	{
		require.Nil(t,
			cfg.MergeFrom(map[string]interface{}{
				"server-id": 1,
				"socket":    "/data/mysql/tmp/mysqld.sock",
			}, ctx))
		content, err := cfg.ToCfgContent()
		require.Nil(t, err)
		newContent := content[cfg.name]
		// CreateMergePatch([]byte(iniConfig), []byte(newContent), cfg.Option)
		patch, err := CreateMergePatch([]byte(iniConfig), []byte(newContent), cfg.Option)
		require.Nil(t, err)
		log.Log.Info("patch : %v", patch)
		require.False(t, patch.IsModify)
	}
}

func TestConfigMapConfig(t *testing.T) {
	cfg, err := NewConfigLoader(CfgOption{
		Type:    CfgCmType,
		Log:     log.FromContext(context.Background()),
		CfgType: appsv1alpha1.INI,
		K8sKey: &K8sConfig{
			CfgKey: client.ObjectKey{
				Name:      "xxxx",    // set cm name
				Namespace: "default", // set cm namespace
			},
			ResourceFn: func(key client.ObjectKey) (map[string]string, error) {
				return map[string]string{
					"my.cnf":      iniConfig,
					"my_test.cnf": iniConfig,
				}, nil
			},
		},
	})

	require.Nil(t, err)
	log.Log.Info("cfg option: %v", cfg.Option)

	require.Equal(t, cfg.fileCount, 2)
	require.NotNil(t, cfg.getCfgViper(NewCfgOptions("my.cnf")))
	require.Nil(t, cfg.getCfgViper(NewCfgOptions("my2.cnf")))

	res, err := cfg.Query("$..slow_query_log_file", NewCfgOptions(""))
	require.Nil(t, err)
	require.NotNil(t, res)

	require.Equal(t, "[\"/data/mysql/mysqld-slow.log\"]", string(res))

	// patch
	{

		ctx := NewCfgOptions("my.cnf",
			func(ctx *CfgOpOption) {
				ctx.IniContext = &IniContext{
					SectionName: "mysqld",
				}
			})

		require.Nil(t,
			cfg.MergeFrom(map[string]interface{}{
				"slow_query_log": 0,
				"general_log":    0,
			}, ctx))

		content, _ := cfg.ToCfgContent()

		patch, err := CreateMergePatch(&K8sConfig{
			Configurations: map[string]string{
				"my.cnf":  iniConfig,
				"my2.cnf": iniConfig,
			},
		}, &K8sConfig{
			Configurations: content,
		}, cfg.Option)

		require.Nil(t, err)
		require.NotNil(t, patch)

		// add config: my_test.cnf
		// delete config: my2.cnf

		_, ok := patch.AddConfig["my_test.cnf"]
		require.True(t, ok)

		_, ok = patch.DeleteConfig["my2.cnf"]
		require.True(t, ok)

		updated, ok := patch.UpdateConfig["my.cnf"]
		require.True(t, ok)

		// update my.cnf
		// update slow_query_log 0
		res, _ := jsonslice.Get(updated, "$.mysqld.slow_query_log")
		require.Equal(t, []byte(`"0"`), res)

		// update general_log 0
		res, _ = jsonslice.Get(updated, "$.mysqld.general_log")
		require.Equal(t, []byte(`"0"`), res)
	}
}
