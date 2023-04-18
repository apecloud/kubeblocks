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

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func TestConfigPatch(t *testing.T) {

	cfg, err := NewConfigLoader(CfgOption{
		Type:    CfgRawType,
		Log:     log.FromContext(context.Background()),
		CfgType: appsv1alpha1.Ini,
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

func TestYamlConfigPatch(t *testing.T) {
	yamlContext := `
net:
  port: 2000
  bindIp:
    type: "string"
    trim: "whitespace"
  tls:
    mode: requireTLS
    certificateKeyFilePassword:
      type: "string"
      digest: b08519162ba332985ac18204851949611ef73835ec99067b85723e10113f5c26
      digest_key: 6d795365637265744b65795374756666
`

	patchOption := CfgOption{
		Type:    CfgTplType,
		CfgType: appsv1alpha1.YAML,
	}
	patch, err := CreateMergePatch(&ConfigResource{ConfigData: map[string]string{"test": ""}}, &ConfigResource{ConfigData: map[string]string{"test": yamlContext}}, patchOption)
	require.Nil(t, err)

	yb, err := yaml.YAMLToJSON([]byte(yamlContext))
	require.Nil(t, err)

	require.Nil(t, err)
	require.Equal(t, yb, patch.UpdateConfig["test"])
}
