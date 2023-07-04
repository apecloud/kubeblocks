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
	"compress/gzip"
	"embed"
	"fmt"
	"strings"

	"github.com/leaanthony/debme"
	"github.com/pkg/errors"
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

type EngineType string

// the supported cluster engine type
const (
	MySQL EngineType = "MySQL"
)

type SchemaPropName string

// the common schema property name
const (
	VersionSchemaProp SchemaPropName = "version"
)

type EngineSchema struct {
	// Schema is the cluster parent helm chart schema, used to render the command flag
	Schema *spec.Schema

	// SubSchema is the sub chart schema, used to render the command flag
	SubSchema *spec.Schema

	// SubChartName is the name (alias if exists) of the sub chart
	SubChartName string
}

var (
	//go:embed charts/*
	charts embed.FS
)

func GetHelmChart(e EngineType) (*chart.Chart, error) {
	chartsFS, err := debme.FS(charts, "charts")
	if err != nil {
		return nil, err
	}

	// load helm chart from embed tgz file
	return loadHelmChart(chartsFS, getEngineChartName(e))
}

// GetManifests gets the cluster manifests
func GetManifests(c *chart.Chart, namespace, name string, values map[string]interface{}) (map[string]string, error) {
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

	rel, err := client.Run(c, values)
	if err != nil {
		return nil, err
	}
	return releaseutil.SplitManifests(rel.Manifest), nil
}

// GetEngineSchema gets the schema for the given cluster engine type.
func GetEngineSchema(c *chart.Chart) (*EngineSchema, error) {
	var err error
	buildSchema := func(bs []byte) (*spec.Schema, error) {
		schema := &spec.Schema{}
		if err = json.Unmarshal(bs, schema); err != nil {
			return nil, errors.Wrapf(err, "failed to build schema for engine %s", c.Name())
		}
		return schema, nil
	}

	// build engine schema
	eSchema := &EngineSchema{}
	eSchema.Schema, err = buildSchema(c.Schema)
	if err != nil {
		return nil, err
	}

	// build extra schema in sub chart, now, we only support one sub chart
	for _, subChart := range c.Dependencies() {
		eSchema.SubChartName = subChart.Name()
		eSchema.SubSchema, err = buildSchema(subChart.Schema)
		if err != nil {
			return nil, err
		}
		break
	}

	// if sub chart has alias, we should use alias instead of chart name
	for _, dep := range c.Metadata.Dependencies {
		if dep.Name != eSchema.SubChartName {
			continue
		}

		if dep.Alias != "" {
			eSchema.SubChartName = dep.Alias
		}
	}

	return eSchema, nil
}

// ValidateValues validates the given values against the schema.
func ValidateValues(schema *EngineSchema, values map[string]interface{}) error {
	if schema == nil {
		return nil
	}

	validateFn := func(s *spec.Schema, values map[string]interface{}) error {
		if s == nil {
			return nil
		}
		v := validate.NewSchemaValidator(s, nil, "", strfmt.Default)
		return v.Validate(values).AsError()
	}

	if err := validateFn(schema.Schema, values); err != nil {
		return err
	}
	return validateFn(schema.SubSchema, values)
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

	if c == nil {
		return nil, fmt.Errorf("failed to load engine helm chart %s", name)
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
