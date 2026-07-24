/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package cluster

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
)

func TestShardingScaleInCredentialFieldsAreOptionalInCRD(t *testing.T) {
	data, err := os.ReadFile("../../../config/crd/bases/apps.kubeblocks.io_clusters.yaml")
	require.NoError(t, err)

	var crd map[string]interface{}
	require.NoError(t, yaml.Unmarshal(data, &crd))

	versionSchema := nestedMap(t, crd, "spec", "versions", "0", "schema", "openAPIV3Schema")
	requestAuthority := nestedMap(t, versionSchema,
		"properties", "status",
		"properties", "shardings",
		"additionalProperties",
		"properties", "scaleIn",
		"properties", "planMaterial",
		"properties", "requestAuthority")

	required, ok := requestAuthority["required"].([]interface{})
	require.True(t, ok)
	require.NotContains(t, required, "credentialSources")

	executorTemplate := nestedMap(t, requestAuthority,
		"properties", "executorTemplates",
		"items")
	required, ok = executorTemplate["required"].([]interface{})
	require.True(t, ok)
	require.NotContains(t, required, "credentialBindings")
}

func TestLegacyShardingScaleInAuthorityOmitsCredentialFields(t *testing.T) {
	authority := appsv1.ShardingScaleInRequestAuthority{
		ExecutorTemplates: []appsv1.ShardingScaleInExecutorTemplate{{}},
	}

	data, err := json.Marshal(authority)
	require.NoError(t, err)
	require.NotContains(t, string(data), `"credentialSources"`)
	require.NotContains(t, string(data), `"credentialBindings"`)
}
