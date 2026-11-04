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

package dataprotection

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func TestNFSStorageClassTemplateOmitsEmptySubDir(t *testing.T) {
	tpl := nfsStorageClassTemplate(t)

	missing, err := renderTemplate("sc", tpl, renderContext{Parameters: map[string]string{
		"nfsServer": "192.0.2.10",
		"nfsShare":  "/export",
	}})
	require.NoError(t, err)
	require.NotContains(t, missing, "<no value>")
	missingSC := unmarshalStorageClass(t, missing)
	_, ok := missingSC.Parameters["subDir"]
	require.False(t, ok)
	require.Equal(t, "192.0.2.10", missingSC.Parameters["server"])
	require.Equal(t, "/export", missingSC.Parameters["share"])

	blank, err := renderTemplate("sc", tpl, renderContext{Parameters: map[string]string{
		"nfsServer": "192.0.2.10",
		"nfsShare":  "/export",
		"nfsSubDir": "",
	}})
	require.NoError(t, err)
	require.NotContains(t, blank, "<no value>")
	_, ok = unmarshalStorageClass(t, blank).Parameters["subDir"]
	require.False(t, ok)

	present, err := renderTemplate("sc", tpl, renderContext{Parameters: map[string]string{
		"nfsServer": "192.0.2.10",
		"nfsShare":  "/export",
		"nfsSubDir": "repo backups",
	}})
	require.NoError(t, err)
	require.Equal(t, "repo backups", unmarshalStorageClass(t, present).Parameters["subDir"])
}

func nfsStorageClassTemplate(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	chart := filepath.Join(filepath.Dir(file), "../../deploy/helm/templates/storageprovider/nfs.yaml")
	raw, err := os.ReadFile(chart)
	require.NoError(t, err)
	text := string(raw)
	start := strings.Index(text, "storageClassTemplate: |\n")
	require.NotEqual(t, -1, start)
	body := text[start+len("storageClassTemplate: |\n"):]
	if end := strings.Index(body, "\n  parametersSchema:"); end >= 0 {
		body = body[:end]
	}
	body = regexp.MustCompile("(?m)^    ").ReplaceAllString(body, "")
	body = regexp.MustCompile(`\{\{ `+"`"+`([\s\S]*?)`+"`"+` \}\}`).ReplaceAllString(body, "$1")
	return strings.TrimSpace(body) + "\n"
}

func unmarshalStorageClass(t *testing.T, content string) storagev1.StorageClass {
	t.Helper()
	var sc storagev1.StorageClass
	require.NoError(t, yaml.Unmarshal([]byte(content), &sc))
	return sc
}
