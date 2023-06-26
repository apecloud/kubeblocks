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

package cluster

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/leaanthony/debme"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"k8s.io/kube-openapi/pkg/validation/strfmt"
	"k8s.io/kube-openapi/pkg/validation/validate"

	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
)

const (
	jsonSchemaFileName = "values.schema.json"
)

type EngineType string

// the supported cluster engine type
const (
	MySQL EngineType = "MySQL"
)

type SchemaPropName string

// the command schema property name
const (
	NameSchemaProp    SchemaPropName = "name"
	VersionSchemaProp SchemaPropName = "version"
)

var (
	//go:embed charts/*
	charts embed.FS
)

// GetManifests gets the cluster manifests
func GetManifests(e EngineType, namespace, name string, values map[string]interface{}) (map[string]string, error) {
	chartsFS, err := debme.FS(charts, "charts")
	if err != nil {
		return nil, err
	}

	// load the chart package to memory from embed tgz file
	chartRequested, err := loadHelmChart(chartsFS, getEngineChartName(e))
	if err != nil {
		return nil, err
	}

	// get the helm chart manifest
	actionCfg, err := helm.NewActionConfig(helm.NewFakeConfig(namespace))
	if err != nil {
		return nil, err
	}
	actionCfg.Log = func(format string, v ...interface{}) {
		fmt.Printf(format, v...)
	}

	client := action.NewInstall(actionCfg)
	client.DryRun = true
	client.Replace = true
	client.ClientOnly = true
	client.ReleaseName = name
	client.Namespace = namespace

	rel, err := client.Run(chartRequested, values)
	if err != nil {
		return nil, err
	}
	return releaseutil.SplitManifests(rel.Manifest), nil
}

// GetEngineSchema gets the schema for the given cluster engine type.
func GetEngineSchema(e EngineType) (*spec.Schema, error) {
	chartsFS, err := debme.FS(charts, "charts")
	if err != nil {
		return nil, err
	}

	chartName := getEngineChartName(e)
	file, err := chartsFS.Open(chartName + ".tgz")
	if err != nil {
		return nil, err
	}

	gr, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		if hdr.Name != filepath.Join(chartName, jsonSchemaFileName) {
			continue
		}

		// found the schema file
		var buf bytes.Buffer
		if _, err = io.Copy(&buf, tr); err != nil {
			return nil, err
		}

		schema := spec.Schema{}
		if err = json.Unmarshal(buf.Bytes(), &schema); err != nil {
			return nil, err
		}
		return &schema, nil
	}

	return nil, fmt.Errorf("failed to build schema for engine %s", e)
}

func ValidateValues(schema *spec.Schema, values map[string]interface{}) error {
	validator := validate.NewSchemaValidator(schema, nil, "", strfmt.Default)
	return validator.Validate(values).AsError()
}

// getEngineChartName gets the chart name for the given cluster engine type.
func getEngineChartName(e EngineType) string {
	eStr := strings.ToLower(string(e))
	switch e {
	case MySQL:
		return "apecloud-" + eStr + "-cluster"
	default:
		return eStr + "-cluster"
	}
}

func loadHelmChart(fs debme.Debme, name string) (*chart.Chart, error) {
	file, err := fs.Open(name + ".tgz")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	c, err := loader.LoadArchive(file)
	if err != nil {
		if err == gzip.ErrHeader {
			return nil, fmt.Errorf("file '%s' does not appear to be a valid chart file (details: %s)", name, err)
		}
	}
	return c, err
}

func SupportedEngines() []EngineType {
	return []EngineType{MySQL}

}

func (e EngineType) String() string {
	return string(e)
}

func (s SchemaPropName) String() string {
	return string(s)
}
